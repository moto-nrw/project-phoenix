package timetracking_test

import (
	"encoding/json"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/workforcetest"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestRebookBatchedTargetsPreserveVersionsOrderAndHalfDayRounding(t *testing.T) {
	t.Parallel()
	f := workforcetest.RebookGrowthFixture(t, 3)
	ctx := testpkg.Ctx(t)
	boundary := testpkg.Date(2026, time.August, 14)
	_, err := f.DB.NewRaw(`UPDATE config.staff_work_schedules SET target_minutes = 481, valid_until = ? WHERE tenant_id = ? AND staff_id = ?`, boundary, testpkg.Tenant(t), f.SubjectID).Exec(ctx)
	require.NoError(t, err)
	for weekday := range 5 {
		testpkg.CreateTestStaffWorkScheduleForTenant(t, f.DB, testpkg.Tenant(t), f.SubjectID, weekday, 483, boundary)
	}
	_, err = f.DB.NewRaw(`UPDATE active.staff_absences SET half_day = TRUE WHERE tenant_id = ? AND staff_id = ?`, testpkg.Tenant(t), f.SubjectID).Exec(ctx)
	require.NoError(t, err)
	before := f.AugustBalance(t)
	for _, dryRun := range []bool{true, false} {
		body := f.Body("Version and rounding contract", dryRun)
		ids := body["absence_ids"].([]string)
		slices.Reverse(ids)
		rec := f.Rebook(t, body)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var result workforce20RebookResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
		require.Equal(t, !dryRun, result.Data.Applied)
		require.Equal(t, 1.5, result.Data.Days)
		// Floor each credited half day separately: 481/2 + 483/2 + 483/2.
		require.Equal(t, 722, result.Data.BalanceDeltaMinutes)
		require.Len(t, result.Data.Allowances, 1)
		require.Equal(t, 1.5, result.Data.Allowances[0].BookingDays)
		for i, absence := range result.Data.Absences {
			require.Equal(t, f.Absences[len(f.Absences)-1-i], absence.ID)
		}
		if dryRun {
			require.Equal(t, before, f.AugustBalance(t))
		}
	}
	require.Equal(t, before+722, f.AugustBalance(t))
}

func TestRebookingCandidateBatchKeepsStaffAndTenantBoundaries(t *testing.T) {
	t.Parallel()
	f := workforcetest.RebookGrowthFixture(t, 3)
	capability := workforcetest.ComposeCapability(t, f.DB, nil)
	ctx := testpkg.Ctx(t)
	rows, err := capability.ListStaffAbsences(ctx, workforce.StaffAbsenceFilter{StaffID: f.SubjectID, IDs: []int64{f.Absences[2], f.Absences[0]}})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.ElementsMatch(t, []int64{f.Absences[0], f.Absences[2]}, []int64{rows[0].ID, rows[1].ID})
	rows, err = capability.ListStaffAbsences(ctx, workforce.StaffAbsenceFilter{IDs: []int64{}})
	require.NoError(t, err)
	require.Empty(t, rows)
	foreignID, _ := testpkg.CreateTestTenant(t, f.DB)
	rows, err = capability.ListStaffAbsences(testpkg.TenantContext(foreignID), workforce.StaffAbsenceFilter{IDs: f.Absences})
	require.NoError(t, err)
	require.Empty(t, rows)
	other := testpkg.CreateTestStaff(t, f.DB, "Rebooking", "Other")
	for _, staffID := range []int64{other.ID, 0} {
		_, err = capability.PreviewAllowanceRebooking(ctx, staffID, f.TypeID, f.Absences)
		require.ErrorIs(t, err, workforce.ErrStaffAbsenceNotFound)
	}
}

func TestRebookingTargetsKeepSparseYearsWithinDailyRangeLimit(t *testing.T) {
	t.Parallel()
	f := workforcetest.RebookGrowthFixture(t, 3)
	ctx := testpkg.Ctx(t)
	for i, day := range []string{"2024-08-09", "2025-08-08", "2026-08-07"} {
		_, err := f.DB.NewRaw(`UPDATE active.staff_absences SET date_start = ?, date_end = ? WHERE tenant_id = ? AND id = ?`, day, day, testpkg.Tenant(t), f.Absences[i]).Exec(ctx)
		require.NoError(t, err)
	}
	for _, dryRun := range []bool{true, false} {
		body := f.Body("Sparse date ranges", dryRun)
		body["absence_type"] = "training"
		delete(body, "absence_type_id")
		rec := f.Rebook(t, body)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var result workforce20RebookResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
		require.Equal(t, !dryRun, result.Data.Applied)
		require.Equal(t, 3.0, result.Data.Days)
		require.Equal(t, 480, result.Data.BalanceDeltaMinutes, "no schedule before 2026 means zero credit in earlier years")
	}
}
