package compose

import (
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/uptrace/bun"
)

type OfferingReviewDirectory = careplan.OfferingReviewDirectory
type OfferingReviewEnrollment = careplan.OfferingReviewEnrollment
type OfferingReviewChild = careplan.OfferingReviewChild
type OfferingReviewRequest = careplan.OfferingReviewRequest
type OfferingReviewPhase = careplan.OfferingReviewPhase
type OfferingReviewBooking = careplan.OfferingReviewBooking
type OfferingReviewCourses = careplan.OfferingReviewCourses
type OfferingReviewCourseRef = careplan.OfferingReviewCourseRef
type OfferingReviewCourse = careplan.OfferingReviewCourse

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
