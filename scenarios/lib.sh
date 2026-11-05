#!/usr/bin/env bash
# Shared helpers for scenarios. API defaults to the local stack.
set -euo pipefail
# ports come from .env (written by make up / make env) so scripts also work when run directly
_env="$(dirname "${BASH_SOURCE[0]}")/../.env"
if [[ -f "$_env" ]]; then set -a; source "$_env"; set +a; fi
API="${API:-http://localhost:${API_PORT:-8080}}"
WORKER="${WORKER:-http://localhost:${WORKER_PORT:-8081}}"

# post <path> <json> : prints "HTTP <code> | <message> | trace_id=<id>"
post() {
  local out code body
  out=$(curl -s -w '\n%{http_code}' -X POST "$API$1" -H 'Content-Type: application/json' -d "$2")
  code=$(tail -n1 <<<"$out")
  body=$(sed '$d' <<<"$out")
  printf '  %-18s HTTP %s | %s | trace_id=%s\n' "$1" "$code" \
    "$(jq -r .message <<<"$body")" "$(jq -r .trace_id <<<"$body")"
}

state() {
  printf '  state %s  api=%s  worker=%s\n' "$1" \
    "$(curl -s "$API/state/$1" | jq -c .)" "$(curl -s "$WORKER/state/$1" | jq -c '{credit}')"
}

title() { printf '\n== %s ==\n' "$1"; }
