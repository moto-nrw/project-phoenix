package active

import (
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
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

// LaborTimeEvaluation is the shared, as-of-now view used by both the web app
// and device flows. Keeping these values behind one function prevents the
// kiosk from reimplementing ArbZG thresholds or mishandling a running break.
type LaborTimeEvaluation struct {
	NetMinutes           int
	BreakMinutes         int
	RequiredBreakMinutes int
	IsBreakCompliant     bool
}

// EvaluateLaborTime applies the same work/break calculations used by session
// history to a single session. A running break is included up to now.
func EvaluateLaborTime(ws *activeModels.WorkSession, breaks []*activeModels.WorkSessionBreak, now time.Time) LaborTimeEvaluation {
	net := netMinutesWithBreaks(ws, breaks, now)
	taken := totalBreakMinutes(ws, breaks, now)
	required := 0
	if net > breakShortThresholdMinutes {
		required = breakLongRequiredMinutes
	} else if net > breakNoneThresholdMinutes {
		required = breakShortRequiredMinutes
	}
	return LaborTimeEvaluation{
		NetMinutes:           net,
		BreakMinutes:         taken,
		RequiredBreakMinutes: required,
		IsBreakCompliant:     taken >= required,
	}
}

// EvaluateDayLaborTime aggregates the ArbZG view over ALL blocks of one
// calendar day (#2402). Net and break minutes are summed per block; the gap
// between two consecutive blocks (checkout → next check-in) additionally
// counts as break time for the compliance judgment — a 90-minute Fahrtzeit
// between Homeoffice and OGS satisfies §4 ArbZG exactly like a stamped break,
// while the required amount is derived from the summed net work of the day.
// The gap itself is NOT part of BreakMinutes (it is simply not work time);
// only the compliance comparison credits it.
//
// `breaksBySession` carries each block's break rows keyed by session ID;
// blocks must be passed in check-in order (the repository guarantees it).
func EvaluateDayLaborTime(sessions []*activeModels.WorkSession, breaksBySession map[int64][]*activeModels.WorkSessionBreak, now time.Time) LaborTimeEvaluation {
	var net, taken, gaps int
	var prevEnd *time.Time
	for _, ws := range sessions {
		breaks := breaksBySession[ws.ID]
		net += netMinutesWithBreaks(ws, breaks, now)
		taken += totalBreakMinutes(ws, breaks, now)
		if prevEnd != nil && ws.CheckInTime.After(*prevEnd) {
			gaps += int(ws.CheckInTime.Sub(*prevEnd).Minutes())
		}
		if ws.CheckOutTime != nil {
			end := *ws.CheckOutTime
			if prevEnd == nil || end.After(*prevEnd) {
				prevEnd = &end
			}
		}
	}
	required := 0
	if net > breakShortThresholdMinutes {
		required = breakLongRequiredMinutes
	} else if net > breakNoneThresholdMinutes {
		required = breakShortRequiredMinutes
	}
	return LaborTimeEvaluation{
		NetMinutes:           net,
		BreakMinutes:         taken,
		RequiredBreakMinutes: required,
		IsBreakCompliant:     taken+gaps >= required,
	}
}

// grossMinutes is the wall-clock span of a session. For an open session (no
// check-out) it measures against now.
func grossMinutes(ws *activeModels.WorkSession, now time.Time) int {
	end := now
	if ws.CheckOutTime != nil {
		end = *ws.CheckOutTime
	}
	return int(end.Sub(ws.CheckInTime).Minutes())
}

// netMinutes calculates net work time in minutes (gross minus the ENDED breaks
// cached in BreakMinutes) for a work session. For an open session (no
// check-out) it measures against now. Net is floored at 0.
//
// A running break is NOT deducted here — callers reporting net time to a
// reader want netMinutesWithBreaks instead.
func netMinutes(ws *activeModels.WorkSession, now time.Time) int {
	net := grossMinutes(ws, now) - ws.BreakMinutes
	if net < 0 {
		return 0
	}
	return net
}

// runningBreakElapsedMinutes is the elapsed duration of a break that has
// started but not ended yet, floored at 0. WorkSession.BreakMinutes caches
// ENDED breaks only (see recalcBreakMinutes), so netMinutes counts every
// minute of a running break as worked time and every reader has to correct for
// it against its own clock. The math lives here once so the Monatskarte and
// /history can't drift apart (#1842).
func runningBreakElapsedMinutes(brk *activeModels.WorkSessionBreak, now time.Time) int {
	if brk == nil || !brk.IsActive() {
		return 0
	}
	elapsed := int(now.Sub(brk.StartedAt).Minutes())
	if elapsed < 0 {
		return 0
	}
	return elapsed
}

// totalBreakMinutes is ALL break time of a session as of `now`: the ended
// breaks cached in WorkSession.BreakMinutes plus the elapsed part of a break
// that is still running. It is the complement of netMinutesWithBreaks — the
// two always satisfy gross = net + totalBreak — so any rule that weighs work
// time against break time must take both from this pair, never mixing one with
// the raw BreakMinutes cache (#1842).
//
// A checked-out session cannot have a running break (checkout ends it), so its
// cache is already complete.
func totalBreakMinutes(ws *activeModels.WorkSession, breaks []*activeModels.WorkSessionBreak, now time.Time) int {
	total := ws.BreakMinutes
	if ws.CheckOutTime != nil {
		return total
	}
	for _, brk := range breaks {
		total += runningBreakElapsedMinutes(brk, now)
	}
	return total
}

// netMinutesWithBreaks is gross work time minus totalBreakMinutes. Use this
// wherever net work time is reported to a reader that also sees the
// Monatskarte: netMinutes alone keeps climbing while a staff member is on
// break, so Ist and Saldo would contradict the card, which deducts the running
// break server-side. Floored at 0.
func netMinutesWithBreaks(ws *activeModels.WorkSession, breaks []*activeModels.WorkSessionBreak, now time.Time) int {
	net := grossMinutes(ws, now) - totalBreakMinutes(ws, breaks, now)
	if net < 0 {
		return 0
	}
	return net
}

// netMinutesByDate splits a session's net time across the Berlin calendar days
// it intersects. Breaks are deducted from the days on which they actually
// occurred, rather than from the session's start day.
func netMinutesByDate(session *activeModels.WorkSession, breaks []*activeModels.WorkSessionBreak, end time.Time, from, to timezone.Date) map[timezone.Date]int {
	minutesByDate := make(map[timezone.Date]int)
	cachedBreakMinutes := session.BreakMinutes
	if !end.After(session.CheckInTime) {
		return minutesByDate
	}
	for day := timezone.DateFromTime(session.CheckInTime); !day.After(timezone.DateFromTime(end)); day = day.AddDays(1) {
		if day.Before(from) || day.After(to) {
			continue
		}
		start := session.CheckInTime
		if midnight := day.BerlinMidnight(); start.Before(midnight) {
			start = midnight
		}
		dayEnd := day.AddDays(1).BerlinMidnight()
		if end.Before(dayEnd) {
			dayEnd = end
		}
		minutes := int(dayEnd.Sub(start).Minutes())
		for _, brk := range breaks {
			breakEnd := end
			if brk.EndedAt != nil && brk.EndedAt.Before(breakEnd) {
				breakEnd = *brk.EndedAt
			}
			breakStart := brk.StartedAt
			if breakStart.Before(start) {
				breakStart = start
			}
			if breakEnd.After(dayEnd) {
				breakEnd = dayEnd
			}
			if breakEnd.After(breakStart) {
				minutes -= int(breakEnd.Sub(breakStart).Minutes())
			}
		}
		// Legacy rows may carry only the cached break total. New rows always
		// have break intervals, which take precedence for correct day mapping.
		if len(breaks) == 0 && cachedBreakMinutes > 0 {
			deduct := min(minutes, cachedBreakMinutes)
			minutes -= deduct
			cachedBreakMinutes -= deduct
		}
		if minutes > 0 {
			minutesByDate[day] = minutes
		}
	}
	return minutesByDate
}

// isOvertime reports whether net work time exceeds the statutory overtime
// threshold (10 hours). It measures the same net time that is reported to the
// reader (netMinutesWithBreaks), so the flag cannot contradict the displayed
// value while a break is running.
func isOvertime(ws *activeModels.WorkSession, breaks []*activeModels.WorkSessionBreak, now time.Time) bool {
	return netMinutesWithBreaks(ws, breaks, now) > overtimeThresholdMinutes
}

// isBreakCompliant reports whether the recorded breaks comply with German labor
// law (§4 ArbZG) given the session's net work time.
//
// Both sides of the comparison come from the same as-of-now picture: the net
// time is the one the reader sees, and the break taken includes the elapsed
// part of a running break. Judging the requirement against the ended-breaks
// cache alone would flag a staff member as non-compliant *while* they are
// taking the very break that makes them compliant.
func isBreakCompliant(ws *activeModels.WorkSession, breaks []*activeModels.WorkSessionBreak, now time.Time) bool {
	return EvaluateLaborTime(ws, breaks, now).IsBreakCompliant
}
