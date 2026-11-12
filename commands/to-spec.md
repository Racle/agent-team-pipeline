---
description: Turn the agreed plan into a spec with ordered tickets, for work spanning several sessions
---

Write a spec from what this conversation already agreed (usually after `/grill`). Do not interview; if a decision is truly missing, ask it with the `question` tool, recommended option first. Extra input: $ARGUMENTS

Save it to `docs/specs/<slug>.md` (or the repo's existing spec location):

- **Goal**: one or two sentences, and what is out of scope.
- **Decisions**: the agreed choices, one line each with the reason.
- **Acceptance criteria**: checkable behaviour, each one testable.
- **Tickets**: numbered vertical slices (each one testable and shippable alone, smallest useful first). Per ticket: what changes, files likely touched, `Blocked by: #n` or none, done when.
- **Open questions**: anything deferred.

Keep it short; it is a handoff to a fresh session, not documentation. Don't write code. Finish by telling the user to start ticket 1 in a new session with: "Implement ticket 1 of docs/specs/<slug>.md".
