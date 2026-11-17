// Package httpserver is the local server the http examples talk to, so the
// http module's tasks run through both backends without the network. The
// example tests start it and pass its address to the program in the
// environment (GOBLIN_HTTP_TEST_URL, and GOBLIN_HTTPS_TEST_URL for a server
// with a self-signed certificate).
package httpserver

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Start starts the plain and the TLS server; close both when done.
func Start() (plain, tls *httptest.Server) {
	s := &state{counts: map[string]int{}}
	return httptest.NewServer(s.mux()), httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"state": "green"})
	}))
}

type state struct {
	mu     sync.Mutex
	counts map[string]int
}

// hit counts the requests to a path, so a handler can fail the first few.
func (s *state) hit(path string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counts[path]++
	return s.counts[path]
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func (s *state) mux() *http.ServeMux {
	m := http.NewServeMux()
	m.HandleFunc("GET /repos/aisk/goblin", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"stargazers_count": 42})
	})
	m.HandleFunc("GET /search", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		writeJSON(w, 200, map[string]any{"items": []any{map[string]any{"name": q.Get("q") + "-page-" + q.Get("page")}}})
	})
	m.HandleFunc("POST /users", func(w http.ResponseWriter, r *http.Request) {
		var user map[string]any
		json.NewDecoder(r.Body).Decode(&user)
		w.Header().Set("Location", "/users/7")
		writeJSON(w, http.StatusCreated, map[string]any{"id": 7, "name": user["name"]})
	})
	m.HandleFunc("POST /contact", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		fmt.Fprintf(w, "thanks %s", r.PostForm.Get("email"))
	})
	upload := func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var names []string
		for name := range r.MultipartForm.Value {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			fmt.Fprintf(w, "field %s=%s\n", name, strings.Join(r.MultipartForm.Value[name], ","))
		}
		names = names[:0]
		for name := range r.MultipartForm.File {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			for _, fh := range r.MultipartForm.File[name] {
				f, _ := fh.Open()
				data, _ := io.ReadAll(f)
				fmt.Fprintf(w, "file %s=%s %s %d bytes\n", name, fh.Filename, fh.Header.Get("Content-Type"), len(data))
			}
		}
	}
	m.HandleFunc("POST /profile", upload)
	m.HandleFunc("POST /reports", upload)
	m.HandleFunc("PUT /storage/", func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		fmt.Fprintf(w, "stored %d bytes, declared %d", len(data), r.ContentLength)
	})
	m.HandleFunc("GET /release.tar.gz", func(w http.ResponseWriter, r *http.Request) {
		zw := gzip.NewWriter(w)
		io.WriteString(zw, "release contents\n")
		zw.Close()
	})
	m.HandleFunc("GET /latest", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/releases/v2", http.StatusFound)
	})
	m.HandleFunc("GET /releases/v2", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "visited", Value: "yes", Path: "/"})
		http.SetCookie(w, &http.Cookie{Name: "theme", Value: "dark, wide", Path: "/"})
		io.WriteString(w, "version 2")
	})
	m.HandleFunc("GET /prices", func(w http.ResponseWriter, r *http.Request) {
		if s.hit("/prices") <= 2 {
			w.Header().Set("Retry-After", "0")
			http.Error(w, "busy", http.StatusServiceUnavailable)
			return
		}
		writeJSON(w, 200, map[string]any{"apple": 3})
	})
	m.HandleFunc("DELETE /users/7", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no such user", http.StatusNotFound)
	})
	m.HandleFunc("GET /jobs/1", func(w http.ResponseWriter, r *http.Request) {
		if s.hit("/jobs/1") <= 2 {
			http.Error(w, "not yet", http.StatusNotFound)
			return
		}
		writeJSON(w, 200, map[string]any{"state": "done"})
	})
	bearer := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer t0ken" {
				http.Error(w, "who are you", http.StatusUnauthorized)
				return
			}
			next(w, r)
		}
	}
	m.HandleFunc("GET /v1/me", bearer(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"id": 7})
	}))
	m.HandleFunc("PATCH /v1/users/7", bearer(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		fmt.Fprintf(w, "patched %s", body)
	}))
	m.HandleFunc("DELETE /v1/sessions/current", bearer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	m.HandleFunc("GET /inbox", func(w http.ResponseWriter, r *http.Request) {
		user, password, ok := r.BasicAuth()
		if !ok || user != "ada" || password != "pw" {
			http.Error(w, "log in", http.StatusUnauthorized)
			return
		}
		io.WriteString(w, "2 new messages")
	})
	m.HandleFunc("POST /login", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.PostForm.Get("password") != "secret" {
			http.Error(w, "wrong password", http.StatusForbidden)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "s-" + r.PostForm.Get("user"), Path: "/"})
		http.SetCookie(w, &http.Cookie{Name: "csrftoken", Value: "c5rf", Path: "/"})
		http.Redirect(w, r, "/account", http.StatusSeeOther)
	})
	session := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if c, err := r.Cookie("session"); err != nil || c.Value != "s-ada" {
				http.Error(w, "log in first", http.StatusUnauthorized)
				return
			}
			next(w, r)
		}
	}
	m.HandleFunc("GET /account", session(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "hello ada") }))
	m.HandleFunc("GET /account/orders", session(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "order 1\norder 2\n")
	}))
	m.HandleFunc("POST /account/email", session(func(w http.ResponseWriter, r *http.Request) {
		c, _ := r.Cookie("csrftoken")
		if r.Header.Get("X-CSRFToken") != c.Value {
			http.Error(w, "bad token", http.StatusForbidden)
			return
		}
		r.ParseForm()
		fmt.Fprintf(w, "email set to %s", r.PostForm.Get("email"))
	}))
	m.HandleFunc("GET /events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, kind := range []string{"start", "progress", "stop"} {
			fmt.Fprintf(w, "event: message\ndata: {\"type\": %q}\n\n", kind)
			w.(http.Flusher).Flush()
		}
	})
	m.HandleFunc("POST /ping", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		fmt.Fprintf(w, "pong %s, signature %d hex digits", body, len(r.Header.Get("X-Signature")))
	})
	m.HandleFunc("GET /short", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/latest", http.StatusMovedPermanently)
	})
	// /headers answers with every header it received, so a program can
	// show what was sent. A User-Agent of goblin/<version> shows without
	// the version, which changes between releases.
	m.HandleFunc("GET /headers", func(w http.ResponseWriter, r *http.Request) {
		var names []string
		for name := range r.Header {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			values := r.Header.Values(name)
			if name == "User-Agent" && len(values) == 1 && strings.HasPrefix(values[0], "goblin/") {
				values = []string{"goblin/<version>"}
			}
			fmt.Fprintf(w, "%s: %s\n", name, strings.Join(values, " | "))
		}
	})
	m.HandleFunc("GET /big.iso", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1048576")
		if r.Method == http.MethodGet {
			w.Write(make([]byte, 1048576))
		}
	})
	m.HandleFunc("GET /feed.xml", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == `"v2"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"v2"`)
		io.WriteString(w, "<feed>v2</feed>\n")
	})
	// /chain/N redirects N times, each hop setting a cookie the next one
	// needs, then answers with the cookies it got.
	m.HandleFunc("GET /chain/{n}", func(w http.ResponseWriter, r *http.Request) {
		n, _ := strconv.Atoi(r.PathValue("n"))
		if prev := r.URL.Query().Get("from"); prev != "" {
			if c, err := r.Cookie("hop" + prev); err != nil || c.Value != "yes" {
				http.Error(w, "missing cookie hop"+prev, http.StatusUnauthorized)
				return
			}
		}
		if n == 0 {
			var got []string
			for _, c := range r.Cookies() {
				got = append(got, c.Name)
			}
			sort.Strings(got)
			fmt.Fprintf(w, "arrived with %s", strings.Join(got, ","))
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "hop" + strconv.Itoa(n), Value: "yes", Path: "/"})
		http.Redirect(w, r, fmt.Sprintf("/chain/%d?from=%d", n-1, n), http.StatusFound)
	})
	// /export compresses whatever was asked for.
	m.HandleFunc("GET /export", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		zw := gzip.NewWriter(w)
		io.WriteString(zw, "id,name\n1,Ada\n")
		zw.Close()
	})
	m.HandleFunc("PUT /up", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/storage/up", http.StatusTemporaryRedirect)
	})
	return m
}
