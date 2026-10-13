Add pagination to the notes list, in both the API (`api/`) and the web UI (`web/`).

API contract for `GET /api/notes`:

- Optional query params `limit` (integer >= 1) and `offset` (integer >= 0, default 0). Without `limit` all notes (from `offset`) are returned, so existing behaviour is unchanged.
- Notes stay ordered by `id` ascending; the response body is still a JSON array (`[]` when the page is empty, never `null`).
- Every list response carries the header `X-Total-Count` with the total number of notes (before pagination).
- A non-integer, negative `offset`, or `limit` < 1 -> `400` with the usual JSON error body.

UI contract (tested with `render(<App />)` and a mocked `fetch`):

- The list is loaded with `GET /api/notes?limit=5&offset=<n>` (page size is fixed at 5).
- Buttons with accessible names `Previous` and `Next` change the page by 5. `Previous` is disabled on the first page; `Next` is disabled when `offset + 5 >= X-Total-Count`.
- The text `Page <current> of <total>` (e.g. `Page 2 of 3`) is displayed, where total = ceil(X-Total-Count / 5).
- Creating or deleting a note refreshes the current page.

Add tests for the new behaviour and keep the existing ones passing.
