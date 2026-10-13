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

func h01Do(t *testing.T, h http.Handler, method, path, body string, hdr ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func h01Obj(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("response is not a JSON object: %q (%v)", rec.Body.String(), err)
	}
	return m
}

func h01Arr(t *testing.T, rec *httptest.ResponseRecorder) []map[string]any {
	t.Helper()
	var a []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &a); err != nil {
		t.Fatalf("response is not a JSON array: %q (%v)", rec.Body.String(), err)
	}
	return a
}

var _ = h01Obj
var _ = h01Arr

func TestHidden01NotFoundMessage(t *testing.T) {
	h := NewServer()
	for _, c := range [][2]string{{"GET", ""}, {"PUT", `{"title":"x","body":""}`}, {"DELETE", ""}} {
		rec := h01Do(t, h, c[0], "/api/notes/999", c[1])
		if rec.Code != 404 {
			t.Fatalf("%s: got %d", c[0], rec.Code)
		}
		if got := h01Obj(t, rec)["error"]; got != "note not found" {
			t.Fatalf("%s: error = %v, want %q", c[0], got, "note not found")
		}
	}
}
