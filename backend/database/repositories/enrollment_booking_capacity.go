package repositories

import (
	"context"
	"fmt"
	"slices"
	"sort"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

func (a EnrollmentBookingProjection) offeringCapacityFacts(ctx context.Context, offeringIDs []int64) ([]careplan.CareOfferingBooking, map[int64]enrollment.OfferingChildFacts, error) {
	bookings, err := a.bookings.CareOfferingBookingsForOfferings(ctx, offeringIDs)
	if err != nil {
		return nil, nil, err
	}
	children, err := a.capacityChildren(ctx, bookings)
	return bookings, children, err
}

func (a EnrollmentBookingProjection) capacityChildren(ctx context.Context, bookings []careplan.CareOfferingBooking) (map[int64]enrollment.OfferingChildFacts, error) {
	ids := make([]int64, 0, len(bookings))
	for _, booking := range bookings {
		ids = append(ids, booking.RequestChildID)
	}
	slices.Sort(ids)
	children, err := a.OfferingChildren(ctx, slices.Compact(ids))
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]enrollment.OfferingChildFacts, len(children))
	for _, child := range children {
		byID[child.ID] = child
	}
	return byID, nil
}

// OfferingCatalogState batches the current selection and the phase's capacity
// into the same owner reads, independent of the number of catalog offerings.
func (a EnrollmentBookingProjection) OfferingCatalogState(ctx context.Context, phaseID, childID int64, from, until enrollment.Date) (*enrollment.OfferingCatalogState, error) {
	if !from.Before(until) {
		return nil, fmt.Errorf("capacity range must not be empty")
	}
	bookings, err := a.bookings.CareOfferingBookingsForPhase(ctx, phaseID)
	if err != nil {
		return nil, err
	}
	children, err := a.capacityChildren(ctx, bookings)
	if err != nil {
		return nil, err
	}
	var current []careplan.CareOfferingBooking
	for _, booking := range bookings {
		if booking.RequestChildID == childID && (booking.ValidFrom == nil || !careplan.Date(from).Before(*booking.ValidFrom)) && (booking.ValidUntil == nil || careplan.Date(from).Before(*booking.ValidUntil)) {
			current = append(current, booking)
		}
	}
	return &enrollment.OfferingCatalogState{Current: enrollmentEffectiveBookingValues(current), CapacityPeaks: offeringCapacityPeaks(bookings, children, nil, from, until)}, nil
}

func capacityChild(children map[int64]enrollment.OfferingChildFacts, childID int64) bool {
	child, ok := children[childID]
	return ok && child.Status != enrollment.ChildStatusRejected && child.Status != enrollment.ChildStatusWithdrawn
}

func (a EnrollmentBookingProjection) OfferingCapacityPeak(ctx context.Context, offeringID int64, excluded []int64, from, until enrollment.Date) (int, error) {
	if !from.Before(until) {
		return 0, fmt.Errorf("capacity range must not be empty")
	}
	bookings, children, err := a.offeringCapacityFacts(ctx, []int64{offeringID})
	if err != nil {
		return 0, err
	}
	return offeringCapacityPeaks(bookings, children, excluded, from, until)[offeringID], nil
}

func (a EnrollmentBookingProjection) OfferingCapacityPeaks(ctx context.Context, offeringIDs []int64, from, until enrollment.Date) (map[int64]int, error) {
	if len(offeringIDs) == 0 {
		return map[int64]int{}, nil
	}
	if !from.Before(until) {
		return nil, fmt.Errorf("capacity range must not be empty")
	}
	bookings, children, err := a.offeringCapacityFacts(ctx, offeringIDs)
	if err != nil {
		return nil, err
	}
	return offeringCapacityPeaks(bookings, children, nil, from, until), nil
}

type offeringCapacityEvent struct {
	date    enrollment.Date
	childID int64
	delta   int
}

// Evaluate each date after applying both ends and starts, preserving the
// half-open boundary and counting each child once even across adjacent rows.
func offeringCapacityPeaks(bookings []careplan.CareOfferingBooking, children map[int64]enrollment.OfferingChildFacts, excluded []int64, from, until enrollment.Date) map[int64]int {
	events := make(map[int64][]offeringCapacityEvent)
	for _, booking := range bookings {
		if !capacityChild(children, booking.RequestChildID) || slices.Contains(excluded, booking.RequestChildID) {
			continue
		}
		start, end := from, until
		if booking.ValidFrom != nil && start.Before(enrollment.Date(*booking.ValidFrom)) {
			start = enrollment.Date(*booking.ValidFrom)
		}
		if booking.ValidUntil != nil && enrollment.Date(*booking.ValidUntil).Before(end) {
			end = enrollment.Date(*booking.ValidUntil)
		}
		if !start.Before(end) {
			continue
		}
		events[booking.CareOfferingID] = append(events[booking.CareOfferingID], offeringCapacityEvent{start, booking.RequestChildID, 1}, offeringCapacityEvent{end, booking.RequestChildID, -1})
	}
	peaks := make(map[int64]int, len(events))
	for offeringID, rows := range events {
		sort.Slice(rows, func(i, j int) bool { return rows[i].date.Before(rows[j].date) })
		active := make(map[int64]int)
		for i := 0; i < len(rows); {
			date := rows[i].date
			for i < len(rows) && rows[i].date == date {
				active[rows[i].childID] += rows[i].delta
				if active[rows[i].childID] == 0 {
					delete(active, rows[i].childID)
				}
				i++
			}
			peaks[offeringID] = max(peaks[offeringID], len(active))
		}
	}
	return peaks
}

func (a EnrollmentBookingProjection) MaterializableOfferingCount(ctx context.Context, offeringID int64, today enrollment.Date) (int, error) {
	bookings, children, err := a.offeringCapacityFacts(ctx, []int64{offeringID})
	if err != nil {
		return 0, err
	}
	count := 0
	for _, booking := range bookings {
		if capacityChild(children, booking.RequestChildID) && (booking.ValidUntil == nil || careplan.Date(today).Before(*booking.ValidUntil)) {
			count++
		}
	}
	return count, nil
}

func (a EnrollmentBookingProjection) OfferingGradeCounts(ctx context.Context, offeringIDs []int64, from, until enrollment.Date) ([]*enrollment.OfferingGradeCount, error) {
	if len(offeringIDs) == 0 {
		return []*enrollment.OfferingGradeCount{}, nil
	}
	if !from.Before(until) {
		return nil, fmt.Errorf("grade level range must not be empty")
	}
	bookings, children, err := a.offeringCapacityFacts(ctx, offeringIDs)
	if err != nil {
		return nil, err
	}
	type gradeKey struct {
		offeringID int64
		grade      int16
		hasGrade   bool
	}
	groups := make(map[gradeKey]map[int64]struct{})
	for _, booking := range bookings {
		if !capacityChild(children, booking.RequestChildID) || (booking.ValidFrom != nil && !booking.ValidFrom.Before(careplan.Date(until))) || (booking.ValidUntil != nil && !careplan.Date(from).Before(*booking.ValidUntil)) {
			continue
		}
		key := gradeKey{offeringID: booking.CareOfferingID}
		if grade := children[booking.RequestChildID].TargetGradeLevel; grade != nil {
			key.grade, key.hasGrade = *grade, true
		}
		if groups[key] == nil {
			groups[key] = make(map[int64]struct{})
		}
		groups[key][booking.RequestChildID] = struct{}{}
	}
	result := make([]*enrollment.OfferingGradeCount, 0, len(groups))
	for key, ids := range groups {
		row := &enrollment.OfferingGradeCount{CareOfferingID: key.offeringID, Count: len(ids)}
		if key.hasGrade {
			row.GradeLevel = new(key.grade)
		}
		result = append(result, row)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].CareOfferingID != result[j].CareOfferingID {
			return result[i].CareOfferingID < result[j].CareOfferingID
		}
		if result[i].GradeLevel == nil {
			return false
		}
		if result[j].GradeLevel == nil {
			return true
		}
		return *result[i].GradeLevel < *result[j].GradeLevel
	})
	return result, nil
}
