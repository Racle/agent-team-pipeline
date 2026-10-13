---
description: Over-engineering audit (ponytail) of the whole repository; outputs a Delete List
agent: team-captain
subtask: true
---

This is ponytail-only mode: run **only the Over-Engineering checklist**; skip Security and Correctness. Audit the whole repository, including untracked files, using glob/grep/read, existing code and not only recent changes.

Output only a Delete List (`file:line -- what to delete or simplify`); write "Delete List: None" if empty. Read-only -- do not modify files.
