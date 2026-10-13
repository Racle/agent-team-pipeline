# Agent pipeline cost benchmark

Compares the **cost** (and completion quality) of the setups doing the same coding tasks:

| mode            | agent                                                                                                                                                                                                                                  | models                                                                                                                                                  |
| --------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **A**           | OLD pipeline: `team-{captain,architect,engineer,forge,inspector,shipper}` (`legacy-agents/`, git HEAD)                                                                                                                                 | captain + architect: Opus 5.5 (via `--model`); engineer/forge/inspector: `github-copilot/claude-sonnet-5.5`; shipper: `github-copilot/claude-haiku-5.5` |
| **B**           | `bench-solo` (`legacy-agents/bench-solo.md`), the original plain Opus agent                                                                                                                                                            | single agent, no delegation, pinned `github-copilot/claude-opus-5.5`                                                                                    |
| **C**           | current `team-captain` (`../agents/team-captain.md`; formerly `team-solo`; the product default)                                                                                                                                        | single agent, no delegation, no model pinned; model passed via `--model` (Opus 5.5)                                                                     |
| **C:\<alias\>** | mode C with a model override, recorded as `C-<alias>`: `C:opus` = `github-copilot/claude-opus-5.5`, `C:sonnet` = `...claude-sonnet-5.5`, `C:haiku` = `...claude-haiku-5.5` (any other `provider/model` id also works in `run-task.sh`) | same agent file as C; the model comes only from `--model`                                                                                               |
| **S**           | (data only) SLIM captain pipeline, batch `batch-20261013-120246`, tasks 13-15 reps 1-2                                                                                                                                                 | excluded from reports by default (default `--modes` = every mode except S); include with `--modes A,B,C,S`                                              |

`legacy-agents/bench-solo.md` was recovered verbatim from the `cat > agents/bench-solo.md` heredoc in opencode session `ses_ed0d1d01affeSwVNGBUrZOZPQT` (the file was never committed); `team-*.md` there are `git show HEAD:agents/<f>.md`.

All modes get the same prompt plus the suffix _"Do not ask questions or wait for approval; proceed. Commit locally when done; no git remote exists."_, run against a fresh copy of the same fixture project, and are judged by the same hidden tests. Caveat: modes A and B on tasks 01-12 (batch `batch-20261013-090801`) ran with the older suffix _"Do not ask questions; proceed. Commit locally when done; no git remote exists."_. Everything lives in this directory; nothing outside `benchmark/` is touched.

## Layout

```
fixture/api   Go notes API (net/http, in-memory store)      fixture/web   Vite+React+TS UI (vitest)
fixture/Makefile   `make test` = go test + vitest
tasks/NN-slug/{prompt.md, meta.env, hidden/{api,web}/...}    15 tasks: 3 trivial, 3 simple, 3 standard, 3 complex, 3 hard
tasks-private/ (gitignored)   v2 tasks on real repos, repos.env, baseline/; variants/{v0,v1,v2}/   see "v2" below
scripts/                 reset.sh, run-task.sh, verify.sh, run-all.sh
cmd/bench                Go collector/reporter (stdlib only)
prices.json              price/multiplier table (GitHub Copilot per-token prices)
results/                 (gitignored) results.csv, report.md, report-full.md, runs/<run_id>/...
```

`results/` is gitignored; the final numbers are preserved in [docs/benchmark-results.md](../docs/benchmark-results.md).

```

```

Note: `../agents/team-captain.md` now equals `variants/v6/team-captain.md`; `variants/v0` is kept as the historical baseline (the old `task: deny` agent), so it no longer matches `agents/`.

## v2: delegation variants, real-repo tasks, multi-turn sessions

v2 asks a different question from v1 ("which pipeline is cheapest?"): **does letting the single captain start subagents (explore) pay off on bigger, unfamiliar code, and does it hurt long sessions?** Everything below is additive; modes A/B/C and tasks 01-15 work as before.

### Variants (`variants/`)

| mode   | agent file                                                                                                                                                                                                                                                            | notes                    |
| ------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------ |
| **V0** | `variants/v0/team-captain.md` = exact copy of `../agents/team-captain.md` (`task: deny`, "never delegate")                                                                                                                                                            | baseline                 |
| **V1** | V0 with `task: allow` and the delegation rule in the Flow ("Work alone by default. Start a subagent only when a loaded skill requires one, or a job would read much more than it returns ... Never delegate planning, small edits, or work needing current context.") |                          |
| **V2** | V1 + `variants/v2/opencode.fragment.json` (`agent.explore.model = github-copilot/claude-haiku-5.5`), deep-merged into the workdir `opencode.json` with `jq`                                                                                                           | cheaper explore subagent |

`run-task.sh <task> V1:sonnet 1` runs a variant with a model override; the CSV mode label is `V1-sonnet` (no `:model` = Opus, label `V1`). `C:<model>` still works.

### Real-repo fixtures and private tasks

The real (proprietary) repos are snapshotted once; they are never modified or copied with `.env` files:

```
scripts/prep-repos.sh [tmapi tmapp hmapi hmapp]
```

For the four repos listed in the gitignored `tasks-private/repos.list` (`<alias> <absolute path>` per line; two internal Go + React project pairs, aliases `tmapi`, `tmapp`, `hmapi`, `hmapp`) it pins `HEAD` into `tasks-private/repos.env`, runs `git archive <sha> | tar -x` into `~/.cache/bench-repos/<repo>@<sha>/` (tracked files only; initialised submodules are archived at their checked-out sha), runs `go mod download` / `npm ci` once (plain `npm ci` fails with an ERESOLVE peer-dependency conflict in both apps, so the script falls back to `npm ci --legacy-peer-deps`), and records the **baseline gate state and timings** in `tasks-private/baseline/<repo>.txt` plus normalised `*.norm` files (line/column numbers stripped) that `verify.sh` diffs against, so a gate that is red on the untouched repo (tsc in both apps, `go vet` in hmapi, 2 failing voucher-export tests in hmapp) only fails on **new** findings. `tsc` on hmapp needs `NODE_OPTIONS=--max-old-space-size=12288` (OOM at the default heap); `verify.sh` sets it.

`tasks-private/` holds the tasks for these proprietary repos (prompts, planted-bug patches, hidden tests, reference patches). It is **gitignored** together with `repos.env` and `results/`; never commit it. `scripts/*` look in `tasks/` first, then `tasks-private/`.

Task `meta.env` keys (v2): `CATEGORY` (`large` | `session`), `FIXTURE=repo:<short>[,<short>]` (unset = the synthetic notes fixture), `GATES="..."`, `NODE_REGISTER` (path of the babel register file for hidden node tests, relative to the app dir; default `test/hidden/register.cjs`, shipped inside `hidden/`).

`reset.sh` lays a single repo out at the workdir root and several as `api/` (names ending `api`) and `app/` (names ending `app`; note that a repo's own `app/` directory is then `app/app/`). Files are copied with `cp -a --reflink=auto` semantics (falling back to hard-linked `node_modules` via `cp -al`; only same-filesystem). If `<task>/setup.patch` exists it is applied **before** the single baseline commit, so the planted bug is invisible in `git diff`/`git log` and in the code-size metrics.

### Gates (`GATES` in `meta.env`)

Default for the synthetic fixture is unchanged (`tests vet tsc build hidden`, recorded as `TESTS VET TSC BUILD HIDDEN`). For real repos: `hidden` (hidden tests; `go test -run TestHidden` per package, `node --require <register> --test` for `*.test.ts`), `gobuild` (`go build ./...`), `govet` (`go vet ./pkg/...`, no new findings vs baseline), `gofmt` (`gofmt -l .`, no new files vs baseline), `tsc` (no new errors vs baseline), `nodetest` (existing node tests, e.g. `npm run test:voucher-export`; the failure count may not exceed the baseline's). `hidden` always runs last. Hidden Go tests live in `pkg/` sub-packages (the packages with no `TestMain`; `package main` needs live services). Hidden files mirror the workdir layout (`hidden/pkg/database/x_test.go`, `hidden/app/test/...`).

### Multi-turn tasks

`<task>/turns/01.md ... NN.md`: turn 1 starts the session; later turns run `opencode run --session <id>` (verified in `opencode run --help`) with the same agent/model. One overall `BENCH_TIMEOUT` (default 3600 s for multi-turn). Events go to `events-01.jsonl ...`. After each turn `<task>/verify-turn-NN.sh <workdir>` (env `TASK`, `BENCH`) runs; it copies the hidden tests of turns `01..NN` from `<task>/hidden-turns/NN/` into the workdir, runs them and removes them again (`HIDDEN_TURN=NN verify.sh`), so the agent never sees them. The CSV gets `turns` and `turns_passed`; the final verdict needs all final gates **and** all turns passed.

### Tasks (tasks-private/)

| task                           | cat     | repo        | what                                                                                                                                 |
| ------------------------------ | ------- | ----------- | ------------------------------------------------------------------------------------------------------------------------------------ |
| `L1-share-range-sort`          | large   | hmapi       | planted bug in `pkg/database` `parseRange` (spaces around numbers make share ranges unparseable -> sorted last); symptom-only prompt |
| `L2-voucher-payment-order`     | large   | hmapp       | planted bug in voucher-export comparison (`PaymentRows` order matters); UI symptom                                                   |
| `L3-sauna-internal-note`       | large   | hmapi+hmapp | add `internal_note` to `Sauna` end to end (model, list column, edit form); static verification                                       |
| `L4-duplicates-company-filter` | large   | hmapi       | `POST /linkageregisters/duplicates/query` ignores `company_id` (planted JSON tag bug); route -> handler -> request model tracing     |
| `M1-due-states`                | session | tmapi       | 5 turns growing a due-date-state feature (pure functions, per-turn hidden tests)                                                     |
| `M2-queue-workload`            | session | tmapi+tmapp | 4 turns: 2 Go (queue counts, ranking) + 2 TS (label, severity summary)                                                               |

`scripts/validate-tasks.sh [glob]` covers `tasks/` and `tasks-private/`: the planted/untouched state must FAIL, `reference.patch` must PASS (multi-turn: one cumulative reference; every `verify-turn-NN.sh` must also pass on it).

### New CSV columns / report

`peak_ctx` (max over the ROOT session's LLM calls of input + cache read + cache write), `sub_cost_usd` / `sub_cost_share` (cost of the sessions below the root), `task_calls_explore|general|other` (task tool calls of the root session by `subagent_type`), `turns`, `turns_passed`, plus `gate_gobuild|govet|gofmt|nodetest`. Old CSVs stay loadable (the header is upgraded on the next append, backup `results.csv.bak-pre-v2`). The report adds: cost per pass, peak_ctx mean/max and subagent share per mode x category; delegation counts per mode x category and mode x task; repo gate failures; multi-turn results.

### Guards in `run-all.sh`

`BENCH_BUDGET` (USD, default 90): summed `est_usd` of the current batch is checked before every run; the batch aborts when reached (the report still prints). `--stop-on-broken`: after rep 1 of a task, if every mode failed it with the _same_ failed gates, later reps of that task are skipped (task/gate probably broken). `BENCH_RUN_MAX_USD` (default 8): a run above it prints a `COST-ABORT note` and skips that task x mode's remaining reps (a running agent cannot be priced mid-run).

### Isolation additions

`run-task.sh` exports `OPENCODE_DISABLE_CLAUDE_CODE_PROMPT=1` (the variable name exists in the opencode 1.18.35 binary next to `OPENCODE_DISABLE_CLAUDE_CODE` and `OPENCODE_DISABLE_CLAUDE_CODE_SKILLS`; its effect was **not verified** by a run). The project `.claude/skills` stay visible on purpose.

### Commands

```
scripts/prep-repos.sh                      # once (free)
scripts/validate-tasks.sh                  # all tasks, no LLM
# smoke (one real, paid Sonnet run, V1, simplest private task):
scripts/run-task.sh L1-share-range-sort V1:sonnet 1
# matrix: Sonnet V0/V1/V2 x L1-L4 x 2 reps; then x M1, M2 x 1 rep
scripts/run-all.sh --modes "V0:sonnet V1:sonnet V2:sonnet" --tasks 'L[1-4]-*' --reps 2 --stop-on-broken
scripts/run-all.sh --modes "V0:sonnet V1:sonnet V2:sonnet" --tasks 'M[12]-*' --reps 1
# later, Opus: same commands with V0 V1 V2 (default model) or V0:opus ...
bin/bench report --batch <batch id> --modes V0-sonnet,V1-sonnet,V2-sonnet
```

## Prerequisites

`opencode` (tested 1.18.35) logged in with GitHub Copilot (`opencode providers list`), Go >= 1.22, Node >= 20 + npm, bash, `timeout`, git. Install the fixture's deps once:

```
(cd fixture/web && npm ci)
make -C fixture test        # must be green
```

## Quick smoke test (one trivial task, all modes, 1 rep)

```
scripts/run-all.sh --reps 1 --tasks '01-*'
```

Or manually: `scripts/run-task.sh 01-fix-404-typo A 1` then `... B 1`, then `bin/bench report`. Set `BENCH_KEEP_WORKDIR=1` to keep the temp workdir for inspection. `BENCH_TIMEOUT` (seconds, default 1800) caps each run; a timeout counts as FAIL but its tokens are still collected.

## Model comparison (C-opus / C-sonnet / C-haiku)

Same agent (`agents/team-captain.md`), three models, strict gate, fresh report for just this batch:

```
scripts/run-all.sh --modes "C:opus C:sonnet C:haiku" --reps 3
bin/bench report --batch <batch id printed by run-all>     # all modes in the batch; ratios: C-haiku/C-sonnet, C-haiku/C-opus, C-sonnet/C-opus
```

Single run: `scripts/run-task.sh 05-search-filter C:sonnet 1`. Old A/B/C rows are untouched (`--batch all` mixes them in).

## Strict completion gate

`scripts/verify.sh <task> <workdir> [gates-file]` is deterministic and PASS requires **all** gates (all always run, none short-circuits):

| gate     | check                                                                                                                      |
| -------- | -------------------------------------------------------------------------------------------------------------------------- |
| `TESTS`  | the project's own tests: `go test -race ./...` (api) and `npx vitest run` (web); run before the hidden tests are copied in |
| `VET`    | `go vet ./...` (api)                                                                                                       |
| `TSC`    | `npx tsc --noEmit` (web)                                                                                                   |
| `BUILD`  | `go build ./...` (api) and `npx vite build` (web)                                                                          |
| `HIDDEN` | the task's hidden tests, `go test -race -run TestHidden` / `vitest run src/hidden`                                         |

`-race` is always on; `GO_TEST_FLAGS` from a task's `meta.env` is appended. Each gate's result is printed, and `results/runs/<id>/gates.env` gets `HIDDEN=1 TESTS=1 VET=0 TSC=1 BUILD=1` (1 = pass, 0 = fail). A timed-out run records no gates.

## Full run

```
scripts/run-all.sh                     # all 15 tasks x 3 modes x 3 reps = 135 paid runs
scripts/run-all.sh --modes "A B" --tasks '1[345]-*' --rep-start 1 --reps 3   # only some modes / missing reps
scripts/run-all.sh --reps 5 --tasks '1*'
```

Mode order rotates per (task, rep). Runs are sequential. The final report is printed and saved to `results/report.md` (the full 135-run report is `results/report-full.md`); rerun any time with `bin/bench report --in results/results.csv` (build with `go build -o bin/bench ./cmd/bench`).

## Hard tasks (13-15)

`CATEGORY=hard` tasks that stress multi-file reasoning: `13-extract-packages` (multi-package Go refactor, behaviour unchanged, new import paths fixed by the prompt), `14-rename-loses-body` (cross-stack bug: partial `PUT` wipes fields; needs API + UI changes), `15-concurrent-counters` (atomic like/merge/stats under concurrency; judged with `go test -race`, set by `GO_TEST_FLAGS=-race` in the task's `meta.env`, read by `verify.sh`). Run just these:

```
scripts/run-all.sh --reps 2 --tasks '1[3-5]-*'
```

History / relabel (batch `batch-20261013-120246`, tasks 13-15 reps 1-2): the rows were first recorded as A/B but ran the SLIM captain (-> now mode `S`) and `team-solo` (-> now mode `C`; results labelled C or team-solo are the same agent as the current `team-captain`). Run dirs under `results/runs/` were NOT renamed: dirs named `13|14|15-*-B-*` are mode C, `*-A-*` are mode S. The pre-relabel CSV is `results/results.csv.bak-pre-relabel`. Full comparison = `batch-20261013-090801` (A, B on 01-12) + that batch (C on 13-15) + new runs; report with `bin/bench report --batch all --modes A,B,C`.

## Validating tasks (no LLM calls)

`scripts/validate-tasks.sh [glob]` checks every task: the untouched fixture must FAIL its hidden tests and `tasks/<task>/reference.patch` (a known-good solution, applied with `git apply`) must PASS. Reference patches live only under `tasks/`, which is never copied into a workdir.

## Reading the report

- **Pass rate**: hidden tests (public HTTP/DOM contract only) pass/fail, per mode x category (plus `ALL`).
- **mean / min / max**: cost per run, over the reps, summed across the entire session tree and all models.
- **cost / successful task**: total cost of _all_ runs in the group (failed ones included) divided by the number of passing runs. This is the number to compare; a cheap mode that fails often is not cheap.
- **Estimated USD** (token counts x `usd_per_mtok`) is the primary number: since 2026-06-01 GitHub Copilot bills per token (AI credits, 1 credit = $0.01). Copilot reports `cost = 0` in sessions, so it is computed from GitHub's published prices. The **estimated premium requests** column (`requests x multiplier`) only matters on legacy annual plans; multipliers are 0 by default.
- Cost-ratio tables: mean cost ratio for every mode pair (A/B, A/C, B/C) per category. `bin/bench report --modes A,C --batch id1,id2` selects modes (default: every mode except `S`) and combines batches (`all` = every batch).
- Per-model breakdown: average per-run requests/tokens/cost per model, per mode, so you can see where the pipeline spends.

`results/results.csv` is long-format, one row per (run, model): `run_id,task,category,mode,rep,pass,model,requests,input,output,reasoning,cache_read,cache_write,est_premium_requests,est_usd,duration_s,batch_id,leak,files_changed,app_added,test_added,total_added,gate_hidden,gate_tests,gate_vet,gate_tsc,gate_build`. The last nine columns are per-run (repeated on each model row) and optional: older CSVs without them still load (the header is upgraded on the next append, backup `results.csv.bak-pre-gates`), and the report only averages runs that have them. Code-size metrics (`run-task.sh`, measured in the workdir before cleanup and before the hidden tests are copied in) are lines added vs the baseline commit, excluding node_modules, lockfiles, build output, `.opencode`: `files_changed` = files touched, `app_added` = non-test source lines (go/ts/tsx/js/jsx/css/html), `test_added` = lines in `*_test.go` / `*.test.ts(x)`, `total_added` = all lines added. `gate_*` are 1/0 per gate (empty = not run). The report adds "Code size" (mean files/app/test/total per mode x category) and "Gate failures" (failed/recorded per gate) tables. Runs without a session id (or without usage) get a placeholder row with model `(none)`, `pass=false` and zero cost, so they still count in the pass rate; the report notes them. `run-all.sh` stamps every row with a `batch_id` and reports only that batch; `bin/bench report --batch <id|id,id|all|latest>` filters, no flag = all rows (mode `S` excluded unless in `--modes`). `leak=true` means the exported session text contains `hidden_` or `/benchmark/tasks/` (the agent may have seen hidden tests); the report warns, treat those runs as suspect. Raw exports of every session are kept in `results/runs/<run_id>/sessions/`; re-pricing is possible by editing `prices.json` and re-running `bench collect --export-dir results/runs/<id>/sessions --session <root id> ...` (rows append; remove the old ones from the CSV).

## Editing prices

`prices.json` contains GitHub Copilot per-token prices from [Models and pricing](https://docs.github.com/en/copilot/reference/copilot-billing/models-and-pricing) (checked 2026-10-13). Reasoning is priced at the output rate; Claude Haiku 5.5 uses its default (<=100K input) tier only. Re-check the page when prices change. Fields:

- `models["<providerID>/<modelID>"].usd_per_mtok` - `input`, `output`, `reasoning`, `cache_read`, `cache_write` (USD per million tokens; reasoning is priced separately because opencode reports it separately from `output`);
- `models[...].multiplier` - legacy premium-request multiplier (annual request-based plans only; 0 by default);
- `billing_unit` - `per_llm_call` (each LLM request counts `multiplier`) or `per_user_prompt` (each _user_ message in any session of the tree - the root prompt plus every subagent delegation prompt - counts `multiplier`). Override per collect with `--billing`.

Model keys must match what the exports contain (e.g. `github-copilot/claude-opus-5.5`, the same string passed to `--model`; check `results/runs/*/sessions/*.json` or the collector's `no price entry for model ...` warnings). **Check the key names before a full run**: `run-task.sh` passes `--strict` to `collect` and exits 3 (after writing the row and cleaning up) when any model has no price entry, but it is cheaper to catch it on the first smoke run.

## Isolation

- `run-task.sh` exports `XDG_CONFIG_HOME` pointing at an empty per-run directory, so the user's global opencode config is invisible: no MCP servers (Engram, Playwright), no plugins, no global agents/skills/instructions. Credentials live in `XDG_DATA_HOME` (`~/.local/share/opencode/auth.json`) and are intentionally kept. Verified with `opencode agent list` / `opencode mcp list`.
- Each run gets a fresh `mktemp -d` copy of `fixture/` (including `node_modules`, copied with reflink when available), its own `git init` + baseline commit, and its own `opencode.json` (permissive permissions, `mcp: {}`).
- Agents are installed project-locally in `<workdir>/.opencode/agents/`: mode A copies `legacy-agents/team-*.md`, mode B `legacy-agents/bench-solo.md`, mode C `../agents/team-captain.md` (run as agent `team-captain`; mode A's old pipeline also has a `team-captain`, which is fine since each workdir is isolated). Nothing needs to be installed globally.
- Hidden tests are never copied into the workdir until the agent has finished (`verify.sh`). The workdir lives under `/tmp` and its `opencode.json` sets `external_directory: deny`, so file tools cannot read the repo. `bash` is still allowed and could in principle `cat` them, hence the post-run `leak` check described above.
- `opencode` runs under `setsid timeout -k 30`, and an EXIT trap kills the process group and removes the workdir even when collection fails.
- Skills from `~/.claude` and `~/.agents` (and other global skill locations) are still discovered by opencode; `XDG_CONFIG_HOME` does not hide them. They load equally in modes A, B and C, so they do not bias the comparison, but they add context tokens to both.
- The verdict is the strict gate (see above), not just the hidden tests.
- Session data still goes to the normal opencode database (sessions titled `bench-<run_id>`); `opencode export` reads from it.

## Caveats

- **Copilot reports cost 0**, so `est_premium_requests` / `est_usd` are estimates built from your `prices.json`.
- **Non-determinism**: LLM runs vary; use >= 3 reps and read min/max, not just the mean. Small tasks are especially noisy.
- All modes (A/B/C) are launched with `--model github-copilot/claude-opus-5.5` (C inherits it via `--model`); subagents use their own frontmatter models.
- The hidden tests judge the _contract in the prompt_, not code quality. The agent's own full suite result is printed in `verify.log` as informational only.
- `per_user_prompt` is an interpretation (see above); Copilot may not bill agent-initiated subagent turns that way.
- Task-tool children are found via task tool parts only; a child whose task call failed before a session was created is warned about and not counted.
- Every task has a `reference.patch`; `scripts/validate-tasks.sh` checks baseline FAIL and reference PASS for all of them.

## Internals (findings from probing opencode 1.18.35)

- **Agent dir**: project-local agents load from `.opencode/agents/*.md` (plural; singular `.opencode/agent/` also works). A project agent overrides a global one with the same name.
- **`opencode run --format json`** prints one JSON event per line. Every event has a top-level `"sessionID":"ses_..."` (root session); the collector takes the first one. Error sample: `cmd/bench/testdata/run-events.jsonl`. `--dir`, `--agent`, `--model`, `--title` are real flags.
- **`opencode export <id>`** prints JSON to stdout (`Exporting session: ...` goes to stderr; the collector tolerates a preamble). Shape: `{info, messages[]}`.
  - `info`: `id, parentID?, agent, model{id,providerID,variant}, cost, tokens{input,output,reasoning,cache{read,write}}, directory, time`. `info.tokens` is the session total, but the collector sums per message so it can split per model.
  - `messages[].info`: `role`; assistant: `providerID, modelID, agent, cost, tokens{total,input,output,reasoning,cache{read,write}}`; user: `model{providerID,modelID}, agent`.
  - `messages[].parts[]`: `step-finish` parts (one per LLM call) carry `tokens{...}` and `cost`; `tool` parts with `tool:"task"` are subagent calls.
- **Request count**: number of `step-finish` parts (fallback: assistant message with non-zero tokens).
- **Child linkage**: a `task` tool part has `state.metadata.sessionId` (child) and `parentSessionId`, plus `state.metadata.model{providerID,modelID}`; the child session's own `info.parentID` points back. `opencode session list` does not list child sessions, and `opencode export <childId>` works. The collector walks `state.metadata.sessionId` recursively with a visited set.
- `cost` is non-zero in these exports only when the provider is priced by opencode; Copilot spend should be treated as unknown, hence the estimates.
- Sanitized samples used by the tests: `cmd/bench/testdata/parent.json`, `child.json` (prompts and outputs redacted).
- `go` flag gotcha: the collector's `--pass` takes a value (`--pass true`), unlike a bare boolean flag.
