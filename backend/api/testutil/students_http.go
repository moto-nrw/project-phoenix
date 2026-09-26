package testutil

import (
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/services"
)

// The compositions the student route suites
// (modules/peopledirectory/inbound/students) drive (#2731), composed in
// services so the suites import neither the retained repositories nor the
// retained models and services.

// NewStudentRouteRows composes the retained repositories the student route
// suites arrange and assert through; a nil command records into the test
// audit store.
func NewStudentRouteRows(db *bun.DB, command services.StudentHTTPTestAuditCommand) (services.StudentHTTPTestRows, error) {
	return services.NewStudentHTTPTestRows(db, command)
}

// NewStudentRoutePeople is the People Directory owner over the test database.
func NewStudentRoutePeople(db *bun.DB) services.StudentHTTPTestPeople {
	return services.NewStudentHTTPTestPeople(db)
}

// NewStudentRouteCompanions composes Care Plan's companion graph over the
// suite's rows; audit may be nil for suites that never widen a plan.
func NewStudentRouteCompanions(rows services.StudentHTTPTestRows, people services.StudentHTTPTestPeople, audit services.StudentHTTPTestChangeRecorder) services.StudentHTTPTestCompanions {
	return services.NewStudentHTTPTestCompanions(rows, people, audit)
}

// NewStudentRouteConsents binds the consent projection and its Audit
// Platform trail over the test database.
func NewStudentRouteConsents(db *bun.DB) services.StudentHTTPTestConsents {
	return services.NewStudentHTTPTestConsents(db)
}

// NewStudentRoutePickupReviewBlocks are the timetable blocks a pickup review
// shows, over the test database.
func NewStudentRoutePickupReviewBlocks(db *bun.DB) services.StudentHTTPTestPickupReviewBlocks {
	return services.NewStudentHTTPTestPickupReviewBlocks(db)
}

// NewStudentRouteMembership is the School Membership owner over the test
// database.
func NewStudentRouteMembership(db *bun.DB) (services.StudentHTTPTestMembership, error) {
	return services.NewStudentHTTPTestMembership(db)
}

// NewStudentRouteCareLifecycle composes the native Care Plan lifecycle over
// the test database.
func NewStudentRouteCareLifecycle(db *bun.DB, config services.StudentHTTPTestCareLifecycle) (services.StudentHTTPTestCareLifecycleOwner, error) {
	return services.NewStudentHTTPTestCareLifecycle(db, config)
}

// MustNewStudentRouteRows is NewStudentRouteRows recording into the test
// audit store; a composition failure is a broken graph and panics.
func MustNewStudentRouteRows(db *bun.DB) services.StudentHTTPTestRows {
	rows, err := services.NewStudentHTTPTestRows(db, nil)
	if err != nil {
		panic(err)
	}
	return rows
}
