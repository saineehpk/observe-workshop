#!/usr/bin/env bash
# One command per exercise:  make ex1 | ex2 | ex3   then   make hint
# Switches to business records if needed, resets balances, plays the case on one website, and prints the
# ticket the way Support sends it: website + username + time + symptom, no trace_id.
# Another player on the same website does the same thing around the same time, like in production.
# `make hint` prints the trace_id and a ready-to-paste prompt for the last exercise.
# Safe to re-run: every run starts from the same state.
source "$(dirname "$0")/lib.sh"
here="$(cd "$(dirname "$0")" && pwd)"
cd "$here/.."
ex="${1:?usage: ex.sh ex1|ex2|ex3|hint}"
state_dir=.ex

grafana_loki() {
  curl -s -u admin:admin -G "http://localhost:${GRAFANA_PORT:-3000}/api/datasources/proxy/uid/loki/loki/api/v1/query_range" \
    --data-urlencode "query=$1" --data-urlencode "start=$(( ($(date +%s) - 900) * 1000000000 ))" --data-urlencode limit=1
}

if [[ "$ex" == "hint" ]]; then
  [[ -f $state_dir/last ]] || { echo "ยังไม่ได้รันแบบฝึก ลอง make ex1 ก่อน" >&2; exit 1; }
  source $state_dir/last
  cat <<EOF
[hint $EX] trace_id: $TID   (เว็บ $TSITE · $TUSER · $TTIME)
  หาเองได้ด้วย:  {namespace="$TSITE", service_name="demo-customer-api"} | json | username="$TUSER"
  ทั้ง trace:    {namespace="$TSITE"} |= "$TID"

  prompt สำหรับ claude:
  ใช้ grafana-local ดึง log ทุกบรรทัดของ trace_id $TID จาก Loki (namespace $TSITE)
  เรียงตามเวลา แล้ววาด flow ทีละ stage ข้าม service
  จากนั้นตอบ 4 ข้อ: พังขั้นไหน / เพราะอะไร / เงินขยับไหม / ผู้เล่นได้คำตอบอะไร
  ติดป้าย Observed / Inferred / Unknown ทุกข้อ ข้อไหนตอบไม่ได้ บอกว่าขาด field อะไร
EOF
  exit 0
fi

if [[ -f .seed.pid ]] && kill -0 "$(cat .seed.pid)" 2>/dev/null; then
  echo "[$ex] seed กำลังเติมข้อมูลอยู่ รอให้จบก่อน (make seed-status) หรือหยุดด้วย make seed-stop" >&2
  exit 1
fi

# case per exercise: website, player, the scenario, which response is the case, the symptom,
# and a normal request by another player on the same website at the same time (the decoy)
case "$ex" in
  ex1)
    site=web-alpha; user=alpha0004; script=02-duplicate-same-minute.sh; pick='HTTP 500'
    symptom='ย้ายยอด 50 จากกระเป๋าเข้าเครดิต ครั้งที่สองขึ้น "เกิดข้อผิดพลาด กรุณาลองใหม่"'
    decoy=(/pocket/exchange '{"username":"alpha0008","amount":50}') ;;
  ex2)
    site=web-charlie; user=charlie0005; script=03-stale-read-overwrite.sh; pick='/checkin/claim'
    symptom='(ทีมบัญชีแจ้ง) ผู้เล่นรับรางวัลเช็คอินได้ 2 ครั้งในวันเดียว'
    decoy=(/checkin/claim '{"username":"charlie0009"}') ;;
  ex3)
    site=web-bravo; user=bravo0006; script=04-rpc-timeout-unknown.sh; pick='/withdraw '
    symptom='ถอน 200 แล้วขึ้น "รายการกำลังดำเนินการ กรุณาตรวจสอบประวัติการถอน" ไม่แน่ใจว่าเงินถูกตัดหรือยัง'
    decoy=(/withdraw '{"username":"bravo0007","amount":200,"delay_ms":150}') ;;
  *) echo "unknown exercise $ex" >&2; exit 1 ;;
esac

mode=business
apps=(api-alpha worker-alpha api-bravo worker-bravo api-charlie worker-charlie)
current="$(docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' "$(docker compose ps -q api-alpha)" 2>/dev/null | sed -n 's/^OBSERVE_MODE=//p')"
echo "[$ex] คืนยอดเงินเริ่มต้นทุกเว็บ"
if [[ "$current" == "$mode" ]]; then
  # same mode: restart keeps the containers, so Alloy keeps every log line of earlier runs
  docker compose restart "${apps[@]}" >/dev/null 2>&1
else
  OBSERVE_MODE=$mode docker compose up -d "${apps[@]}" >/dev/null 2>&1
fi
for s in "${SITES[@]}"; do
  for _ in $(seq 1 60); do curl -sf -H "X-Site: $s" "$API/state/${s#web-}0001" >/dev/null && break; sleep 0.5; done
done

# the pocket case needs both requests inside the same minute
if [[ "$ex" == "ex1" ]] && (( 10#$(date +%S) >= 50 )); then
  echo "[$ex] รอขึ้นนาทีใหม่ $((60 - 10#$(date +%S))) วินาที (เคสนี้ต้องเกิดภายในนาทีเดียวกัน)"
  sleep $((60 - 10#$(date +%S)))
fi

echo "[$ex] จำลองเหตุการณ์ (มีผู้เล่นคนอื่นบนเว็บเดียวกันทำรายการแบบเดียวกันในเวลาใกล้กัน)"
SITE=$site post "${decoy[@]}" >/dev/null
mkdir -p $state_dir
when="$(date +%H:%M)"
SITE=$site "$here/$script" > "$state_dir/$ex.out"
tid="$(grep -m1 -- "$pick" "$state_dir/$ex.out" | grep -o 'trace_id=[0-9a-f]*' | cut -d= -f2 || true)"
printf 'EX=%s\nTSITE=%s\nTUSER=%s\nTTIME=%s\nTID=%s\n' "$ex" "$site" "$user" "$when" "$tid" > "$state_dir/last"

# wait until this run's logs are in Loki, so searching right away finds them
want="{namespace=\"$site\"} |= \"$tid\""
printf '[%s] รอ log เข้า Loki' "$ex"
for _ in $(seq 1 40); do
  grafana_loki "$want" | jq -e '.data.result | length > 0' >/dev/null 2>&1 && break
  printf '.'; sleep 1
done
echo

cat <<EOF

┌─ ticket ─────────────────────────────────────────────
│ เว็บ:     $site
│ ผู้เล่น:   $user
│ เวลา:     ประมาณ $when
│ อาการ:    $symptom
└──────────────────────────────────────────────────────

  prompt สำหรับ claude (ก๊อปไปวางได้เลย):
  ลูกค้าเว็บ $site (namespace $site) username $user แจ้งว่า $symptom ประมาณ $when
  ใช้ grafana-local หา trace_id ของเคสนี้จาก Loki แล้วดึง log ทุกบรรทัดของ trace นั้น
  เรียงตามเวลา วาด flow ทีละ stage ข้าม service แล้วตอบ 4 ข้อ:
  พังขั้นไหน / เพราะอะไร / เงินขยับไหม / ผู้เล่นได้คำตอบอะไร
  ติดป้าย Observed / Inferred / Unknown ทุกข้อ และแสดง query ที่ใช้

  โจทย์: docs/exercises.md ข้อ ${ex:2:1} · หาไม่เจอ: make hint
EOF
