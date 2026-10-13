Make the notes API persist its data to a JSON file so notes survive a restart.

Contract (tested against `NewServer()` with `DATA_FILE` set via the environment):

- `NewServer()` reads the `DATA_FILE` environment variable at call time. When it is set (non-empty), the server uses that path as its data file; when unset/empty it stays purely in-memory exactly as today (separate `NewServer()` instances share nothing).
- On construction, if the file exists its notes are loaded; if it does not exist the server starts empty (no error). The file is created on the first write.
- Every mutation (`POST`, `PUT`, `DELETE`) is written to the file before the response is sent. The file always contains valid JSON.
- Writes must be atomic: write to a temporary file in the same directory and `rename` it over `DATA_FILE`. No temporary files may be left behind in the directory after a request completes (the directory must contain only the data file).
- After loading, newly created notes get an `id` greater than every existing note's `id`.
- Concurrent requests must remain safe (the existing mutex discipline applies to file writes too).

The exact file format is up to you. Add tests for the new behaviour and keep the existing ones passing.
