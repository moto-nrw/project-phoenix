package enrollment

import (
	"context"
)

// RequestChildOfferingHistory lists every booked interval of the child.
func (p *BookingProjection) RequestChildOfferingHistory(ctx context.Context, childID int64) ([]*RequestChildOffering, error) {
	return p.RequestChildOfferingHistoryForChildren(ctx, []int64{childID})
}

// RequestChildOfferingHistoryForChildren lists every booked interval of the
// children, with the notes the parents submitted.
func (p *BookingProjection) RequestChildOfferingHistoryForChildren(ctx context.Context, childIDs []int64) ([]*RequestChildOffering, error) {
	bookings, err := p.bookings.CareOfferingBookingHistory(ctx, childIDs)
	if err != nil {
		return nil, err
	}
	return p.selectionsWithNotes(ctx, childIDs, bookings)
}

// RequestChildOfferingsForChildrenAtDate lists the selections of the
// children that are effective on the date.
func (p *BookingProjection) RequestChildOfferingsForChildrenAtDate(ctx context.Context, childIDs []int64, date Date) ([]*RequestChildOffering, error) {
	dates := make(map[int64]Date, len(childIDs))
	for _, id := range childIDs {
		dates[id] = date
	}
	return p.RequestChildOfferingsAtDates(ctx, dates)
}

// RequestChildOfferingsAtDates lists the selections of each child that are
// effective on the child's date.
func (p *BookingProjection) RequestChildOfferingsAtDates(ctx context.Context, dates map[int64]Date) ([]*RequestChildOffering, error) {
	ids := make([]int64, 0, len(dates))
	for id := range dates {
		ids = append(ids, id)
	}
	bookings, err := p.bookings.CareOfferingBookingsAtDates(ctx, dates)
	if err != nil {
		return nil, err
	}
	return p.selectionsWithNotes(ctx, ids, bookings)
}

// EffectiveOfferingSelectionsAtDates serves capacity and course views that do
// not display submission notes, avoiding an unrelated Enrollment lookup.
func (p *BookingProjection) EffectiveOfferingSelectionsAtDates(ctx context.Context, dates map[int64]Date) ([]*RequestChildOffering, error) {
	bookings, err := p.bookings.CareOfferingBookingsAtDates(ctx, dates)
	if err != nil {
		return nil, err
	}
	return effectiveSelections(bookings), nil
}

// RequestChildOfferingsAtDate lists the child's selections on the date.
// Before the phase starts it shows the first upcoming interval instead.
func (p *BookingProjection) RequestChildOfferingsAtDate(ctx context.Context, childID int64, date Date) ([]*RequestChildOffering, error) {
	rows, err := p.RequestChildOfferingsForChildrenAtDate(ctx, []int64{childID}, date)
	if err != nil || len(rows) > 0 {
		return rows, err
	}
	started, err := p.phaseStartedBy(ctx, childID, date)
	if err != nil || started {
		return nil, err
	}
	// Before the phase starts, show its first upcoming interval. Gaps after
	// the start remain empty; this is not a fallback to legacy storage.
	history, err := p.RequestChildOfferingHistory(ctx, childID)
	if err != nil {
		return nil, err
	}
	return firstUpcomingInterval(history, date), nil
}

// phaseStartedBy reports whether the service of the child's phase has
// started on the date.
func (p *BookingProjection) phaseStartedBy(ctx context.Context, childID int64, date Date) (bool, error) {
	child, err := p.ChildByID(ctx, childID)
	if err != nil {
		return false, err
	}
	request, err := p.RequestByID(ctx, child.RequestID, false)
	if err != nil {
		return false, err
	}
	phase, err := p.Phase(ctx, request.PhaseID)
	if err != nil {
		return false, err
	}
	return !date.Before(phase.ServiceStartDate), nil
}

// firstUpcomingInterval keeps the rows that start at the earliest start
// after the date.
func firstUpcomingInterval(history []*RequestChildOffering, date Date) []*RequestChildOffering {
	var next *Date
	for _, row := range history {
		if row.ValidFrom != nil && date.Before(*row.ValidFrom) && (next == nil || row.ValidFrom.Before(*next)) {
			next = row.ValidFrom
		}
	}
	if next == nil {
		return nil
	}
	var result []*RequestChildOffering
	for _, row := range history {
		if row.ValidFrom != nil && *row.ValidFrom == *next {
			result = append(result, row)
		}
	}
	return result
}

// selectionsWithNotes adds the parents' submitted notes to the selections.
func (p *BookingProjection) selectionsWithNotes(ctx context.Context, childIDs []int64, bookings []OfferingBooking) ([]*RequestChildOffering, error) {
	if len(bookings) == 0 {
		return nil, nil
	}
	choices, err := p.SubmittedOfferingChoices(ctx, childIDs)
	if err != nil {
		return nil, err
	}
	notes := make(map[[2]int64]*string, len(choices))
	for _, choice := range choices {
		notes[[2]int64{choice.RequestChildID, choice.CareOfferingID}] = choice.Notes
	}
	rows := effectiveSelections(bookings)
	for _, row := range rows {
		row.Notes = notes[[2]int64{row.RequestChildID, row.CareOfferingID}]
	}
	return rows, nil
}
