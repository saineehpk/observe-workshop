#!/usr/bin/env bash
# ผู้เล่นย้ายยอดจากกระเป๋าเข้าเครดิต ยอดเท่าเดิมสองครั้งติดกัน แล้วลองยอดอื่น
source "$(dirname "$0")/lib.sh"
title "02 pocket exchange twice"
post /pocket/exchange '{"username":"player004","amount":50}'
post /pocket/exchange '{"username":"player004","amount":50}'
post /pocket/exchange '{"username":"player004","amount":60}'
state player004
