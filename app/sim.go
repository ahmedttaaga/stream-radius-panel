package main

// Test panel (owner 2026-10-28: «محاكاة للسيستم الحالي»): a second copy of this program on :8081 with its own copy of
// the database (radius_sim, made by deploy/sim.sh), in live mode, so cash transactions can really be used on the copied
// subscribers. It must never reach anything outside itself: no ADV, no routers (CoA/disconnect), no WhatsApp/SMS,
// no FreeRADIUS/VPN restarts. RM_SIM=1 turns all of that off here; RM_STAGING=1 (set too) covers the service restarts.

import "errors"

var errSim = errors.New("لوحة التجربة: مفيش أي إرسال لبره (ADV / الراوترات / الرسايل)")

func simMode() bool { return env("RM_SIM", "") == "1" }
