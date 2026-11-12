---
description: Grill me on a plan until we share one understanding, then pick the next step
---

Grill the user on this plan or idea: $ARGUMENTS (if empty, the plan in this conversation).

- Map it as a design tree: each decision branches into the decisions that depend on it. The frontier is every open decision whose prerequisites are settled.
- Each round, ask the whole frontier in one `question` tool call: one entry per decision, your recommended option first, labelled "(Recommended)", phrased so accepting it is one click. Never ask as a markdown list. Then wait.
- Facts are your job: read code, docs and git history yourself (an `explore` subagent for wide searches) instead of asking. Decisions are the user's.
- Challenge weak answers once (edge cases, cost, simpler alternative), then accept the user's call.
- Stop when the frontier is empty: no branch left, nothing silently assumed. Do not edit files during grilling.

Finish with a short summary of the agreed decisions, then ask with the `question` tool: implement now (small) / write a spec with `/to-spec` (multi-session) / keep grilling.
