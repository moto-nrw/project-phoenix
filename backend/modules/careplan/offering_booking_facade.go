package careplan

import "context"

func (m *Module) CareOfferingBookingsForPhase(ctx context.Context, phaseID int64) ([]CareOfferingBooking, error) {
	return m.engine.CareOfferingBookingsForPhase(ctx, phaseID)
}

func (m *Module) CareOfferingBookingsForOfferings(ctx context.Context, offeringIDs []int64) ([]CareOfferingBooking, error) {
	return m.engine.CareOfferingBookingsForOfferings(ctx, offeringIDs)
}

func (m *Module) LockCareOfferingBookings(ctx context.Context, childIDs []int64) error {
	return m.engine.LockCareOfferingBookings(ctx, childIDs)
}

func (m *Module) EndCareOfferingBookings(ctx context.Context, childIDs []int64, until Date) (int64, error) {
	return m.engine.EndCareOfferingBookings(ctx, childIDs, until)
}

func (m *Module) RestoreCareOfferingBookings(ctx context.Context, restores []CareOfferingBookingRestore) (int64, error) {
	return m.engine.RestoreCareOfferingBookings(ctx, restores)
}

func (m *Module) CountCareOfferingBookings(ctx context.Context, childIDs []int64) (int, error) {
	return m.engine.CountCareOfferingBookings(ctx, childIDs)
}

func (m *Module) RecordCareOfferingBookings(ctx context.Context, childID int64, bookings []CareOfferingBooking) error {
	return m.engine.RecordCareOfferingBookings(ctx, childID, bookings)
}

func (m *Module) ReplaceCareOfferingBookings(ctx context.Context, childID int64, bookings []CareOfferingBooking) error {
	return m.engine.ReplaceCareOfferingBookings(ctx, childID, bookings)
}

func (m *Module) ScheduleCareOfferingBookings(ctx context.Context, childID int64, effectiveFrom Date, bookings []CareOfferingBooking) error {
	return m.engine.ScheduleCareOfferingBookings(ctx, childID, effectiveFrom, bookings)
}

func (m *Module) CareOfferingBookingHistory(ctx context.Context, childIDs []int64) ([]CareOfferingBooking, error) {
	return m.engine.CareOfferingBookingHistory(ctx, childIDs)
}

func (m *Module) CareOfferingBookingsAtDates(ctx context.Context, dates map[int64]Date) ([]CareOfferingBooking, error) {
	return m.engine.CareOfferingBookingsAtDates(ctx, dates)
}
