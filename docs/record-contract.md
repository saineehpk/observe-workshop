# Business record contract

ทุก record ที่บอกการตัดสินใจทางธุรกิจ ต้องมี field ชุดนี้เสมอ

| field | ค่า | ความหมาย |
|---|---|---|
| `msg` | `<operation>_<stage>` | ชื่อ event ห้ามเปลี่ยนหลังขึ้น prod (dashboard / alert ผูกอยู่) |
| `trace_id` | hex 32 ตัว | ผูก record ข้าม service ส่งต่อผ่าน header `X-Trace-Id` |
| `operation` | `withdraw`, `withdraw_worker`, `pocket_exchange`, `checkin_claim`, `checkin_stack` | flow ไหน |
| `stage` | `requested`, `completed`, `written`, ... | ขั้นไหนของ flow |
| `outcome` | `accepted` · `rejected` · `failed` · `partial` | ชุดปิด 4 ค่า |
| `error_class` | `none` เมื่อผ่าน หรือชุดปิดต่อ operation | เพราะอะไร (ห้ามใส่ข้อความ error ดิบ) |
| `money_effect` | `none` · `confirmed` · `unknown` | เงินขยับหรือยัง |
| `username` | | ใครโดน |
| `duration_ms` | | ใช้เวลาเท่าไร |

แล้วเพิ่ม field ของ flow แบบ flat เช่น `amount`, `credit_before`, `credit_after`, `stack_read`

## ทำไม `money_effect` มี `unknown`

boolean แบบ `money_moved=true/false` บังคับให้เดา ถ้าเรียกปลายทางแล้ว timeout เราไม่รู้ว่าปลายทางตัดเงินไปแล้วหรือยัง
การบันทึกว่า `unknown` บอกตรงๆ ว่าต้องไปดู record ของฝั่งที่ถือเงิน (ต่อด้วย `trace_id`)

## กฎ level

| เงื่อนไข | level |
|---|---|
| `outcome=accepted` | INFO |
| `outcome=rejected` และ `money_effect=none` | WARN |
| อย่างอื่นทั้งหมด | ERROR |

ERROR จึงแปลว่า "ต้องมีคนดู" ไม่ใช่แค่ "มี error ในโค้ด"

## metric

`workshop_business_records_total{operation, stage, outcome, error_class, money_effect}`

- label ทุกตัวเป็นชุดปิด ค่า `error_class` ที่ไม่อยู่ใน `knownErrorClasses` จะถูกยุบเป็น `other`
- ห้ามใส่ `username`, `trace_id`, ยอดเงิน เป็น label (cardinality ระเบิด) ของพวกนี้อยู่ใน log
- ดูเป็นอัตรา (`rate`) ไม่ใช่ยอดรวม

## record "รับเข้ามาแล้ว"

`withdraw_requested` มี `outcome=accepted` เพราะรับคำขอเข้ามาแล้วจริง
query ที่นับผลลัพธ์จึงต้องกรอง `stage="completed"` เสมอ
