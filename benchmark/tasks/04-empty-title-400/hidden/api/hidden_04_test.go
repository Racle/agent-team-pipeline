package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var _ = strings.NewReader
var _ = json.Marshal

func h04Do(t *testing.T, h http.Handler, method, path, body string, hdr ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func h04Obj(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("response is not a JSON object: %q (%v)", rec.Body.String(), err)
	}
	return m
}

func h04Arr(t *testing.T, rec *httptest.ResponseRecorder) []map[string]any {
	t.Helper()
	var a []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &a); err != nil {
		t.Fatalf("response is not a JSON array: %q (%v)", rec.Body.String(), err)
	}
	return a
}

var _ = h04Obj
var _ = h04Arr

func TestHidden04EmptyTitleRejected(t *testing.T) {
	h := NewServer()
	for _, title := range []string{"", "   ", "\t\n"} {
		body, _ := json.Marshal(map[string]string{"title": title, "body": "x"})
		rec := h04Do(t, h, "POST", "/api/notes", string(body))
		if rec.Code != 400 {
			t.Fatalf("POST title=%q: got %d, want 400", title, rec.Code)
		}
		if got := h04Obj(t, rec)["error"]; got != "title is required" {
			t.Fatalf("POST error = %v", got)
		}
	}
	if n := len(h04Arr(t, h04Do(t, h, "GET", "/api/notes", ""))); n != 0 {
		t.Fatalf("rejected notes must not be stored, have %d", n)
	}
}

func TestHidden04UpdateEmptyTitleRejected(t *testing.T) {
	h := NewServer()
	if rec := h04Do(t, h, "POST", "/api/notes", `{"title":"keep","body":"b"}`); rec.Code != 201 {
		t.Fatalf("valid create: %d", rec.Code)
	}
	rec := h04Do(t, h, "PUT", "/api/notes/1", `{"title":"  ","body":"b"}`)
	if rec.Code != 400 || h04Obj(t, rec)["error"] != "title is required" {
		t.Fatalf("PUT: %d %s", rec.Code, rec.Body.String())
	}
	if got := h04Obj(t, h04Do(t, h, "GET", "/api/notes/1", ""))["title"]; got != "keep" {
		t.Fatalf("title changed to %v", got)
	}
}
