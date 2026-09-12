package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carecompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentcompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
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
}

// NewScheduleReviews binds native owner reads. No retained service is needed
// to resolve a child's recurring schedule or preview a pickup change.
func NewScheduleReviews(db *bun.DB, deps ScheduleReviewDependencies) (careplan.CareScheduleReviewQuery, error) {
	if deps.People == nil {
		return nil, errors.New("care schedule reviews: people are required")
	}
	classes, err := timetablecompose.NewClassArrivalQueries(db, deps.ObserveTimetable)
	if err != nil {
		return nil, err
	}
	blocks, err := timetablecompose.NewPickupReviewQueries(db, deps.ObserveTimetable)
	if err != nil {
		return nil, err
	}
	return carecompose.NewScheduleReviews(db, deps.ObserveCare, carecompose.ScheduleReviewDependencies{
		People:   scheduleReviewDirectory{reviewDirectory: reviewDirectory{people: deps.People}, departures: deps.People},
		Bookings: scheduleReviewBookings{query: enrollmentcompose.New()}, Classes: classes,
		Scope: deps.Scope, BookingsAuthoritative: deps.BookingsAuthoritative, Today: deps.Today,
		Blocks: scheduleReviewBlocks{query: blocks},
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
		ApprovedSelectionsForStudents(context.Context, []int64, enrollment.Date, enrollment.Date) ([]*enrollment.ApprovedOfferingSelection, error)
	}
}

func (b scheduleReviewBookings) ApprovedForStudents(ctx context.Context, ids []int64, from, to careplan.Date) ([]carecompose.ReviewBooking, error) {
	rows, err := b.query.ApprovedSelectionsForStudents(ctx, ids, enrollment.Date(from), enrollment.Date(to))
	if err != nil {
		return nil, err
	}
	result := make([]carecompose.ReviewBooking, 0, len(rows))
	for _, row := range rows {
		if row == nil || row.Selection == nil {
			continue
		}
		selection := row.Selection
		booking := carecompose.ReviewBooking{StudentID: row.StudentID, OfferingID: selection.CareOfferingID, SelectedDays: selection.SelectedDays}
		if selection.ValidFrom != nil {
			booking.ValidFrom = careplan.Date(*selection.ValidFrom)
		}
		if selection.ValidUntil != nil {
			booking.ValidUntil = careplan.Date(*selection.ValidUntil)
		}
		result = append(result, booking)
	}
	return result, nil
}

type scheduleReviewBlocks struct{ query timetable.PickupReviewQuery }

func (b scheduleReviewBlocks) PreviewPickupBlocks(ctx context.Context, input carecompose.PickupReviewImpact) ([]carecompose.PickupReviewBlock, error) {
	rows, err := b.query.PreviewPickupBlocks(ctx, timetable.PickupReviewInput{StudentID: input.StudentID, Date: input.Date.String(), From: input.From, Enrolled: input.Enrolled, AutoExceptionIDs: input.AutoExceptionIDs})
	if err != nil {
		return nil, err
	}
	result := make([]carecompose.PickupReviewBlock, 0, len(rows))
	for _, row := range rows {
		result = append(result, carecompose.PickupReviewBlock{ID: row.ID, Title: row.Title, StartTime: row.StartTime, EndTime: row.EndTime})
	}
	return result, nil
}
