.PHONY: dev build frontend ensure-dist test clean clean-data run fmt vet install

PORT ?= 8080
DB_PATH ?= data/habits.db
PASSWORD ?=

# Build frontend then Go binary (embeds frontend/dist via go:embed)
build: frontend
	go build -o habit-go .

frontend:
	cd frontend && npm ci && npm run build

# go:embed fails on a missing or empty frontend/dist: create a placeholder for dev (D1)
ensure-dist:
	@mkdir -p frontend/dist
	@test -f frontend/dist/index.html || echo '<!doctype html><title>Run make frontend</title>' > frontend/dist/index.html

# Dev: run Go backend and Vite frontend concurrently (frontend proxies /api to :8080)
dev: ensure-dist
	@echo "Starting backend on :$(PORT) and frontend on :5173..."
	@trap 'jobs -p | xargs -r kill' INT TERM EXIT; \
	PORT=$(PORT) DB_PATH=$(DB_PATH) PASSWORD=$(PASSWORD) go run . & \
	cd frontend && npm run dev & \
	wait

# Run built binary
run: build
	PORT=$(PORT) DB_PATH=$(DB_PATH) ./habit-go

test: frontend
	go vet ./...
	go test ./... -count=1
	cd frontend && npm run typecheck && npm test

vet:
	go vet ./...

fmt:
	go fmt ./...

clean:
	rm -f habit-go
	rm -rf frontend/dist

# C16: deleting the database is separate from clean and must be asked for explicitly
clean-data:
	@test "$(CONFIRM)" = "1" || (echo "Refused: this deletes your habits. Run: make clean-data CONFIRM=1" && exit 1)
	rm -f data/*.db data/*.db-wal data/*.db-shm

install:
	cd frontend && npm install
	go mod download
