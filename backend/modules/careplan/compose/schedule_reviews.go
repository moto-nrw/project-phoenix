package compose

import (
	"context"
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type ScheduleReviewDirectory = careplan.ScheduleReviewDirectory
type ReviewBooking = careplan.ReviewBooking
type ReviewBookings = careplan.ReviewBookings
type ReviewClassTimes = careplan.ReviewClassTimes
type PickupReviewImpact = careplan.PickupReviewImpact
type PickupReviewBlock = careplan.PickupReviewBlock
type PickupReviewBlocks = careplan.PickupReviewBlocks

type ScheduleReviewDependencies struct {
	People                ScheduleReviewDirectory
	Bookings              ReviewBookings
	Classes               ReviewClassTimes
	Scope                 ReviewScopeResolver
	BookingsAuthoritative func(context.Context) (bool, error)
	Today                 func() careplan.Date
	// Fingerprint returns the lowercase hexadecimal SHA-256 of the content.
	Fingerprint      func([]byte) string
	Blocks           PickupReviewBlocks
	Logger           *slog.Logger
	StudentDirectory StudentDirectory
}

func NewScheduleReviews(db *bun.DB, observe func(Observation), deps ScheduleReviewDependencies) (careplan.CareScheduleReviewQuery, error) {
	if db == nil || observe == nil || deps.People == nil || deps.Bookings == nil || deps.Classes == nil || deps.Scope == nil || deps.BookingsAuthoritative == nil || deps.Today == nil || deps.Blocks != nil && deps.Fingerprint == nil {
		return nil, errors.New("care schedule reviews: database, observer, people, bookings, classes, scope, booking policy, clock, and impact fingerprint for block previews are required")
	}
	requests, err := NewCareScheduleRequestQueries(db, deps.StudentDirectory, observe)
	if err != nil {
		return nil, err
	}
	queries := application.NewExceptionQueryService(postgres.New(postgres.CarePlanDatabase(db)), observe)
	return application.NewScheduleReviews(application.ScheduleReviewDependencies{RunInTx: tenant.NewTransactionRunner().RunInTx,
		Requests: requests, Schedules: queries,
		People: deps.People, Bookings: deps.Bookings, Classes: deps.Classes, Scope: deps.Scope,
		BookingsAuthoritative: deps.BookingsAuthoritative, Today: deps.Today, Blocks: deps.Blocks, Fingerprint: deps.Fingerprint, Logger: deps.Logger,
	}), nil
}
