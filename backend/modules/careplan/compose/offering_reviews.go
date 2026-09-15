package compose

import (
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/uptrace/bun"
)

type OfferingReviewDirectory = ports.OfferingReviewDirectory
type OfferingReviewEnrollment = ports.OfferingReviewEnrollment
type OfferingReviewChild = ports.OfferingReviewChild
type OfferingReviewRequest = ports.OfferingReviewRequest
type OfferingReviewPhase = ports.OfferingReviewPhase
type OfferingReviewBooking = ports.OfferingReviewBooking
type OfferingReviewCourses = ports.OfferingReviewCourses
type OfferingReviewCourseRef = ports.OfferingReviewCourseRef
type OfferingReviewCourse = ports.OfferingReviewCourse

type OfferingReviewDependencies struct {
	People     OfferingReviewDirectory
	Enrollment OfferingReviewEnrollment
	Courses    OfferingReviewCourses
	Scope      ReviewScopeResolver
	Today      func() careplan.Date
	Logger     *slog.Logger
}

func NewOfferingReviews(db *bun.DB, observe func(Observation), deps OfferingReviewDependencies) (careplan.OfferingReviewQuery, error) {
	if db == nil || observe == nil || deps.People == nil || deps.Enrollment == nil || deps.Courses == nil || deps.Scope == nil || deps.Today == nil {
		return nil, errors.New("offering reviews: database, observer, people, enrollment, courses, scope, and clock are required")
	}
	requests, err := NewOfferingChangeRequestQueries(db, observe)
	if err != nil {
		return nil, err
	}
	return application.NewOfferingReviews(application.OfferingReviewDependencies{
		Requests: requests, Catalog: application.New(postgres.New(carePlanDatabase(db)), observe),
		People: deps.People, Enrollment: deps.Enrollment, Courses: deps.Courses,
		Scope: deps.Scope, Today: deps.Today, Logger: deps.Logger,
	}), nil
}
