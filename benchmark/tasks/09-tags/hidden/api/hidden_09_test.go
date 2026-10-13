package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

var _ = strings.NewReader
var _ = json.Marshal

func h09Do(t *testing.T, h http.Handler, method, path, body string, hdr ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func h09Obj(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("response is not a JSON object: %q (%v)", rec.Body.String(), err)
	}
	return m
}

func h09Arr(t *testing.T, rec *httptest.ResponseRecorder) []map[string]any {
	t.Helper()
	var a []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &a); err != nil {
		t.Fatalf("response is not a JSON array: %q (%v)", rec.Body.String(), err)
	}
	return a
}

var _ = h09Obj
var _ = h09Arr

func TestHidden09Tags(t *testing.T) {
	h := NewServer()
	rec := h09Do(t, h, "POST", "/api/notes", `{"title":"a","body":"","tags":["work","urgent"]}`)
	if rec.Code != 201 {
		t.Fatalf("create: %d", rec.Code)
	}
	if got := h09Obj(t, rec)["tags"]; !reflect.DeepEqual(got, []any{"work", "urgent"}) {
		t.Fatalf("tags = %#v", got)
	}
	// omitted tags -> empty array, not null/absent
	rec = h09Do(t, h, "POST", "/api/notes", `{"title":"b","body":""}`)
	if got, ok := h09Obj(t, rec)["tags"]; !ok || !reflect.DeepEqual(got, []any{}) {
		t.Fatalf("default tags = %#v (present=%v)", got, ok)
	}
	h09Do(t, h, "POST", "/api/notes", `{"title":"c","body":"","tags":["home","urgent"]}`)

	titles := func(path string) string {
		rec := h09Do(t, h, "GET", path, "")
		if rec.Code != 200 {
			t.Fatalf("%s: %d", path, rec.Code)
		}
		var out []string
		for _, n := range h09Arr(t, rec) {
			out = append(out, n["title"].(string))
		}
		return strings.Join(out, ",")
	}
	if got := titles("/api/notes?tag=urgent"); got != "a,c" {
		t.Fatalf("tag=urgent: %q", got)
	}
	if got := titles("/api/notes?tag=home"); got != "c" {
		t.Fatalf("tag=home: %q", got)
	}
	if got := titles("/api/notes?tag=nope"); got != "" {
		t.Fatalf("tag=nope: %q", got)
	}
	if body := strings.TrimSpace(h09Do(t, h, "GET", "/api/notes?tag=nope", "").Body.String()); body != "[]" {
		t.Fatalf("empty result body %q", body)
	}
	if got := titles("/api/notes"); got != "a,b,c" {
		t.Fatalf("no filter: %q", got)
	}
	// PUT replaces tags
	rec = h09Do(t, h, "PUT", "/api/notes/1", `{"title":"a","body":"","tags":["only"]}`)
	if got := h09Obj(t, rec)["tags"]; rec.Code != 200 || !reflect.DeepEqual(got, []any{"only"}) {
		t.Fatalf("put: %d %v", rec.Code, got)
	}
	if got := titles("/api/notes?tag=work"); got != "" {
		t.Fatalf("old tag still matches: %q", got)
	}
	if got := h09Obj(t, h09Do(t, h, "GET", "/api/notes/1", ""))["tags"]; !reflect.DeepEqual(got, []any{"only"}) {
		t.Fatalf("get: %v", got)
	}
}
