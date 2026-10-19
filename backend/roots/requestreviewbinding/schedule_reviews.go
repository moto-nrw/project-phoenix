package requestreviewbinding

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

type ScheduleReviewPeople interface {
	ReviewPeople
	peopledirectory.StudentDepartureQuery
}

type scheduleReviewDirectory struct {
	reviewDirectory
	departures peopledirectory.StudentDepartureQuery
}

func (d scheduleReviewDirectory) DepartureModes(ctx context.Context, ids []int64) (map[int64]map[string][]string, error) {
	return d.departures.ListStudentDepartureModes(ctx, ids)
}

type scheduleReviewBookings struct {
	query interface {
		ApprovedOfferingChildrenForStudents(context.Context, []int64) ([]enrollment.OfferingChildFacts, error)
	}
	bookings careplan.OfferingBookingQueries
}

func (b scheduleReviewBookings) ApprovedForStudents(ctx context.Context, ids []int64, from, to careplan.Date) ([]careplan.ReviewBooking, error) {
	if to.Before(from) {
		return []careplan.ReviewBooking{}, nil
	}
	children, err := b.query.ApprovedOfferingChildrenForStudents(ctx, ids)
	if err != nil {
		return nil, err
	}
	childIDs := make([]int64, 0, len(children))
	students := make(map[int64]int64, len(children))
	for _, child := range children {
		childIDs = append(childIDs, child.ID)
		students[child.ID] = child.StudentID()
	}
	rows, err := b.bookings.CareOfferingBookingHistory(ctx, childIDs)
	if err != nil {
		return nil, err
	}
	result := make([]careplan.ReviewBooking, 0, len(rows))
	for _, row := range rows {
		if (row.ValidUntil != nil && !from.Before(*row.ValidUntil)) || (row.ValidFrom != nil && to.Before(*row.ValidFrom)) {
			continue
		}
		booking := careplan.ReviewBooking{StudentID: students[row.RequestChildID], OfferingID: row.CareOfferingID, SelectedDays: row.EffectiveSelectedDays()}
		if row.ValidFrom != nil {
			booking.ValidFrom = *row.ValidFrom
		}
		if row.ValidUntil != nil {
			booking.ValidUntil = *row.ValidUntil
		}
		result = append(result, booking)
	}
	return result, nil
}

func NewScheduleReviewDirectory(people ScheduleReviewPeople) careplan.ScheduleReviewDirectory {
	return scheduleReviewDirectory{reviewDirectory: reviewDirectory{people: people}, departures: people}
}
func NewScheduleReviewBookings(query interface {
	ApprovedOfferingChildrenForStudents(context.Context, []int64) ([]enrollment.OfferingChildFacts, error)
}, bookings careplan.OfferingBookingQueries) careplan.ReviewBookings {
	return scheduleReviewBookings{query: query, bookings: bookings}
}
