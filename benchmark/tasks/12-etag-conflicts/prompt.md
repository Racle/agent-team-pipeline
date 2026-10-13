Add optimistic concurrency control to note updates (`api/`) and an edit flow with optimistic UI and rollback (`web/`).

API contract:

- Every note JSON object has an integer `version`: `1` when created, incremented by 1 on every successful `PUT`. It appears in list, get, create and update responses.
- `GET /api/notes/{id}`, `POST /api/notes` and `PUT /api/notes/{id}` responses carry the header `ETag: "<version>"` — the version in double quotes, e.g. `"1"`.
- `PUT /api/notes/{id}` honours an optional `If-Match` request header. If present and not equal to the note's current ETag, respond `409` with JSON `{"error": "<non-empty message>"}` and change nothing. If absent, the update proceeds unconditionally (and still bumps the version). A missing note stays `404`.
- The compare-and-update must be atomic: with concurrent `PUT`s carrying the same `If-Match`, exactly one succeeds and the others get `409`.

UI contract (tested with `render(<App />)` and a mocked `fetch`; list items contain `version`):

- Each note has a button named `Edit`. Clicking it reveals a textbox with accessible name `Edit title` (prefilled with the title) and a button named `Save`.
- Clicking `Save` immediately (optimistically, before the request settles) displays the new title in the list and sends `fetch("/api/notes/<id>", { method: "PUT", headers: {"Content-Type": "application/json", "If-Match": "\"<version>\""}, body: JSON.stringify({title, body}) })`, using the note's current `version`.
- On success the list reflects the server's response to the `PUT` (the updated note, including its new `version`; refetching the list afterwards is also fine). A later edit must use the new `version` in `If-Match`.
- On any failure (HTTP 409 or other error status/network error) the note's title is rolled back to its previous value and a toast with `role="alert"` is shown. For 409 the toast text contains the word `conflict` (case-insensitive); for other failures any message.
- The note form's submit button keeps its name `Add note`.

Add tests for the new behaviour and keep the existing ones passing.
