#!/usr/bin/env bash
# Writes .env so every make target, the scenario scripts and plain `docker compose`
# use the same ports. `make up` / `make start` run this automatically when .env is missing.
#
#   make env                      pick free ports: default, or default+300, +301, ... if taken
#   make env GRAFANA_PORT=3400    force one port (others still auto-picked)
#
# Run it while this folder's stack is stopped, otherwise its own ports look "taken".
set -euo pipefail
cd "$(dirname "$0")/.."

if [[ "${1:-}" == "--if-missing" && -f .env ]]; then exit 0; fi

in_use() { (exec 3<>"/dev/tcp/127.0.0.1/$1") 2>/dev/null; }
taken=" "

# pick NAME DEFAULT: keep $NAME if set, else first free port in DEFAULT, DEFAULT+300, DEFAULT+301, ...
pick() {
  local name=$1 port=$2 forced="${!1:-}"
  if [[ -n "$forced" ]]; then port=$forced
  else
    local next=$((port + 300))
    while in_use "$port" || [[ "$taken" == *" $port "* ]]; do port=$next; next=$((next + 1)); done
    if (( port != $2 )); then echo "[env] port $2 is in use, $name=$port" >&2; fi
  fi
  taken+="$port "
  printf '%s=%s\n' "$name" "$port"
}

{
  echo "# created by scripts/env.sh · read by make, scenarios and docker compose · not committed"
  pick GRAFANA_PORT 3000
  pick API_PORT 8080
  pick WORKER_PORT 8081
  pick PROMETHEUS_PORT 9090
  # one docker project per folder (same rule docker compose uses), so each clone is separate
  echo "COMPOSE_PROJECT_NAME=${COMPOSE_PROJECT_NAME:-$(basename "$PWD" | tr 'A-Z' 'a-z' | tr -cd 'a-z0-9_-')}"
} > .env
echo "[env] wrote .env ($(grep -v '^#' .env | tr '\n' ' '))"
