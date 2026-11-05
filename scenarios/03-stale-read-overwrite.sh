#!/usr/bin/env bash
# ผู้เล่นกดรับรางวัลเช็คอิน แล้วกลับมากดอีกครั้งในวันเดียวกัน
SITE="${SITE:-web-charlie}"   # เว็บที่เคสนี้เกิด
source "$(dirname "$0")/lib.sh"
title "03 check-in claimed twice"
u=$(user 5)
post /checkin/claim "{\"username\":\"$u\"}"
sleep 1
post /checkin/claim "{\"username\":\"$u\"}"
state "$u"
