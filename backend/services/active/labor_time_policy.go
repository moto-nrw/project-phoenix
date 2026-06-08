package active

import (
	"time"

	activeModels "github.com/moto-nrw/project-phoenix/models/active"
)

// German labor-law (ArbZG) thresholds applied to staff work sessions. These are
// statutory rules that are identical for every tenant, so they live as named
// constants rather than per-tenant settings. Moved out of the WorkSession model
// (Rule 12: models hold data, not decisions).
const (
	// overtimeThresholdMinutes is the net work time (10h) above which a session
	// counts as overtime.
	overtimeThresholdMinutes = 600

	// breakNoneThresholdMinutes is the net work time (6h) up to which no break is
	// required (§4 ArbZG).
	breakNoneThresholdMinutes = 360
	// breakShortThresholdMinutes is the net work time (9h) up to which a 30-minute
	// break is sufficient (§4 ArbZG).
	breakShortThresholdMinutes = 540
	// breakShortRequiredMinutes is the break required between 6h and 9h of work.
	breakShortRequiredMinutes = 30
	// breakLongRequiredMinutes is the break required above 9h of work.
	breakLongRequiredMinutes = 45
)

// netMinutes calculates net work time in minutes (gross minus breaks) for a work
// session. For an open session (no check-out) it measures against now. Net is
// floored at 0.
func netMinutes(ws *activeModels.WorkSession, now time.Time) int {
	end := now
	if ws.CheckOutTime != nil {
		end = *ws.CheckOutTime
	}
	gross := int(end.Sub(ws.CheckInTime).Minutes())
	net := gross - ws.BreakMinutes
	if net < 0 {
		return 0
	}
	return net
}

// isOvertime reports whether net work time exceeds the statutory overtime
// threshold (10 hours).
func isOvertime(ws *activeModels.WorkSession, now time.Time) bool {
	return netMinutes(ws, now) > overtimeThresholdMinutes
}

// isBreakCompliant reports whether the recorded breaks comply with German labor
// law (§4 ArbZG) given the session's net work time.
func isBreakCompliant(ws *activeModels.WorkSession, now time.Time) bool {
	net := netMinutes(ws, now)
	if net <= breakNoneThresholdMinutes { // <= 6h: no break required
		return true
	}
	if net <= breakShortThresholdMinutes { // <= 9h: 30 min break required
		return ws.BreakMinutes >= breakShortRequiredMinutes
	}
	// > 9h: 45 min break required
	return ws.BreakMinutes >= breakLongRequiredMinutes
}
