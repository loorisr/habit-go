package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"habit-go/internal/db"
)

const testPassword = "s3cret"

func newAuth(t *testing.T, trustProxy bool) (*authConfig, []byte) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	secret, err := db.AppSecret(database)
	if err != nil {
		t.Fatal(err)
	}
	return &authConfig{
		password:   testPassword,
		token:      tokenFor(secret, testPassword),
		trustProxy: trustProxy,
		limiter:    newLoginLimiter(),
	}, secret
}

func login(a *authConfig, password string, remember bool, mods ...func(*http.Request)) *httptest.ResponseRecorder {
	body := `{"password":"` + password + `","remember":` + map[bool]string{true: "true", false: "false"}[remember] + `}`
	req := httptest.NewRequest("POST", "/api/login", strings.NewReader(body))
	req.RemoteAddr = "203.0.113.7:5555"
	for _, m := range mods {
		m(req)
	}
	rec := httptest.NewRecorder()
	a.login(rec, req)
	return rec
}

// B2: the token is no longer sha256(password).
func TestTokenIsNotPlainHash(t *testing.T) {
	a, secret := newAuth(t, false)
	if a.token == "5e884898da28047151d0e56f8dc6292773603d0d6aabbdd62a11ef721d1542d8" {
		t.Fatal("token equals sha256(\"password\")")
	}
	if a.token != tokenFor(secret, testPassword) || len(a.token) != 64 {
		t.Errorf("unexpected token %q", a.token)
	}
	if tokenFor([]byte("other-secret"), testPassword) == a.token {
		t.Error("token does not depend on the secret")
	}
}

func TestLoginSetsTokenCookieAndGrantsAccess(t *testing.T) {
	a, _ := newAuth(t, false)
	rec := login(a, testPassword, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d", rec.Code)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Value != a.token {
		t.Fatalf("cookie = %+v, want token value", cookies)
	}
	req := httptest.NewRequest("GET", "/api/habits", nil)
	req.AddCookie(cookies[0])
	if !a.isAuthenticated(req) {
		t.Error("valid cookie not accepted")
	}
	old := httptest.NewRequest("GET", "/api/habits", nil)
	old.Header.Set("Authorization", "Bearer 5e884898da28047151d0e56f8dc6292773603d0d6aabbdd62a11ef721d1542d8")
	if a.isAuthenticated(old) {
		t.Error("old sha256(password) token still accepted")
	}
}

// B3: without "remember" the cookie is a session cookie; with it, 30 days.
func TestRememberControlsCookieLifetime(t *testing.T) {
	a, _ := newAuth(t, false)
	session := login(a, testPassword, false).Result().Cookies()[0]
	if session.MaxAge != 0 || !session.Expires.IsZero() {
		t.Errorf("session cookie persists: MaxAge=%d Expires=%v", session.MaxAge, session.Expires)
	}
	persistent := login(a, testPassword, true).Result().Cookies()[0]
	if persistent.MaxAge != rememberAge {
		t.Errorf("remembered cookie MaxAge = %d, want %d", persistent.MaxAge, rememberAge)
	}
}

// B4: repeated failures lock the IP, a success before the limit resets it.
func TestLoginRateLimit(t *testing.T) {
	a, _ := newAuth(t, false)
	for i := 0; i < loginMaxFails; i++ {
		if rec := login(a, "wrong", false); rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: status = %d, want 401", i, rec.Code)
		}
	}
	if rec := login(a, testPassword, false); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("locked IP: status = %d, want 429", rec.Code)
	}
	// the window expires
	now := time.Now().Add(loginWindow + time.Minute)
	a.limiter.now = func() time.Time { return now }
	if rec := login(a, testPassword, false); rec.Code != http.StatusOK {
		t.Errorf("after window: status = %d, want 200", rec.Code)
	}
}

// B5: Secure cookie over TLS, or behind a trusted proxy that says https.
func TestCookieSecureFlag(t *testing.T) {
	a, _ := newAuth(t, false)
	plain := login(a, testPassword, true).Result().Cookies()[0]
	if plain.Secure {
		t.Error("plain HTTP cookie marked Secure")
	}
	forwarded := func(r *http.Request) { r.Header.Set("X-Forwarded-Proto", "https") }
	if login(a, testPassword, true, forwarded).Result().Cookies()[0].Secure {
		t.Error("X-Forwarded-Proto trusted without TRUST_PROXY")
	}
	b, _ := newAuth(t, true)
	if !login(b, testPassword, true, forwarded).Result().Cookies()[0].Secure {
		t.Error("trusted proxy https not marked Secure")
	}
}

// B1: only allowlisted origins get CORS headers.
func TestCORSAllowlist(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	h := corsMiddleware(map[string]bool{"https://habits.example": true}, ok)

	req := httptest.NewRequest("GET", "/api/habits", nil)
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("foreign origin got ACAO=%q", got)
	}

	req = httptest.NewRequest("GET", "/api/habits", nil)
	req.Header.Set("Origin", "https://habits.example")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://habits.example" {
		t.Errorf("allowed origin ACAO=%q", got)
	}

	req = httptest.NewRequest("GET", "/api/habits", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("no origin (same-origin) got ACAO=%q", got)
	}
	if rec.Header().Get("X-Frame-Options") != "DENY" || rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("security headers missing")
	}
}

func TestStatusReportsAuthentication(t *testing.T) {
	a, _ := newAuth(t, false)
	req := httptest.NewRequest("GET", "/api/auth/status", nil)
	rec := httptest.NewRecorder()
	a.status(rec, req)
	var got map[string]bool
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !got["protected"] || got["authenticated"] {
		t.Errorf("status = %v, want protected and not authenticated", got)
	}
}
