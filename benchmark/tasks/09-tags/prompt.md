Add tags to notes, in both the API (`api/`) and the web UI (`web/`).

API contract:

- Every note JSON object has `tags`: an array of strings, always present (`[]` when none, never `null`).
- `POST /api/notes` and `PUT /api/notes/{id}` accept an optional `tags` array; `PUT` replaces the stored tags (omitted -> `[]`).
- `GET /api/notes?tag=<name>` returns only notes whose tags contain exactly `<name>` (case-sensitive), ordered by `id`; no match -> `[]`. Without `tag` all notes are returned as before.

UI contract (tested with `render(<App />)` and a mocked `fetch`):

- Each note renders each of its tags as a button with accessible name `Tag: <name>` (e.g. `Tag: work`). Clicking it reloads the list via `GET /api/notes?tag=<name>` (URL-encoded) so only matching notes remain.
- The note form has an additional text input labelled `Tags` taking a comma-separated list; on submit the `POST /api/notes` JSON body includes `tags` as an array of trimmed, non-empty strings (e.g. `"a, b"` -> `["a","b"]`). The submit button keeps its name `Add note`.

Add tests for the new behaviour and keep the existing ones passing.
