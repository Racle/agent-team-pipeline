The notes API (the Go module in the `api/` directory) has a spelling mistake in the error message it returns when a note does not exist.

Fix it so that `GET`, `PUT` and `DELETE` on `/api/notes/{id}` for a note that does not exist respond with status `404` and the JSON body `{"error":"note not found"}`.

Keep all existing tests passing.
