Add validation to the notes API: a note must have a non-blank title.

Contract:

- `POST /api/notes` and `PUT /api/notes/{id}` with a `title` that is empty or contains only whitespace respond `400` with JSON body `{"error":"title is required"}`.
- A rejected request must not create or change any note.
- Requests with a non-blank title behave exactly as before (`201` on create, `200` on update).

Add tests for the new behaviour and keep the existing ones passing.
