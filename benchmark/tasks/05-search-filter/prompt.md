Add search to the notes list endpoint.

Contract for `GET /api/notes?q=<text>`:

- Returns only notes whose `title` OR `body` contains `<text>` as a case-insensitive substring.
- Results stay ordered by `id` ascending and the response is still a JSON array (`[]` when nothing matches, never `null`).
- A missing or empty `q` returns all notes, as today.

Add tests for the new behaviour and keep the existing ones passing.
