package timetracking_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/application/timetracking"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// wtmIntSettings satisfies the service's settings surface with a fixed
// account start date.
type wtmIntSettings struct{ accountStart string }

func (s wtmIntSettings) AccountStartDate(context.Context) (string, error) {
	return s.accountStart, nil
}

// TestWorkTimeMonthSummary_DB exercises the Monatskarte read model against
// the real database: contract target from staff_work_schedules, session
// minutes, sick credit, and the LIVE Übertrag (a late correction in a past
// month immediately flows into the next month's carry).
func TestWorkTimeMonthSummary_DB(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	staff := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Monat", "Karte")
	workTime := api.MustNewWorkforce(testpkg.NewDatabase(t, db))
	repos := api.NewStaffAbsenceSuiteRepositories(testpkg.NewDatabase(t, db))
	ctx := testpkg.TenantContext(tenantID)

	t.Cleanup(func() {
		for _, table := range []string{
			"active.staff_absences",
			"active.work_sessions",
			"config.staff_work_schedules",
		} {
			_, _ = db.NewDelete().ModelTableExpr(table+" AS t").Where("t.tenant_id = ?", tenantID).Exec(context.Background())
		}
	})

	// Contract: Mondays 480 minutes since 2020.
	testpkg.CreateTestStaffWorkScheduleForTenant(t, db, tenantID, staff.ID, timetracking.DayMonday, 480, scheduleValidFrom)

	// One 480-minute session on Monday June 1, 2026.
	checkIn := time.Date(2026, 6, 1, 8, 0, 0, 0, time.UTC)
	checkOut := checkIn.Add(8 * time.Hour)
	session := &timetracking.WorkSession{
		StaffID:     staff.ID,
		Date:        calendar.NewDate(2026, time.June, 1),
		Status:      workforce.WorkSessionStatusPresent,
		Source:      workforce.WorkSessionSourceApp,
		CheckInTime: checkIn, CheckOutTime: &checkOut,
		CreatedBy: staff.ID,
	}
	session.SetTenantID(tenantID)
	require.NoError(t, workforceCompose.NewTimeTrackingTestRows(workTime).Sessions.Create(ctx, session))

	// Reported sick day on Monday June 15, 2026 → credits 480.
	absence := &timetracking.StaffAbsence{
		StaffID:     staff.ID,
		AbsenceType: workforce.AbsenceTypeSick,
		Status:      workforce.AbsenceStatusReported,
		DateStart:   calendar.NewDate(2026, time.June, 15),
		DateEnd:     calendar.NewDate(2026, time.June, 15),
		CreatedBy:   staff.ID,
	}
	absence.SetTenantID(tenantID)
	require.NoError(t, repos.StaffAbsence.Create(ctx, absence))

	svc := timetracking.NewWorkTimeMonthService(
		workforceCompose.NewTimeTrackingTestRows(workTime).Sessions, workforceCompose.NewTimeTrackingTestRows(workTime).Breaks, repos.StaffAbsence, workforceCompose.StaffScheduleAssignments(api.MustNewStaffEmployment(testpkg.NewDatabase(t, db))),
		api.NewWorkScheduleTargets(api.NewStaffWorkScheduleRepository(workTime)), api.NewWorkTimeTargetModels(api.NewWorkTimeModelRepository(workTime)), workforceCompose.NewTimeTrackingShifts(workTime),
		wtmIntSettings{accountStart: "2026-06-01"}, nil,
	)

	// June: 5 Mondays × 480 = 2400 target, 480 actual, 480 sick credit.
	june, err := svc.GetMonthSummary(ctx, staff.ID, 2026, 6)
	require.NoError(t, err)
	assert.Equal(t, 2400, june.TargetMinutes)
	assert.Equal(t, 480, june.ActualMinutes)
	assert.Equal(t, 480, june.CreditedSickMinutes)
	assert.InDelta(t, 1.0, june.SickDays, 0.001)
	assert.Equal(t, 480+480-2400, june.BalanceMinutes)
	assert.Equal(t, 0, june.CarryInMinutes)
	assert.Nil(t, june.PlannedShiftMinutes, "no shifts → kein Dienstplan gepflegt")

	// July's carry = June's closing balance.
	july, err := svc.GetMonthSummary(ctx, staff.ID, 2026, 7)
	require.NoError(t, err)
	assert.Equal(t, june.ClosingBalanceMinutes, july.CarryInMinutes)

	// LIVE Übertrag: a late June correction immediately changes July's carry.
	lateCheckIn := time.Date(2026, 6, 8, 8, 0, 0, 0, time.UTC)
	lateCheckOut := lateCheckIn.Add(4 * time.Hour)
	lateSession := &timetracking.WorkSession{
		StaffID: staff.ID, Date: calendar.NewDate(2026, time.June, 8),
		Status: workforce.WorkSessionStatusPresent, Source: workforce.WorkSessionSourceApp,
		CheckInTime: lateCheckIn, CheckOutTime: &lateCheckOut,
		CreatedBy: staff.ID,
	}
	lateSession.SetTenantID(tenantID)
	require.NoError(t, workforceCompose.NewTimeTrackingTestRows(workTime).Sessions.Create(ctx, lateSession))

	julyAfter, err := svc.GetMonthSummary(ctx, staff.ID, 2026, 7)
	require.NoError(t, err)
	assert.Equal(t, july.CarryInMinutes+240, julyAfter.CarryInMinutes,
		"correction in June must flow into July's live carry")
}
