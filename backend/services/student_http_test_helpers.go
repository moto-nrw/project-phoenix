package services

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/services/users"
)

// The compositions the student route suites
// (modules/peopledirectory/inbound/students) drive (#2731), composed here so
// the suites import neither the retained repositories nor the retained
// models and services.

// StudentHTTPTestRows are the retained repositories the student route suites
// arrange and assert through.
type StudentHTTPTestRows = repositories.StudentTestRepositories

// StudentHTTPTestAuditCommand is the audit command the suite's rows record
// into.
type StudentHTTPTestAuditCommand = auditModels.Command

// StudentHTTPTestChangeRecorder records a companion widening in the
// change history.
type StudentHTTPTestChangeRecorder = users.StudentChangeRecorder

// The owner capabilities the student route suites receive, named here so the
// test support forwarding them imports none of the owners.
type (
	StudentHTTPTestPeople             = peopledirectory.Capability
	StudentHTTPTestCompanions         = careplan.StudentCompanions
	StudentHTTPTestPickupReviewBlocks = carePlanCompose.PickupReviewBlocks
	StudentHTTPTestMembership         = schoolmembership.Capability
	StudentHTTPTestCareLifecycleOwner = careplan.CareLifecycle
)

// NewStudentHTTPTestRows composes the retained repositories over the test
// database, recording into command; nil records into the test audit store.
func NewStudentHTTPTestRows(db *bun.DB, command auditModels.Command) (StudentHTTPTestRows, error) {
	if command == nil {
		command = repositories.NewTestAuditStore(db)
	}
	return repositories.NewStudentTestRepositories(db, command)
}

// NewStudentHTTPTestPeople is the People Directory owner over the test
// database.
func NewStudentHTTPTestPeople(db *bun.DB) peopledirectory.Capability {
	return repositories.MustNewPeopleDirectory(db)
}

// NewStudentHTTPTestCompanions composes Care Plan's companion graph over the
// suite's rows. audit may be nil only for suites that never widen a
// companion's plan.
func NewStudentHTTPTestCompanions(rows StudentHTTPTestRows, people peopledirectory.Capability, audit users.StudentChangeRecorder) careplan.StudentCompanions {
	var recorder repositories.StudentChangeAudit
	if audit != nil {
		recorder = audit
	}
	return repositories.MustNewStudentCompanions(rows.CarePlan, rows.Student, people, recorder)
}

// StudentHTTPTestConsents is the consent projection and trail the student
// routes read and append to.
type StudentHTTPTestConsents = *repositories.StudentConsents

// NewStudentHTTPTestConsents binds the consent projection and the Audit
// Platform trail over the test database.
func NewStudentHTTPTestConsents(db *bun.DB) StudentHTTPTestConsents {
	return repositories.NewStudentConsents(db)
}

// NewStudentHTTPTestPickupReviewBlocks are the timetable blocks a pickup
// review shows, over the test database.
func NewStudentHTTPTestPickupReviewBlocks(db *bun.DB) carePlanCompose.PickupReviewBlocks {
	return repositories.NewPickupReviewBlocks(db)
}

// NewStudentHTTPTestMembership is the School Membership owner over the test
// database.
func NewStudentHTTPTestMembership(db *bun.DB) (schoolmembership.Capability, error) {
	return repositories.NewSchoolMembership(db)
}

// StudentHTTPTestCareLifecycle tunes the Care Plan lifecycle a student route
// suite composes.
type StudentHTTPTestCareLifecycle struct {
	BookingsAuthoritative func(context.Context) (bool, error)
	Today                 func() timezone.Date
	AuditActor            users.RequestAuditActor
}

// NewStudentHTTPTestCareLifecycle composes the native Care Plan lifecycle
// over the test database, recording care-end history through the owner's
// change history.
func NewStudentHTTPTestCareLifecycle(db *bun.DB, config StudentHTTPTestCareLifecycle) (careplan.CareLifecycle, error) {
	repos, err := repositories.NewCareLifecycleTestRepositories(db, nil)
	if err != nil {
		return nil, err
	}
	return repos.NewCareLifecycle(repositories.CareLifecycleTestConfig{
		Audit:                 users.NewStudentAuditService(config.AuditActor, repositories.NewStudentAudit(db)),
		BookingsAuthoritative: config.BookingsAuthoritative,
		Today:                 config.Today,
	})
}

// StudentRoutePersons binds the student routes' person port over the
// module's retained person service.
func (m StudentTestModule) StudentRoutePersons() StudentRoutePersons {
	return NewStudentRoutePersons(m.Users)
}
