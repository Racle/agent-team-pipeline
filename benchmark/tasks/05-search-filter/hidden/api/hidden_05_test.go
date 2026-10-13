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

func h05Do(t *testing.T, h http.Handler, method, path, body string, hdr ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func h05Obj(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("response is not a JSON object: %q (%v)", rec.Body.String(), err)
	}
	return m
}

func h05Arr(t *testing.T, rec *httptest.ResponseRecorder) []map[string]any {
	t.Helper()
	var a []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &a); err != nil {
		t.Fatalf("response is not a JSON array: %q (%v)", rec.Body.String(), err)
	}
	return a
}

var _ = h05Obj
var _ = h05Arr

func TestHidden05SearchFilter(t *testing.T) {
	h := NewServer()
	for _, b := range []string{
		`{"title":"Buy Milk","body":"2 litres"}`,
		`{"title":"Call Bob","body":"about the MILKING shed"}`,
		`{"title":"Gym","body":"legs"}`,
	} {
		if rec := h05Do(t, h, "POST", "/api/notes", b); rec.Code != 201 {
			t.Fatalf("create: %d", rec.Code)
		}
	}
	titles := func(path string) []string {
		rec := h05Do(t, h, "GET", path, "")
		if rec.Code != 200 {
			t.Fatalf("%s: %d", path, rec.Code)
		}
		var out []string
		for _, n := range h05Arr(t, rec) {
			out = append(out, n["title"].(string))
		}
		return out
	}
	if got := titles("/api/notes?q=milk"); strings.Join(got, "|") != "Buy Milk|Call Bob" {
		t.Fatalf("q=milk: %v", got)
	}
	if got := titles("/api/notes?q=LEGS"); strings.Join(got, "|") != "Gym" {
		t.Fatalf("q=LEGS: %v", got)
	}
	if got := titles("/api/notes?q=zzz"); len(got) != 0 {
		t.Fatalf("q=zzz: %v", got)
	}
	if got := titles("/api/notes?q="); len(got) != 3 {
		t.Fatalf("empty q should return all: %v", got)
	}
	if got := titles("/api/notes"); len(got) != 3 {
		t.Fatalf("no q should return all: %v", got)
	}
	// no-match result must be an empty JSON array, not null
	if body := strings.TrimSpace(h05Do(t, h, "GET", "/api/notes?q=zzz", "").Body.String()); body != "[]" {
		t.Fatalf("body = %q, want []", body)
	}
}
