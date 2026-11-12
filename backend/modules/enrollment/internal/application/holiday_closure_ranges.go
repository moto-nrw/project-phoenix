package application

import (
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// traegerChangesWithin keeps only changes covered by the Träger approval.
func traegerChangesWithin(bookings []changedBooking, closure enrollment.HolidayClosure, from, to calendar.Date) []changedBooking {
	// The bounding span batches reads; approval covers only the old and new
	// source ranges at each school, never independent changes in the gaps.
	ranges := map[int64][]enrollment.HolidayClosureRange{}
	add := func(schools []int64, from, to calendar.Date) {
		for _, school := range schools {
			ranges[school] = append(ranges[school], enrollment.HolidayClosureRange{From: from, To: to})
		}
	}
	if !closure.Deleted {
		add(closure.SchoolIDs, closure.StartsOn, closure.EndsOn)
	}
	if closure.Previous != nil {
		add(closure.Previous.SchoolIDs, closure.Previous.StartsOn, closure.Previous.EndsOn)
	}
	var result []changedBooking
	for _, booking := range bookings {
		operation := enrollment.HolidayClosureOperation{From: from, To: to, Ranges: ranges[booking.HostSchoolID], ConstrainRanges: true}
		booking.closed = daysWithinOperation(booking.closed, operation)
		booking.reopened = daysWithinOperation(booking.reopened, operation)
		if len(booking.closed) > 0 || len(booking.reopened) > 0 {
			result = append(result, booking)
		}
	}
	return result
}
