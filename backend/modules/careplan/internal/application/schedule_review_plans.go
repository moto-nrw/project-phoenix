package application

import (
	"context"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// preloadWeeklyPlans publishes complete plans only after every batch succeeds.
// On a batch error the existing pickup facts remain available, and cachedPlan
// retries the remaining inputs in their scalar order for each child separately.
func (s *ScheduleReviews) preloadWeeklyPlans(ctx context.Context, ids []int64, students map[int64]careplan.ReviewStudent, today careplan.Date, cache reviewPlanCache) {
	arrivals, err := s.deps.Schedules.ListArrivalSchedules(ctx, careplan.StudentScheduleFilter{StudentIDs: ids})
	if err != nil {
		return
	}
	classes := make([]string, 0, len(ids))
	for _, id := range ids {
		classes = append(classes, students[id].SchoolClass)
	}
	classTimes, err := s.deps.Classes.ListClassArrivalTimes(ctx, classes)
	if err != nil {
		return
	}
	modes, err := s.deps.People.ListStudentDepartureModes(ctx, ids)
	if err != nil {
		return
	}
	authoritative, err := s.deps.BookingsAuthoritative(ctx)
	if err != nil {
		return
	}
	from, to := reviewWeekRange(today)
	bookings, err := s.deps.Bookings.ApprovedForStudents(ctx, ids, from, to)
	if err != nil {
		return
	}
	offerings, err := s.reviewBookingOfferings(ctx, bookings)
	if err != nil {
		return
	}
	for _, id := range ids {
		cache.plans[id] = reviewPlanFacts{
			pickups: cache.pickups[id], classTimes: classTimes[strings.ToLower(strings.TrimSpace(students[id].SchoolClass))],
			departureModes: modes[id], authoritative: authoritative,
		}
	}
	for _, row := range arrivals {
		if plan, ok := cache.plans[row.StudentID]; ok {
			plan.arrivals = append(plan.arrivals, row)
			cache.plans[row.StudentID] = plan
		}
	}
	cache.attachBookings(bookings, offerings)
}

func (s *ScheduleReviews) reviewBookingOfferings(ctx context.Context, bookings []careplan.ReviewBooking) ([]careplan.CareOffering, error) {
	ids := make([]int64, 0, len(bookings))
	for _, booking := range bookings {
		ids = append(ids, booking.OfferingID)
	}
	if len(ids) == 0 {
		return nil, nil
	}
	return s.deps.Schedules.ListCareOfferings(ctx, careplan.CareOfferingFilter{IDs: ids})
}

func (cache reviewPlanCache) attachBookings(bookings []careplan.ReviewBooking, offerings []careplan.CareOffering) {
	byID := make(map[int64]careplan.CareOffering, len(offerings))
	for _, row := range offerings {
		byID[row.ID] = row
	}
	for _, booking := range bookings {
		plan, visible := cache.plans[booking.StudentID]
		offering, known := byID[booking.OfferingID]
		if visible && known {
			plan.bookings = append(plan.bookings, reviewPlanSelection{offering: offering, selectedDays: booking.SelectedDays, from: booking.ValidFrom, until: booking.ValidUntil})
			cache.plans[booking.StudentID] = plan
		}
	}
}
