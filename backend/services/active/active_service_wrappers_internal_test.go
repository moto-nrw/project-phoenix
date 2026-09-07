package active

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	facilityModels "github.com/moto-nrw/project-phoenix/models/facilities"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type attendanceRepoForActiveWrapperTest struct {
	StudentPresence
	has     bool
	err     error
	gotDate studentpresence.AttendanceFilter
}

func TestActiveGroupVisitsPropagatesGroupLookupFailure(t *testing.T) {
	t.Parallel()
	svc := &service{ServiceDependencies: ServiceDependencies{GroupRepo: &mockGroupRepository{
		findByIDFunc: func(context.Context, interface{}) (*activeModels.Group, error) {
			return nil, errors.New("group storage unavailable")
		},
	}}}
	visits, err := svc.GetActiveGroupVisits(context.Background(), 1)
	require.ErrorIs(t, err, ErrDatabaseOperation)
	assert.NotErrorIs(t, err, ErrActiveGroupNotFound)
	assert.Nil(t, visits)
}

func (r *attendanceRepoForActiveWrapperTest) HasAttendance(_ context.Context, date studentpresence.AttendanceFilter) (bool, error) {
	r.gotDate = date
	return r.has, r.err
}

type roomRepoForActiveWrapperTest struct {
	facilityModels.RoomRepository
	rooms  []*facilityModels.Room
	err    error
	gotIDs []int64
}

func (r *roomRepoForActiveWrapperTest) FindByIDs(_ context.Context, ids []int64) ([]*facilityModels.Room, error) {
	r.gotIDs = append([]int64(nil), ids...)
	return r.rooms, r.err
}

type visitRepoForActiveWrapperTest struct {
	StudentPresence
	rows             []*VisitWithStudentDisplay
	err              error
	gotActiveGroupID int64
}

func (r *visitRepoForActiveWrapperTest) ListVisits(_ context.Context, filter studentpresence.VisitFilter) ([]studentpresence.Visit, error) {
	r.gotActiveGroupID = filter.ActiveGroupIDs[0]
	result := make([]studentpresence.Visit, 0, len(r.rows))
	for _, row := range r.rows {
		result = append(result, studentpresence.Visit{ID: row.VisitID, StudentID: row.StudentID})
	}
	return result, r.err
}

func (r *visitRepoForActiveWrapperTest) ListStudentDisplayFacts(context.Context, []int64) ([]StudentDisplayFacts, error) {
	result := make([]StudentDisplayFacts, 0, len(r.rows))
	for _, row := range r.rows {
		result = append(result, StudentDisplayFacts{ID: row.StudentID})
	}
	return result, r.err
}

type crossTenantRepoForActiveWrapperTest struct {
	students           []activeModels.CrossTenantStudent
	err                error
	gotHostingTenantID int64
}

type displayFactsForActiveTest struct {
	rows []StudentDisplayFacts
	err  error
}

func (d displayFactsForActiveTest) ListStudentDisplayFacts(context.Context, []int64) ([]StudentDisplayFacts, error) {
	return d.rows, d.err
}

func TestVisitDisplayOmitsUnknownStudentsAndPropagatesDirectoryFailure(t *testing.T) {
	t.Parallel()
	presence := &visitRepoForActiveWrapperTest{rows: []*VisitWithStudentDisplay{
		{VisitID: 90, StudentID: 91}, {VisitID: 92, StudentID: 93},
	}}
	sick := true
	photo := "student-photo"
	directory := displayFactsForActiveTest{rows: []StudentDisplayFacts{{
		ID: 93, PersonID: 94, SchoolClass: "3a", Sick: &sick, PhotoPath: &photo,
	}}}
	svc := &service{ServiceDependencies: ServiceDependencies{SchoolPresence: presence, StudentDisplay: directory}}
	rows, err := svc.GetActiveGroupVisitsWithDisplay(context.Background(), 80)
	require.NoError(t, err)
	require.Len(t, rows, 1, "a visit cannot expose a student missing from the tenant directory")
	assert.Equal(t, int64(92), rows[0].VisitID)
	assert.Equal(t, int64(93), rows[0].StudentID)
	assert.Equal(t, int64(94), rows[0].PersonID)
	assert.Equal(t, "3a", rows[0].SchoolClass)
	assert.Equal(t, &sick, rows[0].Sick)
	assert.Equal(t, &photo, rows[0].PhotoPath)

	injected := errors.New("student directory failed")
	svc.StudentDisplay = displayFactsForActiveTest{err: injected}
	rows, err = svc.GetActiveGroupVisitsWithDisplay(context.Background(), 80)
	require.ErrorIs(t, err, injected)
	assert.Nil(t, rows)
}

func (r *crossTenantRepoForActiveWrapperTest) FindCrossTenantStudents(_ context.Context, hostingTenantID int64) ([]activeModels.CrossTenantStudent, error) {
	r.gotHostingTenantID = hostingTenantID
	return r.students, r.err
}

type schoolQueryForActiveWrapperTest struct {
	schools []School
	err     error
	gotIDs  []int64
}

func (q *schoolQueryForActiveWrapperTest) ListSchoolsByID(_ context.Context, ids []int64) ([]School, error) {
	q.gotIDs = append([]int64(nil), ids...)
	return q.schools, q.err
}

type staffRepoForActiveWrapperTest struct {
	userModels.StaffRepository
	staff *userModels.Staff
	err   error
	gotID interface{}
}

func (r *staffRepoForActiveWrapperTest) FindByID(_ context.Context, id interface{}) (*userModels.Staff, error) {
	r.gotID = id
	return r.staff, r.err
}

type groupRepoForActiveWrapperTest struct {
	activeModels.GroupRepository
	group       *activeModels.Group
	groups      map[int64]*activeModels.Group
	err         error
	gotID       interface{}
	gotIDs      []int64
	lockedID    int64
	onRowLocked func()
}

func (r *groupRepoForActiveWrapperTest) FindByID(_ context.Context, id interface{}) (*activeModels.Group, error) {
	r.gotID = id
	return r.group, r.err
}

func (r *groupRepoForActiveWrapperTest) FindByIDForUpdate(_ context.Context, id int64) (*activeModels.Group, error) {
	r.lockedID = id
	if r.onRowLocked != nil {
		r.onRowLocked()
	}
	return r.group, r.err
}

func (r *groupRepoForActiveWrapperTest) FindByIDs(_ context.Context, ids []int64) (map[int64]*activeModels.Group, error) {
	r.gotIDs = append([]int64(nil), ids...)
	return r.groups, r.err
}

func TestActiveServiceThinDelegates(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("has open attendance delegates date and result", func(t *testing.T) {
		date := timezone.DateFromTime(timezone.NewDate(2026, 8, 24).BerlinMidnight())
		repo := &attendanceRepoForActiveWrapperTest{has: true}
		svc := &service{ServiceDependencies: ServiceDependencies{SchoolPresence: repo}}

		hasOpen, err := svc.HasOpenAttendanceOn(ctx, date)

		require.NoError(t, err)
		assert.True(t, hasOpen)
		assert.Equal(t, studentpresence.AttendanceFilter{FromDate: date.String(), UntilDate: date.String(), OpenOnly: true}, repo.gotDate)
	})

	t.Run("has open attendance preserves repository error", func(t *testing.T) {
		expectedErr := errors.New("attendance lookup failed")
		repo := &attendanceRepoForActiveWrapperTest{err: expectedErr}
		svc := &service{ServiceDependencies: ServiceDependencies{SchoolPresence: repo}}

		hasOpen, err := svc.HasOpenAttendanceOn(ctx, timezone.NewDate(2026, 8, 24))

		require.ErrorIs(t, err, expectedErr)
		assert.False(t, hasOpen)
	})

	t.Run("get rooms by ids delegates ids and result", func(t *testing.T) {
		rooms := []*facilityModels.Room{{Name: "Aula"}}
		repo := &roomRepoForActiveWrapperTest{rooms: rooms}
		svc := &service{ServiceDependencies: ServiceDependencies{RoomRepo: repo}}

		got, err := svc.GetRoomsByIDs(ctx, []int64{10, 20})

		require.NoError(t, err)
		assert.Equal(t, rooms, got)
		assert.Equal(t, []int64{10, 20}, repo.gotIDs)
	})

	t.Run("get active group visits with display delegates active group id", func(t *testing.T) {
		rows := []*VisitWithStudentDisplay{{VisitID: 90, StudentID: 91}}
		repo := &visitRepoForActiveWrapperTest{rows: rows}
		svc := &service{ServiceDependencies: ServiceDependencies{SchoolPresence: repo, StudentDisplay: repo}}

		got, err := svc.GetActiveGroupVisitsWithDisplay(ctx, 80)

		require.NoError(t, err)
		assert.Equal(t, rows, got)
		assert.Equal(t, int64(80), repo.gotActiveGroupID)
	})
}

func TestGetActiveGroupsByIDs_Branches(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("empty input returns empty map without repository call", func(t *testing.T) {
		repo := &groupRepoForActiveWrapperTest{}
		svc := &service{ServiceDependencies: ServiceDependencies{GroupRepo: repo}}

		groups, err := svc.GetActiveGroupsByIDs(ctx, nil)

		require.NoError(t, err)
		assert.Empty(t, groups)
		assert.Nil(t, repo.gotIDs)
	})

	t.Run("nil repository map becomes empty map", func(t *testing.T) {
		repo := &groupRepoForActiveWrapperTest{}
		svc := &service{ServiceDependencies: ServiceDependencies{GroupRepo: repo}}

		groups, err := svc.GetActiveGroupsByIDs(ctx, []int64{10, 20})

		require.NoError(t, err)
		assert.Empty(t, groups)
		assert.Equal(t, []int64{10, 20}, repo.gotIDs)
	})

	t.Run("repository error maps to database operation", func(t *testing.T) {
		repo := &groupRepoForActiveWrapperTest{err: errors.New("group lookup failed")}
		svc := &service{ServiceDependencies: ServiceDependencies{GroupRepo: repo}}

		groups, err := svc.GetActiveGroupsByIDs(ctx, []int64{10})

		require.Error(t, err)
		assert.Nil(t, groups)
		assert.Contains(t, err.Error(), ErrDatabaseOperation.Error())
	})
}

func TestGetActiveGroup_Branches(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		group := &activeModels.Group{Model: modelBase.Model{ID: 42}}
		repo := &groupRepoForActiveWrapperTest{group: group}
		svc := &service{ServiceDependencies: ServiceDependencies{GroupRepo: repo}}

		got, err := svc.GetActiveGroup(ctx, 42)

		require.NoError(t, err)
		assert.Equal(t, group, got)
		assert.Equal(t, int64(42), repo.gotID)
	})

	t.Run("database no rows maps to active group not found", func(t *testing.T) {
		repo := &groupRepoForActiveWrapperTest{
			err: &modelBase.DatabaseError{Op: "find active group", Err: modelBase.ErrNotFound},
		}
		svc := &service{ServiceDependencies: ServiceDependencies{GroupRepo: repo}}

		got, err := svc.GetActiveGroup(ctx, 42)

		require.ErrorIs(t, err, ErrActiveGroupNotFound)
		assert.Nil(t, got)
	})

	t.Run("unexpected repository error maps to database operation", func(t *testing.T) {
		repo := &groupRepoForActiveWrapperTest{err: errors.New("group repository unavailable")}
		svc := &service{ServiceDependencies: ServiceDependencies{GroupRepo: repo}}

		got, err := svc.GetActiveGroup(ctx, 42)

		require.ErrorIs(t, err, ErrDatabaseOperation)
		assert.Nil(t, got)
	})

	t.Run("nil result maps to active group not found", func(t *testing.T) {
		repo := &groupRepoForActiveWrapperTest{}
		svc := &service{ServiceDependencies: ServiceDependencies{GroupRepo: repo}}

		got, err := svc.GetActiveGroup(ctx, 42)

		require.ErrorIs(t, err, ErrActiveGroupNotFound)
		assert.Nil(t, got)
	})
}

func TestValidateStaffExists_Branches(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		repo := &staffRepoForActiveWrapperTest{staff: &userModels.Staff{}}
		svc := &service{ServiceDependencies: ServiceDependencies{StaffRepo: repo}}

		err := svc.validateStaffExists(ctx, 42)

		require.NoError(t, err)
		assert.Equal(t, int64(42), repo.gotID)
	})

	t.Run("database no rows maps to staff not found", func(t *testing.T) {
		repo := &staffRepoForActiveWrapperTest{
			err: &modelBase.DatabaseError{Op: "find staff", Err: modelBase.ErrNotFound},
		}
		svc := &service{ServiceDependencies: ServiceDependencies{StaffRepo: repo}}

		err := svc.validateStaffExists(ctx, 42)

		require.ErrorIs(t, err, ErrStaffNotFound)
	})

	t.Run("unexpected repository error is preserved", func(t *testing.T) {
		expectedErr := errors.New("staff repository unavailable")
		repo := &staffRepoForActiveWrapperTest{err: expectedErr}
		svc := &service{ServiceDependencies: ServiceDependencies{StaffRepo: repo}}

		err := svc.validateStaffExists(ctx, 42)

		require.ErrorIs(t, err, expectedErr)
	})
}

func TestClaimActiveGroupLocksLifecycleThroughSupervisorInsert(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	const (
		groupID int64 = 42
		staffID int64 = 84
	)
	group := &activeModels.Group{Model: modelBase.Model{ID: groupID}}
	groupLocked := false
	groupRepo := &groupRepoForActiveWrapperTest{
		group: group,
		onRowLocked: func() {
			groupLocked = true
		},
	}
	staffRepo := &staffRepoForActiveWrapperTest{
		staff: &userModels.Staff{Model: modelBase.Model{ID: staffID}},
	}
	supervisorRepo := &mockGroupSupervisorRepository{
		createFunc: func(_ context.Context, supervisor *activeModels.GroupSupervisor) error {
			assert.True(t, groupLocked, "group lifecycle must be locked before supervisor insert")
			assert.Equal(t, groupID, supervisor.GroupID)
			assert.Equal(t, staffID, supervisor.StaffID)
			return nil
		},
	}
	svc := &service{ServiceDependencies: ServiceDependencies{
		GroupRepo:      groupRepo,
		StaffRepo:      staffRepo,
		SupervisorRepo: supervisorRepo,
	}}

	supervisor, err := svc.ClaimActiveGroup(ctx, groupID, staffID, "supervisor")

	require.NoError(t, err)
	require.NotNil(t, supervisor)
	assert.Equal(t, groupID, groupRepo.lockedID)
}

func TestGetCrossTenantStudents_Branches(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("nil repository returns empty slice", func(t *testing.T) {
		svc := &service{}

		students, err := svc.GetCrossTenantStudents(ctx, 10)

		require.NoError(t, err)
		assert.Empty(t, students)
	})

	t.Run("repository error is wrapped", func(t *testing.T) {
		expectedErr := errors.New("cross tenant lookup failed")
		svc := &service{ServiceDependencies: ServiceDependencies{CrossTenantRepo: &crossTenantRepoForActiveWrapperTest{err: expectedErr}}}

		students, err := svc.GetCrossTenantStudents(ctx, 10)

		require.Error(t, err)
		assert.Nil(t, students)
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("returns repository rows", func(t *testing.T) {
		rows := []activeModels.CrossTenantStudent{{StudentID: 20, HomeTenantID: 30}}
		repo := &crossTenantRepoForActiveWrapperTest{students: rows}
		schools := &schoolQueryForActiveWrapperTest{schools: []School{{ID: 30, Slug: "home"}}}
		svc := &service{ServiceDependencies: ServiceDependencies{CrossTenantRepo: repo, Schools: schools}}

		students, err := svc.GetCrossTenantStudents(ctx, 10)

		require.NoError(t, err)
		require.Len(t, students, 1)
		assert.Equal(t, "home", students[0].HomeTenant)
		assert.Equal(t, int64(10), repo.gotHostingTenantID)
		assert.Equal(t, []int64{30}, schools.gotIDs)
	})
}
