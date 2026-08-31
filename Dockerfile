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
# pure Go build (modernc sqlite, no CGO) - static, works on scratch
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o habit-go .

# Stage 3: pre-create /app/data with correct owner for non-root scratch
FROM alpine:3.24 AS perms
RUN mkdir -p /app/data && chown -R 65532:65532 /app

# Stage 4: minimal runtime from scratch
FROM scratch
COPY --from=perms /app /app
WORKDIR /app
COPY --from=go-builder --chown=65532:65532 /app/habit-go ./
# scratch has no shell/wget -> no HEALTHCHECK (use external check or disable)
# previous alpine healthcheck used wget which is unavailable in scratch
# HEALTHCHECK NONE disables the default
HEALTHCHECK NONE
USER 65532:65532
ENV PORT=8080
ENV DB_PATH=data/habits.db
EXPOSE 8080
VOLUME ["/app/data"]
CMD ["./habit-go"]
