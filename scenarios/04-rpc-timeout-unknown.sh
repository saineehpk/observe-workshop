#!/usr/bin/env bash
# ถอนเงินตอนที่ปลายทางช้า ผู้เล่นได้ข้อความ "กำลังดำเนินการ"
source "$(dirname "$0")/lib.sh"
title "04 slow withdraw"
state player006
post /withdraw '{"username":"player006","amount":200,"delay_ms":3500}'
echo "  ... wait for the worker"
sleep 3
state player006
