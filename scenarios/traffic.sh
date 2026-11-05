#!/usr/bin/env bash
# traffic พื้นหลังแบบสุ่ม: ./scenarios/traffic.sh [seconds]
#   TIMEOUT_PCT  % ของ withdraw บน web-bravo ที่ช้าจนเกิน RPC timeout 2s (ค่าเริ่มต้น 5)
#                เว็บอื่นช้าแค่ 1% (ปลายทางของ bravo ช้ากว่าเว็บอื่น)
#   RATE_SLEEP   เว้นระหว่าง request เป็นวินาที (ค่าเริ่มต้น 0.3 ≈ 3 req/s)
# สัดส่วน: withdraw 40% · pocket 30% · check-in 30% และมีบางคำขอที่ยอดเกิน
# (insufficient_*) หรือเป็น user ที่ไม่มีอยู่จริง (unknown_user)
source "$(dirname "$0")/lib.sh"
end=$((SECONDS + ${1:-120}))
timeout_pct="${TIMEOUT_PCT:-5}"
sites=(web-alpha web-bravo web-charlie)
n=0
send() { curl -s -o /dev/null -X POST "$API$1" -H "X-Site: $site" -H 'Content-Type: application/json' -d "$2" & }

while (( SECONDS < end )); do
  site=${sites[RANDOM % 3]}
  pct=1; [[ "$site" == web-bravo ]] && pct=$timeout_pct
  # users are named after their website (bravo0012); 1% use another site's username -> unknown_user
  u_site=$site; (( RANDOM % 100 == 0 )) && u_site=${sites[RANDOM % 3]}
  u="${u_site#web-}$(printf '%04d' $((RANDOM % 40 + 10)))"
  r=$((RANDOM % 10))
  if (( r < 4 )); then
    amount=$((RANDOM % 5 * 10 + 10)); (( RANDOM % 25 == 0 )) && amount=5000
    delay=$((RANDOM % 250 + 50)); (( RANDOM % 100 < pct )) && delay=2600
    send /withdraw "{\"username\":\"$u\",\"amount\":$amount,\"delay_ms\":$delay}"
  elif (( r < 7 )); then
    amount=$((RANDOM % 3 * 10 + 10)); (( RANDOM % 25 == 0 )) && amount=900
    send /pocket/exchange "{\"username\":\"$u\",\"amount\":$amount}"
  else
    send /checkin/claim "{\"username\":\"$u\"}"
  fi
  n=$((n + 1))
  sleep "${RATE_SLEEP:-0.3}"
done
wait
echo "sent $n requests (timeout_pct=$timeout_pct)"
