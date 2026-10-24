//go:build testhooks

package api

import (
	"context"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	carePlanHTTP "github.com/moto-nrw/project-phoenix/modules/careplan/http"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The compositions the student route suites
// (modules/peopledirectory/inbound/students) drive (#2731), composed here so
// the suites import neither the retained repositories nor the retained
// models and services.

// StudentHTTPTestRows are the Care Plan rows the student route suites
// arrange and assert through.
type StudentHTTPTestRows = StudentTestRepositories

// NewStudentHTTPTestRows composes the retained repositories over the test
// database, recording into command; nil records into the test audit store.
// It returns the Care Plan rows with the Enrollment graph beside them.
func NewStudentHTTPTestRows(db transactionruntime.Database, command auditModels.Command) (StudentHTTPTestRows, EnrollmentTestRepositories, error) {
	if command == nil {
		command = owners.NewTestAuditStore(db)
	}
	return NewStudentTestRepositories(db, command)
}

// NewStudentHTTPTestPeople is the People Directory owner over the test
// database.
func NewStudentHTTPTestPeople(db transactionruntime.Database) peopledirectory.Capability {
	return owners.MustNewPeopleDirectory(db)
}

// NewStudentHTTPTestCompanions composes Care Plan's companion graph over the
// suite's rows. audit may be nil only for suites that never widen a
// companion's plan.
func NewStudentHTTPTestCompanions(rows StudentHTTPTestRows, students peoplePorts.StudentRepository, people peopledirectory.Capability, audit peoplePorts.StudentChangeRecorder) careplan.StudentCompanions {
	var recorder owners.StudentChangeAudit
	if audit != nil {
		recorder = audit
	}
	return MustNewStudentCompanions(rows.CarePlan, students, people, recorder)
}

// StudentHTTPTestConsents is the consent projection and trail the student
// routes read and append to.
type StudentHTTPTestConsents = *owners.StudentConsents

// NewStudentHTTPTestConsents binds the consent projection and the Audit
// Platform trail over the test database.
func NewStudentHTTPTestConsents(db transactionruntime.Database) StudentHTTPTestConsents {
	return NewStudentConsents(db)
}

// NewStudentHTTPTestPickupReviewBlocks are the timetable blocks a pickup
// review shows, over the test database.
func NewStudentHTTPTestPickupReviewBlocks(db transactionruntime.Database) carePlanCompose.PickupReviewBlocks {
	return NewPickupReviewBlocks(db)
}

// NewStudentHTTPTestMembership is the School Membership owner over the test
// database.
func NewStudentHTTPTestMembership(db transactionruntime.Database) (schoolmembership.Capability, error) {
	return owners.NewSchoolMembership(db)
}

// StudentHTTPTestCareLifecycle tunes the Care Plan lifecycle a student route
// suite composes.
type StudentHTTPTestCareLifecycle struct {
	BookingsAuthoritative func(context.Context) (bool, error)
	Today                 func() timezone.Date
	AuditActor            peoplePorts.RequestAuditActor
}

// NewStudentHTTPTestCareLifecycle composes the native Care Plan lifecycle
// over the test database, recording care-end history through the owner's
// change history.
func NewStudentHTTPTestCareLifecycle(db transactionruntime.Database, config StudentHTTPTestCareLifecycle) (careplan.CareLifecycle, carePlanHTTP.CareResumption, error) {
	repos, err := NewCareLifecycleTestRepositories(db, nil)
	if err != nil {
		return nil, nil, err
	}
	deps := CareLifecycleTestConfig{
		Audit:                 peopleCompose.NewStudentAuditService(config.AuditActor, NewStudentAudit(db)),
		BookingsAuthoritative: config.BookingsAuthoritative,
		Today:                 config.Today,
	}
	lifecycle, err := repos.NewCareLifecycle(deps)
	return lifecycle, repos.NewCareResumption(deps), err
}

// MustNewStudentHTTPTestCareResumption builds the workflow beside the domain
// services, not as another field of a root fixture graph.
func MustNewStudentHTTPTestCareResumption(db transactionruntime.Database) carePlanHTTP.CareResumption {
	repos, err := NewCareLifecycleTestRepositories(db, nil)
	if err != nil {
		panic(err)
	}
	return repos.NewCareResumption(CareLifecycleTestConfig{
		Audit: peopleCompose.NewStudentAuditService(requestTokenReads.AuditActor, NewStudentAudit(db)),
	})
}

// MustNewStudentHTTPTestRows is the Care Plan side of NewStudentHTTPTestRows
// recording into the test audit store; a composition failure is a broken
// graph and panics.
func MustNewStudentHTTPTestRows(db transactionruntime.Database) StudentHTTPTestRows {
	rows, _, err := NewStudentHTTPTestRows(db, nil)
	if err != nil {
		panic(err)
	}
	return rows
}

// MustNewStudentHTTPTestEnrollment is the Enrollment graph of
// NewStudentHTTPTestRows recording into the test audit store; a composition
// failure is a broken graph and panics.
func MustNewStudentHTTPTestEnrollment(db transactionruntime.Database) EnrollmentTestRepositories {
	_, enrollment, err := NewStudentHTTPTestRows(db, nil)
	if err != nil {
		panic(err)
	}
	return enrollment
}

// MustNewStudentHTTPTestPeopleRows are the People Directory rows of the
// student route suites over db; a composition failure panics.
func MustNewStudentHTTPTestPeopleRows(db transactionruntime.Database) StudentPeopleRows {
	rows, err := NewStudentPeopleRows(db, owners.MustNewMembershipTestRepositories(db))
	if err != nil {
		panic(err)
	}
	return rows
}

func PendingStudentHTTPTestCareWithdrawals(ctx context.Context, rows owners.CareWithdrawalTestRows, studentID int64) ([]*careplan.WithdrawalCompletion, int, error) {
	return rows.ListPending(ctx, careplan.WithdrawalListFilter{StudentID: studentID, Page: 1, PageSize: 1})
}
