#!/usr/bin/env bash
# Shared helpers for scenarios. API defaults to the local stack.
set -euo pipefail
# ports come from .env (written by make up / make env) so scripts also work when run directly
_env="$(dirname "${BASH_SOURCE[0]}")/../.env"
if [[ -f "$_env" ]]; then set -a; source "$_env"; set +a; fi
# every request goes through the gateway; SITE picks the website (namespace)
API="${API:-http://localhost:${API_PORT:-8080}}"
SITE="${SITE:-web-alpha}"
SITES=(web-alpha web-bravo web-charlie)

# post <path> <json> : prints "HTTP <code> | <message> | trace_id=<id>" (on website $SITE)
post() {
  local out code body
  out=$(curl -s -w '\n%{http_code}' -X POST "$API$1" -H "X-Site: $SITE" -H 'Content-Type: application/json' -d "$2")
  code=$(tail -n1 <<<"$out")
  body=$(sed '$d' <<<"$out")
  printf '  %-18s HTTP %s | %s | trace_id=%s\n' "$1" "$code" \
    "$(jq -r .message <<<"$body")" "$(jq -r .trace_id <<<"$body")"
}

state() {
  printf '  state %s@%s  api=%s  worker=%s\n' "$1" "$SITE" \
    "$(curl -s -H "X-Site: $SITE" "$API/state/$1" | jq -c .)" "$(curl -s -H "X-Site: $SITE" "$API/worker/state/$1" | jq -c '{credit}')"
}

# user N : username N on website $SITE (web-bravo + 6 -> bravo0006)
user() { printf '%s%04d' "${SITE#web-}" "$1"; }

title() { printf '\n== %s ==\n' "$1"; }
