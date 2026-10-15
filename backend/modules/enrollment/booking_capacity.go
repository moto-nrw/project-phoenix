package enrollment

import (
	"context"
	"fmt"
	"slices"
	"sort"
)

func (p *BookingProjection) offeringCapacityFacts(ctx context.Context, offeringIDs []int64) ([]OfferingBooking, map[int64]OfferingChildFacts, error) {
	bookings, err := p.bookings.CareOfferingBookingsForOfferings(ctx, offeringIDs)
	if err != nil {
		return nil, nil, err
	}
	children, err := p.capacityChildren(ctx, bookings)
	return bookings, children, err
}

func (p *BookingProjection) capacityChildren(ctx context.Context, bookings []OfferingBooking) (map[int64]OfferingChildFacts, error) {
	ids := make([]int64, 0, len(bookings))
	for _, booking := range bookings {
		ids = append(ids, booking.RequestChildID)
	}
	slices.Sort(ids)
	children, err := p.OfferingChildren(ctx, slices.Compact(ids))
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]OfferingChildFacts, len(children))
	for _, child := range children {
		byID[child.ID] = child
	}
	return byID, nil
}

// OfferingCatalogState batches the current selection and the phase's capacity
// into the same owner reads, independent of the number of catalog offerings.
func (p *BookingProjection) OfferingCatalogState(ctx context.Context, phaseID, childID int64, from, until Date) (*OfferingCatalogState, error) {
	if !from.Before(until) {
		return nil, fmt.Errorf("capacity range must not be empty")
	}
	bookings, err := p.bookings.CareOfferingBookingsForPhase(ctx, phaseID)
	if err != nil {
		return nil, err
	}
	children, err := p.capacityChildren(ctx, bookings)
	if err != nil {
		return nil, err
	}
	var current []OfferingBooking
	for _, booking := range bookings {
		if booking.RequestChildID == childID && validOn(booking, from) {
			current = append(current, booking)
		}
	}
	return &OfferingCatalogState{Current: effectiveSelections(current), CapacityPeaks: offeringCapacityPeaks(bookings, children, nil, from, until)}, nil
}

// validOn reports whether the booking's half-open interval contains the date.
func validOn(booking OfferingBooking, date Date) bool {
	return (booking.ValidFrom == nil || !date.Before(*booking.ValidFrom)) && (booking.ValidUntil == nil || date.Before(*booking.ValidUntil))
}

// capacityChild reports whether the child occupies a place: every child but
// a rejected or withdrawn one does.
func capacityChild(children map[int64]OfferingChildFacts, childID int64) bool {
	child, ok := children[childID]
	return ok && child.Status != ChildStatusRejected && child.Status != ChildStatusWithdrawn
}

// OfferingCapacityPeak is the most children the offering holds at once in the
// range, without the excluded children.
func (p *BookingProjection) OfferingCapacityPeak(ctx context.Context, offeringID int64, excluded []int64, from, until Date) (int, error) {
	if !from.Before(until) {
		return 0, fmt.Errorf("capacity range must not be empty")
	}
	bookings, children, err := p.offeringCapacityFacts(ctx, []int64{offeringID})
	if err != nil {
		return 0, err
	}
	return offeringCapacityPeaks(bookings, children, excluded, from, until)[offeringID], nil
}

// OfferingCapacityPeaks is the capacity peak of each offering in the range.
func (p *BookingProjection) OfferingCapacityPeaks(ctx context.Context, offeringIDs []int64, from, until Date) (map[int64]int, error) {
	if len(offeringIDs) == 0 {
		return map[int64]int{}, nil
	}
	if !from.Before(until) {
		return nil, fmt.Errorf("capacity range must not be empty")
	}
	bookings, children, err := p.offeringCapacityFacts(ctx, offeringIDs)
	if err != nil {
		return nil, err
	}
	return offeringCapacityPeaks(bookings, children, nil, from, until), nil
}

type offeringCapacityEvent struct {
	date    Date
	childID int64
	delta   int
}

// offeringCapacityPeaks evaluates each date after applying both ends and
// starts, preserving the half-open boundary and counting each child once even
// across adjacent rows.
func offeringCapacityPeaks(bookings []OfferingBooking, children map[int64]OfferingChildFacts, excluded []int64, from, until Date) map[int64]int {
	events := make(map[int64][]offeringCapacityEvent)
	for _, booking := range bookings {
		if !capacityChild(children, booking.RequestChildID) || slices.Contains(excluded, booking.RequestChildID) {
			continue
		}
		start, end, ok := clippedInterval(booking, from, until)
		if !ok {
			continue
		}
		events[booking.CareOfferingID] = append(events[booking.CareOfferingID], offeringCapacityEvent{start, booking.RequestChildID, 1}, offeringCapacityEvent{end, booking.RequestChildID, -1})
	}
	peaks := make(map[int64]int, len(events))
	for offeringID, rows := range events {
		peaks[offeringID] = capacityPeak(rows)
	}
	return peaks
}

// clippedInterval is the booking's interval within the range; ok is false
// when nothing of it remains.
func clippedInterval(booking OfferingBooking, from, until Date) (start, end Date, ok bool) {
	start, end = from, until
	if booking.ValidFrom != nil && start.Before(*booking.ValidFrom) {
		start = *booking.ValidFrom
	}
	if booking.ValidUntil != nil && booking.ValidUntil.Before(end) {
		end = *booking.ValidUntil
	}
	return start, end, start.Before(end)
}

// capacityPeak sweeps the events in date order and returns the most distinct
// children active at once.
func capacityPeak(rows []offeringCapacityEvent) int {
	sort.Slice(rows, func(i, j int) bool { return rows[i].date.Before(rows[j].date) })
	active := make(map[int64]int)
	peak := 0
	for i := 0; i < len(rows); {
		date := rows[i].date
		for ; i < len(rows) && rows[i].date == date; i++ {
			active[rows[i].childID] += rows[i].delta
			if active[rows[i].childID] == 0 {
				delete(active, rows[i].childID)
			}
		}
		peak = max(peak, len(active))
	}
	return peak
}

// MaterializableOfferingCount counts the bookings of the offering that still
// run after today, of children that occupy a place.
func (p *BookingProjection) MaterializableOfferingCount(ctx context.Context, offeringID int64, today Date) (int, error) {
	bookings, children, err := p.offeringCapacityFacts(ctx, []int64{offeringID})
	if err != nil {
		return 0, err
	}
	count := 0
	for _, booking := range bookings {
		if capacityChild(children, booking.RequestChildID) && (booking.ValidUntil == nil || today.Before(*booking.ValidUntil)) {
			count++
		}
	}
	return count, nil
}

// OfferingGradeCounts counts the children of each offering and target grade
// that overlap the range.
func (p *BookingProjection) OfferingGradeCounts(ctx context.Context, offeringIDs []int64, from, until Date) ([]*OfferingGradeCount, error) {
	if len(offeringIDs) == 0 {
		return []*OfferingGradeCount{}, nil
	}
	if !from.Before(until) {
		return nil, fmt.Errorf("grade level range must not be empty")
	}
	bookings, children, err := p.offeringCapacityFacts(ctx, offeringIDs)
	if err != nil {
		return nil, err
	}
	return offeringGradeCounts(bookings, children, from, until), nil
}

type gradeKey struct {
	offeringID int64
	grade      int16
	hasGrade   bool
}

func offeringGradeCounts(bookings []OfferingBooking, children map[int64]OfferingChildFacts, from, until Date) []*OfferingGradeCount {
	groups := make(map[gradeKey]map[int64]struct{})
	for _, booking := range bookings {
		if !capacityChild(children, booking.RequestChildID) || !overlaps(booking, from, until) {
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
	result := make([]*OfferingGradeCount, 0, len(groups))
	for key, ids := range groups {
		row := &OfferingGradeCount{CareOfferingID: key.offeringID, Count: len(ids)}
		if key.hasGrade {
			row.GradeLevel = new(key.grade)
		}
		result = append(result, row)
	}
	sort.Slice(result, func(i, j int) bool { return gradeCountBefore(result[i], result[j]) })
	return result
}

// overlaps reports whether the booking's interval overlaps the half-open
// range.
func overlaps(booking OfferingBooking, from, until Date) bool {
	return (booking.ValidFrom == nil || booking.ValidFrom.Before(until)) && (booking.ValidUntil == nil || from.Before(*booking.ValidUntil))
}

// gradeCountBefore orders by offering, then by grade with the ungraded row
// last.
func gradeCountBefore(a, b *OfferingGradeCount) bool {
	if a.CareOfferingID != b.CareOfferingID {
		return a.CareOfferingID < b.CareOfferingID
	}
	if a.GradeLevel == nil {
		return false
	}
	if b.GradeLevel == nil {
		return true
	}
	return *a.GradeLevel < *b.GradeLevel
}
