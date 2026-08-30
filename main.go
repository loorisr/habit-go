package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	csvhandler "habit-go/internal/csv"
	"habit-go/internal/db"
	"habit-go/internal/entry"
	"habit-go/internal/habit"
)

//go:embed frontend/dist
var frontendFS embed.FS

func main() {
	// CLI flags (env vars are fallback)
	passwordFlag := flag.String("password", "", "Password for protecting access (fallback env PASSWORD)")
	portFlag := flag.String("port", "", "Port to listen on (fallback env PORT, default 8080)")
	dbFlag := flag.String("db", "", "SQLite DB path (fallback env DB_PATH, default data/habits.db)")
	flag.Parse()

	// Resolve password: flag > env PASSWORD > env APP_PASSWORD > env HABIT_PASSWORD
	password := *passwordFlag
	if password == "" {
		password = os.Getenv("PASSWORD")
	}
	if password == "" {
		password = os.Getenv("APP_PASSWORD")
	}
	if password == "" {
		password = os.Getenv("HABIT_PASSWORD")
	}

	dbPath := *dbFlag
	if dbPath == "" {
		dbPath = os.Getenv("DB_PATH")
	}
	if dbPath == "" {
		dbPath = "data/habits.db"
	}
	port := *portFlag
	if port == "" {
		port = os.Getenv("PORT")
	}
	if port == "" {
		port = "8080"
	}

	// Compute expected token (sha256 hex of password) if protection enabled
	var expectedToken string
	if password != "" {
		h := sha256.Sum256([]byte(password))
		expectedToken = hex.EncodeToString(h[:])
		log.Printf("Password protection ENABLED")
	} else {
		log.Printf("Password protection DISABLED (no password set)")
	}

	database, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	defer database.Close()

	habitRepo := habit.NewRepository(database)
	entryRepo := entry.NewRepository(database)

	habitHandler := habit.NewHandler(habitRepo, entryRepo)
	entryHandler := entry.NewHandler(entryRepo)
	csvHandler := csvhandler.NewHandler(database, habitRepo)

	mux := http.NewServeMux()

	// --- Auth routes (always public) ---
	mux.HandleFunc("POST /api/login", loginHandler(password, expectedToken))
	mux.HandleFunc("POST /api/logout", logoutHandler())
	mux.HandleFunc("GET /api/auth/status", authStatusHandler(password != "", expectedToken))

	// API routes
	mux.HandleFunc("GET /api/health", habitHandler.Health)
	mux.HandleFunc("GET /api/habits", habitHandler.List)
	mux.HandleFunc("POST /api/habits", habitHandler.Create)
	mux.HandleFunc("GET /api/habits/{id}", habitHandler.Get)
	mux.HandleFunc("PUT /api/habits/{id}", habitHandler.Update)
	mux.HandleFunc("POST /api/habits/{id}/archive", habitHandler.Archive)
	mux.HandleFunc("POST /api/habits/{id}/restore", habitHandler.Restore)
	mux.HandleFunc("DELETE /api/habits/{id}", habitHandler.Delete)

	mux.HandleFunc("GET /api/habits/{id}/entries", entryHandler.GetRange)
	mux.HandleFunc("PUT /api/habits/{id}/entries/{date}", entryHandler.Upsert)
	mux.HandleFunc("DELETE /api/habits/{id}/entries/{date}", entryHandler.Delete)

	mux.HandleFunc("GET /api/entries", entryHandler.Bulk)

	mux.HandleFunc("GET /api/export/habits", csvHandler.ExportHabits)
	mux.HandleFunc("GET /api/export/entries", csvHandler.ExportEntries)
	mux.HandleFunc("POST /api/import/habits", csvHandler.ImportHabits)
	mux.HandleFunc("POST /api/import/entries", csvHandler.ImportEntries)

	// Static frontend
	distFS, err := fs.Sub(frontendFS, "frontend/dist")
	var fileServer http.Handler
	if err != nil {
		log.Printf("frontend/dist not found, serving without frontend: %v", err)
		fileServer = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		})
	} else {
		fileServer = http.FileServer(http.FS(distFS))
	}

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		// If path starts with /api/ but not matched, 404
		if len(r.URL.Path) >= 5 && r.URL.Path[:5] == "/api/" {
			http.NotFound(w, r)
			return
		}
		// Try to serve file if exists else fallback to index.html for SPA (only for HTML navigations)
		if distFS != nil {
			path := r.URL.Path
			if path == "/" {
				path = "/index.html"
			} else {
				fp := strings.TrimPrefix(path, "/")
				if _, err := fs.Stat(distFS, fp); err == nil {
					fileServer.ServeHTTP(w, r)
					return
				}
				// only fallback to index.html if Accept header wants HTML (SPA navigation)
				accept := r.Header.Get("Accept")
				if accept != "" && !strings.Contains(accept, "text/html") {
					http.NotFound(w, r)
					return
				}
			}
			data, err := fs.ReadFile(distFS, "index.html")
			if err != nil {
				fileServer.ServeHTTP(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/html")
			w.Header().Set("Cache-Control", "no-cache")
			w.Write(data)
			return
		}
		http.NotFound(w, r)
	})

	// Wrap with maxBytes -> auth -> CORS -> logging
	handler := loggingMiddleware(corsMiddleware(maxBytesMiddleware(authMiddleware(mux, expectedToken, password != ""))))

	log.Printf("Starting server on :%s with DB %s", port, dbPath)
	if password != "" {
		log.Printf(" -> Protected by password (use env PASSWORD or -password flag)")
	}
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

// --- Auth helpers ---

func getTokenFromRequest(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	if t := r.Header.Get("X-Auth-Token"); t != "" {
		return t
	}
	if c, err := r.Cookie("auth_token"); err == nil {
		return c.Value
	}
	return ""
}

func isAuthenticated(r *http.Request, expectedToken string) bool {
	if expectedToken == "" {
		return true
	}
	token := getTokenFromRequest(r)
	if token == "" {
		return false
	}
	// constant time compare
	return subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) == 1
}

func authMiddleware(next http.Handler, expectedToken string, protected bool) http.Handler {
	if !protected {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// public paths that bypass auth
		if r.URL.Path == "/api/login" || r.URL.Path == "/api/logout" || r.URL.Path == "/api/auth/status" || r.URL.Path == "/api/health" {
			next.ServeHTTP(w, r)
			return
		}
		// Only protect /api/* ; static frontend is allowed (SPA will redirect to login)
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		// OPTIONS preflight should also bypass? let CORS handle, but still need auth check? For simplicity allow OPTIONS without auth to avoid CORS issues.
		if r.Method == "OPTIONS" {
			next.ServeHTTP(w, r)
			return
		}
		if !isAuthenticated(r, expectedToken) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func loginHandler(password, expectedToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// limit body to 4KB to prevent DoS
		r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
		if password == "" {
			// no protection, always success
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-store")
			json.NewEncoder(w).Encode(map[string]interface{}{"token": "", "message": "no password protection"})
			return
		}
		var body struct {
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		h1 := sha256.Sum256([]byte(body.Password))
		h2 := sha256.Sum256([]byte(password))
		if subtle.ConstantTimeCompare(h1[:], h2[:]) != 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid password"})
			return
		}
		// success: set cookie and return token
		http.SetCookie(w, &http.Cookie{
			Name:     "auth_token",
			Value:    expectedToken,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
			MaxAge:   30 * 24 * 3600,
		})
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		json.NewEncoder(w).Encode(map[string]string{"token": expectedToken})
	}
}

func logoutHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:     "auth_token",
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
		})
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		json.NewEncoder(w).Encode(map[string]string{"status": "logged out"})
	}
}

func authStatusHandler(protected bool, expectedToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authenticated := false
		if !protected {
			authenticated = true
		} else {
			authenticated = isAuthenticated(r, expectedToken)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store, max-age=0")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"protected":     protected,
			"authenticated": authenticated,
		})
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		} else {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Auth-Token")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func maxBytesMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// limit API bodies to 10MB (CSV), 1MB for JSON
		if strings.HasPrefix(r.URL.Path, "/api/") {
			// CSV import can be larger (11MiB to allow multipart overhead for 10MiB file)
			if strings.HasPrefix(r.URL.Path, "/api/import/") {
				r.Body = http.MaxBytesReader(w, r.Body, 11<<20)
			} else {
				r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
			}
		}
		next.ServeHTTP(w, r)
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
