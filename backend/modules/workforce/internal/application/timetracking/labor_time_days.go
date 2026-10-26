package timetracking

import (
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// breakStep is one §4 ArbZG step: above OverMinutes of net work time a break
// of RequiredMinutes is due.
type breakStep struct {
	OverMinutes     int
	RequiredMinutes int
}

// breakSteps are the §4 ArbZG steps in ascending order.
var breakSteps = []breakStep{
	{OverMinutes: breakNoneThresholdMinutes, RequiredMinutes: breakShortRequiredMinutes},
	{OverMinutes: breakShortThresholdMinutes, RequiredMinutes: breakLongRequiredMinutes},
}

// WorkDayLaborHint is the ArbZG view of one workday: every block checked in
// on that Berlin day, evaluated together like the kiosk does (#4804). The
// flags are hints for the reader and block nothing.
//
// A running break is never judged as too short: the person is taking the
// very break the rule asks for. While the day is still running and no break
// is, OverDailyMaxAt and BreakTooShortAt name the instants at which the
// hints would start if work simply continued, so a live clock can show them
// without its own thresholds.
type WorkDayLaborHint struct {
	Date                         calendar.Date
	NetMinutes                   int
	BreakMinutes                 int
	CountedBreakMinutes          int
	RequiredBreakMinutes         int
	IsBreakTooShort              bool
	IsOverDailyMax               bool
	IsRunning                    bool
	OverDailyMaxAt               *time.Time
	BreakTooShortAt              *time.Time
	BreakTooShortRequiredMinutes int
}

// HasHint reports whether the day carries a hint the reader should see.
func (h WorkDayLaborHint) HasHint() bool {
	return h.IsBreakTooShort || h.IsOverDailyMax
}

// WorkDayLaborHints evaluates the blocks per check-in day and returns the
// days inside [from, to] in date order. Blocks must carry their breaks in
// breaksBySession; a block without break rows falls back to its cached total.
func WorkDayLaborHints(sessions []*WorkSession, breaksBySession map[int64][]*WorkSessionBreak, now time.Time, from, to calendar.Date) []WorkDayLaborHint {
	byDay := make(map[calendar.Date][]*WorkSession)
	for _, session := range sessions {
		if session == nil {
			continue
		}
		day := calendar.DateFromTime(session.CheckInTime)
		if day.Before(from) || day.After(to) {
			continue
		}
		byDay[day] = append(byDay[day], session)
	}
	hints := make([]WorkDayLaborHint, 0, len(byDay))
	for day, blocks := range byDay {
		sort.SliceStable(blocks, func(i, j int) bool { return blocks[i].CheckInTime.Before(blocks[j].CheckInTime) })
		hints = append(hints, workDayLaborHint(day, blocks, breaksBySession, now))
	}
	sort.Slice(hints, func(i, j int) bool { return hints[i].Date.Before(hints[j].Date) })
	return hints
}

// CountLaborHintDays counts the days inside [from, to] that carry a hint.
func CountLaborHintDays(sessions []*WorkSession, breaksBySession map[int64][]*WorkSessionBreak, now time.Time, from, to calendar.Date) int {
	count := 0
	for _, hint := range WorkDayLaborHints(sessions, breaksBySession, now, from, to) {
		if hint.HasHint() {
			count++
		}
	}
	return count
}

func workDayLaborHint(day calendar.Date, blocks []*WorkSession, breaksBySession map[int64][]*WorkSessionBreak, now time.Time) WorkDayLaborHint {
	evaluation := EvaluateWorkSessionsLaborTime(blocks, breaksBySession, now)
	running, onBreak := workDayState(blocks, breaksBySession, now)
	hint := WorkDayLaborHint{
		Date:                 day,
		NetMinutes:           evaluation.NetMinutes,
		BreakMinutes:         evaluation.BreakMinutes,
		CountedBreakMinutes:  evaluation.CountedBreakMinutes,
		RequiredBreakMinutes: evaluation.RequiredBreakMinutes,
		IsBreakTooShort:      !evaluation.IsBreakCompliant && !onBreak,
		IsOverDailyMax:       evaluation.NetMinutes > overtimeThresholdMinutes,
		IsRunning:            running,
	}
	if running && !onBreak {
		hint.OverDailyMaxAt, hint.BreakTooShortAt, hint.BreakTooShortRequiredMinutes = upcomingLaborHints(evaluation, now)
	}
	return hint
}

// workDayState reports whether a block of the day is still counting as work
// and whether one of its breaks is running. A block past its live limit has
// stopped counting (BalanceSessionEnd), so it neither runs nor pauses.
func workDayState(blocks []*WorkSession, breaksBySession map[int64][]*WorkSessionBreak, now time.Time) (running, onBreak bool) {
	for _, block := range blocks {
		if block.CheckOutTime != nil || !BalanceSessionEnd(block, now).Equal(now) {
			continue
		}
		running = true
		for _, brk := range breaksBySession[block.ID] {
			if brk != nil && brk.IsActive() {
				onBreak = true
			}
		}
	}
	return running, onBreak
}

// upcomingLaborHints returns the instants at which a day that keeps running
// without a break crosses the daily maximum and the next break step it does
// not yet cover. Net minutes are floored, so the instant adds one minute to
// land past the threshold rather than on it.
func upcomingLaborHints(evaluation LaborTimeEvaluation, now time.Time) (*time.Time, *time.Time, int) {
	var overDailyMaxAt, breakTooShortAt *time.Time
	if evaluation.NetMinutes <= overtimeThresholdMinutes {
		at := now.Add(time.Duration(overtimeThresholdMinutes+1-evaluation.NetMinutes) * time.Minute)
		overDailyMaxAt = &at
	}
	if !evaluation.IsBreakCompliant {
		return overDailyMaxAt, nil, 0
	}
	for _, step := range breakSteps {
		if evaluation.NetMinutes <= step.OverMinutes && evaluation.CountedBreakMinutes < step.RequiredMinutes {
			at := now.Add(time.Duration(step.OverMinutes+1-evaluation.NetMinutes) * time.Minute)
			breakTooShortAt = &at
			return overDailyMaxAt, breakTooShortAt, step.RequiredMinutes
		}
	}
	return overDailyMaxAt, nil, 0
}
