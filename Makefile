.PHONY: help start env up stop down clean reset legacy business mcp s01 s02 s03 s04 scenarios traffic spike seed seed-fast seed-status seed-stop logs

# .env (สร้างอัตโนมัติตอน make up) เก็บพอร์ตที่ว่างและชื่อ project ของโฟลเดอร์นี้
# docker compose อ่านไฟล์นี้เอง สคริปต์ใน scenarios/ และ scripts/ ก็อ่านเอง make จึงไม่ต้อง export อะไร

help:            ## แสดงคำสั่งทั้งหมด
	@grep -E '^[a-z0-9-]+:.*##' $(firstword $(MAKEFILE_LIST)) | awk -F':.*## ' '{printf "  make %-10s %s\n", $$1, $$2}'

start:           ## clone มาแล้วรันคำสั่งนี้คำสั่งเดียว: up + mcp + seed-fast (เบื้องหลัง)
	@$(MAKE) --no-print-directory up
	@$(MAKE) --no-print-directory mcp
	@$(MAKE) --no-print-directory seed-fast
	@echo; echo "พร้อมแล้ว · seed ใช้เวลาประมาณ 3 นาที (make seed-status) · จากนั้น make scenarios แล้วพิมพ์ claude"

env:             ## เลือกพอร์ตว่างใหม่ลง .env (ปกติ make up ทำให้เอง) เช่น make env GRAFANA_PORT=3400
	@./scripts/env.sh

up:              ## เปิด stack ทั้งหมด (โหมด business) พอร์ตไหนไม่ว่างจะเลือกพอร์ตอื่นให้เอง
	@./scripts/env.sh --if-missing
	docker compose up -d --build
	@. ./.env; echo "Grafana http://localhost:$$GRAFANA_PORT · API http://localhost:$$API_PORT · Worker http://localhost:$$WORKER_PORT · Prometheus http://localhost:$$PROMETHEUS_PORT"

stop:            ## ปิด stack แต่เก็บ log/metric ไว้
	docker compose down

down:            ## ปิด stack และลบ log/metric ทั้งหมด
	docker compose down -v

clean:           ## ล้างทุกอย่าง: seed, container, volume, image, MCP และ .env
	-@$(MAKE) --no-print-directory seed-stop
	docker compose down -v --rmi local
	-claude mcp remove grafana-local -s local
	rm -f .env .seed.log .seed.pid

reset:           ## รีสตาร์ต app ให้ยอดเงินกลับค่าเริ่มต้น
	docker compose restart api worker

legacy:          ## สลับ app เป็น log แบบเก่า (ก่อนมี observe)
	OBSERVE_MODE=legacy docker compose up -d api worker

business:        ## สลับ app เป็น business record
	OBSERVE_MODE=business docker compose up -d api worker

mcp:             ## ต่อ Claude Code เข้ากับ Grafana ในเครื่อง (grafana-local)
	./scripts/setup-mcp.sh

s01:             ## scenario 01 happy path
	./scenarios/01-happy-path.sh
s02:             ## scenario 02 ย้ายกระเป๋าซ้ำ
	./scenarios/02-duplicate-same-minute.sh
s03:             ## scenario 03 เช็คอินรับซ้ำ
	./scenarios/03-stale-read-overwrite.sh
s04:             ## scenario 04 ถอนช้าจน timeout
	./scenarios/04-rpc-timeout-unknown.sh
scenarios: s01 s02 s03 s04 ## รันทุก scenario

traffic:         ## ยิง traffic สุ่ม 2 นาที
	./scenarios/traffic.sh 120

spike:           ## ยิง traffic 90 วินาที ที่ withdraw ช้าจน timeout 40%
	TIMEOUT_PCT=40 ./scenarios/traffic.sh 90

seed:            ## ยิง traffic สดเบื้องหลังก่อนคลาส (~10 นาที: ปกติ 4m → spike 90s → ปกติ 4m)
seed-fast:       ## seed แบบสั้น (~3 นาที: ปกติ 60s → spike 40s → ปกติ 60s) ดูด้วย rate(...[1m])
seed-fast: export NORMAL_SECONDS ?= 60
seed-fast: export SPIKE_SECONDS ?= 40
seed-fast: export RATE_SLEEP ?= 0.1
seed seed-fast:
	@if [ -f .seed.pid ] && kill -0 $$(cat .seed.pid) 2>/dev/null; then echo "seed is already running (make seed-status)"; exit 1; fi
	@nohup ./scenarios/seed.sh > .seed.log 2>&1 & echo $$! > .seed.pid
	@echo "$@ started in background. Progress: make seed-status · stop: make seed-stop"

seed-status:     ## ดูความคืบหน้าของ seed
	@tail -n 5 .seed.log 2>/dev/null || echo "no seed run yet"

seed-stop:       ## หยุด seed กลางทาง
	@-kill $$(cat .seed.pid 2>/dev/null) 2>/dev/null; pkill -f scenarios/traffic.sh 2>/dev/null; rm -f .seed.pid; echo "seed stopped"

logs:            ## ดู log ของ app ใน terminal
	docker compose logs -f api worker
