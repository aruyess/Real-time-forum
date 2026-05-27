package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"forum/internal/db"
	"forum/internal/handlers"
)

func newAuth(t *testing.T) *handlers.Auth {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return &handlers.Auth{DB: conn}
}

func postJSON(h http.HandlerFunc, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

const validRegister = `{"nickname":"alice","email":"a@a.com","password":"secret123",` +
	`"age":25,"gender":"female","firstName":"A","lastName":"W"}`

func TestRegisterCreatesUser(t *testing.T) {
	auth := newAuth(t)
	rec := postJSON(auth.Register, "/api/register", validRegister)

	if rec.Code != http.StatusCreated {
		t.Fatalf("got %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["nickname"] != "alice" {
		t.Errorf("nickname=%q, want alice", resp["nickname"])
	}
	if resp["id"] == "" {
		t.Error("response missing id")
	}
}

func TestRegisterValidatesFields(t *testing.T) {
	auth := newAuth(t)

	cases := map[string]string{
		"short password": `{"nickname":"a","email":"a@a.com","password":"x","age":25,"gender":"female","firstName":"A","lastName":"W"}`,
		"bad email":      `{"nickname":"alice","email":"nope","password":"secret123","age":25,"gender":"female","firstName":"A","lastName":"W"}`,
		"underage":       `{"nickname":"alice","email":"a@a.com","password":"secret123","age":5,"gender":"female","firstName":"A","lastName":"W"}`,
		"bad gender":     `{"nickname":"alice","email":"a@a.com","password":"secret123","age":25,"gender":"???","firstName":"A","lastName":"W"}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			rec := postJSON(auth.Register, "/api/register", body)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("got %d, want 400; body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestLoginRejectsWrongCredentials(t *testing.T) {
	auth := newAuth(t)
	// Seed a real user via the same handler — exercises the full registration
	// + bcrypt path so the failing-login assertion isn't dependent on
	// internal helpers.
	if rec := postJSON(auth.Register, "/api/register", validRegister); rec.Code != http.StatusCreated {
		t.Fatalf("seed register: %d %s", rec.Code, rec.Body.String())
	}

	for _, body := range []string{
		`{"identifier":"alice","password":"wrong"}`,
		`{"identifier":"nobody","password":"secret123"}`,
	} {
		rec := postJSON(auth.Login, "/api/login", body)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("got %d, want 401 for body %s", rec.Code, body)
		}
		// Same generic message regardless of which check failed — no
		// information leak about which usernames exist.
		if !strings.Contains(rec.Body.String(), "invalid credentials") {
			t.Errorf("body %s missing generic error", rec.Body.String())
		}
	}
}

func TestLoginAcceptsByEmailOrNickname(t *testing.T) {
	auth := newAuth(t)
	if rec := postJSON(auth.Register, "/api/register", validRegister); rec.Code != http.StatusCreated {
		t.Fatalf("seed: %d", rec.Code)
	}

	for _, ident := range []string{"alice", "a@a.com", "A@A.com"} {
		body := `{"identifier":"` + ident + `","password":"secret123"}`
		rec := postJSON(auth.Login, "/api/login", body)
		if rec.Code != http.StatusOK {
			t.Errorf("identifier=%s: got %d, want 200", ident, rec.Code)
		}
	}
}
