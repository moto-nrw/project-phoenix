package repositories

import (
	"context"
	"errors"
	"log/slog"

	auditRepo "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/uptrace/bun"
)

// CareLifecycleTestRepositories is the repository graph the care-lifecycle
// suites run on, with the lifecycle's owners bound the way the production
// graph binds them.
type CareLifecycleTestRepositories struct {
	TimetableTestRepositories
	CareWithdrawal   usersModels.CareWithdrawalCompletionRepository
	StudentFieldEdit auditModels.StudentFieldEditRepository
	db               *bun.DB
	sources          CareLifecycleOwnerSources
}

func NewCareLifecycleTestRepositories(db *bun.DB, command auditModels.Command) (CareLifecycleTestRepositories, error) {
	tt, err := NewTimetableTestRepositories(db)
	if err != nil {
		return CareLifecycleTestRepositories{}, err
	}
	people, err := NewPeopleDirectory(db)
	if err != nil {
		return CareLifecycleTestRepositories{}, err
	}
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
		CareWithdrawal: newCareWithdrawalCompletionRepository(
			func() careplan.Capability { return care }, func() peopledirectory.StudentQuery { return people }),
		StudentFieldEdit: studentFieldEditCommand{auditRepo.NewStudentFieldEditRepository(newTestAuditRuntime(db)), command},
		db:               db,
		sources: CareLifecycleOwnerSources{
			Students: tt.Student, Persons: tt.Person, Membership: membership,
			People: people, Timetable: tt.Timetable, Calendar: calendar,
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
	Today func() usersModels.CalendarDate
}

// NewCareLifecycle composes the native Care Plan lifecycle over these
// repositories.
func (r CareLifecycleTestRepositories) NewCareLifecycle(config CareLifecycleTestConfig) (careplan.CareLifecycle, error) {
	lock := config.LockCareBookingWrites
	if lock == nil {
		lock = func(context.Context) error { return nil }
	}
	return carePlanCompose.NewCareLifecycle(carePlanCompose.CareLifecycleDependencies{
		DB: r.db, Records: r.CarePlan,
		Owners:                NewCareLifecycleOwners(r.db, r.sources, NewCareEndRecorder(config.Audit)),
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
func MustNewStudentCompanions(records careplan.Capability, students usersModels.StudentRepository, people CompanionStudentLock, audit StudentChangeAudit) careplan.StudentCompanions {
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
func ReplaceStudentCompanions(ctx context.Context, links usersModels.StudentCompanionRepository, studentID int64, edges []*usersModels.StudentCompanion) error {
	repository, ok := links.(companionRepository)
	if !ok {
		return errors.New("companion links are not served by the Care Plan owner")
	}
	values := make([]careplan.CompanionEdge, 0, len(edges))
	for _, edge := range edges {
		if edge == nil {
			return errors.New("companion edge cannot be nil")
		}
		values = append(values, companionToPublic(edge))
	}
	return usersRepo.WrapError("replace student companions", repository.capability.ReplaceCompanionEdges(ctx, studentID, values))
}
