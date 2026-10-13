The notes API server currently always listens on port 8080 (see `api/main.go`).

Make the listen port configurable through the `PORT` environment variable:

- When `PORT` is set (e.g. `PORT=9123`), the server built from `api/` (`go build .`) listens on that port, so `GET http://127.0.0.1:9123/api/notes` returns `200`.
- When `PORT` is unset or empty, it keeps listening on `8080`.

Keep all existing tests passing.
