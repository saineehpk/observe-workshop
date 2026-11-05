#!/usr/bin/env bash
# Create a Viewer service account in the local Grafana and register it as the
# "grafana-local" MCP server for Claude Code, scoped to this directory.
set -euo pipefail
cd "$(dirname "$0")/.."
if [[ -f .env ]]; then set -a; source .env; set +a; fi   # GRAFANA_PORT picked by make up
GRAFANA="${GRAFANA:-http://localhost:${GRAFANA_PORT:-3000}}"
AUTH="admin:admin"

until curl -sf "$GRAFANA/api/health" >/dev/null; do echo "waiting for Grafana..."; sleep 2; done

sa_id=$(curl -sf -u "$AUTH" "$GRAFANA/api/serviceaccounts/search?query=mcp" \
  | jq -r '.serviceAccounts[] | select(.name=="mcp") | .id' | head -n1)
if [[ -z "$sa_id" ]]; then
  sa_id=$(curl -sf -u "$AUTH" -H 'Content-Type: application/json' -X POST "$GRAFANA/api/serviceaccounts" \
    -d '{"name":"mcp","role":"Viewer"}' | jq -r .id)
fi
sa_token=$(curl -sf -u "$AUTH" -H 'Content-Type: application/json' -X POST \
  "$GRAFANA/api/serviceaccounts/$sa_id/tokens" -d "{\"name\":\"mcp-$(date +%s)\"}" | jq -r '.["key"]')

claude mcp remove grafana-local -s local >/dev/null 2>&1 || true
bin=$(command -v mcp-grafana || true)
[[ -z "$bin" && -x "$HOME/go/bin/mcp-grafana" ]] && bin="$HOME/go/bin/mcp-grafana"

if [[ -n "$bin" ]]; then
  claude mcp add grafana-local -s local \
    -e GRAFANA_URL="$GRAFANA" -e GRAFANA_SERVICE_ACCOUNT_TOKEN="$sa_token" -- "$bin"
else
  echo "mcp-grafana not found, using the mcp/grafana Docker image"
  claude mcp add grafana-local -s local \
    -e GRAFANA_URL="${GRAFANA/localhost/host.docker.internal}" -e GRAFANA_SERVICE_ACCOUNT_TOKEN="$sa_token" \
    -- docker run --rm -i -e GRAFANA_URL -e GRAFANA_SERVICE_ACCOUNT_TOKEN mcp/grafana -t stdio
fi
echo "registered grafana-local. Check with: claude mcp list"
