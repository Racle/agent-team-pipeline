package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func h15Do(h http.Handler, method, path, body string) (int, map[string]any) {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var m map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &m)
	return rec.Code, m
}

func h15Create(t *testing.T, h http.Handler, title, body string) {
	t.Helper()
	if code, _ := h15Do(h, "POST", "/api/notes", fmt.Sprintf(`{"title":%q,"body":%q}`, title, body)); code != 201 {
		t.Fatalf("create: %d", code)
	}
}

func h15Run(n int, f func(i int)) {
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			f(i)
		}(i)
	}
	close(start)
	wg.Wait()
}

func TestHidden15LikesField(t *testing.T) {
	h := NewServer()
	h15Create(t, h, "a", "b")
	if _, m := h15Do(h, "GET", "/api/notes/1", ""); m["likes"] != float64(0) {
		t.Fatalf("likes must be 0, got %v", m["likes"])
	}
	code, m := h15Do(h, "POST", "/api/notes/1/like", "")
	if code != 200 || m["likes"] != float64(1) || m["title"] != "a" {
		t.Fatalf("like: %d %v", code, m)
	}
	if _, m := h15Do(h, "PUT", "/api/notes/1", `{"title":"c","body":"d"}`); m["likes"] != float64(1) {
		t.Fatalf("PUT must keep likes: %v", m)
	}
	if code, m := h15Do(h, "POST", "/api/notes/9/like", ""); code != 404 || m["error"] == "" || m["error"] == nil {
		t.Fatalf("like missing: %d %v", code, m)
	}
	if code, _ := h15Do(h, "POST", "/api/notes/abc/like", ""); code != 400 {
		t.Fatalf("like bad id: %d", code)
	}
}

func TestHidden15ConcurrentLikes(t *testing.T) {
	h := NewServer()
	h15Create(t, h, "a", "b")
	h15Create(t, h, "c", "d")
	const goroutines, per = 50, 20
	h15Run(goroutines, func(i int) {
		for j := 0; j < per; j++ {
			h15Do(h, "POST", "/api/notes/1/like", "")
			h15Do(h, "GET", "/api/notes", "") // concurrent readers
			if i%5 == 0 {
				h15Do(h, "PUT", "/api/notes/2", fmt.Sprintf(`{"title":"t%d","body":"b"}`, j))
			}
		}
	})
	if _, m := h15Do(h, "GET", "/api/notes/1", ""); m["likes"] != float64(goroutines*per) {
		t.Fatalf("lost updates: likes=%v want %d", m["likes"], goroutines*per)
	}
}

func TestHidden15Merge(t *testing.T) {
	h := NewServer()
	h15Create(t, h, "a", "AAA")
	h15Create(t, h, "b", "BBB")
	for i := 0; i < 3; i++ {
		h15Do(h, "POST", "/api/notes/1/like", "")
	}
	for i := 0; i < 4; i++ {
		h15Do(h, "POST", "/api/notes/2/like", "")
	}
	code, m := h15Do(h, "POST", "/api/notes/merge", `{"from":1,"into":2}`)
	if code != 200 || m["body"] != "BBB\nAAA" || m["likes"] != float64(7) || m["id"] != float64(2) || m["title"] != "b" {
		t.Fatalf("merge: %d %v", code, m)
	}
	if code, _ := h15Do(h, "GET", "/api/notes/1", ""); code != 404 {
		t.Fatalf("source must be deleted: %d", code)
	}
	if code, m := h15Do(h, "POST", "/api/notes/merge", `{"from":1,"into":2}`); code != 404 || m["error"] == nil || m["error"] == "" {
		t.Fatalf("merge missing source: %d %v", code, m)
	}
	if code, _ := h15Do(h, "POST", "/api/notes/merge", `{"from":2,"into":99}`); code != 404 {
		t.Fatalf("merge missing target: %d", code)
	}
	if _, m := h15Do(h, "GET", "/api/notes/2", ""); m["body"] != "BBB\nAAA" || m["likes"] != float64(7) {
		t.Fatalf("failed merges must change nothing: %v", m)
	}
	if code, _ := h15Do(h, "POST", "/api/notes/merge", `{"from":2,"into":2}`); code != 400 {
		t.Fatalf("self merge: %d", code)
	}
	if code, _ := h15Do(h, "POST", "/api/notes/merge", `{bad`); code != 400 {
		t.Fatalf("bad json: %d", code)
	}
}

func TestHidden15ConcurrentOppositeMerges(t *testing.T) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		for round := 0; round < 100; round++ {
			h := NewServer()
			for _, n := range [][2]string{{"a", "AAA"}, {"b", "BBB"}} {
				if code, _ := h15Do(h, "POST", "/api/notes", fmt.Sprintf(`{"title":%q,"body":%q}`, n[0], n[1])); code != 201 {
					t.Errorf("round %d: create: %d", round, code)
					return
				}
			}
			for i := 0; i < 5; i++ {
				h15Do(h, "POST", "/api/notes/1/like", "")
			}
			for i := 0; i < 7; i++ {
				h15Do(h, "POST", "/api/notes/2/like", "")
			}
			codes := make([]int, 2)
			h15Run(2, func(i int) {
				body := `{"from":1,"into":2}`
				if i == 1 {
					body = `{"from":2,"into":1}`
				}
				codes[i], _ = h15Do(h, "POST", "/api/notes/merge", body)
			})
			if !(codes[0] == 200 && codes[1] == 404) && !(codes[0] == 404 && codes[1] == 200) {
				t.Errorf("round %d: exactly one merge must succeed, got %v", round, codes)
				return
			}
			req := httptest.NewRequest("GET", "/api/notes", nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			var list []map[string]any
			_ = json.Unmarshal(rec.Body.Bytes(), &list)
			if len(list) != 1 || list[0]["likes"] != float64(12) {
				t.Errorf("round %d: survivor must hold 12 likes: %s", round, rec.Body.String())
				return
			}
			b := list[0]["body"].(string)
			if !strings.Contains(b, "AAA") || !strings.Contains(b, "BBB") || len(b) != 7 {
				t.Errorf("round %d: survivor body %q", round, b)
				return
			}
		}
	}()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("deadlock: opposite merges did not finish")
	}
}

func TestHidden15ConcurrentMergeChain(t *testing.T) {
	h := NewServer()
	const n = 40
	for i := 0; i < n; i++ {
		h15Create(t, h, fmt.Sprint("n", i), "x")
		h15Do(h, "POST", fmt.Sprintf("/api/notes/%d/like", i+1), "")
	}
	// merge every note into note 1 concurrently while others like note 1
	h15Run(n-1, func(i int) {
		h15Do(h, "POST", "/api/notes/merge", fmt.Sprintf(`{"from":%d,"into":1}`, i+2))
		h15Do(h, "POST", "/api/notes/1/like", "")
	})
	_, m := h15Do(h, "GET", "/api/notes/1", "")
	if m["likes"] != float64(n+(n-1)) {
		t.Fatalf("likes=%v want %d", m["likes"], n+(n-1))
	}
	if got := len(strings.Split(m["body"].(string), "\n")); got != n {
		t.Fatalf("body has %d parts want %d", got, n)
	}
}

func TestHidden15Stats(t *testing.T) {
	h := NewServer()
	if code, m := h15Do(h, "GET", "/api/stats", ""); code != 200 || m["requests"] != float64(0) || m["notes"] != float64(0) {
		t.Fatalf("initial stats: %d %v", code, m)
	}
	h15Create(t, h, "a", "b")                 // 1
	h15Do(h, "GET", "/api/notes/9", "")       // 2 (404 counts)
	h15Do(h, "GET", "/api/notes/abc", "")     // 3 (400 counts)
	h15Do(h, "GET", "/api/stats", "")         // not counted
	h15Do(h, "POST", "/api/notes/merge", `{`) // 4
	code, m := h15Do(h, "GET", "/api/stats", "")
	if code != 200 || m["requests"] != float64(4) || m["notes"] != float64(1) {
		t.Fatalf("stats: %d %v", code, m)
	}
	const goroutines, per = 40, 25
	h15Run(goroutines, func(i int) {
		for j := 0; j < per; j++ {
			switch j % 3 {
			case 0:
				h15Do(h, "GET", "/api/notes", "")
			case 1:
				h15Do(h, "POST", "/api/notes/1/like", "")
			default:
				h15Do(h, "GET", "/api/stats", "")
			}
		}
	})
	counted := 0
	for j := 0; j < per; j++ {
		if j%3 != 2 {
			counted++
		}
	}
	want := float64(4 + goroutines*counted)
	if _, m := h15Do(h, "GET", "/api/stats", ""); m["requests"] != want {
		t.Fatalf("requests=%v want %v", m["requests"], want)
	}
}
