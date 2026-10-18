//go:build testhooks

package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"
	owner0 "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"

	studentDirectoryView "github.com/moto-nrw/project-phoenix/modules/studentdirectoryview/compose"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"

	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// CareLifecycleTestRepositories is the repository graph the care-lifecycle
// suites run on, with the lifecycle's owners bound the way the production
// graph binds them. StudentFieldEdit is filled by the composition that owns
// Audit persistence (NewCareLifecycleTestRepositories).
type CareLifecycleTestRepositories struct {
	TimetableTestRepositories
	CareWithdrawal   CareWithdrawalRecords
	StudentFieldEdit auditModels.StudentFieldEditRepository
	db               transactionruntime.Database
	sources          CareLifecycleOwnerSources
}

func NewCareLifecycleTestRepositoriesWithMembers(db transactionruntime.Database, members MembershipTestRepositories) (CareLifecycleTestRepositories, error) {
	tt, err := NewTimetableTestRepositoriesWithMembers(db, members)
	if err != nil {
		return CareLifecycleTestRepositories{}, err
	}
	people := members.People
	membership, err := NewSchoolMembership(db)
	if err != nil {
		return CareLifecycleTestRepositories{}, err
	}
	calendar, err := NewSchoolCalendar(db)
	if err != nil {
		return CareLifecycleTestRepositories{}, err
	}
	care := tt.CarePlan
	return CareLifecycleTestRepositories{
		TimetableTestRepositories: tt,
		CareWithdrawal:            NewLateBoundCareWithdrawalCompletionRepository(func() careplan.Capability { return care }),
		db:                        db,
		sources: CareLifecycleOwnerSources{
			Students: tt.Student, Persons: tt.Person, Membership: membership,
			People: people, Timetable: tt.Timetable, Calendar: calendar,
			Presence: NewUnloggedStudentPresence(db), PlanReads: newPresenceProjectionReads(db),
			Enrollment: tt.Enrollment(),
		},
	}, nil
}

// CareLifecycleTestConfig tunes the composed lifecycle for one suite.
type CareLifecycleTestConfig struct {
	// Audit records the care-end history; companion suites share it.
	Audit                 StudentChangeAudit
	BookingsAuthoritative func(context.Context) (bool, error)
	// LockCareBookingWrites is the booking-write gate. nil takes none.
	LockCareBookingWrites func(context.Context) error
	// Today defaults to the Berlin calendar day.
	Today func() owner0.Date
}

// NewCareLifecycle composes the native Care Plan lifecycle over these
// repositories.
func (r CareLifecycleTestRepositories) NewCareLifecycle(config CareLifecycleTestConfig) (careplan.CareLifecycle, error) {
	lock := config.LockCareBookingWrites
	if lock == nil {
		lock = func(context.Context) error { return nil }
	}
	return carePlanCompose.NewCareLifecycle(carePlanCompose.CareLifecycleDependencies{
		StudentDirectory: studentDirectoryView.NewReader(),
		DB:               r.db.Pool(), Records: r.CarePlan,
		Owners:                NewCareLifecycleOwners(r.sources, NewCareEndRecorder(config.Audit)),
		LockCareBookingWrites: lock,
		BookingsAuthoritative: config.BookingsAuthoritative,
		Fingerprint:           securityruntime.Fingerprint,
		Logger:                slog.Default(),
		Today:                 config.Today,
	})
}

// MustNewStudentCompanions composes the companion capability over a test
// graph's Care Plan records, student repository and People Directory. audit
// may be nil when the suite never widens a plan.
func MustNewStudentCompanions(records careplan.Capability, students peoplePorts.StudentRepository, people CompanionStudentLock, audit StudentChangeAudit) careplan.StudentCompanions {
	companions, err := carePlanCompose.NewCompanions(records, NewCompanionStudents(students, people, audit))
	if err != nil {
		panic(err)
	}
	return companions
}

// ReplaceStudentCompanions makes edges the child's complete companion set
// through the Care Plan owner behind links, a repository from
// NewStudentCompanionRepository. It stages departure-companion fixtures for
// suites that may not name Care Plan's edge type; production writes go
// through the companion capability's rules instead.
func ReplaceStudentCompanions(ctx context.Context, links CompanionRepository, studentID int64, edges []*careplan.CompanionEdge) error {
	repository := links
	values := make([]careplan.CompanionEdge, 0, len(edges))
	for _, edge := range edges {
		if edge == nil {
			return errors.New("companion edge cannot be nil")
		}
		values = append(values, careplan.CompanionEdge{
			ID: edge.ID, TenantID: edge.TenantID, CreatedAt: edge.CreatedAt, UpdatedAt: edge.UpdatedAt,
			StudentLowID: edge.StudentLowID, StudentHighID: edge.StudentHighID, Weekday: edge.Weekday,
		})
	}
	return WrapRetainedError("replace student companions", repository.Capability.ReplaceCompanionEdges(ctx, studentID, values))
}

// NewStudentCompanionEdge builds one stored "walks home with" edge between two
// children on one weekday, normalized into the low/high order the DB CHECK
// requires, for suites that stage companion fixtures through
// ReplaceStudentCompanions. Production edges come from Care Plan's domain.
func NewStudentCompanionEdge(studentID, companionID int64, weekday int) (*careplan.CompanionEdge, error) {
	if studentID <= 0 || companionID <= 0 {
		return nil, departure.ErrCompanionStudentIDRequired
	}
	if studentID == companionID {
		return nil, departure.ErrCompanionSelfLink
	}
	if _, ok := peopleValues.CompanionWeekdayKeys[weekday]; !ok {
		return nil, fmt.Errorf("%w: got %d", departure.ErrCompanionInvalidWeekday, weekday)
	}
	low, high := studentID, companionID
	if low > high {
		low, high = high, low
	}
	return &careplan.CompanionEdge{StudentLowID: low, StudentHighID: high, Weekday: weekday}, nil
}
