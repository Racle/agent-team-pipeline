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

func h07Do(t *testing.T, h http.Handler, method, path, body string, hdr ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func h07Obj(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("response is not a JSON object: %q (%v)", rec.Body.String(), err)
	}
	return m
}

func h07Arr(t *testing.T, rec *httptest.ResponseRecorder) []map[string]any {
	t.Helper()
	var a []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &a); err != nil {
		t.Fatalf("response is not a JSON array: %q (%v)", rec.Body.String(), err)
	}
	return a
}

var _ = h07Obj
var _ = h07Arr

func TestHidden07DoneFlag(t *testing.T) {
	h := NewServer()
	rec := h07Do(t, h, "POST", "/api/notes", `{"title":"a","body":"b"}`)
	if rec.Code != 201 {
		t.Fatalf("create: %d", rec.Code)
	}
	if d, ok := h07Obj(t, rec)["done"]; !ok || d != false {
		t.Fatalf("new note must have done=false, got %v (present=%v)", d, ok)
	}
	rec = h07Do(t, h, "PATCH", "/api/notes/1", `{"done":true}`, "Content-Type", "application/json")
	if rec.Code != 200 {
		t.Fatalf("patch: %d %s", rec.Code, rec.Body.String())
	}
	n := h07Obj(t, rec)
	if n["done"] != true || n["title"] != "a" || n["body"] != "b" {
		t.Fatalf("patch result: %v", n)
	}
	if got := h07Obj(t, h07Do(t, h, "GET", "/api/notes/1", ""))["done"]; got != true {
		t.Fatalf("done not persisted: %v", got)
	}
	// PUT leaves done unchanged
	if rec := h07Do(t, h, "PUT", "/api/notes/1", `{"title":"a2","body":"b2"}`); rec.Code != 200 || h07Obj(t, rec)["done"] != true {
		t.Fatalf("PUT must keep done: %d %s", rec.Code, rec.Body.String())
	}
	// toggle back
	if rec := h07Do(t, h, "PATCH", "/api/notes/1", `{"done":false}`); rec.Code != 200 || h07Obj(t, rec)["done"] != false {
		t.Fatalf("patch false: %d %s", rec.Code, rec.Body.String())
	}
	// list contains the field
	if l := h07Arr(t, h07Do(t, h, "GET", "/api/notes", "")); len(l) != 1 || l[0]["done"] != false {
		t.Fatalf("list: %v", l)
	}
}

func TestHidden07PatchErrors(t *testing.T) {
	h := NewServer()
	if rec := h07Do(t, h, "PATCH", "/api/notes/42", `{"done":true}`); rec.Code != 404 {
		t.Fatalf("missing: %d", rec.Code)
	}
	h07Do(t, h, "POST", "/api/notes", `{"title":"a","body":"b"}`)
	if rec := h07Do(t, h, "PATCH", "/api/notes/1", `{nope`); rec.Code != 400 {
		t.Fatalf("bad json: %d", rec.Code)
	}
	if rec := h07Do(t, h, "PATCH", "/api/notes/x", `{"done":true}`); rec.Code != 400 {
		t.Fatalf("bad id: %d", rec.Code)
	}
}
