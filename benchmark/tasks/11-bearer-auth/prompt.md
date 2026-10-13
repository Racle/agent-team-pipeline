Add optional bearer-token authentication to the notes API (`api/`) and a token login to the web UI (`web/`).

API contract (tested against `NewServer()` with `API_TOKEN` set via the environment):

- `NewServer()` reads the `API_TOKEN` environment variable at call time. When it is unset/empty, nothing changes (no auth).
- When set, every `/api/*` request must carry the header `Authorization: Bearer <API_TOKEN>` (scheme `Bearer`, exact token match). Otherwise respond `401` with JSON `{"error":"unauthorized"}` and the response header `WWW-Authenticate: Bearer`. This applies before any routing/validation, to all methods and paths (including ids that do not exist).
- A valid token behaves exactly as today (e.g. an unknown note id is still `404`).
- Compare tokens in constant time.

UI contract (tested with `render(<App />)`, a mocked `fetch` and jsdom `localStorage`):

- The token is stored in `localStorage` under the key `api_token`.
- If no token is stored, `App` renders a text input labelled `API token` and a button named `Log in`, and makes NO `fetch` calls. Submitting a token stores it, hides the login form, and loads the notes.
- Every API request sends the header `Authorization: Bearer <token>` (set via `headers`, plain object or `Headers`).
- A `401` response from the API removes the stored token and shows the login form again.
- While logged in a button named `Log out` is shown; clicking it removes the token and shows the login form.
- The note form's submit button keeps its name `Add note`.

Add tests for the new behaviour and keep the existing ones passing.
