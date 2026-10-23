---
description: Grill me on a plan and record GLOSSARY.md/ADRs (Matt Pocock skill)
---

Load the `grill-with-docs` skill with the skill tool and follow it. Arguments: $ARGUMENTS
Ask every question round with the `question` tool: one entry per question, your recommended option first, labelled "(Recommended)". Do not write questions as a markdown list.
When the interview is done, summarise the agreed decisions, then ask with the `question` tool whether to proceed to a plan or `/implement`.

If the skill is not found, stop and tell the user to install Matt Pocock's skills: `npx skills@latest add mattpocock/skills`.
