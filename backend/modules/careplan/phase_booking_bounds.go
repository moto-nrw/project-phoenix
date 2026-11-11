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
// bookings take the same locks. An empty phase takes no lock. The caller holds the recurrence gate and
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
	bookings, err := m.lockedPhaseBookings(ctx, move.PhaseID)
	if err != nil || len(bookings) == 0 {
		return err
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

// lockedPhaseBookings locks the bookings of every application of the phase
// and returns them as read under those locks. An application whose first
// bookings commit while the command waits is locked in a further round, so
// no booking is decided on without its lock.
func (m *OfferingBookings) lockedPhaseBookings(ctx context.Context, phaseID int64) ([]CareOfferingBooking, error) {
	locked := map[int64]bool{}
	for {
		bookings, err := m.engine.CareOfferingBookingsForPhase(ctx, phaseID)
		if err != nil {
			return nil, fmt.Errorf("list bookings of phase %d: %w", phaseID, err)
		}
		var missing []int64
		for _, id := range requestChildIDs(bookings) {
			if !locked[id] {
				missing = append(missing, id)
			}
		}
		if len(missing) == 0 {
			return bookings, nil
		}
		if err := m.engine.LockCareOfferingBookings(ctx, missing); err != nil {
			return nil, fmt.Errorf("lock bookings of phase %d: %w", phaseID, err)
		}
		for _, id := range missing {
			locked[id] = true
		}
	}
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
	var changes []CareOfferingBookingBounds
	for _, followed := range followPhaseBounds(bookings, move) {
		if followed.empty() {
			return nil, &PhaseBookingBoundsConflictError{RequestChildID: followed.booking.RequestChildID, CareOfferingID: followed.booking.CareOfferingID}
		}
		booking := followed.booking
		changes = append(changes, CareOfferingBookingBounds{
			ID: booking.ID, RequestChildID: booking.RequestChildID,
			ValidFromWas: booking.ValidFrom, ValidUntilWas: booking.ValidUntil,
			ValidFrom: storedDay(followed.from), ValidUntil: storedDay(followed.until),
		})
	}
	return changes, nil
}

// followedBooking is a booking whose bounds follow a phase move, with the
// bounds it gets.
type followedBooking struct {
	booking     CareOfferingBooking
	from, until calendar.Date
}

// empty reports whether the followed bounds leave nothing of the booking.
func (f followedBooking) empty() bool {
	return f.from != "" && f.until != "" && !f.from.Before(f.until)
}

// followPhaseBounds decides, in booking order, which bookings' bounds follow
// the move and where to. The rules are phaseBookingBoundChanges'.
func followPhaseBounds(bookings []CareOfferingBooking, move PhaseBoundsMove) []followedBooking {
	oldFrom, oldUntil := move.StartBefore, move.EndBefore.AddDays(1)
	newFrom, newUntil := latestDay(move.StartAfter, move.Today), latestDay(move.EndAfter.AddDays(1), move.Today)
	startMoves := move.StartAfter != move.StartBefore && oldFrom.After(move.Today)
	endMoves := move.EndAfter != move.EndBefore && oldUntil.After(move.Today)
	earliest, latest := outerBounds(bookings)
	var followed []followedBooking
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
		followed = append(followed, followedBooking{booking: booking, from: from, until: until})
	}
	return followed
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
