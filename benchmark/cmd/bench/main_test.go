package main

import (
	"bytes"
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixtureExport(t *testing.T) exporter {
	t.Helper()
	return func(id string) ([]byte, error) {
		switch id {
		case "ses_f7f7dd943ffeFMGnvvklAXnkeQ":
			return os.ReadFile("testdata/parent.json")
		case "ses_f7f7bf833ffea8uK5zeARa7kcr":
			return os.ReadFile("testdata/child.json")
		}
		return nil, os.ErrNotExist
	}
}

func TestWalkSumsTreeAndPerModel(t *testing.T) {
	per, warns := walk("ses_f7f7dd943ffeFMGnvvklAXnkeQ", fixtureExport(t), nil)
	if len(warns) != 0 {
		t.Fatalf("unexpected warnings: %v", warns)
	}
	opus := per["github-copilot/claude-opus-5"]
	sonnet := per["github-copilot/claude-sonnet-5"]
	if opus == nil || sonnet == nil {
		t.Fatalf("models: %v", per)
	}
	// parent sample: 3 assistant messages with step-finish parts
	if opus.Requests != 3 || opus.Input != 6 || opus.Output != 113+476+2586 || opus.Reasoning != 154+159+720 ||
		opus.CacheRead != 0+34834+40486 || opus.CacheW != 34834+5001+1559 {
		t.Fatalf("opus usage: %+v", opus)
	}
	if opus.UserPrompts != 1 || sonnet.UserPrompts != 1 {
		t.Fatalf("user prompts: %d %d", opus.UserPrompts, sonnet.UserPrompts)
	}
	if sonnet.Requests == 0 || sonnet.Output == 0 {
		t.Fatalf("child session not counted: %+v", sonnet)
	}
}

func TestWalkWarnsOnMissingChildAndHandlesCycles(t *testing.T) {
	exp := func(id string) ([]byte, error) {
		if id == "ses_f7f7dd943ffeFMGnvvklAXnkeQ" {
			return os.ReadFile("testdata/parent.json")
		}
		return nil, os.ErrNotExist
	}
	per, warns := walk("ses_f7f7dd943ffeFMGnvvklAXnkeQ", exp, nil)
	if len(warns) != 1 || !strings.Contains(warns[0], "ses_f7f7bf833ffea8uK5zeARa7kcr") {
		t.Fatalf("warns: %v", warns)
	}
	if per["github-copilot/claude-opus-5"] == nil {
		t.Fatal("parent usage lost")
	}
	// self-referencing doc must terminate
	self := func(id string) ([]byte, error) {
		return []byte(`{"info":{"id":"s"},"messages":[{"info":{"role":"assistant","providerID":"p","modelID":"m","tokens":{"input":1}},"parts":[{"type":"tool","tool":"task","state":{"metadata":{"sessionId":"s"}}}]}]}`), nil
	}
	per, _ = walk("s", self, nil)
	if per["p/m"].Requests != 1 {
		t.Fatalf("cycle: %+v", per["p/m"])
	}
}

func TestPreambleIsTolerated(t *testing.T) {
	exp := func(string) ([]byte, error) {
		return []byte("Exporting session: s\n{\"info\":{\"id\":\"s\"},\"messages\":[]}"), nil
	}
	if _, warns := walk("s", exp, nil); len(warns) != 0 {
		t.Fatalf("%v", warns)
	}
}

func TestCollectAndReport(t *testing.T) {
	dir := t.TempDir()
	prices := filepath.Join(dir, "prices.json")
	_ = os.WriteFile(prices, []byte(`{"billing_unit":"per_llm_call","models":{
	 "github-copilot/claude-opus-5":{"multiplier":3,"usd_per_mtok":{"input":10,"output":100,"reasoning":100,"cache_read":1,"cache_write":10}},
	 "github-copilot/claude-sonnet-5":{"multiplier":1,"usd_per_mtok":{"input":3,"output":15,"reasoning":15,"cache_read":0.3,"cache_write":3}}}}`), 0o644)
	out := filepath.Join(dir, "r.csv")
	var stderr bytes.Buffer
	// map ids -> files in a temp export dir
	ed := filepath.Join(dir, "exp")
	_ = os.MkdirAll(ed, 0o755)
	for id, src := range map[string]string{"ses_f7f7dd943ffeFMGnvvklAXnkeQ": "testdata/parent.json", "ses_f7f7bf833ffea8uK5zeARa7kcr": "testdata/child.json"} {
		b, _ := os.ReadFile(src)
		_ = os.WriteFile(filepath.Join(ed, id+".json"), b, 0o644)
	}
	args := func(mode, pass string) []string {
		return []string{"--session", "ses_f7f7dd943ffeFMGnvvklAXnkeQ", "--task", "01-x", "--category", "trivial", "--mode", mode, "--rep", "1",
			"--pass", pass, "--duration", "42", "--prices", prices, "--out", out, "--export-dir", ed}
	}
	if err := cmdCollect(args("A", "true"), &stderr); err != nil {
		t.Fatal(err)
	}
	if err := cmdCollect(args("B", "false"), &stderr); err != nil {
		t.Fatal(err)
	}
	fh, _ := os.Open(out)
	recs, err := csv.NewReader(fh).ReadAll()
	fh.Close()
	if err != nil || len(recs) != 5 { // header + 2 models x 2 runs
		t.Fatalf("csv: %v %v", err, recs)
	}
	var rep bytes.Buffer
	if err := cmdReport([]string{"--in", out}, &rep, &stderr); err != nil {
		t.Fatal(err)
	}
	s := rep.String()
	for _, want := range []string{"| A | trivial | 1 | 1/1 (100%)", "| B | trivial | 1 | 0/1 (0%)", "n/a (0 passes)", "Per-model breakdown", "claude-sonnet-5"} {
		if !strings.Contains(s, want) {
			t.Fatalf("report missing %q:\n%s", want, s)
		}
	}
}

func TestBuildRowsBillingUnitsAndPrice(t *testing.T) {
	per := map[string]*usage{"p/m": {Requests: 4, UserPrompts: 2, Input: 1e6, Output: 2e6}}
	pr := prices{Models: map[string]modelPrice{"p/m": {Multiplier: 2}}}
	pr.Models["p/m"] = func() modelPrice {
		m := pr.Models["p/m"]
		m.USDPerMtok.Input, m.USDPerMtok.Output = 3, 15
		return m
	}()
	rows, _ := buildRows("r", "t", "c", "A", 1, true, 1, per, pr, "per_llm_call", "b", false)
	if rows[0][13] != "8" || rows[0][14] != "33.000000" {
		t.Fatalf("per_llm_call: %v", rows[0])
	}
	rows, warns := buildRows("r", "t", "c", "A", 1, true, 1, per, pr, "per_user_prompt", "b", false)
	if rows[0][13] != "4" || len(warns) != 0 {
		t.Fatalf("per_user_prompt: %v %v", rows[0], warns)
	}
	_, warns = buildRows("r", "t", "c", "A", 1, true, 1, per, prices{}, "per_llm_call", "b", false)
	if len(warns) != 1 {
		t.Fatalf("expected missing-price warning")
	}
}

func TestSessionlessPlaceholderStrictAndBatch(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "r.csv")
	pj := filepath.Join(dir, "p.json")
	os.WriteFile(pj, []byte(`{"models":{}}`), 0o644)
	var stderr bytes.Buffer
	ed := filepath.Join(dir, "exp")
	_ = os.MkdirAll(ed, 0o755)
	for id, src := range map[string]string{"ses_f7f7dd943ffeFMGnvvklAXnkeQ": "testdata/parent.json", "ses_f7f7bf833ffea8uK5zeARa7kcr": "testdata/child.json"} {
		b, _ := os.ReadFile(src)
		_ = os.WriteFile(filepath.Join(ed, id+".json"), b, 0o644)
	}
	base := []string{"collect", "--task", "t", "--category", "trivial", "--mode", "A", "--prices", pj, "--out", out, "--batch", "b1", "--pass", "true"}
	if err := runCLI(append(base, "--session", ""), &bytes.Buffer{}, &stderr); err != nil {
		t.Fatal(err)
	}
	var rep bytes.Buffer
	if err := runCLI([]string{"report", "--in", out, "--batch", "b1"}, &rep, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rep.String(), "| A | trivial | 1 | 0/1 (0%)") || !strings.Contains(rep.String(), "no usage data") {
		t.Fatalf("placeholder not counted as failed run:\n%s", rep.String())
	}
	rep.Reset()
	_ = runCLI([]string{"report", "--in", out, "--batch", "other"}, &rep, &stderr)
	if strings.Contains(rep.String(), "| A |") {
		t.Fatalf("batch filter leaked rows")
	}
	if err := runCLI(append(base, "--session", "ses_f7f7dd943ffeFMGnvvklAXnkeQ", "--export-dir", ed, "--strict"), &bytes.Buffer{}, &stderr); err == nil || !strings.Contains(err.Error(), "no price entry") {
		t.Fatalf("strict should fail on missing price, got %v", err)
	}
}

func TestRetryingExportTruncatedThenValid(t *testing.T) {
	calls := 0
	exp := retryingExport(func(string) ([]byte, error) {
		calls++
		if calls < 3 {
			return []byte(`{"messages":[{"info"`), nil
		}
		return []byte(`{"messages":[]}`), nil
	}, 3, 0)
	raw, err := exp("s")
	if err != nil || calls != 3 || string(raw) != `{"messages":[]}` {
		t.Fatalf("raw=%q calls=%d err=%v", raw, calls, err)
	}
	bad := retryingExport(func(string) ([]byte, error) { return []byte(`{"a"`), nil }, 3, 0)
	if _, err := bad("s"); err == nil {
		t.Fatal("expected error for persistently truncated export")
	}
}

func TestMetricsGatesAndModelModes(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "r.csv")
	// an old-format CSV (no extra columns) must stay loadable and be upgraded on append
	old := strings.Join(csvHeader[:baseCols], ",") + "\nold-1,t,trivial,A,1,true,m,1,0,0,0,0,0,0,1.5,1.0,b0,false\n"
	if err := os.WriteFile(out, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	mf, gf := filepath.Join(dir, "metrics.env"), filepath.Join(dir, "gates.env")
	_ = os.WriteFile(mf, []byte("FILES_CHANGED=3\nAPP_ADDED=10\nTEST_ADDED=5\nTOTAL_ADDED=16\n"), 0o644)
	_ = os.WriteFile(gf, []byte("HIDDEN=1\nTESTS=1\nVET=0\nTSC=1\nBUILD=1\n"), 0o644)
	for _, m := range []string{"C-opus", "C-sonnet", "C-haiku"} {
		args := []string{"collect", "--session", "", "--task", "t", "--category", "trivial", "--mode", m, "--rep", "1",
			"--pass", "false", "--run-id", "r-" + m, "--prices", filepath.Join(dir, "none.json"), "--out", out,
			"--batch", "b1", "--metrics-file", mf, "--gates-file", gf}
		_ = os.WriteFile(filepath.Join(dir, "none.json"), []byte("{}"), 0o644)
		if err := runCLI(args, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
			t.Fatal(err)
		}
	}
	var rep bytes.Buffer
	if err := runCLI([]string{"report", "--in", out, "--batch", "b1"}, &rep, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"C-sonnet / C-opus", "C-haiku / C-opus", "| C-opus | trivial | 1 | 3.0 | 10.0 | 5.0 | 16.0 |", "| C-opus | trivial | 0/1 | 0/1 | 1/1 | 0/1 | 0/1 |"} {
		if !strings.Contains(rep.String(), want) {
			t.Errorf("report missing %q:\n%s", want, rep.String())
		}
	}
	rep.Reset()
	if err := runCLI([]string{"report", "--in", out}, &rep, &bytes.Buffer{}); err != nil || !strings.Contains(rep.String(), "| A |") {
		t.Fatalf("old rows not loadable: %v\n%s", err, rep.String())
	}
}

func v2Export(id string) ([]byte, error) {
	switch id {
	case "ses_v2root", "ses_v2explore", "ses_v2general":
		return os.ReadFile("testdata/" + strings.Replace(id, "ses_v2", "v2-", 1) + ".json")
	}
	return nil, os.ErrNotExist
}

func TestWalkExRootPeakSubCostAndTaskCalls(t *testing.T) {
	per, info, warns := walkEx("ses_v2root", v2Export, nil)
	// the third task call has no child session id: warned, still counted in the call counts
	if len(warns) != 1 || !strings.Contains(warns[0], "no child sessionId") {
		t.Fatalf("warns: %v", warns)
	}
	// root steps: 10+0+1000, 5+1000+200, 2+3000+100 -> peak 3102; child contexts (9100) must NOT count
	if info.PeakCtx != 3102 {
		t.Fatalf("peak ctx = %v, want 3102", info.PeakCtx)
	}
	if info.TaskExplore != 1 || info.TaskGeneral != 1 || info.TaskOther != 1 {
		t.Fatalf("task calls: %+v", info)
	}
	if info.Sub["github-copilot/claude-haiku-5.5"] == nil || info.Sub["github-copilot/claude-sonnet-5.5"].Requests != 1 {
		t.Fatalf("sub usage: %+v", info.Sub)
	}
	// the root's own sonnet requests (3) are not part of Sub; all = root 3 + general 1
	if per["github-copilot/claude-sonnet-5.5"].Requests != 4 {
		t.Fatalf("total sonnet requests: %+v", per["github-copilot/claude-sonnet-5.5"])
	}
}

func TestCollectV2ColumnsAndReport(t *testing.T) {
	dir := t.TempDir()
	prices := filepath.Join(dir, "prices.json")
	_ = os.WriteFile(prices, []byte(`{"models":{
	 "github-copilot/claude-sonnet-5.5":{"usd_per_mtok":{"input":1000000,"output":0,"reasoning":0,"cache_read":0,"cache_write":0}},
	 "github-copilot/claude-haiku-5.5":{"usd_per_mtok":{"input":1000000,"output":0,"reasoning":0,"cache_read":0,"cache_write":0}}}}`), 0o644)
	ed := filepath.Join(dir, "exp")
	_ = os.MkdirAll(ed, 0o755)
	for _, id := range []string{"ses_v2root", "ses_v2explore", "ses_v2general"} {
		b, _ := v2Export(id)
		_ = os.WriteFile(filepath.Join(ed, id+".json"), b, 0o644)
	}
	mf := filepath.Join(dir, "metrics.env")
	_ = os.WriteFile(mf, []byte("TURNS=5\nTURNS_PASSED=3\n"), 0o644)
	out := filepath.Join(dir, "r.csv")
	// old-format CSV rows must stay loadable
	_ = os.WriteFile(out, []byte(strings.Join(csvHeader[:baseCols], ",")+"\nold-1,t,trivial,A,1,true,m,1,0,0,0,0,0,0,1.5,1.0,b0,false\n"), 0o644)
	for i, mode := range []string{"V0", "V1-sonnet"} {
		args := []string{"collect", "--session", "ses_v2root", "--task", "M1", "--category", "session", "--mode", mode, "--rep", "1",
			"--pass", []string{"false", "true"}[i], "--run-id", "r-" + mode, "--prices", prices, "--out", out, "--export-dir", ed, "--batch", "b2", "--metrics-file", mf}
		if err := runCLI(args, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
			t.Fatal(err)
		}
	}
	fh, _ := os.Open(out)
	cr := csv.NewReader(fh)
	cr.FieldsPerRecord = -1 // the old row is short
	recs, err := cr.ReadAll()
	fh.Close()
	if err != nil {
		t.Fatal(err)
	}
	idx := map[string]int{}
	for i, h := range recs[0] {
		idx[h] = i
	}
	last := recs[len(recs)-1]
	// sonnet in = 10+5+2 (root) + 50 (general) = 67, haiku in = 100 -> total 167 USD, sub = 150 -> share 0.8982
	for col, want := range map[string]string{"peak_ctx": "3102", "sub_cost_usd": "150.000000", "sub_cost_share": "0.8982", "task_calls_explore": "1",
		"task_calls_general": "1", "task_calls_other": "1", "turns": "5", "turns_passed": "3"} {
		if got := last[idx[col]]; got != want {
			t.Errorf("%s = %q, want %q", col, got, want)
		}
	}
	var rep bytes.Buffer
	if err := runCLI([]string{"report", "--in", out}, &rep, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"## Efficiency: cost per pass", "| V1-sonnet | session | 1 | 1/1 | 167 | 3102 | 3102 |", "| V0 | task M1 | 1 | 1.00 | 1.00 | 1.00 |",
		"## Multi-turn", "| V1-sonnet | M1 | 1 | 5 | 3.00 | 1/1 |", "| A |"} {
		if !strings.Contains(rep.String(), want) {
			t.Errorf("report missing %q:\n%s", want, rep.String())
		}
	}
	// header upgraded, old backup kept
	if _, err := os.Stat(out + ".bak-pre-gates"); err != nil {
		t.Errorf("expected header backup: %v", err)
	}
}

func TestModeRankOrdersVariants(t *testing.T) {
	got := orderModes(map[string]bool{"V2-sonnet": true, "V0": true, "V1-opus": true, "V1": true, "A": true, "V1-sonnet": true})
	if strings.Join(got, ",") != "A,V0,V1,V1-sonnet,V1-opus,V2-sonnet" {
		t.Fatalf("order = %v", got)
	}
}
