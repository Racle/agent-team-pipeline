package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

var _ = strings.NewReader
var _ = json.Marshal

func h12Do(t *testing.T, h http.Handler, method, path, body string, hdr ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func h12Obj(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("response is not a JSON object: %q (%v)", rec.Body.String(), err)
	}
	return m
}

func h12Arr(t *testing.T, rec *httptest.ResponseRecorder) []map[string]any {
	t.Helper()
	var a []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &a); err != nil {
		t.Fatalf("response is not a JSON array: %q (%v)", rec.Body.String(), err)
	}
	return a
}

var _ = h12Obj
var _ = h12Arr

func TestHidden12VersionAndETag(t *testing.T) {
	h := NewServer()
	rec := h12Do(t, h, "POST", "/api/notes", `{"title":"a","body":""}`)
	if rec.Code != 201 || h12Obj(t, rec)["version"] != float64(1) {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("ETag"); got != `"1"` {
		t.Fatalf("POST ETag = %q", got)
	}
	rec = h12Do(t, h, "GET", "/api/notes/1", "")
	if rec.Header().Get("ETag") != `"1"` || h12Obj(t, rec)["version"] != float64(1) {
		t.Fatalf("GET: etag=%q body=%s", rec.Header().Get("ETag"), rec.Body.String())
	}
	if l := h12Arr(t, h12Do(t, h, "GET", "/api/notes", "")); len(l) != 1 || l[0]["version"] != float64(1) {
		t.Fatalf("list: %v", l)
	}
}

func TestHidden12IfMatch(t *testing.T) {
	h := NewServer()
	h12Do(t, h, "POST", "/api/notes", `{"title":"a","body":""}`)

	rec := h12Do(t, h, "PUT", "/api/notes/1", `{"title":"b","body":""}`, "If-Match", `"1"`)
	if rec.Code != 200 || rec.Header().Get("ETag") != `"2"` || h12Obj(t, rec)["version"] != float64(2) {
		t.Fatalf("matching PUT: %d etag=%q %s", rec.Code, rec.Header().Get("ETag"), rec.Body.String())
	}
	// stale
	rec = h12Do(t, h, "PUT", "/api/notes/1", `{"title":"stale","body":""}`, "If-Match", `"1"`)
	if rec.Code != 409 {
		t.Fatalf("stale PUT: got %d want 409", rec.Code)
	}
	if e, _ := h12Obj(t, rec)["error"].(string); e == "" {
		t.Fatalf("409 needs a JSON error body: %s", rec.Body.String())
	}
	// unchanged by the failed attempt
	rec = h12Do(t, h, "GET", "/api/notes/1", "")
	if h12Obj(t, rec)["title"] != "b" || rec.Header().Get("ETag") != `"2"` {
		t.Fatalf("state after 409: %s etag=%q", rec.Body.String(), rec.Header().Get("ETag"))
	}
	// no If-Match -> unconditional, still bumps the version
	rec = h12Do(t, h, "PUT", "/api/notes/1", `{"title":"c","body":""}`)
	if rec.Code != 200 || rec.Header().Get("ETag") != `"3"` {
		t.Fatalf("unconditional PUT: %d etag=%q", rec.Code, rec.Header().Get("ETag"))
	}
	// If-Match on a missing note is still 404
	if rec := h12Do(t, h, "PUT", "/api/notes/77", `{"title":"c","body":""}`, "If-Match", `"1"`); rec.Code != 404 {
		t.Fatalf("missing: %d", rec.Code)
	}
}

func TestHidden12ConcurrentWritersExactlyOneWins(t *testing.T) {
	h := NewServer()
	h12Do(t, h, "POST", "/api/notes", `{"title":"a","body":""}`)
	var wg sync.WaitGroup
	codes := make([]int, 8)
	for i := range codes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes[i] = h12Do(t, h, "PUT", "/api/notes/1", `{"title":"x","body":""}`, "If-Match", `"1"`).Code
		}()
	}
	wg.Wait()
	ok, conflict := 0, 0
	for _, c := range codes {
		switch c {
		case 200:
			ok++
		case 409:
			conflict++
		}
	}
	if ok != 1 || conflict != 7 {
		t.Fatalf("codes = %v, want exactly one 200 and seven 409", codes)
	}
}
