#!/usr/bin/env bash
set -e
# run.sh - compile and run habit-go with password protection (password=test by default)
# Usage:
#   ./run.sh                       # runs with PASSWORD=test, PORT=8080
#   PASSWORD=mypass ./run.sh       # custom password via env
#   ./run.sh --password mypass --port 3000
#   ./run.sh --no-build            # skip build step

PASSWORD_VAL="${PASSWORD:-test}"
PORT_VAL="${PORT:-8080}"
DB_PATH_VAL="${DB_PATH:-data/habits.db}"
NEED_BUILD=true

# parse optional args: --password, --port, --db, --no-build
while [[ $# -gt 0 ]]; do
  case "$1" in
    --password|-p)
      PASSWORD_VAL="$2"; shift 2;;
    --port)
      PORT_VAL="$2"; shift 2;;
    --db)
      DB_PATH_VAL="$2"; shift 2;;
    --no-build)
      NEED_BUILD=false; shift;;
    -h|--help)
      echo "Usage: $0 [--password PASS] [--port PORT] [--db PATH] [--no-build]"
      echo "  Env vars PASSWORD, PORT, DB_PATH also respected (default password=test)"
      exit 0;;
    *)
      # pass through as password if positional
      if [[ "$1" == --* ]]; then echo "Unknown option $1"; exit 1; fi
      PASSWORD_VAL="$1"; shift;;
  esac
done

ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT"

if [[ "$NEED_BUILD" == true ]]; then
  echo "==> Building frontend..."
  (cd frontend && npm ci && npm run build)
  echo "==> Building Go binary (habit-go)..."
  go build -o habit-go .
else
  echo "==> Skipping build (--no-build)"
fi

if [[ ! -f habit-go ]]; then
  echo "Error: habit-go binary not found. Run without --no-build first." >&2
  exit 1
fi

if [[ -n "$PASSWORD_VAL" ]]; then
  echo "==> Starting habit-go on :$PORT_VAL with DB $DB_PATH_VAL (password protection ENABLED)"
else
  echo "==> Starting habit-go on :$PORT_VAL with DB $DB_PATH_VAL (no password)"
fi
echo "    Open http://localhost:$PORT_VAL"
echo "    Press Ctrl+C to stop"

# Use env var for password to avoid exposing in ps output
PASSWORD="$PASSWORD_VAL" PORT="$PORT_VAL" DB_PATH="$DB_PATH_VAL" ./habit-go -port "$PORT_VAL" -db "$DB_PATH_VAL"
