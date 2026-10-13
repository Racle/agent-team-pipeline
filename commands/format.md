---
description: Format all modified files using detected project formatters
subtask: true
---

Only format; do NOT build or run tests.

Detect and run all applicable formatters on the git-dirty files in this project. If $ARGUMENTS is provided, format only those specific files instead of all dirty files.

After formatting, report which formatters ran and what files were changed.
