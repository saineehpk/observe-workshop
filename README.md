# Observe Workshop

Workshop ฝึกทำ observability กับ flow ที่มีเรื่องเงิน: ถอนเงิน ย้ายกระเป๋า และรับรางวัลเช็คอิน
ทุกอย่างรันในเครื่องตัวเอง ข้อมูลทั้งหมดเป็นข้อมูลจำลอง

เป้าหมายของ workshop: **รู้แค่ username หรือ trace_id ต้องตอบได้ 4 ข้อ**

1. พังที่ขั้นไหน (`stage`)
2. เพราะอะไร (`error_class` + ตัวเลขที่ระบบใช้ตัดสิน)
3. เงินขยับหรือยัง (`money_effect` + ยอดก่อน/หลัง)
4. ผู้เล่นได้คำตอบอะไรกลับไป

## สิ่งที่ต้องมี

- Docker Desktop (หรือ Docker Engine + Compose v2)
- Claude Code
- `jq`, `curl`, `make`
- (แนะนำ) Go 1.24+ เพื่อติดตั้ง `mcp-grafana` ถ้าไม่มี สคริปต์จะใช้ Docker image `mcp/grafana` แทน

```bash
go install github.com/grafana/mcp-grafana/cmd/mcp-grafana@latest
```

## เริ่มใช้งาน

```bash
git clone https://github.com/saineehpk/observe-workshop.git
cd observe-workshop
make start       # เปิด stack + ต่อ Claude (MCP grafana-local) + เติมข้อมูล 3 นาทีเบื้องหลัง
make scenarios   # รัน scenario ทั้ง 4 แล้วจด trace_id
claude           # เปิด Claude Code ในโฟลเดอร์นี้ แล้วเริ่มถามได้เลย
```

ไม่ต้องตั้งค่าอะไร `make start` พิมพ์ URL ออกมาให้ ปกติคือ:

- Grafana: http://localhost:3000 ไม่ต้อง login ก็เปิด **Explore** ได้ (admin / admin ถ้าต้องตั้งค่า)
- API: http://localhost:8080 · Worker: http://localhost:8081 · Prometheus: http://localhost:9090

ถ้าพอร์ตไหนมีโปรแกรมอื่นใช้อยู่ จะเลือกพอร์ตว่างให้เอง (เช่น Grafana 3000 → 3300) แล้วเก็บไว้ใน `.env`
ทุกคำสั่ง `make` และ scenario ใช้ค่าเดียวกัน ดูค่าที่ได้ด้วย `cat .env` (ความหมายของแต่ละค่าอยู่ใน `.env.example`)
แต่ละโฟลเดอร์เป็น docker project แยกกัน clone ไว้สองที่ก็รันพร้อมกันได้ log ไม่ปนกัน

ใน Explore ของ Loki ทุกบรรทัดที่มี `trace_id` กดขยายแล้วจะมีลิงก์ **ดู log ทั้ง trace** เปิด log ของทุก service ใน trace นั้น

`make help` แสดงคำสั่งทั้งหมด

## ระบบจำลองมีอะไร

```text
             ┌───────────────────────────── demo-customer-api ─────────────────────────────┐
player ──►   │ POST /withdraw ──RPC (timeout 2s)──► demo-withdraw-worker  (ตัดเครดิตจริง)   │
             │ POST /pocket/exchange   (เขียน ledger ที่มี unique hash)                    │
             │ POST /checkin/claim ──► statement-consumer (อ่านจาก replica ที่ช้า 500ms)    │
             └─────────────────────────────────────────────────────────────────────────────┘
stdout (JSON) ──► Alloy ──► Loki        /metrics ──► Prometheus        ทั้งคู่ ──► Grafana ──► MCP ──► Claude
```

| Scenario | คำสั่ง | สิ่งที่ผู้เล่นเห็น |
|---|---|---|
| 01 happy path | `make s01` | ทุกอย่างสำเร็จ |
| 02 ย้ายกระเป๋าสองครั้ง | `make s02` | ครั้งที่สองได้ "เกิดข้อผิดพลาด" |
| 03 เช็คอินสองครั้ง | `make s03` | รับรางวัลได้สองครั้งในวันเดียว |
| 04 ถอนตอนปลายทางช้า | `make s04` | ได้ "รายการกำลังดำเนินการ" |

แต่ละ scenario พิมพ์ `trace_id` ออกมา ใช้ค้นต่อใน Loki หรือถาม Claude ได้

## เติมข้อมูลให้ Grafana

| คำสั่ง | ทำอะไร |
|---|---|
| `make seed` | **รันก่อนเริ่มคลาส** ยิง traffic เบื้องหลังประมาณ 10 นาที: ปกติ 4 นาที → ปลายทางช้า 90 วินาที → ปกติ 4 นาที ดูความคืบหน้าด้วย `make seed-status` |
| `make seed-fast` | แบบสั้นประมาณ 3 นาที: ปกติ 60 วินาที → ปลายทางช้า 40 วินาที → ปกติ 60 วินาที ยิงถี่ขึ้น 3 เท่า (`RATE_SLEEP=0.1`) spike สั้นมาก ต้องดูด้วย `rate(...[1m])` |
| `make traffic` | traffic สุ่ม 2 นาที (timeout ประมาณ 5% ของ withdraw) |
| `make spike` | traffic 90 วินาที ที่ withdraw ช้าจน timeout 40% |

traffic มีเคสปนอยู่แล้ว: ยอดไม่พอ (`insufficient_*`), user ที่ไม่มีอยู่จริง, ย้ายกระเป๋าซ้ำในนาทีเดียว และเช็คอินที่ถูกเขียนทับ
ปรับได้ด้วย env: `TIMEOUT_PCT`, `RATE_SLEEP`, `NORMAL_SECONDS`, `SPIKE_SECONDS`, `SPIKE_TIMEOUT_PCT`

`make seed` และ `make seed-fast` เขียน annotation tag `spike` ไว้ใน Grafana บอกช่วงที่ปลายทางช้า ใช้เป็นเฉลยของแบบฝึกข้อ 5

log และ metric เก็บใน Docker volume: `make stop` ปิดแล้วข้อมูลยังอยู่ ส่วน `make down` ลบทิ้งหมด

## สองโหมดของ log

| โหมด | คำสั่ง | หน้าตา |
|---|---|---|
| legacy | `make legacy` | `withdraw rpc error: context deadline exceeded` ไม่มี trace_id ไม่มีโครงสร้าง |
| business | `make business` | JSON record ตาม [docs/record-contract.md](docs/record-contract.md) |

สลับโหมดแล้วรัน scenario เดิม จะเห็นว่าแต่ละโหมดตอบ 4 ข้อข้างบนได้ต่างกันแค่ไหน

## เอกสาร

- [docs/record-contract.md](docs/record-contract.md) โครง business record และกฎ level
- [docs/exercises.md](docs/exercises.md) แบบฝึกหัด
- [docs/claude-prompts.md](docs/claude-prompts.md) prompt ตัวอย่างสำหรับให้ Claude ดึง observe

## query ที่ใช้บ่อย

```logql
{namespace="workshop"} |= "<trace_id>"
{service_name="demo-customer-api"} | json | outcome!="accepted"
sum by (operation, error_class) (count_over_time({namespace="workshop"} | json | stage="completed" [5m]))
```

```promql
sum by (operation, outcome, error_class, money_effect) (rate(workshop_business_records_total{stage="completed"}[5m]))
```

## ปัญหาที่เจอบ่อย

| อาการ | วิธีแก้ |
|---|---|
| `port is already allocated` | มีโปรแกรมเปิดพอร์ตนั้นหลังจากที่ `.env` ถูกสร้างไปแล้ว ให้ `make stop` → `make env` (เลือกพอร์ตว่างใหม่) → `make up` → `make mcp` ต้อง `make stop` ก่อนเสมอ ไม่งั้น container ที่ค้างจากรอบที่พังอาจไม่ได้ต่อ network |
| Prometheus ไม่มีข้อมูล หรือ Grafana ขึ้น 502 | `docker compose up -d --force-recreate prometheus` |
| `make seed` บอกว่า could not write annotation | Grafana ไม่ได้อยู่ที่พอร์ตใน `.env` ดูด้วย `cat .env` ถ้าไม่ตรงให้ `make stop` → `make env` → `make up` |
| ยอดเงินเพี้ยนจากรอบก่อน | `make reset` |

## ปิด

```bash
make stop    # ปิด เก็บข้อมูลไว้
make down    # ปิด และลบ log/metric ทั้งหมด
```

### ล้างทุกอย่างหลังจบ workshop (หรือเริ่มใหม่จากศูนย์)

```bash
make clean                       # หยุด seed, ลบ container/network/volume/image ของ api/worker, MCP grafana-local และ .env
cd .. && rm -rf observe-workshop # ลบโฟลเดอร์ repo (ถ้าไม่ใช้แล้ว)
```

เช็คว่าไม่เหลืออะไรค้าง: `docker ps -a --filter name=observe-workshop` และ `docker volume ls --filter name=observe-workshop` ต้องว่าง
(ชื่อ project = ชื่อโฟลเดอร์ ถ้า clone ไว้ชื่ออื่นให้เปลี่ยนตาม)
