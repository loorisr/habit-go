# syntax=docker/dockerfile:1
# Multi-stage build for habit-go (Go + Vite)

# Stage 1: build frontend
FROM node:24-alpine AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend ./
RUN npm run build

# Stage 2: build Go binary (embeds frontend/dist)
FROM golang:1.27-alpine AS go-builder
ENV GOTOOLCHAIN=auto
WORKDIR /app
# dependencies first for better caching
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend-builder /app/frontend/dist ./frontend/dist
# pure Go build (modernc sqlite, no CGO)
RUN CGO_ENABLED=0 go build -o habit-go .

# Stage 3: minimal runtime
FROM alpine:3.24 AS runtime
WORKDIR /app
RUN apk add --no-cache ca-certificates wget \
 && adduser -D -s /bin/sh appuser \
 && mkdir -p data && chown appuser:appuser data
COPY --from=go-builder /app/habit-go ./
RUN chown appuser:appuser habit-go
USER appuser
ENV PORT=8080
ENV DB_PATH=data/habits.db
EXPOSE 8080
VOLUME ["/app/data"]
# use shell form so $PORT is expanded at runtime
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:${PORT:-8080}/api/health || exit 1
CMD ["./habit-go"]
