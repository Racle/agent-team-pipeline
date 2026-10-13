package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var _ = strings.NewReader
var _ = json.Marshal

func h10Do(t *testing.T, h http.Handler, method, path, body string, hdr ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func h10Obj(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("response is not a JSON object: %q (%v)", rec.Body.String(), err)
	}
	return m
}

func h10Arr(t *testing.T, rec *httptest.ResponseRecorder) []map[string]any {
	t.Helper()
	var a []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &a); err != nil {
		t.Fatalf("response is not a JSON array: %q (%v)", rec.Body.String(), err)
	}
	return a
}

var _ = h10Obj
var _ = h10Arr

func TestHidden10PersistenceAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "data.json")
	t.Setenv("DATA_FILE", file)

	h1 := NewServer()
	h10Do(t, h1, "POST", "/api/notes", `{"title":"one","body":"1"}`)
	h10Do(t, h1, "POST", "/api/notes", `{"title":"two","body":"2"}`)
	h10Do(t, h1, "POST", "/api/notes", `{"title":"three","body":"3"}`)
	h10Do(t, h1, "PUT", "/api/notes/2", `{"title":"two!","body":"22"}`)
	h10Do(t, h1, "DELETE", "/api/notes/3", "")

	// file exists and is valid JSON after mutations
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("data file not written: %v", err)
	}
	if !json.Valid(raw) {
		t.Fatalf("data file is not valid JSON: %q", raw)
	}

	// "restart": new server instance, same DATA_FILE
	h2 := NewServer()
	list := h10Arr(t, h10Do(t, h2, "GET", "/api/notes", ""))
	if len(list) != 2 || list[0]["title"] != "one" || list[1]["title"] != "two!" || list[1]["body"] != "22" {
		t.Fatalf("after restart: %v", list)
	}
	// ids keep increasing, never reused (deleted id 3 may not be reused... next id must exceed max existing)
	rec := h10Do(t, h2, "POST", "/api/notes", `{"title":"new","body":""}`)
	id := h10Obj(t, rec)["id"].(float64)
	if id <= 2 {
		t.Fatalf("new id %v collides with existing notes", id)
	}
	h3 := NewServer()
	if l := h10Arr(t, h10Do(t, h3, "GET", "/api/notes", "")); len(l) != 3 {
		t.Fatalf("third instance sees %d notes, want 3", len(l))
	}
}

func TestHidden10AtomicWriteLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DATA_FILE", filepath.Join(dir, "data.json"))
	h := NewServer()
	for i := 0; i < 5; i++ {
		h10Do(t, h, "POST", "/api/notes", `{"title":"x","body":""}`)
	}
	h10Do(t, h, "DELETE", "/api/notes/1", "")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "data.json" {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("dir should contain only data.json, has %v", names)
	}
}

func TestHidden10MissingFileStartsEmptyAndCreatesIt(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "sub-not-needed.json")
	t.Setenv("DATA_FILE", file)
	h := NewServer()
	if body := strings.TrimSpace(h10Do(t, h, "GET", "/api/notes", "").Body.String()); body != "[]" {
		t.Fatalf("fresh start body %q", body)
	}
	h10Do(t, h, "POST", "/api/notes", `{"title":"a","body":""}`)
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("file not created on first write: %v", err)
	}
}

func TestHidden10WithoutEnvStaysInMemory(t *testing.T) {
	t.Setenv("DATA_FILE", "")
	h1 := NewServer()
	h10Do(t, h1, "POST", "/api/notes", `{"title":"a","body":""}`)
	h2 := NewServer()
	if body := strings.TrimSpace(h10Do(t, h2, "GET", "/api/notes", "").Body.String()); body != "[]" {
		t.Fatalf("instances must be independent without DATA_FILE, got %q", body)
	}
}
