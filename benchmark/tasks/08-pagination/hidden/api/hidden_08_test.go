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

func h08Do(t *testing.T, h http.Handler, method, path, body string, hdr ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func h08Obj(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("response is not a JSON object: %q (%v)", rec.Body.String(), err)
	}
	return m
}

func h08Arr(t *testing.T, rec *httptest.ResponseRecorder) []map[string]any {
	t.Helper()
	var a []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &a); err != nil {
		t.Fatalf("response is not a JSON array: %q (%v)", rec.Body.String(), err)
	}
	return a
}

var _ = h08Obj
var _ = h08Arr

func TestHidden08Pagination(t *testing.T) {
	h := NewServer()
	for i := 1; i <= 7; i++ {
		b, _ := json.Marshal(map[string]string{"title": "n" + string(rune('0'+i)), "body": ""})
		h08Do(t, h, "POST", "/api/notes", string(b))
	}
	ids := func(path string) ([]int, *httptest.ResponseRecorder) {
		rec := h08Do(t, h, "GET", path, "")
		if rec.Code != 200 {
			t.Fatalf("%s: %d %s", path, rec.Code, rec.Body.String())
		}
		var out []int
		for _, n := range h08Arr(t, rec) {
			out = append(out, int(n["id"].(float64)))
		}
		return out, rec
	}
	eq := func(a, b []int) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}
	got, rec := ids("/api/notes?limit=3&offset=0")
	if !eq(got, []int{1, 2, 3}) || rec.Header().Get("X-Total-Count") != "7" {
		t.Fatalf("page1: %v total=%q", got, rec.Header().Get("X-Total-Count"))
	}
	if got, _ := ids("/api/notes?limit=3&offset=3"); !eq(got, []int{4, 5, 6}) {
		t.Fatalf("page2: %v", got)
	}
	got, rec = ids("/api/notes?limit=3&offset=6")
	if !eq(got, []int{7}) || rec.Header().Get("X-Total-Count") != "7" {
		t.Fatalf("page3: %v", got)
	}
	got, rec = ids("/api/notes?limit=3&offset=50")
	if len(got) != 0 || strings.TrimSpace(rec.Body.String()) != "[]" || rec.Header().Get("X-Total-Count") != "7" {
		t.Fatalf("beyond end: %v %q", got, rec.Body.String())
	}
	if got, _ := ids("/api/notes?limit=2"); !eq(got, []int{1, 2}) {
		t.Fatalf("limit only: %v", got)
	}
	got, rec = ids("/api/notes")
	if len(got) != 7 || rec.Header().Get("X-Total-Count") != "7" {
		t.Fatalf("no params: %v total=%q", got, rec.Header().Get("X-Total-Count"))
	}
}

func TestHidden08InvalidParams(t *testing.T) {
	h := NewServer()
	for _, q := range []string{"limit=abc", "limit=-1", "limit=0", "offset=-1", "offset=x"} {
		if rec := h08Do(t, h, "GET", "/api/notes?"+q, ""); rec.Code != 400 {
			t.Fatalf("%s: got %d want 400", q, rec.Code)
		}
	}
}
