.PHONY: dev build frontend test clean run fmt vet install

PORT ?= 8080
DB_PATH ?= data/habits.db
PASSWORD ?=

# Build frontend then Go binary (embeds frontend/dist via go:embed)
build: frontend
	go build -o habit-go .

frontend:
	cd frontend && npm ci && npm run build

# Dev: run Go backend and Vite frontend concurrently (frontend proxies /api to :8080)
dev:
	@echo "Starting backend on :$(PORT) and frontend on :5173..."
	@trap 'jobs -p | xargs -r kill' INT TERM EXIT; \
	PORT=$(PORT) DB_PATH=$(DB_PATH) PASSWORD=$(PASSWORD) go run . & \
	cd frontend && npm run dev & \
	wait

# Run built binary
run: build
	PORT=$(PORT) DB_PATH=$(DB_PATH) ./habit-go

test:
	go test ./... -count=1 && go vet ./... && cd frontend && npm run build

vet:
	go vet ./...

fmt:
	go fmt ./...

clean:
	rm -f habit-go
	rm -rf frontend/dist
	rm -rf data/*.db data/*.db-wal data/*.db-shm

install:
	cd frontend && npm install
	go mod download
