---
description: Over-engineering review (ponytail) of current changes; outputs a Delete List
agent: team-captain
subtask: true
---

This is ponytail-only mode: run **only the Over-Engineering checklist**; skip Security and Correctness.

Review the current changes (staged + unstaged) in this diff:

!`git diff HEAD`

Also inspect these untracked files with read/glob (they are absent from `git diff HEAD`):

!`git ls-files --others --exclude-standard`

For a whole-repository review, use `/ponytail-audit` instead.

Output only a Delete List (`file:line -- what to delete or simplify`); write "Delete List: None" if empty. Read-only -- do not modify files.
