package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	usersModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/services"
)

// CareLifecycleTestRepositories is the repository graph the care-lifecycle
// suites run on, with the lifecycle's owners bound the way the production
// graph binds them. StudentFieldEdit is filled by the composition that owns
// Audit persistence (NewCareLifecycleTestRepositories).
type CareLifecycleTestRepositories struct {
	TimetableTestRepositories
	CareWithdrawal   usersModels.CareWithdrawalCompletionRepository
	StudentFieldEdit auditModels.StudentFieldEditRepository
	db               transactionruntime.Database
	sources          services.CareLifecycleOwnerSources
}

func NewCareLifecycleTestRepositoriesWithMembers(db transactionruntime.Database, members services.MembershipTestRepositories) (CareLifecycleTestRepositories, error) {
	tt, err := NewTimetableTestRepositoriesWithMembers(db, members)
	if err != nil {
		return CareLifecycleTestRepositories{}, err
	}
	people := members.People
	membership, err := services.NewSchoolMembership(db.Pool())
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
		CareWithdrawal:            services.NewLateBoundCareWithdrawalCompletionRepository(func() careplan.Capability { return care }),
		db:                        db,
		sources: services.CareLifecycleOwnerSources{
			Students: tt.Student, Persons: tt.Person, Membership: membership,
			People: people, Timetable: tt.Timetable, Calendar: calendar,
			Presence: services.NewUnloggedStudentPresence(db.Pool()), PlanReads: services.MustPresenceReads(db.Pool()),
		},
	}, nil
}

// CareLifecycleTestConfig tunes the composed lifecycle for one suite.
type CareLifecycleTestConfig struct {
	// Audit records the care-end history; companion suites share it.
	Audit                 services.StudentChangeAudit
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
		DB: r.db.Pool(), Records: r.CarePlan,
		Owners:                services.NewCareLifecycleOwners(r.sources, services.NewCareEndRecorder(config.Audit)),
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
func MustNewStudentCompanions(records careplan.Capability, students usersModels.StudentRepository, people services.CompanionStudentLock, audit services.StudentChangeAudit) careplan.StudentCompanions {
	companions, err := carePlanCompose.NewCompanions(records, services.NewCompanionStudents(students, people, audit))
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
	repository, ok := links.(services.CompanionRepository)
	if !ok {
		return errors.New("companion links are not served by the Care Plan owner")
	}
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
	return services.WrapRetainedError("replace student companions", repository.Capability.ReplaceCompanionEdges(ctx, studentID, values))
}

// NewStudentCompanionEdge builds one stored "walks home with" edge between two
// children on one weekday, normalized into the low/high order the DB CHECK
// requires, for suites that stage companion fixtures through
// ReplaceStudentCompanions. Production edges come from Care Plan's domain.
func NewStudentCompanionEdge(studentID, companionID int64, weekday int) (*usersModels.StudentCompanion, error) {
	if studentID <= 0 || companionID <= 0 {
		return nil, usersModels.ErrCompanionStudentIDRequired
	}
	if studentID == companionID {
		return nil, usersModels.ErrCompanionSelfLink
	}
	if _, ok := usersModels.CompanionWeekdayKeys[weekday]; !ok {
		return nil, fmt.Errorf("%w: got %d", usersModels.ErrCompanionInvalidWeekday, weekday)
	}
	low, high := studentID, companionID
	if low > high {
		low, high = high, low
	}
	return &usersModels.StudentCompanion{StudentLowID: low, StudentHighID: high, Weekday: weekday}, nil
}
