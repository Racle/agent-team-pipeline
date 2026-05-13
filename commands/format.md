---
description: Format all modified files using detected project formatters
agent: team-forge
subtask: true
model: github-copilot/claude-sonnet-4.6
---

Run **only Phase 1 (Format)** from your instructions. Do NOT run Phase 2 (Build) or Phase 3 (Test).

Detect and run all applicable formatters on the git-dirty files in this project. If $ARGUMENTS is provided, format only those specific files instead of all dirty files.

After formatting, report which formatters ran and what files were changed. Use the standard Format Results output format.
