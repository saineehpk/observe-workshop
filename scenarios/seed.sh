#!/usr/bin/env bash
# Pre-fill Loki and Prometheus before class so Grafana already has history:
#   normal traffic → timeout spike → normal traffic
# The spike window is written as a Grafana annotation (tag "spike") so learners
# can check whether they found the same window from the data alone.
#   NORMAL_SECONDS (default 240) · SPIKE_SECONDS (default 90) · SPIKE_TIMEOUT_PCT (default 40)
source "$(dirname "$0")/lib.sh"
GRAFANA="${GRAFANA:-http://localhost:${GRAFANA_PORT:-3000}}"
normal="${NORMAL_SECONDS:-240}"
spike="${SPIKE_SECONDS:-90}"
here="$(dirname "$0")"

now_ms() { echo $(( $(date +%s) * 1000 )); }

echo "[seed] $(date +%T) phase 1: normal traffic ${normal}s"
"$here/traffic.sh" "$normal"

echo "[seed] $(date +%T) phase 2: timeout spike ${spike}s"
spike_start=$(now_ms)
TIMEOUT_PCT="${SPIKE_TIMEOUT_PCT:-40}" "$here/traffic.sh" "$spike"
spike_end=$(now_ms)

curl -s -o /dev/null -u admin:admin -H 'Content-Type: application/json' -X POST "$GRAFANA/api/annotations" \
  -d "{\"time\":$spike_start,\"timeEnd\":$spike_end,\"tags\":[\"spike\",\"workshop\"],\"text\":\"เฉลย: ช่วงที่ปลายทางช้า (TIMEOUT_PCT=${SPIKE_TIMEOUT_PCT:-40})\"}" \
  && echo "[seed] annotation 'spike' written to Grafana" \
  || echo "[seed] could not write annotation (Grafana at $GRAFANA?)"

echo "[seed] $(date +%T) phase 3: normal traffic ${normal}s"
"$here/traffic.sh" "$normal"
echo "[seed] $(date +%T) done"
