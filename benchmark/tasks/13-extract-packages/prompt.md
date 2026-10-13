Refactor the Go API (`api/`) into packages, with **no behaviour change**: every HTTP response (status, headers, JSON, error messages) must stay byte-for-byte identical.

Required package layout (module path is `notesapi`):

- `notesapi/internal/store` — everything now in `store.go`: `Note`, `Store`, `NewStore()` and the methods `List`, `Get`, `Create`, `Update`, `Delete` with unchanged signatures and semantics. This package must not import `net/http` or `notesapi/internal/validate`.
- `notesapi/internal/validate` — request validation helpers extracted from `server.go`:
  - `type NoteInput struct { Title string; Body string }` with JSON tags `title` and `body`.
  - `func DecodeNoteInput(r io.Reader) (NoteInput, error)` — returns an error when the JSON is invalid (the handler then answers `400` `{"error":"invalid JSON body"}` as today).
  - `func ParseID(s string) (int, error)` — returns an error for a non-integer id (the handler then answers `400` `{"error":"invalid id"}` as today).
    This package must not import `net/http` or `notesapi/internal/store`.
- Package `main` (`api/`) keeps only HTTP wiring: `main.go` and `server.go`. It must no longer declare `Note`, `Store`, `NewStore` or `noteInput`. `store.go` is removed from `api/`.
- `func NewServer() http.Handler` keeps working as before, and a new `func NewServerWithStore(st *store.Store) http.Handler` builds the same handler on top of an existing store (so a test can inspect the store the handlers write to). `NewServer()` is `NewServerWithStore(store.NewStore())`.

Move/adapt the existing tests so they still pass (the existing `server_test.go` references `Note`; update it). Add unit tests for the two new packages. `make test` (`go vet` + `go test`) must be green.
