// Command bench collects token/cost data from opencode session exports and
// reports on it.
//
//	bench collect --session ses_x --task 01-x --category trivial --mode A --rep 1 --pass true --duration 123 --prices prices.json --out results/results.csv
//	bench report --in results/results.csv
package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ---- export JSON (subset of `opencode export`, see README "Internals") ----

type tokens struct {
	Input     float64 `json:"input"`
	Output    float64 `json:"output"`
	Reasoning float64 `json:"reasoning"`
	Cache     struct {
		Read  float64 `json:"read"`
		Write float64 `json:"write"`
	} `json:"cache"`
}

func (t tokens) zero() bool {
	return t.Input == 0 && t.Output == 0 && t.Reasoning == 0 && t.Cache.Read == 0 && t.Cache.Write == 0
}

type modelRef struct {
	ProviderID string `json:"providerID"`
	ID         string `json:"id"`
	ModelID    string `json:"modelID"`
}

func (m modelRef) key() string {
	id := m.ModelID
	if id == "" {
		id = m.ID
	}
	return m.ProviderID + "/" + id
}

type exportDoc struct {
	Info struct {
		ID       string `json:"id"`
		ParentID string `json:"parentID"`
	} `json:"info"`
	Messages []struct {
		Info struct {
			Role       string    `json:"role"`
			ProviderID string    `json:"providerID"`
			ModelID    string    `json:"modelID"`
			Model      *modelRef `json:"model"` // user messages
			Tokens     tokens    `json:"tokens"`
		} `json:"info"`
		Parts []struct {
			Type   string  `json:"type"`
			Tool   string  `json:"tool"`
			Tokens *tokens `json:"tokens"`
			State  struct {
				Status string `json:"status"`
				Input  struct {
					SubagentType string `json:"subagent_type"`
				} `json:"input"`
				Metadata struct {
					SessionID string `json:"sessionId"`
				} `json:"metadata"`
			} `json:"state"`
		} `json:"parts"`
	} `json:"messages"`
}

// ---- prices ----

type modelPrice struct {
	Multiplier float64 `json:"multiplier"`
	USDPerMtok struct {
		Input      float64 `json:"input"`
		Output     float64 `json:"output"`
		Reasoning  float64 `json:"reasoning"`
		CacheRead  float64 `json:"cache_read"`
		CacheWrite float64 `json:"cache_write"`
	} `json:"usd_per_mtok"`
}

type prices struct {
	Todo        bool                  `json:"todo"`
	BillingUnit string                `json:"billing_unit"`
	Models      map[string]modelPrice `json:"models"`
}

func loadPrices(path string) (prices, error) {
	var p prices
	b, err := os.ReadFile(path)
	if err != nil {
		return p, err
	}
	if err := json.Unmarshal(b, &p); err != nil {
		return p, fmt.Errorf("%s: %w", path, err)
	}
	return p, nil
}

// ---- aggregation ----

// usage is the summed usage of one model across a whole session tree.
type usage struct {
	Requests, UserPrompts                       int
	Input, Output, Reasoning, CacheRead, CacheW float64
}

type exporter func(id string) ([]byte, error)

// opencodeExport runs `opencode export id` redirecting stdout to a temp file
// (not a pipe): piped output was intermittently truncated at 65536 bytes when
// opencode exited before flushing the pipe.
func opencodeExport(id string) ([]byte, error) {
	tmp, err := os.CreateTemp("", "bench-export-*.json")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "opencode", "export", id)
	cmd.Stdout = tmp
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return os.ReadFile(tmp.Name())
}

// retryingExport wraps exp, validating the output as JSON and retrying up to
// attempts times with a growing backoff. Fails if no attempt yields valid JSON.
func retryingExport(exp exporter, attempts int, backoff time.Duration) exporter {
	return func(id string) ([]byte, error) {
		var last error
		for i := 0; i < attempts; i++ {
			if i > 0 {
				time.Sleep(backoff * time.Duration(i))
			}
			raw, err := exp(id)
			if err == nil && !json.Valid(trimToJSON(raw)) {
				err = fmt.Errorf("invalid/truncated export JSON (%d bytes)", len(raw))
			}
			if err == nil {
				return raw, nil
			}
			last = err
		}
		return nil, fmt.Errorf("export %s failed after %d attempts: %w", id, attempts, last)
	}
}

// walkInfo is what walkEx reports besides the per-model usage of the whole tree.
type walkInfo struct {
	// PeakCtx is the largest context window the ROOT session sent in one LLM call: max over its assistant
	// steps (step-finish parts; the message totals when a message has none) of input + cache.read + cache.write.
	PeakCtx float64
	// Sub is the usage of every session below the root (subagents), per model.
	Sub map[string]*usage
	// TaskExplore/TaskGeneral/TaskOther count the task tool calls made by the ROOT session by subagent_type.
	TaskExplore, TaskGeneral, TaskOther int
}

// leaks reports whether a session export mentions hidden tests or the benchmark task directories.
func leaks(raw []byte) bool {
	for _, k := range []string{"hidden_", "/benchmark/tasks/", "/benchmark/tasks-private/"} {
		if bytes.Contains(raw, []byte(k)) {
			return true
		}
	}
	return false
}

func ctxOf(t tokens) float64 { return t.Input + t.Cache.Read + t.Cache.Write }

// walk is walkEx without the extra root/sub statistics.
func walk(root string, export exporter, onDoc func(id string, raw []byte)) (map[string]*usage, []string) {
	out, _, warns := walkEx(root, export, onDoc)
	return out, warns
}

// walkEx recursively visits the session tree rooted at id. Children are
// discovered through task tool parts (state.metadata.sessionId). Returns
// per-model usage of the whole tree, root/subagent statistics; warnings
// describe unresolved children / failed exports.
func walkEx(root string, export exporter, onDoc func(id string, raw []byte)) (map[string]*usage, walkInfo, []string) {
	out := map[string]*usage{}
	info := walkInfo{Sub: map[string]*usage{}}
	var warns []string
	visited := map[string]bool{}
	var visit func(id, from string)
	visit = func(id, from string) {
		if visited[id] {
			return
		}
		visited[id] = true
		raw, err := export(id)
		if err != nil {
			warns = append(warns, fmt.Sprintf("session %s (child of %s): export failed: %v", id, from, err))
			return
		}
		if onDoc != nil {
			onDoc(id, raw)
		}
		var doc exportDoc
		if err := json.Unmarshal(trimToJSON(raw), &doc); err != nil {
			warns = append(warns, fmt.Sprintf("session %s: bad export JSON: %v", id, err))
			return
		}
		get := func(k string) *usage {
			if out[k] == nil {
				out[k] = &usage{}
			}
			return out[k]
		}
		sub := func(k string) *usage { // same model key, tracked separately for subagent sessions
			if info.Sub[k] == nil {
				info.Sub[k] = &usage{}
			}
			return info.Sub[k]
		}
		isRoot := id == root
		var children []string
		for _, m := range doc.Messages {
			switch m.Info.Role {
			case "user":
				if m.Info.Model != nil {
					get(m.Info.Model.key()).UserPrompts++
				}
			case "assistant":
				key := modelRef{ProviderID: m.Info.ProviderID, ModelID: m.Info.ModelID}.key()
				u := get(key)
				var su *usage
				if !isRoot {
					su = sub(key)
				}
				addTo := func(t tokens) {
					u.add(t)
					if su != nil {
						su.add(t)
					}
					if isRoot && ctxOf(t) > info.PeakCtx {
						info.PeakCtx = ctxOf(t)
					}
				}
				steps := 0
				for _, p := range m.Parts {
					if p.Type == "step-finish" && p.Tokens != nil {
						steps++
						addTo(*p.Tokens)
					}
				}
				if steps == 0 && !m.Info.Tokens.zero() { // no step parts: fall back to message totals
					addTo(m.Info.Tokens)
				}
			}
			for _, p := range m.Parts {
				if p.Type != "tool" || p.Tool != "task" {
					continue
				}
				if isRoot {
					switch p.State.Input.SubagentType {
					case "explore":
						info.TaskExplore++
					case "general":
						info.TaskGeneral++
					default:
						info.TaskOther++
					}
				}
				if sid := p.State.Metadata.SessionID; sid != "" {
					children = append(children, sid)
				} else {
					warns = append(warns, fmt.Sprintf("session %s: task call with no child sessionId (status=%q); its cost is NOT counted", id, p.State.Status))
				}
			}
		}
		for _, c := range children {
			visit(c, id)
		}
	}
	visit(root, "(root)")
	return out, info, warns
}

func (u *usage) add(t tokens) {
	u.Requests++
	u.Input += t.Input
	u.Output += t.Output
	u.Reasoning += t.Reasoning
	u.CacheRead += t.Cache.Read
	u.CacheW += t.Cache.Write
}

// trimToJSON drops any non-JSON preamble (e.g. a log line) before the first '{'.
func trimToJSON(b []byte) []byte {
	if i := bytes.IndexByte(b, '{'); i > 0 {
		return b[i:]
	}
	return b
}

// ---- collect ----

// baseCols is the width of buildRows output (the leading csvHeader columns, before the per-run extras).
const baseCols = 18

// requiredCols are the columns older CSVs already have; everything after is optional when reading.
const requiredCols = 16

// extraKeys are the per-run extras, repeated on every model row of a run, in CSV order after the base columns:
// code-size metrics (metrics.env), gates (gates.env; HIDDEN..BUILD are the v1 gates, GOBUILD..NODETEST the v2 repo gates),
// multi-turn counters (metrics.env) and the values the collector computes from the session tree.
var extraKeys = []string{"FILES_CHANGED", "APP_ADDED", "TEST_ADDED", "TOTAL_ADDED",
	"HIDDEN", "TESTS", "VET", "TSC", "BUILD", "GOBUILD", "GOVET", "GOFMT", "NODETEST",
	"TURNS", "TURNS_PASSED",
	"PEAK_CTX", "SUB_COST_USD", "SUB_COST_SHARE", "TASK_CALLS_EXPLORE", "TASK_CALLS_GENERAL", "TASK_CALLS_OTHER"}

// indexes into extraKeys
const (
	xGateFirst, xGateLast = 4, 12 // HIDDEN..NODETEST
	xV1GateLast           = 8     // BUILD
	xTurns                = 13
	xTurnsPassed          = 14
	xPeakCtx              = 15
	xSubUSD               = 16
	xSubShare             = 17
	xCallsExplore         = 18
	xCallsGeneral         = 19
	xCallsOther           = 20
)

var csvHeader = func() []string {
	h := []string{"run_id", "task", "category", "mode", "rep", "pass", "model", "requests", "input", "output", "reasoning", "cache_read", "cache_write", "est_premium_requests", "est_usd", "duration_s", "batch_id", "leak"}
	for i, k := range extraKeys {
		name := strings.ToLower(k)
		if i >= xGateFirst && i <= xGateLast {
			name = "gate_" + name
		}
		h = append(h, name)
	}
	return h
}()

// readEnvFile parses KEY=VALUE lines; a missing file yields an empty map.
func readEnvFile(path string) map[string]string {
	m := map[string]string{}
	if path == "" {
		return m
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return m
	}
	for _, l := range strings.Split(string(b), "\n") {
		if k, v, ok := strings.Cut(strings.TrimSpace(l), "="); ok {
			m[k] = v
		}
	}
	return m
}

// extrasFromFiles builds the extra cells (empty when unknown) from the metrics and gates env files; the cells the
// collector computes itself (PEAK_CTX..) stay empty here, see setComputed.
func extrasFromFiles(metricsFile, gatesFile string) []string {
	m, g := readEnvFile(metricsFile), readEnvFile(gatesFile)
	out := make([]string, len(extraKeys))
	for i, k := range extraKeys {
		if v, ok := m[k]; ok {
			out[i] = v
		} else {
			out[i] = g[k]
		}
	}
	return out
}

// setComputed fills the session-tree derived extras: peak context, subagent cost/share and task call counts.
func setComputed(extras []string, per map[string]*usage, info walkInfo, pr prices) {
	total, sub := usdOf(per, pr), usdOf(info.Sub, pr)
	extras[xPeakCtx] = f(info.PeakCtx)
	extras[xSubUSD] = strconv.FormatFloat(sub, 'f', 6, 64)
	if total > 0 {
		extras[xSubShare] = strconv.FormatFloat(sub/total, 'f', 4, 64)
	} else {
		extras[xSubShare] = "0"
	}
	extras[xCallsExplore] = strconv.Itoa(info.TaskExplore)
	extras[xCallsGeneral] = strconv.Itoa(info.TaskGeneral)
	extras[xCallsOther] = strconv.Itoa(info.TaskOther)
}

// usdOf prices per-model usage with the price table (models without an entry cost 0).
func usdOf(per map[string]*usage, pr prices) float64 {
	var sum float64
	for m, u := range per {
		p := pr.Models[m].USDPerMtok
		sum += (u.Input*p.Input + u.Output*p.Output + u.Reasoning*p.Reasoning + u.CacheRead*p.CacheRead + u.CacheW*p.CacheWrite) / 1e6
	}
	return sum
}

// withExtras appends the per-run extra cells to every row.
func withExtras(rows [][]string, extra []string) [][]string {
	for i := range rows {
		rows[i] = append(rows[i], extra...)
	}
	return rows
}

func f(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

func buildRows(runID, task, category, mode string, rep int, pass bool, duration float64, per map[string]*usage, pr prices, unit, batch string, leak bool) ([][]string, []string) {
	var warns []string
	models := make([]string, 0, len(per))
	for m := range per {
		models = append(models, m)
	}
	sort.Strings(models)
	var rows [][]string
	for _, m := range models {
		u := per[m]
		price, ok := pr.Models[m]
		if !ok {
			warns = append(warns, fmt.Sprintf("no price entry for model %q: premium requests and USD estimated as 0", m))
		}
		units := float64(u.Requests)
		if unit == "per_user_prompt" {
			units = float64(u.UserPrompts)
		}
		p := price.USDPerMtok
		usd := (u.Input*p.Input + u.Output*p.Output + u.Reasoning*p.Reasoning + u.CacheRead*p.CacheRead + u.CacheW*p.CacheWrite) / 1e6
		rows = append(rows, []string{runID, task, category, mode, strconv.Itoa(rep), strconv.FormatBool(pass), m,
			strconv.Itoa(u.Requests), f(u.Input), f(u.Output), f(u.Reasoning), f(u.CacheRead), f(u.CacheW),
			f(units * price.Multiplier), strconv.FormatFloat(usd, 'f', 6, 64), strconv.FormatFloat(duration, 'f', 1, 64), batch, strconv.FormatBool(leak)})
	}
	if len(rows) == 0 { // keep the run visible in the report
		rows = append(rows, []string{runID, task, category, mode, strconv.Itoa(rep), strconv.FormatBool(pass), "(none)", "0", "0", "0", "0", "0", "0", "0", "0.000000", strconv.FormatFloat(duration, 'f', 1, 64), batch, strconv.FormatBool(leak)})
	}
	return rows, warns
}

func appendCSV(path string, rows [][]string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	_, statErr := os.Stat(path)
	if statErr == nil {
		if err := upgradeHeader(path); err != nil {
			return err
		}
	}
	fh, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer fh.Close()
	w := csv.NewWriter(fh)
	if statErr != nil {
		_ = w.Write(csvHeader)
	}
	_ = w.WriteAll(rows)
	w.Flush()
	return w.Error()
}

// upgradeHeader rewrites an older CSV's header line to the current one (old rows stay short; readers treat
// missing trailing cells as empty). The old file is kept as <path>.bak-pre-gates (v0) or <path>.bak-pre-v2.
func upgradeHeader(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	head, rest, _ := strings.Cut(string(b), "\n")
	if head == strings.Join(csvHeader, ",") || head == "" {
		return nil
	}
	bak := path + ".bak-pre-v2"
	if !strings.Contains(head, "files_changed") {
		bak = path + ".bak-pre-gates"
	}
	if err := os.WriteFile(bak, b, 0o644); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.Join(csvHeader, ",")+"\n"+rest), 0o644)
}

func cmdCollect(args []string, stderr io.Writer) error {
	fs := flag.NewFlagSet("collect", flag.ContinueOnError)
	fs.SetOutput(stderr)
	session := fs.String("session", "", "root session id (required)")
	task := fs.String("task", "", "task name")
	category := fs.String("category", "", "task category")
	mode := fs.String("mode", "", "mode label: A, B, C, V0, V1, V2, optionally with -<model alias> (e.g. V1-sonnet)")
	rep := fs.Int("rep", 1, "repetition number")
	passStr := fs.String("pass", "false", "whether verification passed (true|false)")
	duration := fs.Float64("duration", 0, "wall-clock seconds")
	pricesPath := fs.String("prices", "prices.json", "prices file")
	out := fs.String("out", "results/results.csv", "CSV to append to")
	runID := fs.String("run-id", "", "run id (default: <task>-<mode>-r<rep>)")
	saveDir := fs.String("save-dir", "", "save raw session exports into this directory")
	exportDir := fs.String("export-dir", "", "read <id>.json from this directory instead of running `opencode export`")
	billing := fs.String("billing", "", "override billing_unit (per_llm_call|per_user_prompt)")
	strict := fs.Bool("strict", false, "fail (non-zero, after writing rows) if any model has no price entry")
	batch := fs.String("batch", "", "batch id recorded in the CSV")
	metricsFile := fs.String("metrics-file", "", "metrics.env (FILES_CHANGED, APP_ADDED, TEST_ADDED, TOTAL_ADDED, TURNS, TURNS_PASSED) to record")
	gatesFile := fs.String("gates-file", "", "gates.env (HIDDEN, TESTS, VET, TSC, BUILD, GOBUILD, GOVET, GOFMT, NODETEST = 1/0) to record")
	if err := fs.Parse(args); err != nil {
		return err
	}
	pass, err := strconv.ParseBool(*passStr)
	if err != nil {
		return fmt.Errorf("--pass: %w", err)
	}
	pr, err := loadPrices(*pricesPath)
	if err != nil {
		return err
	}
	unit := pr.BillingUnit
	if *billing != "" {
		unit = *billing
	}
	if unit == "" {
		unit = "per_llm_call"
	}
	if unit != "per_llm_call" && unit != "per_user_prompt" {
		return fmt.Errorf("unknown billing unit %q", unit)
	}
	if pr.Todo {
		fmt.Fprintln(stderr, "WARNING: prices file is still marked todo=true; premium/USD figures are placeholders")
	}
	id := *runID
	if id == "" {
		id = fmt.Sprintf("%s-%s-r%d", *task, *mode, *rep)
	}
	exp := func(sid string) ([]byte, error) {
		if *exportDir != "" {
			return os.ReadFile(filepath.Join(*exportDir, sid+".json"))
		}
		return retryingExport(opencodeExport, 3, 500*time.Millisecond)(sid)
	}
	var save func(string, []byte)
	if *saveDir != "" {
		_ = os.MkdirAll(*saveDir, 0o755)
		save = func(sid string, raw []byte) { _ = os.WriteFile(filepath.Join(*saveDir, sid+".json"), raw, 0o644) }
	}
	extras := extrasFromFiles(*metricsFile, *gatesFile)
	if *session == "" { // sessionless run (no id in events): record a placeholder so the run stays visible
		fmt.Fprintln(stderr, "WARNING: no session id; writing placeholder row (pass forced to false)")
		rows, _ := buildRows(id, *task, *category, *mode, *rep, false, *duration, nil, pr, unit, *batch, false)
		return appendCSV(*out, withExtras(rows, extras))
	}
	leak := false
	save2 := func(sid string, raw []byte) {
		if leaks(raw) {
			leak = true
		}
		if save != nil {
			save(sid, raw)
		}
	}
	per, winfo, warns := walkEx(*session, exp, save2)
	setComputed(extras, per, winfo, pr)
	if leak {
		fmt.Fprintln(stderr, "WARNING: LEAK: session mentions \"hidden_\" or \"/benchmark/tasks/\"; the agent may have seen hidden tests")
	}
	rows, w2 := buildRows(id, *task, *category, *mode, *rep, pass, *duration, per, pr, unit, *batch, leak)
	for _, w := range append(warns, w2...) {
		fmt.Fprintln(stderr, "WARNING:", w)
	}
	if len(per) == 0 {
		fmt.Fprintln(stderr, "WARNING: no usage found for session", *session)
		if pass { // never record a passing run as a silent $0 row
			return fmt.Errorf("session %s passed but no usage found (export failed?); row NOT written", *session)
		}
	}
	if err := appendCSV(*out, withExtras(rows, extras)); err != nil {
		return err
	}
	if *strict {
		for _, w := range w2 {
			if strings.Contains(w, "no price entry") {
				return errors.New("strict: " + w)
			}
		}
	}
	return nil
}

// ---- report ----

type row struct {
	runID, category, mode, model, pass, batch string
	task                                      string
	leak                                      bool
	requests                                  int
	input, output, reasoning, cr, cw          float64
	prem, usd, dur                            float64
	extra                                     []string // extraKeys order; "" = not recorded (old CSV / timeout)
}

type run struct {
	id, category, mode, task string
	pass, leak, none         bool
	prem, usd                float64
	extra                    []string
}

func readCSV(r io.Reader) ([]row, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1 // old rows are shorter than the current header
	recs, err := cr.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(recs) == 0 {
		return nil, errors.New("empty CSV")
	}
	idx := map[string]int{}
	for i, h := range recs[0] {
		idx[h] = i
	}
	for _, h := range csvHeader[:requiredCols] {
		if _, ok := idx[h]; !ok {
			return nil, fmt.Errorf("missing column %q", h)
		}
	}
	var rows []row
	for _, rec := range recs[1:] {
		g := func(k string) string { return rec[idx[k]] }
		n := func(k string) float64 { v, _ := strconv.ParseFloat(g(k), 64); return v }
		req, _ := strconv.Atoi(g("requests"))
		opt := func(k string) string {
			if i, ok := idx[k]; ok && i < len(rec) {
				return rec[i]
			}
			return ""
		}
		extra := make([]string, len(extraKeys))
		for i := range extraKeys {
			extra[i] = opt(csvHeader[baseCols+i])
		}
		rows = append(rows, row{runID: g("run_id"), task: g("task"), category: g("category"), mode: g("mode"), model: g("model"), pass: g("pass"),
			batch: opt("batch_id"), leak: opt("leak") == "true", requests: req,
			input: n("input"), output: n("output"), reasoning: n("reasoning"), cr: n("cache_read"), cw: n("cache_write"),
			prem: n("est_premium_requests"), usd: n("est_usd"), dur: n("duration_s"), extra: extra})
	}
	return rows, nil
}

type stat struct {
	n             int
	sum, min, max float64
}

func (s *stat) add(v float64) {
	if s.n == 0 || v < s.min {
		s.min = v
	}
	if s.n == 0 || v > s.max {
		s.max = v
	}
	s.n++
	s.sum += v
}
func (s stat) mean() float64 {
	if s.n == 0 {
		return 0
	}
	return s.sum / float64(s.n)
}

type group struct {
	runs        int
	passes      int
	prem, usd   stat
	failedCosts float64
	// code-size means over runs that recorded metrics: files, app, test, total
	size [4]stat
	// gate failures / runs with the gate recorded, in extraKeys[xGateFirst:] order (9 gates)
	gateFail, gateSeen [9]int
	// v2 statistics over the runs that recorded them
	peakCtx, subShare, subUSD, turns, turnsPassed stat
	calls                                         [3]stat // explore, general, other task calls per run
	usdAll                                        float64 // total USD of all runs (cost per pass)
}

// num parses a recorded extra cell; ok is false for an empty/unparseable cell.
func num(s string) (float64, bool) {
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	return v, err == nil
}

func (g *group) addV2(x *run) {
	g.usdAll += x.usd
	if v, ok := num(x.extra[xPeakCtx]); ok {
		g.peakCtx.add(v)
	}
	if v, ok := num(x.extra[xSubShare]); ok {
		g.subShare.add(v)
	}
	if v, ok := num(x.extra[xSubUSD]); ok {
		g.subUSD.add(v)
	}
	if v, ok := num(x.extra[xTurns]); ok {
		g.turns.add(v)
		if p, ok := num(x.extra[xTurnsPassed]); ok {
			g.turnsPassed.add(p)
		}
	}
	for i, k := range []int{xCallsExplore, xCallsGeneral, xCallsOther} {
		if v, ok := num(x.extra[k]); ok {
			g.calls[i].add(v)
		}
	}
}

func writeReport(w io.Writer, rows []row) {
	runs := map[string]*run{}
	var order []string
	for _, r := range rows {
		x := runs[r.runID]
		if x == nil {
			x = &run{id: r.runID, category: r.category, mode: r.mode, task: r.task, pass: r.pass == "true", extra: r.extra}
			runs[r.runID] = x
			order = append(order, r.runID)
		}
		x.leak = x.leak || r.leak
		x.none = x.none || r.model == "(none)"
		x.prem += r.prem
		x.usd += r.usd
	}
	modes := map[string]bool{}
	cats := map[string]bool{}
	groups := map[[2]string]*group{}
	add := func(mode, cat string, x *run) {
		k := [2]string{mode, cat}
		g := groups[k]
		if g == nil {
			g = &group{}
			groups[k] = g
		}
		g.runs++
		if x.pass {
			g.passes++
		}
		g.prem.add(x.prem)
		g.usd.add(x.usd)
		for i := 0; i < 4; i++ {
			if v, err := strconv.ParseFloat(x.extra[i], 64); err == nil && x.extra[i] != "" {
				g.size[i].add(v)
			}
		}
		g.addV2(x)
		for i := 0; i < len(g.gateSeen); i++ {
			if v := x.extra[xGateFirst+i]; v != "" {
				g.gateSeen[i]++
				if v == "0" {
					g.gateFail[i]++
				}
			}
		}
	}
	for _, id := range order {
		x := runs[id]
		modes[x.mode], cats[x.category] = true, true
		add(x.mode, x.category, x)
		add(x.mode, "ALL", x)
	}
	ml := orderModes(modes)
	cl := orderCats(cats)
	cl = append(cl, "ALL")

	fmt.Fprintf(w, "# Benchmark report\n\n%d runs, %d result rows.\n\n", len(runs), len(rows))
	leaks, nones := 0, 0
	for _, x := range runs {
		if x.leak {
			leaks++
		}
		if x.none {
			nones++
		}
	}
	if leaks > 0 {
		fmt.Fprintf(w, "**WARNING: %d run(s) flagged `leak` (session mentions hidden tests); their results are suspect.**\n\n", leaks)
	}
	if nones > 0 {
		fmt.Fprintf(w, "Note: %d run(s) have no usage data (sessionless/failed collect); counted with pass as recorded and zero cost.\n\n", nones)
	}
	fmt.Fprintln(w, "Cost per run = premium requests / USD summed over the whole session tree (all models). Cost per successful task = total cost of ALL runs in the group (failures included) / number of passing runs.")
	fmt.Fprintln(w)
	section := func(title, unit string, pick func(*group) (stat, float64)) {
		fmt.Fprintf(w, "## %s\n\n| mode | category | runs | pass rate | mean | min | max | cost / successful task |\n|---|---|---|---|---|---|---|---|\n", title)
		for _, m := range ml {
			for _, c := range cl {
				g := groups[[2]string{m, c}]
				if g == nil {
					continue
				}
				s, total := pick(g)
				per := "n/a (0 passes)"
				if g.passes > 0 {
					per = fmt.Sprintf("%.4g", total/float64(g.passes))
				}
				fmt.Fprintf(w, "| %s | %s | %d | %d/%d (%.0f%%) | %.4g | %.4g | %.4g | %s |\n", m, c, g.runs, g.passes, g.runs, 100*float64(g.passes)/float64(g.runs), s.mean(), s.min, s.max, per)
			}
		}
		fmt.Fprintf(w, "\nUnits: %s.\n\n", unit)
	}
	section("Cost model 1: estimated premium requests", "estimated premium requests", func(g *group) (stat, float64) { return g.prem, g.prem.sum })
	section("Cost model 2: estimated USD (token prices)", "USD", func(g *group) (stat, float64) { return g.usd, g.usd.sum })

	for i := 0; i < len(ml); i++ {
		for j := i + 1; j < len(ml); j++ {
			a, b := ml[i], ml[j]
			fmt.Fprintf(w, "## Mean cost ratio %s / %s\n\n| category | premium requests | USD |\n|---|---|---|\n", a, b)
			for _, c := range cl {
				ga, gb := groups[[2]string{a, c}], groups[[2]string{b, c}]
				if ga == nil || gb == nil {
					continue
				}
				fmt.Fprintf(w, "| %s | %s | %s |\n", c, ratio(ga.prem.mean(), gb.prem.mean()), ratio(ga.usd.mean(), gb.usd.mean()))
			}
			fmt.Fprintln(w)
		}
	}

	fmt.Fprintf(w, "## Code size (mean lines added vs baseline, runs with metrics)\n\n| mode | category | runs | files | app | test | total |\n|---|---|---|---|---|---|---|\n")
	for _, m := range ml {
		for _, c := range cl {
			if g := groups[[2]string{m, c}]; g != nil && g.size[0].n > 0 {
				fmt.Fprintf(w, "| %s | %s | %d | %.1f | %.1f | %.1f | %.1f |\n", m, c, g.size[0].n, g.size[0].mean(), g.size[1].mean(), g.size[2].mean(), g.size[3].mean())
			}
		}
	}
	fmt.Fprintf(w, "\nfiles = files changed; app = non-test source lines added; test = lines added in `*_test.go`/`*.test.ts(x)`; total = all lines added. Excludes node_modules, lockfiles, build output, `.opencode`.\n\n")
	fmt.Fprintf(w, "## Gate failures (failed / runs with gate recorded)\n\n| mode | category | hidden | tests | vet | tsc | build |\n|---|---|---|---|---|---|---|\n")
	for _, m := range ml {
		for _, c := range cl {
			g := groups[[2]string{m, c}]
			if g == nil || sum(g.gateSeen[:5]) == 0 {
				continue
			}
			fmt.Fprintf(w, "| %s | %s |", m, c)
			for i := 0; i < 5; i++ {
				fmt.Fprintf(w, " %d/%d |", g.gateFail[i], g.gateSeen[i])
			}
			fmt.Fprintln(w)
		}
	}
	fmt.Fprintln(w)
	writeV2(w, ml, cl, groups, runs, order)

	// per-model breakdown per mode (average per run)
	fmt.Fprintf(w, "## Per-model breakdown (average per run)\n\n| mode | model | requests | input | output | reasoning | cache_read | cache_write | premium req | USD |\n|---|---|---|---|---|---|---|---|---|---|\n")
	type agg struct{ req, in, out, rea, cr, cw, prem, usd float64 }
	byMM := map[[2]string]*agg{}
	runsPerMode := map[string]int{}
	for _, x := range runs {
		runsPerMode[x.mode]++
	}
	var keys [][2]string
	for _, r := range rows {
		k := [2]string{r.mode, r.model}
		a := byMM[k]
		if a == nil {
			a = &agg{}
			byMM[k] = a
			keys = append(keys, k)
		}
		a.req += float64(r.requests)
		a.in += r.input
		a.out += r.output
		a.rea += r.reasoning
		a.cr += r.cr
		a.cw += r.cw
		a.prem += r.prem
		a.usd += r.usd
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i][0] != keys[j][0] {
			return keys[i][0] < keys[j][0]
		}
		return keys[i][1] < keys[j][1]
	})
	for _, k := range keys {
		a, n := byMM[k], float64(runsPerMode[k[0]])
		fmt.Fprintf(w, "| %s | %s | %.1f | %.0f | %.0f | %.0f | %.0f | %.0f | %.4g | %.4g |\n", k[0], k[1], a.req/n, a.in/n, a.out/n, a.rea/n, a.cr/n, a.cw/n, a.prem/n, a.usd/n)
	}
	fmt.Fprintln(w)
}

func sum(v []int) int {
	t := 0
	for _, x := range v {
		t += x
	}
	return t
}

// writeV2 prints the v2 sections: cost per pass + context + subagent share, repo gates, delegation counts per
// mode x category and mode x task, and multi-turn results. Sections without data are omitted (old CSVs).
func writeV2(w io.Writer, ml, cl []string, groups map[[2]string]*group, runs map[string]*run, order []string) {
	hasV2 := false
	for _, g := range groups {
		if g.peakCtx.n > 0 {
			hasV2 = true
		}
	}
	if !hasV2 {
		return
	}
	fmt.Fprintf(w, "## Efficiency: cost per pass, peak context, subagent share\n\n| mode | category | runs | pass | cost / pass (USD) | peak_ctx mean | peak_ctx max | sub cost mean (USD) | sub share mean |\n|---|---|---|---|---|---|---|---|---|\n")
	for _, m := range ml {
		for _, c := range cl {
			g := groups[[2]string{m, c}]
			if g == nil || g.peakCtx.n == 0 {
				continue
			}
			per := "n/a (0 passes)"
			if g.passes > 0 {
				per = fmt.Sprintf("%.4g", g.usdAll/float64(g.passes))
			}
			fmt.Fprintf(w, "| %s | %s | %d | %d/%d | %s | %.0f | %.0f | %.4g | %.1f%% |\n", m, c, g.runs, g.passes, g.runs, per, g.peakCtx.mean(), g.peakCtx.max, g.subUSD.mean(), 100*g.subShare.mean())
		}
	}
	fmt.Fprintf(w, "\npeak_ctx = largest single LLM call context (input + cache read + cache write tokens) of the ROOT session; sub share = cost of subagent sessions / total cost of the run.\n\n")

	// delegation counts: mode x category (groups) and mode x task
	fmt.Fprintf(w, "## Delegation: task tool calls per run (root session)\n\n| mode | category / task | runs | explore | general | other |\n|---|---|---|---|---|---|\n")
	for _, m := range ml {
		for _, c := range cl {
			if g := groups[[2]string{m, c}]; g != nil && g.calls[0].n > 0 {
				fmt.Fprintf(w, "| %s | %s | %d | %.2f | %.2f | %.2f |\n", m, c, g.calls[0].n, g.calls[0].mean(), g.calls[1].mean(), g.calls[2].mean())
			}
		}
	}
	byTask := map[[2]string]*group{}
	var tkeys [][2]string
	for _, id := range order {
		x := runs[id]
		k := [2]string{x.mode, x.task}
		g := byTask[k]
		if g == nil {
			g = &group{}
			byTask[k] = g
			tkeys = append(tkeys, k)
		}
		g.addV2(x)
		g.runs++
		if x.pass {
			g.passes++
		}
	}
	sort.SliceStable(tkeys, func(i, j int) bool {
		a, b := tkeys[i], tkeys[j]
		ra, rb := modeRank(a[0]), modeRank(b[0])
		if ra != rb {
			return ra < rb
		}
		if a[0] != b[0] {
			return a[0] < b[0]
		}
		return a[1] < b[1]
	})
	for _, k := range tkeys {
		g := byTask[k]
		if g.calls[0].n > 0 {
			fmt.Fprintf(w, "| %s | task %s | %d | %.2f | %.2f | %.2f |\n", k[0], k[1], g.calls[0].n, g.calls[0].mean(), g.calls[1].mean(), g.calls[2].mean())
		}
	}
	fmt.Fprintln(w)

	if sum(gateSeenAll(groups, 5, 9)) > 0 {
		fmt.Fprintf(w, "## Repo gate failures (failed / recorded)\n\n| mode | category | hidden | gobuild | govet | gofmt | nodetest | tsc |\n|---|---|---|---|---|---|---|---|\n")
		for _, m := range ml {
			for _, c := range cl {
				g := groups[[2]string{m, c}]
				if g == nil || sum(g.gateSeen[5:]) == 0 {
					continue
				}
				fmt.Fprintf(w, "| %s | %s |", m, c)
				for _, i := range []int{0, 5, 6, 7, 8, 3} {
					fmt.Fprintf(w, " %d/%d |", g.gateFail[i], g.gateSeen[i])
				}
				fmt.Fprintln(w)
			}
		}
		fmt.Fprintln(w)
	}

	if anyTurns := func() bool {
		for _, g := range byTask {
			if g.turns.n > 0 {
				return true
			}
		}
		return false
	}(); anyTurns {
		fmt.Fprintf(w, "## Multi-turn: turns passed (verify-turn checks)\n\n| mode | task | runs | turns | turns passed (mean) | pass |\n|---|---|---|---|---|---|\n")
		for _, k := range tkeys {
			if g := byTask[k]; g.turns.n > 0 {
				fmt.Fprintf(w, "| %s | %s | %d | %.0f | %.2f | %d/%d |\n", k[0], k[1], g.turns.n, g.turns.mean(), g.turnsPassed.mean(), g.passes, g.runs)
			}
		}
		fmt.Fprintln(w)
	}
}

// gateSeenAll sums the seen counters of gates [from,to) over all groups.
func gateSeenAll(groups map[[2]string]*group, from, to int) []int {
	out := make([]int, to-from)
	for _, g := range groups {
		for i := from; i < to; i++ {
			out[i-from] += g.gateSeen[i]
		}
	}
	return out
}

func ratio(a, b float64) string {
	if b == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.2fx", a/b)
}

func sortedKeys(m map[string]bool) []string {
	var s []string
	for k := range m {
		s = append(s, k)
	}
	sort.Strings(s)
	return s
}

// orderModes sorts modes: A, B, C, S first, then model modes cheapest-first (C-haiku, C-sonnet, C-opus), then the
// rest alphabetically. Pairwise ratio tables are earlier/later, so cheaper-model / C-opus ratios fall out naturally.
func orderModes(m map[string]bool) []string {
	s := sortedKeys(m)
	sort.SliceStable(s, func(i, j int) bool { return modeRank(s[i]) < modeRank(s[j]) })
	return s
}

// modeRank orders mode labels: A B C S, the C model modes cheapest first, then V0 V1 V2 each followed by its model
// variants (V1, V1-haiku, V1-sonnet, V1-opus), then the rest.
func modeRank(k string) int {
	rank := map[string]int{"A": 0, "B": 1, "C": 2, "S": 3, "C-haiku": 4, "C-sonnet": 5, "C-opus": 6}
	if v, ok := rank[k]; ok {
		return v
	}
	if len(k) >= 2 && k[0] == 'V' && k[1] >= '0' && k[1] <= '9' {
		base := 10 + 10*int(k[1]-'0')
		switch strings.TrimPrefix(k[2:], "-") {
		case "":
			return base
		case "haiku":
			return base + 1
		case "sonnet":
			return base + 2
		case "opus":
			return base + 3
		}
		return base + 4
	}
	return 100
}

func orderCats(m map[string]bool) []string {
	known := []string{"trivial", "simple", "standard", "complex", "hard", "large", "session"}
	var out []string
	for _, k := range known {
		if m[k] {
			out = append(out, k)
			delete(m, k)
		}
	}
	return append(out, sortedKeys(m)...)
}

// filterRows keeps rows whose batch is in the comma list (empty or "all" = any)
// and whose mode is in the modes comma list (empty = any).
func filterRows(rows []row, batch, modes string) []row {
	set := func(v string) map[string]bool {
		m := map[string]bool{}
		for _, x := range strings.Split(v, ",") {
			if x = strings.TrimSpace(x); x != "" {
				m[x] = true
			}
		}
		return m
	}
	bs, ms := set(batch), set(modes)
	var out []row
	for _, r := range rows {
		if len(bs) > 0 && !bs["all"] && !bs[r.batch] {
			continue
		}
		if len(ms) > 0 && !ms[r.mode] {
			continue
		}
		out = append(out, r)
	}
	return out
}

func cmdReport(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.SetOutput(stderr)
	in := fs.String("in", "results/results.csv", "CSV to read")
	batch := fs.String("batch", "", "batch filter: \"all\"/empty = every batch, \"latest\" = last batch in the CSV, or a comma list of batch ids")
	modes := fs.String("modes", "", "comma list of modes to include (default: every mode except S = slim-captain runs)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	fh, err := os.Open(*in)
	if err != nil {
		return err
	}
	defer fh.Close()
	rows, err := readCSV(fh)
	if err != nil {
		return err
	}
	if *batch == "latest" {
		for _, r := range rows {
			if r.batch != "" {
				*batch = r.batch
			}
		}
	}
	if *modes == "" {
		seen := map[string]bool{}
		var ml []string
		for _, r := range rows {
			if r.mode != "S" && !seen[r.mode] {
				seen[r.mode] = true
				ml = append(ml, r.mode)
			}
		}
		*modes = strings.Join(ml, ",")
	}
	rows = filterRows(rows, *batch, *modes)
	writeReport(stdout, rows)
	return nil
}

func runCLI(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: bench collect|recollect|report [flags]")
	}
	switch args[0] {
	case "collect":
		return cmdCollect(args[1:], stderr)
	case "recollect":
		return cmdRecollect(args[1:], stderr)
	case "report":
		return cmdReport(args[1:], stdout, stderr)
	}
	return fmt.Errorf("unknown command %q", args[0])
}

func main() {
	if err := runCLI(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "bench:", err)
		os.Exit(1)
	}
}

// cmdRecollect re-exports every session of each run in a batch (from the
// run's events.jsonl root session id) into runs/<id>/sessions and rewrites
// the batch's CSV rows, keeping run_id/task/category/mode/rep/pass/duration.
func cmdRecollect(args []string, stderr io.Writer) error {
	fs := flag.NewFlagSet("recollect", flag.ContinueOnError)
	fs.SetOutput(stderr)
	runsDir := fs.String("runs-dir", "results/runs", "directory of run dirs")
	batch := fs.String("batch", "", "batch id to recollect (required)")
	in := fs.String("in", "results/results.csv", "CSV to rewrite (backed up to <in>.bak)")
	pricesPath := fs.String("prices", "prices.json", "prices file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *batch == "" {
		return errors.New("--batch required")
	}
	pr, err := loadPrices(*pricesPath)
	if err != nil {
		return err
	}
	unit := pr.BillingUnit
	if unit == "" {
		unit = "per_llm_call"
	}
	data, err := os.ReadFile(*in)
	if err != nil {
		return err
	}
	if err := os.WriteFile(*in+".bak", data, 0o644); err != nil {
		return err
	}
	recs, err := csv.NewReader(bytes.NewReader(data)).ReadAll()
	if err != nil {
		return err
	}
	var kept, meta [][]string // meta: first row per run of the batch, in order
	seen := map[string]bool{}
	for i, r := range recs {
		if i == 0 || len(r) < baseCols {
			continue
		}
		if r[16] != *batch {
			kept = append(kept, r)
		} else if !seen[r[0]] {
			seen[r[0]] = true
			meta = append(meta, r)
		}
	}
	idRe := regexp.MustCompile(`"sessionID":"(ses_[A-Za-z0-9]+)"`)
	exp := retryingExport(opencodeExport, 3, 500*time.Millisecond)
	var out [][]string
	var failed []string
	for _, m := range meta {
		id := m[0]
		dir := filepath.Join(*runsDir, id)
		ev, err := os.ReadFile(filepath.Join(dir, "events.jsonl"))
		var sid string
		if err == nil {
			if mm := idRe.FindSubmatch(ev); mm != nil {
				sid = string(mm[1])
			}
		}
		rep, _ := strconv.Atoi(m[4])
		pass, _ := strconv.ParseBool(m[5])
		dur, _ := strconv.ParseFloat(m[15], 64)
		if sid == "" {
			fmt.Fprintf(stderr, "WARNING: %s: no session id; keeping placeholder\n", id)
			rows, _ := buildRows(id, m[1], m[2], m[3], rep, pass, dur, nil, pr, unit, *batch, false)
			out = append(out, withExtras(rows, extraOf(m))...)
			continue
		}
		sdir := filepath.Join(dir, "sessions")
		_ = os.MkdirAll(sdir, 0o755)
		leak := false
		per, winfo, warns := walkEx(sid, exp, func(s string, raw []byte) {
			if leaks(raw) {
				leak = true
			}
			_ = os.WriteFile(filepath.Join(sdir, s+".json"), raw, 0o644)
		})
		rows, w2 := buildRows(id, m[1], m[2], m[3], rep, pass, dur, per, pr, unit, *batch, leak)
		ex := extraOf(m)
		setComputed(ex, per, winfo, pr)
		for _, w := range append(warns, w2...) {
			fmt.Fprintf(stderr, "WARNING: %s: %s\n", id, w)
		}
		if len(per) == 0 {
			failed = append(failed, id)
		}
		out = append(out, withExtras(rows, ex)...)
	}
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write(csvHeader)
	_ = w.WriteAll(kept)
	_ = w.WriteAll(out)
	w.Flush()
	if err := os.WriteFile(*in, buf.Bytes(), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(stderr, "recollected %d runs (%d rows)\n", len(meta), len(out))
	if len(failed) > 0 {
		return fmt.Errorf("no usage for %d runs: %s", len(failed), strings.Join(failed, ", "))
	}
	return nil
}

// extraOf returns a CSV record's per-run extra cells (padded with empties for older rows).
func extraOf(rec []string) []string {
	out := make([]string, len(extraKeys))
	if len(rec) > baseCols {
		copy(out, rec[baseCols:])
	}
	return out
}
