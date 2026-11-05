# Prompt ตัวอย่าง

ใช้ใน Claude Code ที่เปิดในโฟลเดอร์นี้ หลัง `make mcp` (Claude จะเห็น tool ของ `grafana-local`)

## ไล่ flow จาก trace_id

```text
ใช้ grafana-local ดึง log ทุกบรรทัดของ trace_id <x> จาก Loki (datasource uid: loki)
เรียงตามเวลา แล้ววาด flow ทีละ stage ข้าม service
บอกว่าแต่ละจุดตัดสินใจอะไร และใช้ตัวเลขอะไรตัดสิน
```

## เช็คว่า observe ตอบครบไหม

```text
จาก record ของ trace_id <x> ตอบ 4 ข้อ:
พังขั้นไหน / เพราะอะไร / เงินขยับไหม / ผู้เล่นได้คำตอบอะไร
ติดป้าย Observed / Inferred / Unknown ทุกข้อ
ข้อไหนตอบไม่ได้ บอกว่าขาด field อะไร หรือขาด record ตรงไหน
```

## ภาพรวมจาก metric

```text
ใช้ Prometheus (uid: prometheus) สรุปอัตราต่อนาทีของ workshop_business_records_total
ใน 10 นาทีล่าสุด แยก operation × outcome × error_class × money_effect (เฉพาะ stage="completed")
แล้วชี้ว่าแถวไหนเงินเสี่ยง (money_effect เป็น confirmed หรือ unknown แต่ outcome ไม่ใช่ accepted)
```

## หาสาเหตุจากอาการ

```text
ผู้เล่น <username> แจ้งว่า "<อาการ>" ช่วงเวลา <เวลา>
หา record ที่เกี่ยวใน Loki แล้วเสนอสมมติฐาน 2–3 ข้อ
แต่ละข้อบอกว่า log ไหนสนับสนุน log ไหนค้าน และต้องดูอะไรเพิ่มเพื่อตัดสิน
ยังไม่ต้องแก้โค้ด
```

## เสนอ record ที่ควรเพิ่ม

```text
อ่าน app/ แล้วเสนอ record ที่ควรเพิ่มเพื่อให้เคส <x> ตอบ 4 ข้อได้ครบ
ใช้โครงตาม docs/record-contract.md ห้ามเปลี่ยนชื่อ msg ที่มีอยู่แล้ว
บอกชื่อ msg, stage, outcome, error_class, money_effect และ field ที่ต้องใส่
```

## เทคนิค

- ให้ Claude แสดง query ที่ใช้ทุกครั้ง จะได้เอาไปรันเองใน Grafana Explore
- ถ้า Claude สรุปเกินหลักฐาน ให้ถามกลับว่า "log บรรทัดไหนบอกแบบนั้น"
- query นับจำนวนอย่าใส่ `| json` ถ้าไม่จำเป็น บรรทัดที่ parse ไม่ได้จะหายไปจากผล
