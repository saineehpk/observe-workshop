#!/usr/bin/env bash
# ผู้เล่นย้ายยอดจากกระเป๋าเข้าเครดิต ยอดเท่าเดิมสองครั้งติดกัน แล้วลองยอดอื่น
SITE="${SITE:-web-alpha}"   # เว็บที่เคสนี้เกิด
source "$(dirname "$0")/lib.sh"
title "02 pocket exchange twice"
u=$(user 4)
post /pocket/exchange "{\"username\":\"$u\",\"amount\":50}"
post /pocket/exchange "{\"username\":\"$u\",\"amount\":50}"
post /pocket/exchange "{\"username\":\"$u\",\"amount\":60}"
state "$u"
