---
description: Over-engineering review (ponytail) of current changes; outputs a Delete List
agent: team-inspector
subtask: true
---

This is ponytail-only mode: run **only Part 3 (Over-Engineering Pass)**. Skip Parts 1-2. Skip P5 if no approved plan exists.

Review the current changes (staged + unstaged) in this diff:

!`git diff HEAD`

Also inspect these untracked files with read/glob (they are absent from `git diff HEAD`):

!`git ls-files --others --exclude-standard`

For a whole-repository review, use `/ponytail-audit` instead.

Output only the Delete List; write "Delete List: None" if empty. Read-only -- do not modify files.
