# AI Agent Team Pipeline for OpenCode

> A cheap single-agent setup for [OpenCode](https://opencode.ai).

![opencode](https://img.shields.io/badge/for-opencode-4ADE80)
![benchmark](https://img.shields.io/badge/benchmark-~49%25%20cheaper-brightgreen)
![pass rate](https://img.shields.io/badge/hidden%20tests-45%2F45-blue)

_Based on [san3jaya/agent-team-pipeline](https://github.com/san3jaya/agent-team-pipeline)._

## Why

The default agent, `team-captain` (a single agent; called `team-solo` during benchmarking), is about **49% cheaper** than the old 6-agent pipeline ($0.548 → $0.281 per task) and about **10% cheaper** than a plain Opus agent ($0.311; within noise per category; see caveats), with the same pass rate: 45/45 hidden-test passes for each setup (15 tasks × 3 reps).

Prices are GitHub Copilot per-token prices, checked 2026-10-13. Every subagent session rebuilds its context, so the old pipeline wrote about 115K cache-write tokens per run against about 30K for the single agent. Delegation only pays off when the work is big enough to amortize that. Details: [docs/benchmark-results.md](docs/benchmark-results.md).

## Quick start

**Linux / macOS:**

```bash
curl -sSL https://raw.githubusercontent.com/Racle/agent-team-pipeline/master/scripts/install.sh | bash
```

Or manually:

```bash
git clone https://github.com/Racle/agent-team-pipeline ~/.local/share/agent-team-pipeline
~/.local/share/agent-team-pipeline/scripts/install.sh
```

**Windows (PowerShell):**

```powershell
git clone https://github.com/Racle/agent-team-pipeline $env:LOCALAPPDATA\agent-team-pipeline
& "$env:LOCALAPPDATA\agent-team-pipeline\scripts\install.ps1"
```

**Updating:** `agent-team-update` (Linux/macOS) or `cd $env:LOCALAPPDATA\agent-team-pipeline; git pull; .\scripts\install.ps1` (Windows). The update command does not switch `default_agent`; see the **Migration** note at the end of this file.
**Backups:** the installer backs up an existing `agents/` and `commands/` to `*.bak.<timestamp>`.
**Uninstalling:** `scripts/uninstall.sh` / `scripts\uninstall.ps1`.

> [!IMPORTANT]
> The install script sets `team-captain` as `default_agent`. **Existing users:** re-run the installer or update to pick up the new `team-captain` file. The Architect, Engineer, Forge, Inspector and Shipper agents are gone.

## Agent

`team-captain` is the only agent. It inherits whatever model you select and is not pinned to a provider. Flow: brief plan -> your approval -> minimal diff -> format every changed file + tests -> self-review (security and over-engineering checklist) -> commit only if asked. It works alone by default (one session, one cache) and starts a subagent only when a skill needs it or a job reads far more than it returns. The built-in `explore` subagent runs on Haiku (`github-copilot/claude-haiku-5.5`); the installer sets `agent.explore.model` in your `opencode.json` unless you already have one. Non-Copilot users: change or remove that setting. The system prompt is deliberately short (it is re-sent every session).

**Model recommendation.** Sonnet for everyday work; Haiku for small, well-specified jobs; Opus for unfamiliar or design-heavy work. Keep to one task per session.

**Skills policy.** Skills are loaded sparingly: only when a skill's description clearly matches the current step, once, and never ones that duplicate the agent workflow. Project skills in `.opencode/agents/skills/` are picked up automatically.

## Usage

```
> Add a search feature to the users page
# team-captain: short plan -> you approve -> implement -> format + test -> self-review
```

Ask for a commit explicitly (`... and commit it`); otherwise nothing is committed.

### Commands

- `/format` -- format git-dirty files (or the files you pass).
- `/ponytail-review` -- over-engineering review of current changes; outputs a Delete List.
- `/ponytail-audit` -- same, for the whole repository.

### Behaviour

- Plans are brief; `team-captain` waits for your approval unless your request says to proceed without questions or approval.
- Optional persistent memory (e.g. [Engram](https://github.com/Gentleman-Programming/engram)): used only if `mem_*` tools exist, in 2-3 lines of prompt.

Inspired by [Ponytail](https://github.com/DietrichGebert/ponytail) by Dietrich Gebert (MIT): minimal diff, reuse before writing, no speculative abstractions.

See [docs/DESIGN.md](docs/DESIGN.md) for the design rationale.

## Benchmark

Mean USD per run (15 tasks × 3 reps per setup, hidden tests, 100% pass rate for all):

| Category | Old pipeline (6 agents) | Plain Opus | team-captain (new, single agent; called team-solo during benchmarking) |
| -------- | ----------------------: | ---------: | ---------------------------------------------------------------------: |
| trivial  |                  $0.234 |     $0.145 |                                                                 $0.148 |
| simple   |                  $0.446 |     $0.226 |                                                                 $0.198 |
| standard |                  $0.683 |     $0.393 |                                                                 $0.369 |
| complex  |                  $0.707 |     $0.424 |                                                                 $0.356 |
| hard     |                  $0.668 |     $0.368 |                                                                 $0.335 |
| **All**  |              **$0.548** | **$0.311** |                                                             **$0.281** |

Full tables, method and caveats: [docs/benchmark-results.md](docs/benchmark-results.md). How to run it: [benchmark/README.md](benchmark/README.md).

Reproduce (paid runs): `benchmark/scripts/run-all.sh`

## Changelog

### 2026-10-13 (v6)

**Changed**

- `team-captain`: `task: allow` plus a delegation rule (work alone by default; subagent only when a skill requires it or a job reads far more than it returns).
- Built-in `explore` subagent now runs on Haiku: the installers set `agent.explore.model = github-copilot/claude-haiku-5.5` only if you have none, and the uninstallers remove it only if it is unchanged. Non-Copilot users should change or remove this setting.
- Stricter verify: always format every changed file, even for one-line changes.

**Evidence** (real-repo benchmark on two internal Go+React projects: 117 runs, $14.97; 1-2 reps, planted bugs)

- Delegation was used in 0/31 runs when optional, so the rule costs nothing when unneeded.
- Haiku passed 10/10 at about $0.011/run; Sonnet about $0.11; Opus about $0.30.
- The missed `gofmt` was fixed in 8/8 runs.

### 2026-10-13

**Changed**

- `team-captain` is now a single agent (the former multi-agent orchestrator was replaced). Prompt size: 1056 lines across six agents → 40 in one file.

**Added**

- `benchmark/` cost benchmark (15 tasks, hidden tests) and [docs/benchmark-results.md](docs/benchmark-results.md).

**Removed**

- `team-architect`, `team-engineer`, `team-forge`, `team-inspector`, `team-shipper` (kept for the benchmark in `benchmark/legacy-agents/`).

**Migration**

- None needed for `default_agent` (still `team-captain`); reinstall/update picks up the new file. Remove references to the removed agents from custom configs.
