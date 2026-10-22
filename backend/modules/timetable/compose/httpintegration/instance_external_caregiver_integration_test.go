package httpintegration_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
	fixtures "github.com/moto-nrw/project-phoenix/modules/timetable/fixturetest"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// External caregivers (#3823, #4573) are planned wherever staff is planned:
// regular events, single occurrences and spontaneous activities. Whether a
// new external may be chosen (account or open invitation) is the pickers'
// rule (#4573 D4); the server accepts any staff member of the school, so
// existing assignments of accountless externals stay editable.

// createAccountBackedExternal creates a staff member of the tenant with a
// login account and an active guest profile: an external caregiver with an
// account (#4573).
func createAccountBackedExternal(t *testing.T, db *bun.DB, ctx context.Context, tenantID int64) int64 {
	t.Helper()
	staff, _ := testpkg.CreateTestStaffWithAccountForTenant(t, db, tenantID, "Extern", fmt.Sprintf("MitKonto-%d", time.Now().UnixNano()))
	guest := &testpkg.Guest{TenantID: tenantID, StaffID: staff.ID, ActivityExpertise: "Musik"}
	_, err := db.NewInsert().Model(guest).ModelTableExpr(`users.guests`).Exec(ctx)
	require.NoError(t, err)
	return staff.ID
}

// An accountless external is accepted on a planned single occurrence. Before
// #4573 the server refused every external outside spontaneous activities.
func TestInstance_CreatePlanned_AcceptsExternalCaregiver(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	external := testpkg.CreateTestGuest(t, s.db, "Musik")

	created, err := s.svc.CreateInstance(s.ctx, timetable.CreateInstanceInput{
		Date:            calendar.TodayDate().AddDays(7),
		StartTime:       time.Date(1, 1, 1, 9, 0, 0, 0, time.UTC),
		EndTime:         time.Date(1, 1, 1, 10, 0, 0, 0, time.UTC),
		Title:           "Geplante Musik-AG",
		RoomID:          s.roomID,
		ActivityGroupID: &s.tmplID,
		StaffIDs:        []int64{external.StaffID},
	})

	require.NoError(t, err)
	require.NotNil(t, created)
	assert.False(t, created.IsSpontaneous)
	assert.Equal(t, 1, countRowsWhere(t, s, "schedule.instance_staff", "instance_id", created.ID))
}

func TestInstance_CreateSpontaneous_AllowsExternalCaregiver(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	external := testpkg.CreateTestGuest(t, s.db, "Musik")
	spontaneous := true

	created, err := s.svc.CreateInstance(s.ctx, timetable.CreateInstanceInput{
		Date:          calendar.TodayDate(),
		StartTime:     time.Date(1, 1, 1, 9, 0, 0, 0, time.UTC),
		EndTime:       time.Date(1, 1, 1, 10, 0, 0, 0, time.UTC),
		Title:         "Spontane Musik-AG",
		RoomID:        s.roomID,
		IsSpontaneous: &spontaneous,
		StaffIDs:      []int64{external.StaffID},
	})

	require.NoError(t, err)
	require.NotNil(t, created)
	assert.True(t, created.IsSpontaneous)
	assert.Equal(t, 1, countRowsWhere(t, s, "schedule.instance_staff", "instance_id", created.ID))
}

func TestInstance_UpdateSpontaneous_PreservesExternalCaregiver(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	instance := seedSpontaneousInstance(t, s, false)
	external := testpkg.CreateTestGuest(t, s.db, "Musik")
	staff := fixtures.NewInstanceStaff(func(fixtureRow *fixtures.InstanceStaff) {
		fixtureRow.InstanceID = instance.ID
		fixtureRow.StaffID = external.StaffID
	})
	staff.SetTenantID(testpkg.Tenant(t))
	_, err := s.db.NewInsert().Model(staff).ModelTableExpr(`schedule.instance_staff`).Exec(s.ctx)
	require.NoError(t, err)

	updated, err := s.svc.UpdatePlanned(s.ctx, instance.ID, timetable.UpdateInstanceInput{
		Date:      calendar.Date(instance.Date),
		StartTime: instance.StartTime,
		EndTime:   instance.EndTime,
		Title:     "Spontane Musik-AG geändert",
		RoomID:    s.roomID,
		StaffIDs:  []int64{external.StaffID},
	}, nil)

	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.True(t, updated.IsSpontaneous)
	assert.Equal(t, 1, countRowsWhere(t, s, "schedule.instance_staff", "instance_id", instance.ID))
}

// Continuing a spontaneous activity as a planned series keeps its external.
// Before #4573 the conversion refused the external.
func TestInstance_UpdateSpontaneousToPlanned_KeepsExternalCaregiver(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	instance := seedSpontaneousInstance(t, s, false)
	external := testpkg.CreateTestGuest(t, s.db, "Musik")
	staff := fixtures.NewInstanceStaff(func(fixtureRow *fixtures.InstanceStaff) {
		fixtureRow.InstanceID = instance.ID
		fixtureRow.StaffID = external.StaffID
	})
	staff.SetTenantID(testpkg.Tenant(t))
	_, err := s.db.NewInsert().Model(staff).ModelTableExpr(`schedule.instance_staff`).Exec(s.ctx)
	require.NoError(t, err)

	updated, err := s.svc.UpdatePlanned(s.ctx, instance.ID, timetable.UpdateInstanceInput{
		Date:             calendar.Date(instance.Date),
		StartTime:        instance.StartTime,
		EndTime:          instance.EndTime,
		Title:            "Als geplant fortsetzen",
		RoomID:           s.roomID,
		ActivityGroupID:  &s.tmplID,
		CalendarPeriodID: &s.period.ID,
		StaffIDs:         []int64{external.StaffID},
	}, nil)

	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, 1, countRowsWhere(t, s, "schedule.instance_staff", "instance_id", instance.ID))
}

// An external with an account may be the only staff member of a planned
// single occurrence.
func TestInstance_CreatePlanned_AllowsAccountBackedExternalAsOnlyStaff(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	externalID := createAccountBackedExternal(t, s.db, s.ctx, testpkg.Tenant(t))

	created, err := s.svc.CreateInstance(s.ctx, timetable.CreateInstanceInput{
		Date:            calendar.TodayDate().AddDays(7),
		StartTime:       time.Date(1, 1, 1, 9, 0, 0, 0, time.UTC),
		EndTime:         time.Date(1, 1, 1, 10, 0, 0, 0, time.UTC),
		Title:           "Geplante Musik-AG",
		RoomID:          s.roomID,
		ActivityGroupID: &s.tmplID,
		StaffIDs:        []int64{externalID},
	})

	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, 1, countRowsWhere(t, s, "schedule.instance_staff", "instance_id", created.ID))
	assert.Equal(t, 1, countRowsWhere(t, s, "schedule.instance_staff", "staff_id", externalID))
}

// A regular event (Regeltermin) may have an external with an account as its
// only and responsible staff member; every occurrence carries them as lead.
func TestTemplate_AccountBackedExternalAsOnlyAndPrimaryStaffReachesOccurrences(t *testing.T) {
	t.Parallel()

	monday := futureMonday(1)
	s := makeScenario(t, activityContract.WeekdayMonday, monday)
	defer s.runCleanup(t)
	externalID := createAccountBackedExternal(t, s.db, s.ctx, s.tenantID)

	result, err := s.timetableData.Templates.CreateTemplate(s.ctx, timetable.CreateTemplateCommand{
		Name:             fmt.Sprintf("AG Musik-%d", time.Now().UnixNano()),
		Type:             activityContract.GroupTypeActivity,
		Weekdays:         []int{activityContract.WeekdayMonday},
		StartTime:        time.Date(2000, 1, 1, 14, 0, 0, 0, time.UTC),
		EndTime:          time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC),
		RoomID:           s.roomID,
		CategoryID:       s.categoryID,
		MaxParticipants:  20,
		CalendarPeriodID: &s.period.ID,
		TargetGroupType:  activityContract.TargetGroupTypeNone,
		StaffIDs:         []int64{externalID},
		PrimaryStaffID:   &externalID,
		RosterValidFrom:  monday.AddDays(-30),
		GradeLevelMax:    testpkg.MaxSchoolGradeLevel,
	})
	require.NoError(t, err)
	registerSourcedTemplateCleanup(t, s, result.TemplateID, result.TimeframeID)

	_, err = s.materialization.MaterializeForTenant(s.ctx, monday, monday.AddDays(7), timetable.MaterializationSourceManual)
	require.NoError(t, err)

	for _, date := range []calendar.Date{monday, monday.AddDays(7)} {
		assert.Equal(t, []int64{externalID}, instanceStaffIDsOn(t, s, result.TemplateID, date), "occurrence %s", date)
		assert.Equal(t, externalID, instanceStaffPrimaryOn(t, s, result.TemplateID, date), "occurrence %s", date)
	}
}
