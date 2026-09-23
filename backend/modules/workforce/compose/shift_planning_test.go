package compose

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	facilitiesModels "github.com/moto-nrw/project-phoenix/models/facilities"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The Dienstplan composition over the Workforce capability (#3418). The
// timetable side of the week grid is empty here on purpose: what these tests
// pin is that the shift rows round-trip through the capability with the
// public string dates and wall clocks, that the service error kinds reach the
// caller as Workforce sentinels, and that a composition without a plan export
// still serves every other route.

type planningStaffDirectory struct{ staff *usersModels.Staff }

func (d planningStaffDirectory) FindByID(_ context.Context, id any) (*usersModels.Staff, error) {
	if value, ok := id.(int64); ok && value == d.staff.ID {
		return d.staff, nil
	}
	return nil, sql.ErrNoRows
}

func (d planningStaffDirectory) ListAllWithPerson(context.Context) ([]*usersModels.Staff, error) {
	return []*usersModels.Staff{d.staff}, nil
}

func (d planningStaffDirectory) FindWithPersonByIDs(_ context.Context, _ []int64) (map[int64]*usersModels.Staff, error) {
	return map[int64]*usersModels.Staff{d.staff.ID: d.staff}, nil
}

// planningTimetable stands in for the Timetable owner: the Dienstplan reads
// it, never writes it, so an empty answer is a complete one here.
type planningTimetable struct{}

func (planningTimetable) FindByTenantAndDateRange(context.Context, scheduleModels.Date, scheduleModels.Date) ([]*scheduleModels.ActivityInstance, error) {
	return nil, nil
}

func (planningTimetable) FindByIDs(context.Context, []int64) ([]*scheduleModels.ActivityInstance, error) {
	return nil, nil
}

type planningInstanceStaff struct{}

func (planningInstanceStaff) FindByInstanceIDs(context.Context, []int64) ([]*scheduleModels.InstanceStaff, error) {
	return nil, nil
}

func (planningInstanceStaff) FindByStaffAndDateRange(context.Context, int64, scheduleModels.Date, scheduleModels.Date) ([]*scheduleModels.InstanceStaff, error) {
	return nil, nil
}

type planningRooms struct{}

func (planningRooms) FindByIDs(context.Context, []int64) ([]*facilitiesModels.Room, error) {
	return nil, nil
}

type planningGroups struct{}

func (planningGroups) FindByIDs(context.Context, []int64) ([]*activitiesModels.Group, error) {
	return nil, nil
}

type planningPeriods struct{}

func (planningPeriods) FindByID(context.Context, any) (*scheduleModels.CalendarPeriod, error) {
	return nil, sql.ErrNoRows
}

func planningDependencies(db *bun.DB, capability workforce.Capability, staff *usersModels.Staff) ShiftPlanningDependencies {
	return ShiftPlanningDependencies{
		Workforce: capability, Staff: planningStaffDirectory{staff: staff},
		CalendarPeriods: planningPeriods{}, Instances: planningTimetable{},
		InstanceStaff: planningInstanceStaff{}, Rooms: planningRooms{}, ActivityGroups: planningGroups{},
		DB: db,
	}
}

func TestNewShiftPlanningRequiresTheOwnersItComposesOver(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	staff := testpkg.CreateTestStaff(t, db, "Planning", "Compose")
	capability := buildWorkforce(t, db)
	complete := planningDependencies(db, capability, staff)

	for name, strip := range map[string]func(*ShiftPlanningDependencies){
		"workforce":        func(d *ShiftPlanningDependencies) { d.Workforce = nil },
		"staff":            func(d *ShiftPlanningDependencies) { d.Staff = nil },
		"calendar periods": func(d *ShiftPlanningDependencies) { d.CalendarPeriods = nil },
		"instances":        func(d *ShiftPlanningDependencies) { d.Instances = nil },
		"instance staff":   func(d *ShiftPlanningDependencies) { d.InstanceStaff = nil },
		"rooms":            func(d *ShiftPlanningDependencies) { d.Rooms = nil },
		"activity groups":  func(d *ShiftPlanningDependencies) { d.ActivityGroups = nil },
		"database":         func(d *ShiftPlanningDependencies) { d.DB = nil },
	} {
		t.Run(name, func(t *testing.T) {
			deps := complete
			strip(&deps)
			planning, err := NewShiftPlanning(deps)
			require.Error(t, err)
			assert.Nil(t, planning)
		})
	}

	planning, err := NewShiftPlanning(complete)
	require.NoError(t, err)
	require.NotNil(t, planning)
	assert.NotNil(t, planning.ShiftTypes)
	assert.NotNil(t, planning.Assignments)
	assert.NotNil(t, planning.Overview)
}

func TestShiftPlanningServesTheDienstplanOverTheCapability(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	staff := testpkg.CreateTestStaff(t, db, "Planning", "Facade")
	capability := buildWorkforce(t, db)
	composition, err := NewShiftPlanning(planningDependencies(db, capability, staff))
	require.NoError(t, err)
	// A composition without the printable plan still serves every route.
	facade := composition.Planning(nil)

	day := timezone.NewDate(2026, 9, 21)
	input := workforce.StaffShiftInput{
		StaffID: staff.ID, ActorStaffID: staff.ID, Date: day.String(),
		StartTime: "08:30:00", EndTime: "14:45:00", BreakMinutes: 30,
	}
	var created workforce.PlannedShift
	require.NoError(t, testpkg.WithTenantTx(t, context.Background(), db, testpkg.Tenant(t), func(ctx context.Context, _ bun.Tx) error {
		created, err = facade.CreateShift(ctx, workforce.CreateStaffShift{StaffShiftInput: input})
		return err
	}))
	assert.Equal(t, day.String(), created.Date, "the wire keeps the calendar day in DateLayout")
	assert.Equal(t, "08:30:00", created.StartTime, "wall clocks keep ClockLayout precision")
	assert.Equal(t, "14:45:00", created.EndTime)
	assert.Equal(t, 30, created.BreakMinutes)
	assert.Equal(t, testpkg.Tenant(t), created.TenantID)

	shifts, err := facade.ListShifts(testpkg.Ctx(t), workforce.ShiftRange{From: day.String(), To: day.String(), StaffID: staff.ID})
	require.NoError(t, err)
	require.Len(t, shifts, 1)
	assert.Equal(t, created.ID, shifts[0].ID)

	// The service sentinels reach the caller as the public kinds the routes
	// classify on, with the service's own wording preserved.
	overlapErr := testpkg.WithTenantTx(t, context.Background(), db, testpkg.Tenant(t), func(ctx context.Context, _ bun.Tx) error {
		_, err := facade.CreateShift(ctx, workforce.CreateStaffShift{StaffShiftInput: input})
		return err
	})
	require.ErrorIs(t, overlapErr, workforce.ErrStaffShiftOverlap)
	assert.Equal(t, "shift overlaps an existing shift on this day", overlapErr.Error())

	_, rangeErr := facade.ListShifts(testpkg.Ctx(t), workforce.ShiftRange{From: "nonsense", To: day.String()})
	require.ErrorIs(t, rangeErr, workforce.ErrInvalidStaffShift)
	_, tooLargeErr := facade.ListShifts(testpkg.Ctx(t), workforce.ShiftRange{From: day.String(), To: day.AddDays(120).String(), StaffID: staff.ID})
	require.ErrorIs(t, tooLargeErr, workforce.ErrStaffShiftRangeTooLarge)

	_, assignmentsErr := composition.Assignments.ListStaffAssignments(testpkg.Ctx(t), staff.ID, day.String(), day.AddDays(120).String())
	require.ErrorIs(t, assignmentsErr, workforce.ErrStaffShiftRangeTooLarge)
	assignments, err := composition.Assignments.ListStaffAssignments(testpkg.Ctx(t), staff.ID, day.String(), day.String())
	require.NoError(t, err)
	assert.Empty(t, assignments)

	overview, err := composition.Overview.Overview(testpkg.Ctx(t), day.String(), day.String())
	require.NoError(t, err)
	assert.True(t, overview.DienstplanInUse, "the created shift marks the week as planned")
	require.Len(t, overview.Shifts, 1)
	assert.Equal(t, "08:30:00", overview.Shifts[0].StartTime)

	_, exportErr := facade.ExportPlan(testpkg.Ctx(t), workforce.PlanExportRequest{
		From: day.String(), To: day.String(), Template: "persons", Variant: "aushang", Format: "pdf",
	})
	require.EqualError(t, exportErr, "plan export is not configured")
}

func TestShiftRowsKeepTheRepositoryErrorShapes(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	rows := NewShiftRows(buildWorkforce(t, db))
	ctx := testpkg.Ctx(t)

	_, err := rows.FindByID(ctx, int64(987654321))
	require.Error(t, err)
	var databaseErr *modelBase.DatabaseError
	require.ErrorAs(t, err, &databaseErr, "a missing row stays a repository database error")
	assert.ErrorIs(t, err, modelBase.ErrNotFound)
	assert.ErrorIs(t, err, sql.ErrNoRows, "callers classify the driver no-rows value")

	_, err = rows.FindByID(ctx, "not-an-id")
	require.ErrorAs(t, err, &databaseErr)
	assert.ErrorContains(t, err, "unsupported id type")

	require.ErrorContains(t, rows.Create(ctx, nil), "StaffShift cannot be nil or zero value")

	// A validation failure keeps its bare wording instead of being wrapped as
	// a database error, as the row models' own Validate() reported it.
	invalid := &scheduleModels.StaffShift{StaffID: 0}
	err = rows.Create(ctx, invalid)
	require.Error(t, err)
	assert.False(t, errors.As(err, &databaseErr), "validation is the caller's fault, not the database's")
}
