package timetracking

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Work time credit (#6851, #6852).
//
// A completed block carries the school's credit rule as it stood when the
// block was closed. The credited time is derived from the raw stamps and that
// rule on every read, in this order:
//
//  1. stamped duration: check-out minus check-in, in whole minutes
//  2. plus the pack-up minutes (only blocks ended by a regular checkout)
//  3. rounded to the interval in the rule's direction
//  4. minus the breaks, as for every block
//
// The stamps are never rewritten, so a correction recomputes the credit from
// the corrected stamps and cannot count it twice, and a later settings change
// leaves blocks closed before it untouched. Open blocks are credited nothing.
// The ArbZG hints keep judging the stamped time (EvaluateLaborTime).

// workTimeCredit is the credited time of one block on top of its stamped net
// time: PackUpMinutes added for a regular checkout, RoundingMinutes the signed
// difference the rounding made.
type workTimeCredit struct {
	PackUpMinutes   int
	RoundingMinutes int
}

func (c workTimeCredit) total() int { return c.PackUpMinutes + c.RoundingMinutes }

// creditRule is the rule the block was closed with.
func (ws *WorkSession) creditRule() workforce.WorkSessionCreditRule {
	return workforce.WorkSessionCreditRule{
		PackUpMinutes:           ws.PackUpMinutes,
		RoundingIntervalMinutes: ws.RoundingIntervalMinutes,
		RoundingMode:            ws.RoundingMode,
	}
}

func (ws *WorkSession) setCreditRule(rule workforce.WorkSessionCreditRule) {
	ws.PackUpMinutes = rule.PackUpMinutes
	ws.RoundingIntervalMinutes = rule.RoundingIntervalMinutes
	ws.RoundingMode = rule.RoundingMode
}

// sessionCredit derives the credit of a completed block from its stamps and
// its stored rule. An open block, or one without a positive duration, gets
// none.
func sessionCredit(ws *WorkSession) workTimeCredit {
	if ws == nil || ws.CheckOutTime == nil || !ws.CheckOutTime.After(ws.CheckInTime) {
		return workTimeCredit{}
	}
	withPackUp := grossMinutes(ws, *ws.CheckOutTime) + ws.PackUpMinutes
	rounded := roundWorkMinutes(withPackUp, ws.RoundingIntervalMinutes, ws.RoundingMode)
	return workTimeCredit{PackUpMinutes: ws.PackUpMinutes, RoundingMinutes: rounded - withPackUp}
}

// roundWorkMinutes rounds a duration to a multiple of interval. Nearest is
// "kaufmännisch": exactly half an interval rounds up. An interval of 0 or less
// leaves the duration unchanged.
func roundWorkMinutes(minutes, interval int, mode string) int {
	if interval <= 0 || minutes <= 0 {
		return minutes
	}
	rest := minutes % interval
	if rest == 0 {
		return minutes
	}
	down := minutes - rest
	switch mode {
	case workforce.WorkTimeRoundingUp:
		return down + interval
	case workforce.WorkTimeRoundingDown:
		return down
	default:
		if rest*2 >= interval {
			return down + interval
		}
		return down
	}
}

// creditedNetMinutes is the net work time a reader sees for one block: the
// stamped gross time plus credit minus all breaks, floored once at 0.
func creditedNetMinutes(ws *WorkSession, breaks []*WorkSessionBreak, now time.Time) int {
	return max(0, grossMinutes(ws, now)+sessionCredit(ws).total()-totalBreakMinutes(ws, breaks, now))
}

// creditedNetMinutesByDate is netMinutesByDate with the block's credit booked
// on the Berlin day its last stamped minute falls on. The credit only applies
// when end is the block's own check-out: a block measured against a live
// limit is still open.
func creditedNetMinutesByDate(session *WorkSession, breaks []*WorkSessionBreak, end time.Time, from, to calendar.Date) map[calendar.Date]int {
	if session.CheckOutTime == nil || !end.Equal(*session.CheckOutTime) {
		return netMinutesByDate(session, breaks, end, from, to)
	}
	// Keep the full break deduction until after credit, even when breaks
	// exceed the stamped duration. Only the effective difference is booked.
	credit := creditedNetMinutes(session, breaks, end) - netMinutesWithBreaks(session, breaks, end)
	if credit == 0 {
		return netMinutesByDate(session, breaks, end, from, to)
	}
	firstDay := calendar.DateFromTime(session.CheckInTime)
	lastDay := calendar.DateFromTime(end.Add(-time.Nanosecond))
	minutesByDate := netMinutesByDate(session, breaks, end, firstDay, lastDay)
	applyCreditByDate(minutesByDate, credit, firstDay, lastDay)
	// Filter only after deductions have reached any earlier occupied days.
	for day := range minutesByDate {
		if day.Before(from) || day.After(to) {
			delete(minutesByDate, day)
		}
	}
	return minutesByDate
}

// applyCreditByDate books additions on the last occupied day and carries a
// rounding deduction backwards until it is fully consumed, without negatives.
func applyCreditByDate(minutesByDate map[calendar.Date]int, credit int, firstDay, lastDay calendar.Date) {
	if credit > 0 {
		minutesByDate[lastDay] += credit
		return
	}
	for day := lastDay; credit < 0 && !day.Before(firstDay); day = day.AddDays(-1) {
		deducted := min(minutesByDate[day], -credit)
		minutesByDate[day] -= deducted
		credit += deducted
		if minutesByDate[day] == 0 {
			delete(minutesByDate, day)
		}
	}
}

// closeRegularCheckout closes a block by a regular checkout. It earns the
// pack-up minutes; the rule is stored on the block, so later settings changes
// leave it alone.
func (s *workSessionService) closeRegularCheckout(ctx context.Context, sessionID int64, now time.Time) error {
	credit, err := s.closingCreditRule(ctx, true)
	if err != nil {
		return err
	}
	closed, err := s.repo.CloseWorkSession(ctx, sessionID, now, false, credit)
	if err = workSessionWriteError("close session", err); err != nil {
		return fmt.Errorf("failed to close session: %w", err)
	}
	if !closed {
		return workforce.ErrNoActiveSession
	}
	return nil
}

// closeForgottenSession closes a block the nightly cleanup found still open,
// with the rounding of the run and without pack-up minutes. It reports how
// many blocks it closed: 0 when another writer closed it first.
func (s *workSessionService) closeForgottenSession(ctx context.Context, sessionID int64, closeAt time.Time, credit workforce.WorkSessionCreditRule) (int, error) {
	closed, err := s.repo.CloseWorkSession(ctx, sessionID, closeAt, true, credit)
	if err = workSessionWriteError("close session", err); err != nil {
		return 0, fmt.Errorf("failed to close session %d: %w", sessionID, err)
	}
	if !closed {
		return 0, nil
	}
	return 1, nil
}

// creditEditThatCloses stores the credit rule on a block an edit closes. Such
// an edit is no regular checkout: it gets the rounding of now and no pack-up
// minutes. A block that was already closed keeps the rule it was closed with,
// so a corrected stamp recomputes the same credit.
func (s *workSessionService) creditEditThatCloses(ctx context.Context, session *WorkSession, wasOpen bool) error {
	if !wasOpen || session.CheckOutTime == nil {
		return nil
	}
	credit, err := s.closingCreditRule(ctx, false)
	if err != nil {
		return err
	}
	session.setCreditRule(credit)
	return nil
}

// closingCreditRule reads the school's credit rule for a block that is being
// closed now. Only a regular checkout earns the pack-up minutes; the rounding
// applies to every completed block.
func (s *workSessionService) closingCreditRule(ctx context.Context, regularCheckout bool) (workforce.WorkSessionCreditRule, error) {
	var rule workforce.WorkSessionCreditRule
	if s.settings == nil {
		return rule, errors.New("work time credit: time-tracking settings are not wired")
	}
	packUp, interval, mode, err := s.settings.WorkTimeCredit(ctx)
	if err != nil {
		return workforce.WorkSessionCreditRule{}, fmt.Errorf("failed to read work time credit settings: %w", err)
	}
	if interval > 0 {
		rule.RoundingIntervalMinutes, rule.RoundingMode = interval, mode
	}
	if regularCheckout {
		rule.PackUpMinutes = packUp
	}
	return rule, nil
}
