package repositories

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

func (a EnrollmentBookingProjection) RequestChildOfferingHistory(ctx context.Context, childID int64) ([]*enrollment.RequestChildOffering, error) {
	return a.RequestChildOfferingHistoryForChildren(ctx, []int64{childID})
}

func (a EnrollmentBookingProjection) RequestChildOfferingHistoryForChildren(ctx context.Context, childIDs []int64) ([]*enrollment.RequestChildOffering, error) {
	bookings, err := a.bookings.CareOfferingBookingHistory(ctx, childIDs)
	if err != nil {
		return nil, err
	}
	return a.enrollmentBookingValues(ctx, childIDs, bookings)
}

func (a EnrollmentBookingProjection) RequestChildOfferingsForChildrenAtDate(ctx context.Context, childIDs []int64, date enrollment.Date) ([]*enrollment.RequestChildOffering, error) {
	dates := make(map[int64]enrollment.Date, len(childIDs))
	for _, id := range childIDs {
		dates[id] = date
	}
	return a.RequestChildOfferingsAtDates(ctx, dates)
}

func (a EnrollmentBookingProjection) RequestChildOfferingsAtDates(ctx context.Context, dates map[int64]enrollment.Date) ([]*enrollment.RequestChildOffering, error) {
	ownerDates := make(map[int64]careplan.Date, len(dates))
	ids := make([]int64, 0, len(dates))
	for id, date := range dates {
		ownerDates[id] = careplan.Date(date)
		ids = append(ids, id)
	}
	bookings, err := a.bookings.CareOfferingBookingsAtDates(ctx, ownerDates)
	if err != nil {
		return nil, err
	}
	return a.enrollmentBookingValues(ctx, ids, bookings)
}

// EffectiveOfferingSelectionsAtDates serves capacity and course views that do
// not display submission notes, avoiding an unrelated Enrollment lookup.
func (a EnrollmentBookingProjection) EffectiveOfferingSelectionsAtDates(ctx context.Context, dates map[int64]enrollment.Date) ([]*enrollment.RequestChildOffering, error) {
	ownerDates := make(map[int64]careplan.Date, len(dates))
	for id, date := range dates {
		ownerDates[id] = careplan.Date(date)
	}
	bookings, err := a.bookings.CareOfferingBookingsAtDates(ctx, ownerDates)
	if err != nil {
		return nil, err
	}
	return enrollmentEffectiveBookingValues(bookings), nil
}

func (a EnrollmentBookingProjection) RequestChildOfferingsAtDate(ctx context.Context, childID int64, date enrollment.Date) ([]*enrollment.RequestChildOffering, error) {
	rows, err := a.RequestChildOfferingsForChildrenAtDate(ctx, []int64{childID}, date)
	if err != nil || len(rows) > 0 {
		return rows, err
	}
	child, err := a.ChildByID(ctx, childID)
	if err != nil {
		return nil, err
	}
	request, err := a.RequestByID(ctx, child.RequestID, false)
	if err != nil {
		return nil, err
	}
	phase, err := a.Phase(ctx, request.PhaseID)
	if err != nil {
		return nil, err
	}
	if !date.Before(phase.ServiceStartDate) {
		return nil, nil
	}
	// Before the phase starts, show its first upcoming interval. Gaps after
	// the start remain empty; this is not a fallback to legacy storage.
	history, err := a.RequestChildOfferingHistory(ctx, childID)
	if err != nil {
		return nil, err
	}
	var next *enrollment.Date
	for _, row := range history {
		if row.ValidFrom != nil && date.Before(*row.ValidFrom) && (next == nil || row.ValidFrom.Before(*next)) {
			next = row.ValidFrom
		}
	}
	var result []*enrollment.RequestChildOffering
	for _, row := range history {
		if next != nil && row.ValidFrom != nil && *row.ValidFrom == *next {
			result = append(result, row)
		}
	}
	return result, nil
}

func (a EnrollmentBookingProjection) enrollmentBookingValues(ctx context.Context, childIDs []int64, bookings []careplan.CareOfferingBooking) ([]*enrollment.RequestChildOffering, error) {
	if len(bookings) == 0 {
		return nil, nil
	}
	choices, err := a.SubmittedOfferingChoices(ctx, childIDs)
	if err != nil {
		return nil, err
	}
	notes := make(map[[2]int64]*string, len(choices))
	for _, choice := range choices {
		notes[[2]int64{choice.RequestChildID, choice.CareOfferingID}] = choice.Notes
	}
	rows := enrollmentEffectiveBookingValues(bookings)
	for _, row := range rows {
		row.Notes = notes[[2]int64{row.RequestChildID, row.CareOfferingID}]
	}
	return rows, nil
}

func enrollmentEffectiveBookingValues(bookings []careplan.CareOfferingBooking) []*enrollment.RequestChildOffering {
	rows := make([]*enrollment.RequestChildOffering, 0, len(bookings))
	for _, booking := range bookings {
		rows = append(rows, &enrollment.RequestChildOffering{
			ID: booking.ID, TenantID: booking.TenantID, RequestChildID: booking.RequestChildID, CareOfferingID: booking.CareOfferingID,
			SelectedDays: booking.EffectiveSelectedDays(), ManualSelectedDays: booking.ManualSelectedDays, AutomaticSelectedDays: booking.AutomaticSelectedDays,
			CreatedAt: booking.CreatedAt, UpdatedAt: booking.UpdatedAt,
			ValidFrom: enrollmentBookingDate(booking.ValidFrom), ValidUntil: enrollmentBookingDate(booking.ValidUntil),
		})
	}
	return rows
}
