#!/usr/bin/env bash
# ถอนเงินตอนที่ปลายทางช้า ผู้เล่นได้ข้อความ "กำลังดำเนินการ"
SITE="${SITE:-web-bravo}"   # เว็บที่เคสนี้เกิด
source "$(dirname "$0")/lib.sh"
title "04 slow withdraw"
u=$(user 6)
state "$u"
post /withdraw "{\"username\":\"$u\",\"amount\":200,\"delay_ms\":3500}"
echo "  ... wait for the worker"
sleep 3
state "$u"
