package schedule_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type plannedNowBudgetPersonService struct {
	person *usersModel.Person
	staff  *usersModel.Staff
}

func (s plannedNowBudgetPersonService) FindByAccountID(context.Context, int64) (*usersModel.Person, error) {
	return s.person, nil
}

func (s plannedNowBudgetPersonService) GetByIDs(context.Context, []int64) (map[int64]*usersModel.Person, error) {
	return nil, nil
}

func (s plannedNowBudgetPersonService) GetStaffByPersonID(context.Context, int64) (*usersModel.Staff, error) {
	return s.staff, nil
}

func (s plannedNowBudgetPersonService) GetStaffWithPersonByIDs(context.Context, []int64) (map[int64]*usersModel.Staff, error) {
	return nil, nil
}

type plannedNowBudgetCareDayService struct{}

func (plannedNowBudgetCareDayService) ResolveForDate(context.Context, []int64, timezone.Date) (map[int64]scheduleSvc.CareDayStatus, error) {
	return map[int64]scheduleSvc.CareDayStatus{}, nil
}

func (plannedNowBudgetCareDayService) ResolveForRange(context.Context, []int64, timezone.Date, timezone.Date) (map[int64]map[timezone.Date]scheduleSvc.CareDayStatus, error) {
	return map[int64]map[timezone.Date]scheduleSvc.CareDayStatus{}, nil
}

type plannedNowBudgetSettings struct{}

func (plannedNowBudgetSettings) ResolveBool(context.Context, string) (bool, error) {
	return false, nil
}

func (plannedNowBudgetSettings) ResolveString(context.Context, string) (string, error) {
	return "", nil
}

func (plannedNowBudgetSettings) ResolveInt(context.Context, string) (int, error) {
	return 15, nil
}

type unusedPlannedNowInstanceService struct{ scheduleSvc.InstanceService }
type unusedPlannedNowActiveService struct {
	scheduleSvc.OperationActiveService
}
type unusedPlannedNowArrivalService struct {
	scheduleSvc.OperationArrivalService
}
type unusedPlannedNowPickupService struct {
	scheduleSvc.OperationPickupService
}

func TestTimetableOperationsPlannedNowQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	date := timezone.NewDate(2026, time.October, 19)
	now := date.BerlinMidnight().Add(13*time.Hour + 50*time.Minute)
	staff := testpkg.CreateTestStaff(t, db, "PlannedBudget", "Teacher")
	room := testpkg.CreateTestRoom(t, db, "PlannedBudgetRoom")

	repos := repositories.NewFactory(db)
	service := scheduleSvc.NewTimetableOperationsService(scheduleSvc.TimetableOperationsDependencies{
		InstanceRepo:       repos.ActivityInstance,
		InstanceStaffRepo:  repos.InstanceStaff,
		InstanceStudents:   repos.InstanceStudent,
		InstanceService:    unusedPlannedNowInstanceService{},
		ActiveGroupRepo:    repos.ActiveGroup,
		ActivityGroupRepo:  repos.ActivityGroup,
		ActiveService:      unusedPlannedNowActiveService{},
		ArrivalService:     unusedPlannedNowArrivalService{},
		PickupService:      unusedPlannedNowPickupService{},
		SupervisorRepo:     repos.GroupSupervisor,
		VisitRepo:          repos.ActiveVisit,
		StudentRepo:        repos.Student,
		EducationGroupRepo: repos.Group,
		RoomRepo:           repos.Room,
		PersonService: plannedNowBudgetPersonService{
			person: staff.Person,
			staff:  staff,
		},
		CareDayService: plannedNowBudgetCareDayService{},
		Settings:       plannedNowBudgetSettings{},
		DB:             db,
		Now:            func() time.Time { return now },
	})

	created := 0
	addInstances := func(n int) {
		for range n {
			instance := testpkg.CreateTestActivityInstance(t, db, date, room.ID, testpkg.ActivityInstanceOpts{
				StartHHMM: "14:00",
				EndHHMM:   "15:00",
				Title:     fmt.Sprintf("Planned budget %d", created),
			})
			student := testpkg.CreateTestStudent(t, db, "PlannedBudget", fmt.Sprintf("Student%d", created), "PB1")
			testpkg.CreateTestInstanceStaff(t, db, instance.ID, staff.ID, testpkg.InstanceStaffOpts{IsPrimary: true})
			testpkg.CreateTestInstanceStudent(t, db, instance.ID, student.ID, scheduleModel.AttendanceStatusExpected)
			created++
		}
	}

	counter := testpkg.CaptureQueries(t, db)
	run := func() int {
		counter.Reset()
		result, err := service.PlannedNow(ctx, 1, false, date, now, scheduleSvc.PlannedNowOptions{})
		require.NoError(t, err)
		require.Len(t, result, created)
		return counter.Total()
	}

	addInstances(3)
	smallCount := run()
	addInstances(5)
	largeCount := run()
	t.Logf("query budget: 3 planned instances → %d queries, 8 planned instances → %d queries", smallCount, largeCount)
	assert.Equal(t, smallCount, largeCount, "query count must not grow with the planned instance count")
	testpkg.AssertQueryBudget(t, "services.schedule.planned_now", counter.Queries())
}
