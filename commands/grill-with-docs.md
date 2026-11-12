---
description: Grill me on a plan and record the glossary and decisions (ADRs) as we go
---

Grill the user on this plan or idea: $ARGUMENTS (if empty, the plan in this conversation).

- Map it as a design tree: each decision branches into the decisions that depend on it. The frontier is every open decision whose prerequisites are settled.
- Each round, ask the whole frontier in one `question` tool call: one entry per decision, your recommended option first, labelled "(Recommended)", phrased so accepting it is one click. Never ask as a markdown list. Then wait.
- Facts are your job: read code, docs and git history yourself (an `explore` subagent for wide searches) instead of asking. Decisions are the user's.
- Challenge weak answers once (edge cases, cost, simpler alternative), then accept the user's call. Flag terms used inconsistently with the code or `GLOSSARY.md`.
- Stop when the frontier is empty: no branch left, nothing silently assumed.

Docs (the only files you edit; follow existing locations if the repo has them):

- `GLOSSARY.md`: when a domain term is settled, add or fix one line: **Term**: meaning; avoid: synonyms.
- `docs/adr/NNNN-<slug>.md`: only for decisions that are hard to reverse, surprising, or chose between real alternatives. Sections: Context, Decision, Consequences; a few lines each.

Finish with a short summary of the agreed decisions and the docs touched, then ask with the `question` tool: implement now (small) / write a spec with `/to-spec` (multi-session) / keep grilling.
