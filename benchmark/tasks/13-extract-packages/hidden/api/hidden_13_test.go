package main

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"notesapi/internal/store"
	"notesapi/internal/validate"
)

func h13Do(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func h13Imports(t *testing.T, dir string) map[string]bool {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool { return !strings.HasSuffix(fi.Name(), "_test.go") }, parser.ImportsOnly)
	if err != nil || len(pkgs) == 0 {
		t.Fatalf("parse %s: %v", dir, err)
	}
	out := map[string]bool{}
	for _, p := range pkgs {
		for _, f := range p.Files {
			for _, im := range f.Imports {
				path, _ := strconv.Unquote(im.Path.Value)
				out[path] = true
			}
		}
	}
	return out
}

func TestHidden13StorePackage(t *testing.T) {
	s := store.NewStore()
	a := s.Create("a", "b")
	if a.ID != 1 || a.Title != "a" || a.Body != "b" || a.CreatedAt.IsZero() {
		t.Fatalf("create: %+v", a)
	}
	s.Create("c", "d")
	if l := s.List(); len(l) != 2 || l[0].ID != 1 || l[1].ID != 2 {
		t.Fatalf("list: %+v", l)
	}
	u, ok := s.Update(1, "x", "y")
	if !ok || u.Title != "x" || u.Body != "y" || u.UpdatedAt.Before(u.CreatedAt) {
		t.Fatalf("update: %+v %v", u, ok)
	}
	if _, ok := s.Update(9, "x", "y"); ok {
		t.Fatal("update of missing note must fail")
	}
	if g, ok := s.Get(1); !ok || g.Title != "x" {
		t.Fatalf("get: %+v %v", g, ok)
	}
	if !s.Delete(1) || s.Delete(1) {
		t.Fatal("delete semantics")
	}
	if n := s.Create("e", ""); n.ID != 3 {
		t.Fatalf("ids must not be reused, got %d", n.ID)
	}
}

func TestHidden13ValidatePackage(t *testing.T) {
	in, err := validate.DecodeNoteInput(strings.NewReader(`{"title":"t","body":"b"}`))
	if err != nil || in.Title != "t" || in.Body != "b" {
		t.Fatalf("decode: %+v %v", in, err)
	}
	b, _ := json.Marshal(validate.NoteInput{Title: "t", Body: "b"})
	if string(b) != `{"title":"t","body":"b"}` {
		t.Fatalf("json tags: %s", b)
	}
	if _, err := validate.DecodeNoteInput(strings.NewReader(`{bad`)); err == nil {
		t.Fatal("invalid JSON must error")
	}
	if id, err := validate.ParseID("42"); err != nil || id != 42 {
		t.Fatalf("parse: %d %v", id, err)
	}
	if _, err := validate.ParseID("x1"); err == nil {
		t.Fatal("non-integer id must error")
	}
}

func TestHidden13NewServerWithStoreSharesStore(t *testing.T) {
	st := store.NewStore()
	h := NewServerWithStore(st)
	if rec := h13Do(h, "POST", "/api/notes", `{"title":"a","body":"b"}`); rec.Code != 201 {
		t.Fatalf("create: %d", rec.Code)
	}
	if n, ok := st.Get(1); !ok || n.Title != "a" {
		t.Fatalf("handler did not write to the given store: %+v %v", n, ok)
	}
	st.Create("direct", "")
	rec := h13Do(h, "GET", "/api/notes", "")
	var list []store.Note
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list) != 2 {
		t.Fatalf("handler did not read from the given store: %s", rec.Body.String())
	}
	if rec := h13Do(NewServer(), "GET", "/api/notes", ""); strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("NewServer must start empty: %s", rec.Body.String())
	}
}

func TestHidden13BehaviourUnchanged(t *testing.T) {
	h := NewServer()
	cases := []struct {
		method, path, body string
		code               int
		errMsg             string
	}{
		{"POST", "/api/notes", `{bad`, 400, "invalid JSON body"},
		{"GET", "/api/notes/abc", "", 400, "invalid id"},
		{"PUT", "/api/notes/abc", `{}`, 400, "invalid id"},
		{"PUT", "/api/notes/1", `{bad`, 400, "invalid JSON body"},
		{"GET", "/api/notes/9", "", 404, "note not fount"},
		{"PUT", "/api/notes/9", `{"title":"x"}`, 404, "note not fount"},
		{"DELETE", "/api/notes/9", "", 404, "note not fount"},
		{"DELETE", "/api/notes/abc", "", 400, "invalid id"},
	}
	for _, c := range cases {
		rec := h13Do(h, c.method, c.path, c.body)
		var m map[string]string
		_ = json.Unmarshal(rec.Body.Bytes(), &m)
		if rec.Code != c.code || m["error"] != c.errMsg || rec.Header().Get("Content-Type") != "application/json" {
			t.Errorf("%s %s: got %d %q ct=%q", c.method, c.path, rec.Code, rec.Body.String(), rec.Header().Get("Content-Type"))
		}
	}
	rec := h13Do(h, "POST", "/api/notes", `{"title":"a","body":"b"}`)
	if rec.Code != 201 {
		t.Fatalf("create: %d", rec.Code)
	}
	var m map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &m)
	for _, k := range []string{"id", "title", "body", "created_at", "updated_at"} {
		if _, ok := m[k]; !ok {
			t.Errorf("note JSON lost field %q: %s", k, rec.Body.String())
		}
	}
	if len(m) != 5 {
		t.Errorf("note JSON must keep exactly its 5 fields: %s", rec.Body.String())
	}
	if rec := h13Do(h, "DELETE", "/api/notes/1", ""); rec.Code != 204 || rec.Body.Len() != 0 {
		t.Errorf("delete: %d %q", rec.Code, rec.Body.String())
	}
}

func TestHidden13Structure(t *testing.T) {
	if _, err := os.Stat("store.go"); err == nil {
		t.Error("api/store.go must be removed (moved into internal/store)")
	}
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool { return !strings.HasSuffix(fi.Name(), "_test.go") }, 0)
	if err != nil {
		t.Fatal(err)
	}
	banned := map[string]bool{"Note": true, "Store": true, "NewStore": true, "noteInput": true}
	for _, p := range pkgs {
		for fname, f := range p.Files {
			for _, d := range f.Decls {
				switch d := d.(type) {
				case *ast.GenDecl:
					for _, sp := range d.Specs {
						if ts, ok := sp.(*ast.TypeSpec); ok && banned[ts.Name.Name] {
							t.Errorf("%s still declares type %s", fname, ts.Name.Name)
						}
					}
				case *ast.FuncDecl:
					if d.Recv == nil && banned[d.Name.Name] {
						t.Errorf("%s still declares func %s", fname, d.Name.Name)
					}
					if d.Recv != nil {
						if star, ok := d.Recv.List[0].Type.(*ast.StarExpr); ok {
							if id, ok := star.X.(*ast.Ident); ok && id.Name == "Store" {
								t.Errorf("%s still declares methods on Store", fname)
							}
						}
					}
				}
			}
		}
	}
	st := h13Imports(t, filepath.Join("internal", "store"))
	if st["net/http"] || st["notesapi/internal/validate"] {
		t.Errorf("internal/store imports %v; must not import net/http or internal/validate", st)
	}
	va := h13Imports(t, filepath.Join("internal", "validate"))
	if va["net/http"] || va["notesapi/internal/store"] {
		t.Errorf("internal/validate imports %v; must not import net/http or internal/store", va)
	}
}
