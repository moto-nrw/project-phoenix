package careplan

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// PhaseBookingBounds is the command a phase edit carries the bounds of its
// offerings' bookings with (#6597, ADR 0205). It runs in the edit's
// transaction, locks the bookings of every affected application and decides
// under those locks which bound still is the phase's.
type PhaseBookingBounds interface {
	FollowPhaseBookingBounds(ctx context.Context, move PhaseBoundsMove) error
}

// PhaseBoundsMove is a phase's service window before and after an edit, with
// the day the edit happens. Bounds are calendar days; the end is the last
// day of care, the booking keeps it as its exclusive day after.
type PhaseBoundsMove struct {
	PhaseID     int64
	StartBefore calendar.Date
	StartAfter  calendar.Date
	EndBefore   calendar.Date
	EndAfter    calendar.Date
	Today       calendar.Date
}

// PhaseBookingBoundsConflictError refuses a phase edit that would leave a
// booking empty: a bound that follows the phase would pass the booking's
// other bound, which a dated switch or a care end set.
type PhaseBookingBoundsConflictError struct {
	RequestChildID int64
	CareOfferingID int64
}

func (e *PhaseBookingBoundsConflictError) Error() string {
	return fmt.Sprintf("phase change would leave the booking of request child %d for care offering %d empty",
		e.RequestChildID, e.CareOfferingID)
}

// CareOfferingBookingBounds is one booking's validity before and after a
// phase edit. The store writes the new bounds only while the row still has
// the old ones.
type CareOfferingBookingBounds struct {
	ID             int64
	ValidFromWas   *Date
	ValidUntilWas  *Date
	ValidFrom      *Date
	ValidUntil     *Date
	RequestChildID int64
}

// FollowPhaseBookingBounds moves the bookings' bounds that the phase set
// (#6597). The bookings of every application of the phase are locked in
// ascending order before they are read again and decided on; writers of
// bookings take the same locks. The caller holds the recurrence gate and
// the phase row; the command takes no other gate.
func (m *OfferingBookings) FollowPhaseBookingBounds(ctx context.Context, move PhaseBoundsMove) (err error) {
	started := time.Now()
	changed := int64(0)
	defer func() { m.observe("follow_phase", "command", started, 1, changed, err) }()

	if err := move.validate(); err != nil {
		return err
	}
	if move.StartBefore == move.StartAfter && move.EndBefore == move.EndAfter {
		return nil
	}
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	bookings, err := m.engine.CareOfferingBookingsForPhase(ctx, move.PhaseID)
	if err != nil {
		return fmt.Errorf("list bookings of phase %d: %w", move.PhaseID, err)
	}
	if len(bookings) == 0 {
		return nil
	}
	if err := m.engine.LockCareOfferingBookings(ctx, requestChildIDs(bookings)); err != nil {
		return fmt.Errorf("lock bookings of phase %d: %w", move.PhaseID, err)
	}
	if bookings, err = m.engine.CareOfferingBookingsForPhase(ctx, move.PhaseID); err != nil {
		return fmt.Errorf("list locked bookings of phase %d: %w", move.PhaseID, err)
	}
	changes, err := phaseBookingBoundChanges(bookings, move)
	if err != nil || len(changes) == 0 {
		return err
	}
	if err := m.engine.MoveCareOfferingBookingBounds(ctx, changes); err != nil {
		return fmt.Errorf("move booking bounds of phase %d: %w", move.PhaseID, err)
	}
	changed = int64(len(changes))
	return nil
}

func (move PhaseBoundsMove) validate() error {
	if move.PhaseID <= 0 {
		return fmt.Errorf("phase_id is required")
	}
	for _, day := range []calendar.Date{move.StartBefore, move.StartAfter, move.EndBefore, move.EndAfter, move.Today} {
		if _, err := calendar.ParseDate(day.String()); err != nil {
			return fmt.Errorf("invalid phase bound: %w", err)
		}
	}
	return nil
}

func requestChildIDs(bookings []CareOfferingBooking) []int64 {
	ids := make([]int64, 0, len(bookings))
	for _, booking := range bookings {
		ids = append(ids, booking.RequestChildID)
	}
	slices.Sort(ids)
	return slices.Compact(ids)
}

// phaseBookingBoundChanges decides which bounds follow the phase. A bound
// follows only when it still equals the phase's old bound and is the outer
// bound of the application's bookings of that offering: the start of the
// earliest, the end of the latest booking. The inner bounds of a dated
// switch and a bound set by hand or by a care end stay.
//
// History stays: a start moves only while it lies after today, an end only
// while the booking still applies today, and no bound moves before today.
// A booking the move would leave empty refuses the whole edit.
func phaseBookingBoundChanges(bookings []CareOfferingBooking, move PhaseBoundsMove) ([]CareOfferingBookingBounds, error) {
	oldFrom, oldUntil := move.StartBefore, move.EndBefore.AddDays(1)
	newFrom, newUntil := latestDay(move.StartAfter, move.Today), latestDay(move.EndAfter.AddDays(1), move.Today)
	startMoves := move.StartAfter != move.StartBefore && oldFrom.After(move.Today)
	endMoves := move.EndAfter != move.EndBefore && oldUntil.After(move.Today)
	earliest, latest := outerBounds(bookings)
	var changes []CareOfferingBookingBounds
	for _, booking := range bookings {
		key := bookingChain{booking.RequestChildID, booking.CareOfferingID}
		from, until := bookingDay(booking.ValidFrom), bookingDay(booking.ValidUntil)
		movesFrom := startMoves && from == oldFrom && earliest[key] == oldFrom
		movesUntil := endMoves && until == oldUntil && latest[key] == oldUntil
		if !movesFrom && !movesUntil {
			continue
		}
		if movesFrom {
			from = newFrom
		}
		if movesUntil {
			until = newUntil
		}
		if from != "" && until != "" && !from.Before(until) {
			return nil, &PhaseBookingBoundsConflictError{RequestChildID: booking.RequestChildID, CareOfferingID: booking.CareOfferingID}
		}
		changes = append(changes, CareOfferingBookingBounds{
			ID: booking.ID, RequestChildID: booking.RequestChildID,
			ValidFromWas: booking.ValidFrom, ValidUntilWas: booking.ValidUntil,
			ValidFrom: storedDay(from), ValidUntil: storedDay(until),
		})
	}
	return changes, nil
}

type bookingChain struct{ requestChildID, careOfferingID int64 }

// outerBounds returns per application and offering the earliest start and
// the latest end. An open bound counts as outermost.
func outerBounds(bookings []CareOfferingBooking) (map[bookingChain]calendar.Date, map[bookingChain]calendar.Date) {
	earliest, latest := map[bookingChain]calendar.Date{}, map[bookingChain]calendar.Date{}
	openStart, openEnd := map[bookingChain]bool{}, map[bookingChain]bool{}
	for _, booking := range bookings {
		key := bookingChain{booking.RequestChildID, booking.CareOfferingID}
		if from := bookingDay(booking.ValidFrom); from == "" {
			openStart[key] = true
		} else if current, ok := earliest[key]; !ok || from.Before(current) {
			earliest[key] = from
		}
		if until := bookingDay(booking.ValidUntil); until == "" {
			openEnd[key] = true
		} else if current, ok := latest[key]; !ok || until.After(current) {
			latest[key] = until
		}
	}
	for key := range openStart {
		delete(earliest, key)
	}
	for key := range openEnd {
		delete(latest, key)
	}
	return earliest, latest
}

func latestDay(day, other calendar.Date) calendar.Date {
	if other.After(day) {
		return other
	}
	return day
}

func bookingDay(day *Date) calendar.Date {
	if day == nil {
		return ""
	}
	return calendar.Date(*day)
}

func storedDay(day calendar.Date) *Date {
	if day == "" {
		return nil
	}
	return new(Date(day))
}
