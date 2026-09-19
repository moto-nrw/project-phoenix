package careplan

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"
)

var ErrCareOfferingBookingConflict = errors.New("care offering booking interval already has different effective days")

// CareOfferingBooking contains effective care only. Enrollment owns the
// original submission's selected days and notes independently of this history.
type CareOfferingBooking struct {
	ID                    int64
	TenantID              int64
	RequestChildID        int64
	CareOfferingID        int64
	ManualSelectedDays    []string
	AutomaticSelectedDays []string
	ValidFrom             *Date
	ValidUntil            *Date
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

func (b CareOfferingBooking) EffectiveSelectedDays() []string {
	var days []string
	for _, day := range []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"} {
		if slices.Contains(b.ManualSelectedDays, day) || slices.Contains(b.AutomaticSelectedDays, day) {
			days = append(days, day)
		}
	}
	// Historical rows can contain unrecognized days. Preserve them rather
	// than turning an invalid selection into an empty fixed-offering selection,
	// which consumers interpret as booking every available weekday.
	for _, source := range [][]string{b.ManualSelectedDays, b.AutomaticSelectedDays} {
		for _, day := range source {
			if !slices.Contains(days, day) {
				days = append(days, day)
			}
		}
	}
	return days
}

type OfferingBookingQueries interface {
	CareOfferingBookingsForPhase(context.Context, int64) ([]CareOfferingBooking, error)
	CareOfferingBookingsForOfferings(context.Context, []int64) ([]CareOfferingBooking, error)
	CountCareOfferingBookings(context.Context, []int64) (int, error)
	CareOfferingBookingHistory(context.Context, []int64) ([]CareOfferingBooking, error)
	CareOfferingBookingsAtDates(context.Context, map[int64]Date) ([]CareOfferingBooking, error)
}

type OfferingBookingCommands interface {
	LockCareOfferingBookings(context.Context, []int64) error
	EndCareOfferingBookings(context.Context, []int64, Date) (int64, error)
	RestoreCareOfferingBookings(context.Context, []CareOfferingBookingRestore) (int64, error)
	RecordCareOfferingBookings(context.Context, int64, []CareOfferingBooking) error
	ReplaceCareOfferingBookings(context.Context, int64, []CareOfferingBooking) error
	ScheduleCareOfferingBookings(context.Context, int64, Date, []CareOfferingBooking) error
}

type CareOfferingBookingRestore struct {
	Booking    CareOfferingBooking
	WasDeleted bool
}

// LockCareOfferingBookings requires the workflow's ambient transaction so the
// lock remains held while it snapshots and ends care across owners.
func (m *OfferingBookings) LockCareOfferingBookings(ctx context.Context, childIDs []int64) (err error) {
	started := time.Now()
	defer func() { m.observe("lock", "command", started, int64(len(childIDs)), -1, err) }()

	return m.engine.LockCareOfferingBookings(ctx, childIDs)
}

func (m *OfferingBookings) EndCareOfferingBookings(ctx context.Context, childIDs []int64, until Date) (changed int64, err error) {
	started := time.Now()
	defer func() { m.observe("end", "command", started, int64(len(childIDs)), changed, err) }()

	if _, err := time.Parse(time.DateOnly, string(until)); err != nil {
		return 0, fmt.Errorf("invalid care end: %w", err)
	}
	err = m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		changed, err = m.engine.EndCareOfferingBookings(ctx, childIDs, until)
		return err
	})
	return changed, err
}

func (m *OfferingBookings) RestoreCareOfferingBookings(ctx context.Context, restores []CareOfferingBookingRestore) (restored int64, err error) {
	started := time.Now()
	defer func() { m.observe("restore", "command", started, int64(len(restores)), restored, err) }()

	for _, restore := range restores {
		if restore.Booking.ID <= 0 {
			return 0, fmt.Errorf("restored booking ID is required")
		}
		if err := validateCareOfferingBookingIdentityAndDates(restore.Booking.RequestChildID, restore.Booking); err != nil {
			return 0, err
		}
	}
	err = m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		restored, err = m.engine.RestoreCareOfferingBookings(ctx, restores)
		return err
	})
	return restored, err
}

type offeringBookingEngine interface {
	OfferingBookingQueries
	OfferingBookingCommands
}

type OfferingBookings struct {
	observer     func(OfferingBookingObservation)
	engine       offeringBookingEngine
	transactions interface {
		RunInTx(context.Context, func(context.Context) error) error
	}
}

func NewOfferingBookings(engine offeringBookingEngine, transactions interface {
	RunInTx(context.Context, func(context.Context) error) error
}, observers ...func(OfferingBookingObservation)) *OfferingBookings {
	if engine == nil || transactions == nil {
		panic("care offering bookings: engine and transactions are required")
	}
	module := &OfferingBookings{engine: engine, transactions: transactions}
	if len(observers) > 0 {
		module.observer = observers[0]
	}
	return module
}

func (m *OfferingBookings) RecordCareOfferingBookings(ctx context.Context, childID int64, bookings []CareOfferingBooking) (err error) {
	started := time.Now()
	defer func() { m.observe("record", "command", started, int64(len(bookings)), -1, err) }()

	if err := validateCareOfferingBookings(childID, bookings); err != nil {
		return err
	}
	return m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		return m.engine.RecordCareOfferingBookings(ctx, childID, bookings)
	})
}

// ReplaceCareOfferingBookings corrects the child's complete effective plan.
// It does not change Enrollment's submitted choices.
func (m *OfferingBookings) ReplaceCareOfferingBookings(ctx context.Context, childID int64, bookings []CareOfferingBooking) (err error) {
	started := time.Now()
	defer func() { m.observe("replace", "command", started, int64(len(bookings)), -1, err) }()

	if err := validateCareOfferingBookings(childID, bookings); err != nil {
		return err
	}
	return m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		return m.engine.ReplaceCareOfferingBookings(ctx, childID, bookings)
	})
}

// Restoring a persisted snapshot preserves historical weekday payloads. New
// commands additionally validate weekdays in validateCareOfferingBookings.
func validateCareOfferingBookingIdentityAndDates(childID int64, booking CareOfferingBooking) error {
	if childID <= 0 {
		return fmt.Errorf("request_child_id is required")
	}
	if booking.CareOfferingID <= 0 || (booking.RequestChildID != 0 && booking.RequestChildID != childID) {
		return fmt.Errorf("care offering booking requires matching child and offering IDs")
	}
	for _, date := range []*Date{booking.ValidFrom, booking.ValidUntil} {
		if date != nil {
			if _, err := time.Parse(time.DateOnly, string(*date)); err != nil {
				return fmt.Errorf("invalid care offering booking date: %w", err)
			}
		}
	}
	if booking.ValidFrom != nil && booking.ValidUntil != nil && !booking.ValidFrom.Before(*booking.ValidUntil) {
		return fmt.Errorf("care offering booking validity must be nonempty")
	}

	return nil
}

func validateCareOfferingBookings(childID int64, bookings []CareOfferingBooking) error {
	if childID <= 0 {
		return fmt.Errorf("request_child_id is required")
	}
	for _, booking := range bookings {
		if err := validateCareOfferingBookingIdentityAndDates(childID, booking); err != nil {
			return err
		}
		for _, days := range [][]string{booking.ManualSelectedDays, booking.AutomaticSelectedDays} {
			for _, day := range days {
				if !slices.Contains([]string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}, day) {
					return fmt.Errorf("invalid effective care day %q", day)
				}
			}
		}
	}
	return nil
}

// ScheduleCareOfferingBookings replaces care from the effective date onward.
// Earlier intervals are retained, with exclusive ends at the switch date.
func (m *OfferingBookings) ScheduleCareOfferingBookings(ctx context.Context, childID int64, effectiveFrom Date, bookings []CareOfferingBooking) (err error) {
	started := time.Now()
	defer func() { m.observe("schedule", "command", started, int64(len(bookings)), -1, err) }()

	if _, err := time.Parse(time.DateOnly, string(effectiveFrom)); err != nil {
		return fmt.Errorf("invalid booking switch date: %w", err)
	}
	bookings = slices.Clone(bookings)
	seen := make(map[int64]bool, len(bookings))
	for i := range bookings {
		if seen[bookings[i].CareOfferingID] {
			return fmt.Errorf("duplicate offering in booking switch")
		}
		seen[bookings[i].CareOfferingID] = true
		bookings[i].ValidFrom = &effectiveFrom
	}
	if err := validateCareOfferingBookings(childID, bookings); err != nil {
		return err
	}
	return m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		return m.engine.ScheduleCareOfferingBookings(ctx, childID, effectiveFrom, bookings)
	})
}

func (m *OfferingBookings) CareOfferingBookingsForPhase(ctx context.Context, phaseID int64) (rows []CareOfferingBooking, err error) {
	started := time.Now()
	defer func() { m.observe("for_phase", "query", started, 0, int64(len(rows)), err) }()

	if phaseID <= 0 {
		return nil, fmt.Errorf("phase_id is required")
	}
	err = m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		rows, err = m.engine.CareOfferingBookingsForPhase(ctx, phaseID)
		return err
	})
	return rows, err
}

func (m *OfferingBookings) CareOfferingBookingsForOfferings(ctx context.Context, offeringIDs []int64) (rows []CareOfferingBooking, err error) {
	started := time.Now()
	defer func() { m.observe("for_offerings", "query", started, int64(len(offeringIDs)), int64(len(rows)), err) }()

	if len(offeringIDs) == 0 {
		return []CareOfferingBooking{}, nil
	}
	err = m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		rows, err = m.engine.CareOfferingBookingsForOfferings(ctx, offeringIDs)
		return err
	})
	return rows, err
}

func (m *OfferingBookings) CountCareOfferingBookings(ctx context.Context, childIDs []int64) (count int, err error) {
	started := time.Now()
	defer func() { m.observe("count", "query", started, int64(len(childIDs)), int64(count), err) }()

	if len(childIDs) == 0 {
		return 0, nil
	}
	err = m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		count, err = m.engine.CountCareOfferingBookings(ctx, childIDs)
		return err
	})
	return count, err
}

func (m *OfferingBookings) CareOfferingBookingHistory(ctx context.Context, childIDs []int64) (rows []CareOfferingBooking, err error) {
	started := time.Now()
	defer func() { m.observe("history", "query", started, int64(len(childIDs)), int64(len(rows)), err) }()

	if len(childIDs) == 0 {
		return []CareOfferingBooking{}, nil
	}
	err = m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		rows, err = m.engine.CareOfferingBookingHistory(ctx, childIDs)
		return err
	})
	return rows, err
}

func (m *OfferingBookings) CareOfferingBookingsAtDates(ctx context.Context, dates map[int64]Date) (rows []CareOfferingBooking, err error) {
	started := time.Now()
	defer func() { m.observe("at_dates", "query", started, int64(len(dates)), int64(len(rows)), err) }()

	if len(dates) == 0 {
		return []CareOfferingBooking{}, nil
	}
	for childID, date := range dates {
		if childID <= 0 {
			return nil, fmt.Errorf("request_child_id is required")
		}
		if _, err := time.Parse(time.DateOnly, string(date)); err != nil {
			return nil, fmt.Errorf("invalid effective care date: %w", err)
		}
	}
	err = m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		rows, err = m.engine.CareOfferingBookingsAtDates(ctx, dates)
		return err
	})
	return rows, err
}
