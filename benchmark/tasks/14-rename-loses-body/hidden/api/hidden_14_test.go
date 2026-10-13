package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func h14Do(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func h14Note(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("not a JSON object: %q", rec.Body.String())
	}
	return m
}

func TestHidden14PartialPut(t *testing.T) {
	h := NewServer()
	created := h14Note(t, h14Do(h, "POST", "/api/notes", `{"title":"a","body":"keep me"}`))

	rec := h14Do(h, "PUT", "/api/notes/1", `{"title":"renamed"}`)
	m := h14Note(t, rec)
	if rec.Code != 200 || m["title"] != "renamed" || m["body"] != "keep me" {
		t.Fatalf("title-only PUT: %d %v", rec.Code, m)
	}
	if m["created_at"] != created["created_at"] {
		t.Errorf("created_at changed: %v -> %v", created["created_at"], m["created_at"])
	}

	m = h14Note(t, h14Do(h, "PUT", "/api/notes/1", `{"body":"new body"}`))
	if m["title"] != "renamed" || m["body"] != "new body" {
		t.Fatalf("body-only PUT: %v", m)
	}

	m = h14Note(t, h14Do(h, "PUT", "/api/notes/1", `{"title":null,"body":null}`))
	if m["title"] != "renamed" || m["body"] != "new body" {
		t.Fatalf("null fields must keep values: %v", m)
	}

	rec = h14Do(h, "PUT", "/api/notes/1", `{}`)
	m = h14Note(t, rec)
	if rec.Code != 200 || m["title"] != "renamed" || m["body"] != "new body" {
		t.Fatalf("empty PUT: %d %v", rec.Code, m)
	}

	m = h14Note(t, h14Do(h, "PUT", "/api/notes/1", `{"body":""}`))
	if m["body"] != "" || m["title"] != "renamed" {
		t.Fatalf("explicit empty body must clear it: %v", m)
	}
	m = h14Note(t, h14Do(h, "PUT", "/api/notes/1", `{"title":"","body":"x"}`))
	if m["title"] != "" || m["body"] != "x" {
		t.Fatalf("explicit empty title must replace it: %v", m)
	}

	// persisted, not just echoed
	m = h14Note(t, h14Do(h, "GET", "/api/notes/1", ""))
	if m["title"] != "" || m["body"] != "x" {
		t.Fatalf("GET after PUT: %v", m)
	}
}

func TestHidden14PutErrors(t *testing.T) {
	h := NewServer()
	h14Do(h, "POST", "/api/notes", `{"title":"a","body":"b"}`)
	if rec := h14Do(h, "PUT", "/api/notes/1", `{bad`); rec.Code != 400 {
		t.Errorf("bad json: %d", rec.Code)
	}
	if rec := h14Do(h, "PUT", "/api/notes/9", `{"title":"x"}`); rec.Code != 404 {
		t.Errorf("missing: %d", rec.Code)
	}
	if rec := h14Do(h, "PUT", "/api/notes/abc", `{}`); rec.Code != 400 {
		t.Errorf("bad id: %d", rec.Code)
	}
	// a failed PUT changes nothing
	if m := h14Note(t, h14Do(h, "GET", "/api/notes/1", "")); m["title"] != "a" || m["body"] != "b" {
		t.Errorf("note changed by failed PUT: %v", m)
	}
}
