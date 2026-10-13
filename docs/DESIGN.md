# Design & Architecture

## Summary

`team-captain` is a single agent that inherits your default model. The design goal is **minimum LLM spend at equal pass rate on these tasks**.

## Why: the benchmark

`benchmark/` runs the same tasks (15 tasks across 5 categories, 3 reps, judged by hidden tests) through the old 6-agent pipeline (Captain, Architect, Engineer, Forge, Inspector, Shipper), a plain Opus agent and the new single-agent `team-captain` (called `team-solo` during benchmarking). Details: [benchmark-results.md](benchmark-results.md).

| Result                 | Old pipeline (6 agents) | Plain Opus | team-captain (new, single agent; called team-solo during benchmarking) |
| ---------------------- | ----------------------- | ---------- | ---------------------------------------------------------------------- |
| Hidden-test pass rate  | 45/45                   | 45/45      | 45/45                                                                  |
| Mean cost per run      | $0.548                  | $0.311     | $0.281                                                                 |
| Cache-write tokens/run | ~115K                   | ~32K       | ~30K                                                                   |

The cost gap is driven by cache writes (A/C = 1.95x): each subagent starts a new session, so its system prompt, tool definitions and the context handed to it are written to the cache again, and the Captain's session grows with every report. No measurable quality difference (all runs passed), so the extra structure bought nothing measurable on these small and medium tasks.

## Decisions

- **Single agent.** One session, one cache. Flow: brief plan -> user approval -> minimal diff -> format + tests -> self-review (security and over-engineering checklist) -> commit only if asked. The approval gate is skipped when the request explicitly says to proceed without questions or approval (needed for headless runs).
- **No delegation.** Architect, Engineer, Forge, Inspector and Shipper were removed: each extra session rewrites its context to the cache, which the benchmark showed is the main cost driver. Formatting, tests, self-review and git rules live in the one agent.
- **Short prompt.** The system prompt is re-sent every session, so the agent file is about 40 lines. No resume files, budgets, or efficiency reports.
- **Language agnostic.** Prompts use universal concepts (formatter, test runner); project skills and config carry technology specifics.
- **Sparing skill use.** Agents load a skill only when its description clearly matches the step, once, and skip workflow-orchestration skills that duplicate the prompt; loading costs tokens.
- **Optional memory.** If Engram-style `mem_*` tools exist, the agent saves decisions/bugfixes and a session summary; otherwise nothing changes.

## Benchmark v2 (real repos)

v6 was chosen from a second benchmark on two internal Go+React projects: 117 runs, $14.97 total. Delegation was used in 0/31 runs when optional. Haiku passed 10/10 at about $0.011/run; Sonnet cost about $0.11 and Opus about $0.30. The v6 format rule fixed the missed `gofmt` in 8/8 runs, and `explore` runs on Haiku. Caveats: 1-2 reps per cell, planted bugs, peak context only 19-44K tokens (too small to show delegation savings).

## Model

`team-captain` inherits your default model and is not pinned, so it stays provider-agnostic.

## Migration

Nothing to do for `default_agent` (still `team-captain`); re-run the installer or update. References to `team-architect`, `team-engineer`, `team-forge`, `team-inspector`, `team-shipper` must be removed from custom configs.

## Ponytail

The ponytail rules ("least code that works": need it? reuse codebase? stdlib? one line? minimum new code) are condensed in the agent prompt rather than shared: agent markdown has no include mechanism and `install.sh` only symlinks `agents/` and `commands/`.
