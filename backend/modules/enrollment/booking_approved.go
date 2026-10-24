package enrollment

import (
	"context"
	"sort"
)

// ApprovedSelectionsForOfferings lists the approved bookings of the offerings
// that still run on or after the date, with the student each one serves.
func (p *BookingProjection) ApprovedSelectionsForOfferings(ctx context.Context, offeringIDs []int64, onOrAfter Date) ([]*ApprovedOfferingSelection, error) {
	bookings, children, err := p.offeringCapacityFacts(ctx, offeringIDs)
	if err != nil {
		return nil, err
	}
	var active []OfferingBooking
	for _, booking := range bookings {
		if booking.ValidUntil == nil || onOrAfter.Before(*booking.ValidUntil) {
			active = append(active, booking)
		}
	}
	return approvedSelections(active, children), nil
}

// ApprovedSelectionsForStudents lists the approved bookings of the students
// that overlap the inclusive range.
func (p *BookingProjection) ApprovedSelectionsForStudents(ctx context.Context, studentIDs []int64, from, to Date) ([]*ApprovedOfferingSelection, error) {
	if len(studentIDs) == 0 || to.Before(from) {
		return []*ApprovedOfferingSelection{}, nil
	}
	children, err := p.ApprovedOfferingChildrenForStudents(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(children))
	byID := make(map[int64]OfferingChildFacts, len(children))
	for _, child := range children {
		ids = append(ids, child.ID)
		byID[child.ID] = child
	}
	bookings, err := p.bookings.CareOfferingBookingHistory(ctx, ids)
	if err != nil {
		return nil, err
	}
	var active []OfferingBooking
	for _, booking := range bookings {
		if (booking.ValidUntil == nil || from.Before(*booking.ValidUntil)) && (booking.ValidFrom == nil || !to.Before(*booking.ValidFrom)) {
			active = append(active, booking)
		}
	}
	return approvedSelections(active, byID), nil
}

func approvedSelections(bookings []OfferingBooking, children map[int64]OfferingChildFacts) []*ApprovedOfferingSelection {
	rows := effectiveSelections(bookings)
	result := make([]*ApprovedOfferingSelection, 0, len(rows))
	for _, row := range rows {
		child, ok := children[row.RequestChildID]
		if ok && child.Status == ChildStatusApproved && child.StudentID() != 0 {
			result = append(result, &ApprovedOfferingSelection{Selection: row, StudentID: child.StudentID()})
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Selection.ID < result[j].Selection.ID })
	return result
}
