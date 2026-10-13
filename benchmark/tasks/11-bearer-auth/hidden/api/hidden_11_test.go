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

func h11Do(t *testing.T, h http.Handler, method, path, body string, hdr ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func h11Obj(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("response is not a JSON object: %q (%v)", rec.Body.String(), err)
	}
	return m
}

func h11Arr(t *testing.T, rec *httptest.ResponseRecorder) []map[string]any {
	t.Helper()
	var a []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &a); err != nil {
		t.Fatalf("response is not a JSON array: %q (%v)", rec.Body.String(), err)
	}
	return a
}

var _ = h11Obj
var _ = h11Arr

func TestHidden11AuthRequiredWhenTokenSet(t *testing.T) {
	t.Setenv("API_TOKEN", "s3cret")
	h := NewServer()
	for _, c := range [][3]string{
		{"GET", "/api/notes", ""},
		{"POST", "/api/notes", `{"title":"a","body":""}`},
		{"GET", "/api/notes/1", ""},
		{"PUT", "/api/notes/1", `{"title":"a","body":""}`},
		{"DELETE", "/api/notes/1", ""},
	} {
		rec := h11Do(t, h, c[0], c[1], c[2])
		if rec.Code != 401 {
			t.Fatalf("%s %s without token: %d", c[0], c[1], rec.Code)
		}
		if h11Obj(t, rec)["error"] != "unauthorized" {
			t.Fatalf("body: %s", rec.Body.String())
		}
		if !strings.HasPrefix(rec.Header().Get("WWW-Authenticate"), "Bearer") {
			t.Fatalf("missing WWW-Authenticate: Bearer header")
		}
	}
	for _, auth := range []string{"Bearer wrong", "Basic s3cret", "s3cret", "Bearer", "bearer s3cret "} {
		if rec := h11Do(t, h, "GET", "/api/notes", "", "Authorization", auth); rec.Code != 401 {
			t.Fatalf("Authorization %q: got %d", auth, rec.Code)
		}
	}
}

func TestHidden11ValidTokenWorks(t *testing.T) {
	t.Setenv("API_TOKEN", "s3cret")
	h := NewServer()
	auth := []string{"Authorization", "Bearer s3cret"}
	if rec := h11Do(t, h, "POST", "/api/notes", `{"title":"a","body":""}`, auth...); rec.Code != 201 {
		t.Fatalf("create: %d", rec.Code)
	}
	if rec := h11Do(t, h, "GET", "/api/notes", "", auth...); rec.Code != 200 || len(h11Arr(t, rec)) != 1 {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
	if rec := h11Do(t, h, "GET", "/api/notes/99", "", auth...); rec.Code != 404 {
		t.Fatalf("authorized missing note must still be 404, got %d", rec.Code)
	}
	if rec := h11Do(t, h, "DELETE", "/api/notes/1", "", auth...); rec.Code != 204 {
		t.Fatalf("delete: %d", rec.Code)
	}
}

func TestHidden11NoTokenMeansOpen(t *testing.T) {
	t.Setenv("API_TOKEN", "")
	h := NewServer()
	if rec := h11Do(t, h, "GET", "/api/notes", ""); rec.Code != 200 {
		t.Fatalf("no API_TOKEN: got %d", rec.Code)
	}
}
