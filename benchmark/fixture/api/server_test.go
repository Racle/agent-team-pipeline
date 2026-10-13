package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func do(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestCreateAndList(t *testing.T) {
	h := NewServer()
	rec := do(t, h, "POST", "/api/notes", `{"title":"a","body":"b"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: got %d", rec.Code)
	}
	var n Note
	if err := json.Unmarshal(rec.Body.Bytes(), &n); err != nil || n.ID != 1 || n.Title != "a" {
		t.Fatalf("bad note %+v err=%v", n, err)
	}
	rec = do(t, h, "GET", "/api/notes", "")
	var list []Note
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if rec.Code != 200 || len(list) != 1 {
		t.Fatalf("list: %d %v", rec.Code, list)
	}
}

func TestGetUpdateDelete(t *testing.T) {
	h := NewServer()
	do(t, h, "POST", "/api/notes", `{"title":"a","body":"b"}`)
	if rec := do(t, h, "GET", "/api/notes/1", ""); rec.Code != 200 {
		t.Fatalf("get: %d", rec.Code)
	}
	rec := do(t, h, "PUT", "/api/notes/1", `{"title":"c","body":"d"}`)
	var n Note
	_ = json.Unmarshal(rec.Body.Bytes(), &n)
	if rec.Code != 200 || n.Title != "c" {
		t.Fatalf("put: %d %+v", rec.Code, n)
	}
	if rec := do(t, h, "DELETE", "/api/notes/1", ""); rec.Code != 204 {
		t.Fatalf("delete: %d", rec.Code)
	}
	if rec := do(t, h, "GET", "/api/notes/1", ""); rec.Code != 404 {
		t.Fatalf("get after delete: %d", rec.Code)
	}
}

func TestErrors(t *testing.T) {
	h := NewServer()
	if rec := do(t, h, "POST", "/api/notes", `{bad`); rec.Code != 400 {
		t.Fatalf("bad json: %d", rec.Code)
	}
	if rec := do(t, h, "GET", "/api/notes/abc", ""); rec.Code != 400 {
		t.Fatalf("bad id: %d", rec.Code)
	}
	if rec := do(t, h, "PUT", "/api/notes/9", `{"title":"x"}`); rec.Code != 404 {
		t.Fatalf("put missing: %d", rec.Code)
	}
	if rec := do(t, h, "DELETE", "/api/notes/9", ""); rec.Code != 404 {
		t.Fatalf("delete missing: %d", rec.Code)
	}
}
