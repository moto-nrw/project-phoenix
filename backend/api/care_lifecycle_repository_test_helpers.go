//go:build testhooks

package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	studentDirectoryView "github.com/moto-nrw/project-phoenix/modules/studentdirectoryview/compose"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	usersModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// CareLifecycleTestRepositories are the Care Plan rows the care-lifecycle
// suites run on, with the lifecycle's owners bound the way the production
// graph binds them.
type CareLifecycleTestRepositories struct {
	carePlan careplan.Capability
	// lifecycle are the lifecycle's dependencies over the Care Plan rows and
	// its owners, bound without the change trail, which each suite supplies
	// through its audit (NewCareLifecycle).
	lifecycle carePlanCompose.CareLifecycleDependencies
}

// CareWithdrawal is the People Directory's withdrawal completion port over
// the Care Plan rows.
func (r CareLifecycleTestRepositories) CareWithdrawal() usersModels.CareWithdrawalCompletionRepository {
	return NewCareWithdrawalCompletionRepository(r.carePlan)
}

func NewCareLifecycleTestRepositoriesWithMembers(db transactionruntime.Database, members MembershipTestRepositories) (CareLifecycleTestRepositories, error) {
	tt, err := NewTimetableTestRepositoriesWithMembers(db, members)
	if err != nil {
		return CareLifecycleTestRepositories{}, err
	}
	carePlan, err := NewTimetableCarePlanRows(db, tt, members)
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
	return CareLifecycleTestRepositories{
		carePlan: carePlan.CarePlan,
		lifecycle: carePlanCompose.CareLifecycleDependencies{
			StudentDirectory: studentDirectoryView.NewReader(),
			DB:               db.Pool(), Records: carePlan.CarePlan,
			Owners: NewCareLifecycleOwners(
				members.Student, members.Person, membership, people, tt.Timetable, calendar,
				NewUnloggedStudentPresence(db), newPresenceProjectionReads(db), tt.Enrollment(), nil,
			),
			Fingerprint: securityruntime.Fingerprint,
			Logger:      slog.Default(),
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
	deps := r.lifecycle
	deps.Owners.ChangeTrail = NewCareEndRecorder(config.Audit)
	deps.LockCareBookingWrites = lock
	deps.BookingsAuthoritative = config.BookingsAuthoritative
	deps.Today = config.Today
	return carePlanCompose.NewCareLifecycle(deps)
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
func ReplaceStudentCompanions(ctx context.Context, links peoplePorts.StudentCompanionRepository, studentID int64, edges []*usersModels.StudentCompanion) error {
	repository, ok := links.(CompanionRepository)
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
	return WrapRetainedError("replace student companions", repository.Capability.ReplaceCompanionEdges(ctx, studentID, values))
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
