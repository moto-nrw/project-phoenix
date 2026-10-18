package api

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carecompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	enrollmentcompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	requestreviewcompose "github.com/moto-nrw/project-phoenix/modules/requestreview/compose"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	timetablecompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// NewRequestReviewMasterData builds the owner query and binds its directory port.
func NewRequestReviewMasterData(db transactionruntime.Database, students carecompose.StudentDirectory,
	people requestreviewcompose.MasterDataPeople, scope careplan.ReviewScopeResolver, today func() careplan.Date,
	observe func(carecompose.Observation),
) (careplan.MasterDataReviewQuery, error) {
	requests, err := carecompose.NewStudentDataRequestQueries(db.Pool(), students, observe)
	if err != nil {
		return nil, err
	}
	return carecompose.NewMasterDataReviews(carecompose.MasterDataReviewDependencies{
		Requests: requests, People: requestreviewcompose.NewMasterDataReviewDirectory(people), Scope: scope, Today: today,
	})
}

// NewRequestReviewSchedules binds the already constructed Enrollment and Timetable reads.
func NewRequestReviewSchedules(db transactionruntime.Database, students carecompose.StudentDirectory,
	people requestreviewcompose.ScheduleReviewPeople, scope careplan.ReviewScopeResolver,
	bookingsAuthoritative func(context.Context) (bool, error), today func() careplan.Date,
	observeCare func(carecompose.Observation), observeTimetable func(timetablecompose.Observation), blocks careplan.PickupReviewBlocks,
) (careplan.CareScheduleReviewQuery, error) {
	if people == nil || blocks == nil {
		return nil, errors.New("care schedule reviews: people and pickup review blocks are required")
	}
	classes, err := timetablecompose.NewClassArrivalQueries(db.Pool(), observeTimetable)
	if err != nil {
		return nil, err
	}
	return carecompose.NewScheduleReviews(db.Pool(), observeCare, carecompose.ScheduleReviewDependencies{
		People:   requestreviewcompose.NewScheduleReviewDirectory(people),
		Bookings: requestreviewcompose.NewScheduleReviewBookings(enrollmentcompose.New(), carecompose.NewOfferingBookings()), Classes: classes,
		Scope: scope, BookingsAuthoritative: bookingsAuthoritative, Today: today,
		Blocks: blocks, Fingerprint: securityruntime.Fingerprint, StudentDirectory: students,
	})
}

// NewRequestReviewOfferings binds the owner reviews to native projection readers.
func NewRequestReviewOfferings(db transactionruntime.Database, people requestreviewcompose.OfferingReviewPeople,
	scope careplan.ReviewScopeResolver, today func() careplan.Date,
	observeCare func(carecompose.Observation), observeTimetable func(timetablecompose.Observation),
) (careplan.OfferingReviewQuery, error) {
	if people == nil {
		return nil, errors.New("offering reviews: people are required")
	}
	courses, err := timetablecompose.NewCourseGroupQueries(db.Pool(), observeTimetable)
	if err != nil {
		return nil, err
	}
	return carecompose.NewOfferingReviews(db.Pool(), observeCare, carecompose.OfferingReviewDependencies{
		People:     requestreviewcompose.NewOfferingReviewDirectory(people),
		Enrollment: requestreviewcompose.NewOfferingReviewEnrollment(enrollmentcompose.New(), carecompose.NewOfferingBookings()),
		Courses:    requestreviewcompose.NewOfferingReviewCourses(courses), Scope: scope, Today: today,
	})
}
