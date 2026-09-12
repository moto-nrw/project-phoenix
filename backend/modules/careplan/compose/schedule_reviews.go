package compose

import (
	"context"
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/uptrace/bun"
)

type ScheduleReviewDirectory = ports.ScheduleReviewDirectory
type ReviewBooking = ports.ReviewBooking
type ReviewBookings = ports.ReviewBookings
type ReviewClassTimes = ports.ReviewClassTimes
type PickupReviewImpact = ports.PickupReviewImpact
type PickupReviewBlock = ports.PickupReviewBlock
type PickupReviewBlocks = ports.PickupReviewBlocks

type ScheduleReviewDependencies struct {
	People                ScheduleReviewDirectory
	Bookings              ReviewBookings
	Classes               ReviewClassTimes
	Scope                 ReviewScopeResolver
	BookingsAuthoritative func(context.Context) (bool, error)
	Today                 func() careplan.Date
	Blocks                PickupReviewBlocks
	Logger                *slog.Logger
}

func NewScheduleReviews(db *bun.DB, observe func(Observation), deps ScheduleReviewDependencies) (careplan.CareScheduleReviewQuery, error) {
	if db == nil || observe == nil || deps.People == nil || deps.Bookings == nil || deps.Classes == nil || deps.Scope == nil || deps.BookingsAuthoritative == nil || deps.Today == nil {
		return nil, errors.New("care schedule reviews: database, observer, people, bookings, classes, scope, booking policy, and clock are required")
	}
	requests, err := NewCareScheduleRequestQueries(db, observe)
	if err != nil {
		return nil, err
	}
	return application.NewScheduleReviews(application.ScheduleReviewDependencies{
		Requests: requests, Schedules: application.New(postgres.New(carePlanDatabase(db)), observe),
		People: deps.People, Bookings: deps.Bookings, Classes: deps.Classes, Scope: deps.Scope,
		BookingsAuthoritative: deps.BookingsAuthoritative, Today: deps.Today, Blocks: deps.Blocks, Logger: deps.Logger,
	}), nil
}
