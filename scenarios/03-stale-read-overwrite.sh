#!/usr/bin/env bash
# ผู้เล่นกดรับรางวัลเช็คอิน แล้วกลับมากดอีกครั้งในวันเดียวกัน
source "$(dirname "$0")/lib.sh"
title "03 check-in claimed twice"
post /checkin/claim '{"username":"player005"}'
sleep 1
post /checkin/claim '{"username":"player005"}'
state player005
