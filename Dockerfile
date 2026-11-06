# syntax=docker/dockerfile:1
# Multi-stage build for habit-go (Go + Vite)

# Build stages run on the build machine and cross-compile for the target (D3):
# arm64 images do not need QEMU for the Go compile.

# Stage 1: build frontend (output does not depend on the target architecture)
FROM --platform=$BUILDPLATFORM node:24-alpine AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend ./
RUN npm run build

# Stage 2: build Go binary (embeds frontend/dist)
FROM --platform=$BUILDPLATFORM golang:1-alpine AS go-builder
ARG TARGETOS
ARG TARGETARCH
ENV GOTOOLCHAIN=auto
WORKDIR /app
# dependencies first for better caching
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend-builder /app/frontend/dist ./frontend/dist
# pure Go build (modernc sqlite, no CGO) - static, works on scratch
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o habit-go .

# Stage 3: pre-create /app/data with correct owner for non-root scratch
FROM --platform=$BUILDPLATFORM alpine:3.24 AS perms
RUN mkdir -p /app/data && chown -R 65532:65532 /app

# Stage 4: minimal runtime from scratch
FROM scratch
COPY --from=perms /app /app
WORKDIR /app
COPY --from=go-builder --chown=65532:65532 /app/habit-go ./
# scratch has no shell or wget: the binary checks itself (D5)
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 CMD ["/app/habit-go", "-healthcheck"]
USER 65532:65532
ENV PORT=8080
ENV DB_PATH=data/habits.db
EXPOSE 8080
VOLUME ["/app/data"]
CMD ["./habit-go"]
