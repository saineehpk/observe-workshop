#!/usr/bin/env bash
# ทุกอย่างผ่าน ใช้ดูหน้าตา record ปกติ
source "$(dirname "$0")/lib.sh"
title "01 happy path"
post /withdraw        '{"username":"player001","amount":100,"delay_ms":150}'
post /pocket/exchange '{"username":"player002","amount":50}'
post /checkin/claim   '{"username":"player003"}'
