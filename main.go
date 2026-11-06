package main

import (
	"embed"
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
	trustProxy := os.Getenv("TRUST_PROXY") == "true"
	corsAllowed := map[string]bool{}
	for _, o := range strings.Split(os.Getenv("CORS_ORIGINS"), ",") {
		if o = strings.TrimSpace(o); o != "" {
			corsAllowed[o] = true
		}
	}

	database, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	defer database.Close()

	auth := &authConfig{password: password, trustProxy: trustProxy, limiter: newLoginLimiter()}
	if auth.protected() {
		secret, err := db.AppSecret(database)
		if err != nil {
			log.Fatalf("app secret: %v", err)
		}
		auth.token = tokenFor(secret, password)
		log.Printf("Password protection ENABLED")
	} else {
		log.Printf("Password protection DISABLED (no password set)")
	}

	habitRepo := habit.NewRepository(database)
	entryRepo := entry.NewRepository(database)

	habitHandler := habit.NewHandler(habitRepo, entryRepo)
	entryHandler := entry.NewHandler(entryRepo)
	csvHandler := csvhandler.NewHandler(database)

	mux := http.NewServeMux()

	// --- Auth routes (always public) ---
	mux.HandleFunc("POST /api/login", auth.login)
	mux.HandleFunc("POST /api/logout", auth.logout)
	mux.HandleFunc("GET /api/auth/status", auth.status)

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
		if strings.HasPrefix(r.URL.Path, "/api/") {
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

	// Wrap with logging -> CORS -> body limit -> auth
	handler := loggingMiddleware(corsMiddleware(corsAllowed, maxBytesMiddleware(auth.middleware(mux))))

	log.Printf("Starting server on :%s with DB %s", port, dbPath)
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
