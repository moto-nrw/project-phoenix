package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carecompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentcompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	timetablecompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/uptrace/bun"
)

type ScheduleReviewPeople interface {
	ReviewPeople
	peopledirectory.StudentDepartureQuery
}

type TimetableObservation = timetablecompose.Observation

type ScheduleReviewDependencies struct {
	People                ScheduleReviewPeople
	Scope                 carecompose.ReviewScopeResolver
	BookingsAuthoritative func(context.Context) (bool, error)
	Today                 func() careplan.Date
	ObserveCare           func(CareObservation)
	ObserveTimetable      func(timetablecompose.Observation)
	// Blocks previews the blocks a pickup change would excuse; the root
	// binds it over the Timetable plan and the Student Presence execution.
	Blocks carecompose.PickupReviewBlocks
}

// NewScheduleReviews binds native owner reads. No retained service is needed
// to resolve a child's recurring schedule or preview a pickup change.
func NewScheduleReviews(db *bun.DB, deps ScheduleReviewDependencies) (careplan.CareScheduleReviewQuery, error) {
	if deps.People == nil || deps.Blocks == nil {
		return nil, errors.New("care schedule reviews: people and pickup review blocks are required")
	}
	classes, err := timetablecompose.NewClassArrivalQueries(db, deps.ObserveTimetable)
	if err != nil {
		return nil, err
	}
	return carecompose.NewScheduleReviews(db, deps.ObserveCare, carecompose.ScheduleReviewDependencies{
		People:   scheduleReviewDirectory{reviewDirectory: reviewDirectory{people: deps.People}, departures: deps.People},
		Bookings: scheduleReviewBookings{query: enrollmentcompose.New(), bookings: carecompose.NewOfferingBookings()}, Classes: classes,
		Scope: deps.Scope, BookingsAuthoritative: deps.BookingsAuthoritative, Today: deps.Today,
		Blocks: deps.Blocks, Fingerprint: pickupImpactFingerprint,
	})
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

func (b scheduleReviewBookings) ApprovedForStudents(ctx context.Context, ids []int64, from, to careplan.Date) ([]carecompose.ReviewBooking, error) {
	if to.Before(from) {
		return []carecompose.ReviewBooking{}, nil
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
	result := make([]carecompose.ReviewBooking, 0, len(rows))
	for _, row := range rows {
		if (row.ValidUntil != nil && !from.Before(*row.ValidUntil)) || (row.ValidFrom != nil && to.Before(*row.ValidFrom)) {
			continue
		}
		booking := carecompose.ReviewBooking{StudentID: students[row.RequestChildID], OfferingID: row.CareOfferingID, SelectedDays: row.EffectiveSelectedDays()}
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
