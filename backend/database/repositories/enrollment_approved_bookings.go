package repositories

import (
	"context"
	"sort"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

func (a EnrollmentBookingProjection) ApprovedSelectionsForOfferings(ctx context.Context, offeringIDs []int64, onOrAfter enrollment.Date) ([]*enrollment.ApprovedOfferingSelection, error) {
	bookings, children, err := a.offeringCapacityFacts(ctx, offeringIDs)
	if err != nil {
		return nil, err
	}
	var active []careplan.CareOfferingBooking
	for _, booking := range bookings {
		if booking.ValidUntil == nil || careplan.Date(onOrAfter).Before(*booking.ValidUntil) {
			active = append(active, booking)
		}
	}
	return approvedBookingValues(active, children), nil
}

func (a EnrollmentBookingProjection) ApprovedSelectionsForStudents(ctx context.Context, studentIDs []int64, from, to enrollment.Date) ([]*enrollment.ApprovedOfferingSelection, error) {
	if len(studentIDs) == 0 || to.Before(from) {
		return []*enrollment.ApprovedOfferingSelection{}, nil
	}
	children, err := a.ApprovedOfferingChildrenForStudents(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(children))
	byID := make(map[int64]enrollment.OfferingChildFacts, len(children))
	for _, child := range children {
		ids = append(ids, child.ID)
		byID[child.ID] = child
	}
	bookings, err := a.bookings.CareOfferingBookingHistory(ctx, ids)
	if err != nil {
		return nil, err
	}
	var active []careplan.CareOfferingBooking
	for _, booking := range bookings {
		if (booking.ValidUntil == nil || careplan.Date(from).Before(*booking.ValidUntil)) && (booking.ValidFrom == nil || !careplan.Date(to).Before(*booking.ValidFrom)) {
			active = append(active, booking)
		}
	}
	return approvedBookingValues(active, byID), nil
}

func approvedBookingValues(bookings []careplan.CareOfferingBooking, children map[int64]enrollment.OfferingChildFacts) []*enrollment.ApprovedOfferingSelection {
	rows := enrollmentEffectiveBookingValues(bookings)
	result := make([]*enrollment.ApprovedOfferingSelection, 0, len(rows))
	for _, row := range rows {
		child, ok := children[row.RequestChildID]
		if ok && child.Status == enrollment.ChildStatusApproved && child.StudentID() != 0 {
			result = append(result, &enrollment.ApprovedOfferingSelection{Selection: row, StudentID: child.StudentID()})
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Selection.ID < result[j].Selection.ID })
	return result
}
