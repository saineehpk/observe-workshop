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
claude           # เปิด Claude Code ในโฟลเดอร์นี้ แล้วเริ่มถามได้เลย
```

อัปเดตเป็นเวอร์ชันล่าสุด: `git pull && make start` (build ใหม่ ต่อ MCP ใหม่ และเติมข้อมูลให้)

ไม่ต้องตั้งค่าอะไร `make start` พิมพ์ URL ออกมาให้ ปกติคือ:

- Grafana: http://localhost:3000 ไม่ต้อง login ก็เปิด **Explore** ได้ (admin / admin ถ้าต้องตั้งค่า)
- API (gateway ของทุกเว็บ): http://localhost:8080 เลือกเว็บด้วย header `X-Site` · Prometheus: http://localhost:9090

ถ้าพอร์ตไหนมีโปรแกรมอื่นใช้อยู่ จะเลือกพอร์ตว่างให้เอง (เช่น Grafana 3000 → 3300) แล้วเก็บไว้ใน `.env`
ทุกคำสั่ง `make` และ scenario ใช้ค่าเดียวกัน ดูค่าที่ได้ด้วย `cat .env` (ความหมายของแต่ละค่าอยู่ใน `.env.example`)
แต่ละโฟลเดอร์เป็น docker project แยกกัน clone ไว้สองที่ก็รันพร้อมกันได้ log ไม่ปนกัน

ดู log: เมนู **Explore** → `Loki` → `{namespace="web-alpha"} | json` แต่ละบรรทัดแสดงเป็น JSON มีแถบ Fields ด้านซ้ายไว้กรอง
กดขยายบรรทัดที่มี `trace_id` จะมี 2 ลิงก์:

- **Tempo** เปิด trace เป็น waterfall ของ span ทุก service (เห็นเลยว่าขั้นไหนช้า ขั้นไหน error)
- **ดู log ทั้ง trace** เปิด log ของทุก service ใน trace นั้น

ใน trace view กดไอคอน log ที่ span จะกลับมาที่ log ของ trace นั้นได้

`make help` แสดงคำสั่งทั้งหมด

## ระบบจำลองมีอะไร

จำลองหลายเว็บแบบของจริง: **1 เว็บ = 1 namespace** ทุกเว็บรันโค้ดชุดเดียวกัน แต่ config และผู้เล่นแยกกัน

| เว็บ (namespace) | service |
|---|---|
| `web-alpha` | `demo-customer-api`, `demo-withdraw-worker` |
| `web-bravo` | `demo-customer-api`, `demo-withdraw-worker` |
| `web-charlie` | `demo-customer-api`, `demo-withdraw-worker` |

username ขึ้นต้นด้วยชื่อเว็บแล้วตามด้วยเลข เช่น `alpha0004`, `bravo0006`, `charlie0005` (เว็บละ 0001–0050)
ticket จริงมีแค่ชื่อเว็บกับ username: กรองด้วย `namespace` ของเว็บนั้นก่อน แล้วค่อยกรอง username

```text
player ──► gateway (header X-Site) ──► demo-customer-api ของเว็บนั้น
             POST /withdraw ──RPC (timeout 2s, traceparent)──► demo-withdraw-worker (ตัดเครดิตจริง)
             POST /pocket/exchange   (เขียน ledger)
             POST /checkin/claim ──► statement-consumer (อัปเดตสถานะเช็คอิน)
stdout (JSON) ──► Alloy ──► Loki     /metrics ──► Prometheus     OTLP spans ──► Tempo
                     ทั้งหมด ──► Grafana ──► MCP ──► Claude
```

label `namespace` (ชื่อเว็บ) มีทั้งใน log, metric และ trace

ยิงคำขอเองได้ด้วย curl:

```bash
curl -s -H 'X-Site: web-bravo' localhost:8080/state/bravo0006
curl -s -H 'X-Site: web-bravo' -H 'Content-Type: application/json' \
  -d '{"username":"bravo0006","amount":100}' localhost:8080/withdraw
```

## แบบฝึก

`make ex1` … `make ex4` คืนยอดเงิน จำลองเคส แล้วพิมพ์ **ticket** (เว็บ + username + เวลา + อาการ) พร้อม prompt สำหรับ Claude
หาเคสไม่เจอใช้ `make hint` รายละเอียดโจทย์อยู่ใน [docs/exercises.md](docs/exercises.md)

scenario แบบไม่คืนยอดเงินยังใช้ได้: `make s01` … `make s04` (พิมพ์ trace_id ออกมาตรงๆ)

## เติมข้อมูลให้ Grafana

| คำสั่ง | ทำอะไร |
|---|---|
| `make seed` | **รันก่อนเริ่มคลาส** ยิง traffic เบื้องหลังประมาณ 10 นาที: ปกติ 4 นาที → ปลายทางช้า 90 วินาที → ปกติ 4 นาที ดูความคืบหน้าด้วย `make seed-status` |
| `make seed-fast` | แบบสั้นประมาณ 3 นาที: ปกติ 60 วินาที → ปลายทางช้า 40 วินาที → ปกติ 60 วินาที ยิงถี่ขึ้น 3 เท่า (`RATE_SLEEP=0.1`) spike สั้นมาก ต้องดูด้วย `rate(...[1m])` |
| `make traffic` | traffic สุ่มทุกเว็บ 2 นาที |
| `make spike` | traffic 90 วินาที ที่ปลายทางของเว็บหนึ่งช้าจน timeout บ่อย |

traffic มีเคสปนอยู่แล้ว: ยอดไม่พอ (`insufficient_*`), user ที่ไม่มีอยู่จริง, ย้ายกระเป๋าซ้ำในนาทีเดียว และเช็คอินที่ถูกเขียนทับ
ปรับได้ด้วย env: `TIMEOUT_PCT`, `RATE_SLEEP`, `NORMAL_SECONDS`, `SPIKE_SECONDS`, `SPIKE_TIMEOUT_PCT`

`make seed` และ `make seed-fast` เขียน annotation tag `spike` ไว้ใน Grafana บอกช่วงที่ปลายทางช้า ใช้เป็นเฉลยของแบบฝึกข้อ 5

log และ metric เก็บใน Docker volume: `make stop` ปิดแล้วข้อมูลยังอยู่ ส่วน `make down` ลบทิ้งหมด

## สองโหมดของ log

| โหมด | คำสั่ง | หน้าตา |
|---|---|---|
| legacy | `make legacy` | `withdraw rpc error: context deadline exceeded` ไม่มี trace_id ไม่มีโครงสร้าง |
| business | `make business` | JSON record ตาม [docs/record-contract.md](docs/record-contract.md) |

สลับโหมดแล้วรัน scenario เดิม (หรือใช้ `make ex1` กับ `make ex1-business`) จะเห็นว่าแต่ละโหมดตอบ 4 ข้อข้างบนได้ต่างกันแค่ไหน

## เอกสาร

- [docs/record-contract.md](docs/record-contract.md) โครง business record และกฎ level
- [docs/exercises.md](docs/exercises.md) แบบฝึกหัด
- [docs/claude-prompts.md](docs/claude-prompts.md) prompt ตัวอย่างสำหรับให้ Claude ดึง observe

## query ที่ใช้บ่อย

```logql
{namespace="web-bravo", service_name="demo-customer-api"} | json | username="bravo0006"
{namespace=~"web-.+"} |= "<trace_id>"
{namespace=~"web-.+", service_name="demo-customer-api"} | json | stage="completed" | outcome!="accepted"
sum by (namespace, operation, error_class) (count_over_time({namespace=~"web-.+"} | json | stage="completed" [5m]))
```

```promql
sum by (namespace, operation, outcome, error_class, money_effect) (rate(workshop_business_records_total{stage="completed"}[1m]))
```

## ปัญหาที่เจอบ่อย

| อาการ | วิธีแก้ |
|---|---|
| `port is already allocated` | มีโปรแกรมเปิดพอร์ตนั้นหลังจากที่ `.env` ถูกสร้างไปแล้ว ให้ `make stop` → `make env` (เลือกพอร์ตว่างใหม่) → `make up` → `make mcp` ต้อง `make stop` ก่อนเสมอ ไม่งั้น container ที่ค้างจากรอบที่พังอาจไม่ได้ต่อ network |
| Prometheus ไม่มีข้อมูล หรือ Grafana ขึ้น 502 | `docker compose up -d --force-recreate prometheus` |
| `make seed` บอกว่า could not write annotation | Grafana ไม่ได้อยู่ที่พอร์ตใน `.env` ดูด้วย `cat .env` ถ้าไม่ตรงให้ `make stop` → `make env` → `make up` |
| ยอดเงินเพี้ยนจากรอบก่อน | `make reset` (หรือ `make exN` ซึ่งคืนยอดให้เอง) |
| `make exN` บอกว่า seed กำลังเติมข้อมูลอยู่ | รอ `make seed-status` ขึ้น done หรือ `make seed-stop` |

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
