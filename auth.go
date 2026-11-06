package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	cookieName    = "auth_token"
	rememberAge   = 30 * 24 * 3600
	loginMaxFails = 5
	loginWindow   = 15 * time.Minute
)

// tokenFor derives the session token: HMAC-SHA256 keyed by the server secret over the password (B2).
// Unlike sha256(password) it cannot be checked offline, and it changes when the secret changes.
func tokenFor(secret []byte, password string) string {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(password))
	return hex.EncodeToString(m.Sum(nil))
}

type authConfig struct {
	password   string // empty: no protection
	token      string
	trustProxy bool // honour X-Forwarded-For / X-Forwarded-Proto (TRUST_PROXY=true)
	limiter    *loginLimiter
}

func (a *authConfig) protected() bool { return a.password != "" }

func (a *authConfig) isSecure(r *http.Request) bool {
	return r.TLS != nil || (a.trustProxy && r.Header.Get("X-Forwarded-Proto") == "https")
}

func (a *authConfig) clientIP(r *http.Request) string {
	if a.trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			return strings.TrimSpace(strings.Split(xff, ",")[0])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func getTokenFromRequest(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	if t := r.Header.Get("X-Auth-Token"); t != "" {
		return t
	}
	if c, err := r.Cookie(cookieName); err == nil {
		return c.Value
	}
	return ""
}

func (a *authConfig) isAuthenticated(r *http.Request) bool {
	if !a.protected() {
		return true
	}
	token := getTokenFromRequest(r)
	if token == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(a.token)) == 1
}

// middleware protects /api/* except the public auth and health routes.
func (a *authConfig) middleware(next http.Handler) http.Handler {
	if !a.protected() {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/login", "/api/logout", "/api/auth/status", "/api/health":
			next.ServeHTTP(w, r)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/api/") || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		if !a.isAuthenticated(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *authConfig) login(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if !a.protected() {
		json.NewEncoder(w).Encode(map[string]interface{}{"token": "", "message": "no password protection"})
		return
	}
	ip := a.clientIP(r)
	if !a.limiter.allow(ip) {
		w.Header().Set("Retry-After", "900")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]string{"error": "too many attempts, try again later"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	var body struct {
		Password string `json:"password"`
		Remember bool   `json:"remember"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	h1 := sha256.Sum256([]byte(body.Password))
	h2 := sha256.Sum256([]byte(a.password))
	if subtle.ConstantTimeCompare(h1[:], h2[:]) != 1 {
		a.limiter.fail(ip)
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid password"})
		return
	}
	a.limiter.reset(ip)

	// B3: without "remember", the cookie is a session cookie (gone when the browser closes).
	cookie := &http.Cookie{
		Name:     cookieName,
		Value:    a.token,
		Path:     "/",
		HttpOnly: true,
		Secure:   a.isSecure(r), // B5
		SameSite: http.SameSiteStrictMode,
	}
	if body.Remember {
		cookie.MaxAge = rememberAge
	}
	http.SetCookie(w, cookie)
	json.NewEncoder(w).Encode(map[string]string{"token": a.token})
}

func (a *authConfig) logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   a.isSecure(r),
		SameSite: http.SameSiteStrictMode,
	})
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(map[string]string{"status": "logged out"})
}

func (a *authConfig) status(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store, max-age=0")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"protected":     a.protected(),
		"authenticated": !a.protected() || a.isAuthenticated(r),
	})
}

// corsMiddleware only answers for origins in the allowlist (B1). Same-origin use needs no CORS at all.
func corsMiddleware(allowed map[string]bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		w.Header().Add("Vary", "Origin")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		if origin != "" && allowed[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Auth-Token")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// loginLimiter locks an IP for loginWindow after loginMaxFails failed passwords (B4).
type loginLimiter struct {
	mu    sync.Mutex
	fails map[string]*failRecord
	now   func() time.Time
}

type failRecord struct {
	count int
	first time.Time
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{fails: make(map[string]*failRecord), now: time.Now}
}

func (l *loginLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	rec, ok := l.fails[ip]
	if !ok {
		return true
	}
	if l.now().Sub(rec.first) > loginWindow {
		delete(l.fails, ip)
		return true
	}
	return rec.count < loginMaxFails
}

func (l *loginLimiter) fail(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if len(l.fails) > 10000 { // bound memory against many spoofed addresses
		for k, v := range l.fails {
			if now.Sub(v.first) > loginWindow {
				delete(l.fails, k)
			}
		}
	}
	rec, ok := l.fails[ip]
	if !ok || now.Sub(rec.first) > loginWindow {
		rec = &failRecord{first: now}
		l.fails[ip] = rec
	}
	rec.count++
}

func (l *loginLimiter) reset(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, ip)
}
