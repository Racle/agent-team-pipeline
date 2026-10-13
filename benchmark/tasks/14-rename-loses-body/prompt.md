Bug report: "When I rename a note in the app, its body text disappears."

The web UI currently has no rename flow (the note list only has `Delete`), and the API's `PUT /api/notes/{id}` replaces the whole note. Fix this properly, in both `api/` and `web/`.

API contract (`PUT /api/notes/{id}`):

- The JSON body may contain `title` and/or `body`. A field that is **absent or `null`** keeps the note's current value. A field that is present (including the empty string `""`) replaces it. `{}` is valid, changes nothing and answers `200` with the note.
- `created_at` never changes. Invalid JSON is still `400` `{"error":"invalid JSON body"}`; an unknown id is still `404`; a non-integer id is still `400`.
- Response is the full updated note, `200`.

UI contract (tested with `render(<App />)` and a mocked `fetch`):

- Each note has a button named `Rename`. Clicking it reveals a textbox with accessible name `New title` (prefilled with the current title) and a button named `Confirm`.
- Clicking `Confirm` sends `fetch("/api/notes/<id>", { method: "PUT", headers: {"Content-Type": "application/json"}, body: JSON.stringify({title}) })` — **only the title is sent** — then the list shows the server's updated note (title changed, body intact). The rename box closes on success.
- If the request fails (non-2xx or network error) the old title and body stay displayed, an element with `role="alert"` and text containing `rename failed` (case-insensitive) is shown, and the rename box stays open so the user can retry.
- The note form (`Add note`) and `Delete` keep working.

Add tests for the new behaviour and keep the existing ones passing.
