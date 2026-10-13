Add a "done" flag to notes, in both the API (`api/`) and the web UI (`web/`).

API contract:

- Every note JSON object has a boolean field `done`, `false` for new notes (also in list responses).
- New endpoint `PATCH /api/notes/{id}` with JSON body `{"done": <bool>}` updates only the `done` flag and returns `200` with the full updated note JSON. Unknown id -> `404`, malformed JSON or non-numeric id -> `400` (same JSON error shape as other endpoints).
- `PUT /api/notes/{id}` must leave `done` unchanged.

UI contract (tested with `render(<App />)` and a mocked `fetch`):

- Each note has a checkbox whose accessible name is `Done: <title>` (e.g. `Done: Alpha`), checked when `done` is true.
- Toggling it calls `fetch("/api/notes/<id>", { method: "PATCH", headers: {"Content-Type": "application/json"}, body: JSON.stringify({done: <new value>}) })`, then refreshes the list from `GET /api/notes`.

Add tests for the new behaviour and keep the existing ones passing.
