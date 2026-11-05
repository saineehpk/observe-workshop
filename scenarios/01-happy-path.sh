#!/usr/bin/env bash
# ทุกอย่างผ่าน ใช้ดูหน้าตา record ปกติ
SITE="${SITE:-web-alpha}"   # เว็บที่เคสนี้เกิด
source "$(dirname "$0")/lib.sh"
title "01 happy path"
post /withdraw        "{\"username\":\"$(user 1)\",\"amount\":100,\"delay_ms\":150}"
post /pocket/exchange "{\"username\":\"$(user 2)\",\"amount\":50}"
post /checkin/claim   "{\"username\":\"$(user 3)\"}"
