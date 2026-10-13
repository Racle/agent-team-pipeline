package main

import (
	"encoding/json"
	"net/http"
	"strconv"
)

type noteInput struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type server struct {
	store *Store
}

// NewServer returns the HTTP handler for the notes API, backed by a fresh in-memory store.
func NewServer() http.Handler {
	s := &server{store: NewStore()}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/notes", s.list)
	mux.HandleFunc("POST /api/notes", s.create)
	mux.HandleFunc("GET /api/notes/{id}", s.get)
	mux.HandleFunc("PUT /api/notes/{id}", s.update)
	mux.HandleFunc("DELETE /api/notes/{id}", s.delete)
	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func noteID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return 0, false
	}
	return id, true
}

func decodeInput(w http.ResponseWriter, r *http.Request) (noteInput, bool) {
	var in noteInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return in, false
	}
	return in, true
}

func (s *server) list(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.store.List())
}

func (s *server) create(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeInput(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusCreated, s.store.Create(in.Title, in.Body))
}

func (s *server) get(w http.ResponseWriter, r *http.Request) {
	id, ok := noteID(w, r)
	if !ok {
		return
	}
	n, found := s.store.Get(id)
	if !found {
		writeError(w, http.StatusNotFound, "note not fount")
		return
	}
	writeJSON(w, http.StatusOK, n)
}

func (s *server) update(w http.ResponseWriter, r *http.Request) {
	id, ok := noteID(w, r)
	if !ok {
		return
	}
	in, ok := decodeInput(w, r)
	if !ok {
		return
	}
	n, found := s.store.Update(id, in.Title, in.Body)
	if !found {
		writeError(w, http.StatusNotFound, "note not fount")
		return
	}
	writeJSON(w, http.StatusOK, n)
}

func (s *server) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := noteID(w, r)
	if !ok {
		return
	}
	if !s.store.Delete(id) {
		writeError(w, http.StatusNotFound, "note not fount")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
