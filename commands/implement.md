---
description: Implement a ticket, spec or agreed plan test-first (Matt Pocock skill)
---

Load the `implement` skill with the skill tool and follow it. Arguments: $ARGUMENTS
Do not commit: if the skill says to commit, stop instead, summarise the diff and ask with the `question` tool whether to commit.
When running `code-review`, review the uncommitted working tree (`git diff <base>`, not `<base>...HEAD`), since nothing is committed yet.

If the skill is not found, stop and tell the user to install Matt Pocock's skills: `npx skills@latest add mattpocock/skills`.
