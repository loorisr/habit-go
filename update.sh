#!/usr/bin/env bash
set -euo pipefail
# update.sh - update Go modules and frontend npm packages to latest versions.
# Usage:
#   ./update.sh                    # update both Go and frontend to latest
#   ./update.sh --go-only          # only Go modules
#   ./update.sh --frontend-only    # only frontend npm packages
#   ./update.sh --check            # show outdated deps without changing anything
#   ./update.sh --skip-verify      # skip build verification at the end
#
# Go: uses `go get -u ./...` + `go mod tidy` (respects go.mod toolchain).
# npm: uses `npx npm-check-updates -u` to bump package.json to latest,
#      then `npm install`. Falls back to `npm update` if ncu can't run (offline).

UPDATE_GO=true
UPDATE_FRONTEND=true
CHECK_ONLY=false
SKIP_VERIFY=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --go-only) UPDATE_FRONTEND=false; shift;;
    --frontend-only) UPDATE_GO=false; shift;;
    --check) CHECK_ONLY=true; shift;;
    --skip-verify) SKIP_VERIFY=true; shift;;
    -h|--help)
      echo "Usage: $0 [--go-only] [--frontend-only] [--check] [--skip-verify]"
      echo "  --go-only        Update Go modules only"
      echo "  --frontend-only  Update frontend npm packages only"
      echo "  --check          Show outdated deps, don't modify anything"
      echo "  --skip-verify    Skip 'go build' + frontend build verification"
      exit 0;;
    *) echo "Unknown option $1 (try --help)" >&2; exit 1;;
  esac
done

ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT"

command_exists() { command -v "$1" >/dev/null 2>&1; }

if [[ "$UPDATE_GO" == true ]]; then
  if ! command_exists go; then
    echo "Error: 'go' not found in PATH, skipping Go update." >&2
  elif [[ "$CHECK_ONLY" == true ]]; then
    echo "==> Go: outdated modules (go list -m -u all)"
    go list -m -u all
  else
    echo "==> Go: updating modules to latest..."
    echo "    go version: $(go version)"
    go get -u ./...
    go mod tidy
    echo "==> Go: updated. Current modules:"
    go list -m all
  fi
fi

if [[ "$UPDATE_FRONTEND" == true ]]; then
  if ! command_exists npm; then
    echo "Error: 'npm' not found in PATH, skipping frontend update." >&2
  elif [[ ! -f frontend/package.json ]]; then
    echo "Error: frontend/package.json not found, skipping frontend update." >&2
  elif [[ "$CHECK_ONLY" == true ]]; then
    echo "==> frontend: outdated packages (npm outdated)"
    (cd frontend && npm outdated || true)
  else
    echo "==> frontend: updating npm packages to latest..."
    echo "    npm version: $(npm --version), node: $(node --version 2>/dev/null || echo 'n/a')"
    if (cd frontend && npx --yes npm-check-updates -u); then
      echo "    package.json bumped via npm-check-updates, installing..."
      (cd frontend && npm install)
    else
      echo "    WARNING: npm-check-updates failed (offline?), falling back to 'npm update' (semver-range only)." >&2
      (cd frontend && npm update)
    fi
    echo "==> frontend: updated. Outdated remainder (empty = all latest):"
    (cd frontend && npm outdated || true)
  fi
fi

if [[ "$CHECK_ONLY" == true || "$SKIP_VERIFY" == true ]]; then
  echo "==> Done (no verification)."
  exit 0
fi

echo "==> Verifying builds..."
if [[ "$UPDATE_GO" == true ]] && command_exists go; then
  go build -o /dev/null . && echo "    Go build: OK"
fi
if [[ "$UPDATE_FRONTEND" == true ]] && command_exists npm; then
  (cd frontend && npm run build) && echo "    frontend build: OK"
fi
echo "==> All done. Review git diff (go.mod, go.sum, frontend/package.json, frontend/package-lock.json), run tests (make test), then commit."
