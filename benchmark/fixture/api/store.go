package main

import (
	"sort"
	"sync"
	"time"
)

// Note is a single note record.
type Note struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Store is an in-memory, mutex-guarded note store.
type Store struct {
	mu     sync.Mutex
	nextID int
	notes  map[int]Note
}

// NewStore returns an empty Store.
func NewStore() *Store {
	return &Store{nextID: 1, notes: map[int]Note{}}
}

// List returns all notes ordered by ID.
func (s *Store) List() []Note {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Note, 0, len(s.notes))
	for _, n := range s.notes {
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Get returns the note with the given ID.
func (s *Store) Get(id int) (Note, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, ok := s.notes[id]
	return n, ok
}

// Create stores a new note and returns it.
func (s *Store) Create(title, body string) Note {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	n := Note{ID: s.nextID, Title: title, Body: body, CreatedAt: now, UpdatedAt: now}
	s.nextID++
	s.notes[n.ID] = n
	return n
}

// Update replaces title and body of an existing note.
func (s *Store) Update(id int, title, body string) (Note, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, ok := s.notes[id]
	if !ok {
		return Note{}, false
	}
	n.Title, n.Body, n.UpdatedAt = title, body, time.Now().UTC()
	s.notes[id] = n
	return n, true
}

// Delete removes a note and reports whether it existed.
func (s *Store) Delete(id int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.notes[id]
	delete(s.notes, id)
	return ok
}
