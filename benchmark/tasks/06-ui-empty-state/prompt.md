In the web UI (`web/`), show an empty state when the loaded notes list is empty.

Contract (tested with `render(<App />)` and a mocked `fetch` for `GET /api/notes`):

- When the API returns `[]`, the page shows the exact text `No notes yet` and renders no list items (`role="listitem"`).
- When the API returns one or more notes, the text `No notes yet` is not present.

Add a test and keep the existing ones passing.
