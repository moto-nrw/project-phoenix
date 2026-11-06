package timetracking

import (
	"encoding/json"
	"net/http"
	"slices"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestRebookingMonthReadFailureRemainsInternalError(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	f := setupRebookFixture(t, 30)
	_, err := f.tc.db.NewRaw(`ALTER TABLE active.staff_month_balance_snapshots RENAME COLUMN closing_balance_minutes TO unavailable_balance`).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	for _, dryRun := range []bool{true, false} {
		rec := f.rebook(t, f.body("Month read failure", dryRun))
		require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
		require.Equal(t, []string{"comp_time:", "comp_time:", "comp_time:"}, f.storedTypes(t))
	}
}

func TestRebookingVacationReadsUnselectedDaysLive(t *testing.T) {
	t.Parallel()
	f := setupRebookFixture(t, 30)
	require.Equal(t, http.StatusOK, putVacationQuota(t, f.tc, f.token, f.subjectID, 2026, 30).Code)
	insertAbsenceRow(t, f.tc, map[string]any{
		"staff_id": f.subjectID, "absence_type": "vacation", "date_start": "2026-07-06", "date_end": "2026-07-06",
		"working_days": 1, "status": "approved", "created_by": f.subjectID,
	})
	for _, dryRun := range []bool{true, false} {
		body := f.body("Live vacation account", dryRun)
		body["absence_type"], body["absence_type_id"] = "vacation", nil
		rec := f.rebook(t, body)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var result struct {
			Data struct {
				Vacation []struct {
					RemainingBefore float64 `json:"remaining_before"`
					RemainingAfter  float64 `json:"remaining_after"`
				} `json:"vacation"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
		require.Len(t, result.Data.Vacation, 1)
		require.Equal(t, 29.0, result.Data.Vacation[0].RemainingBefore)
		require.Equal(t, 26.0, result.Data.Vacation[0].RemainingAfter)
	}
}

func TestRebookingMonthReadsIgnoreOtherStaffAndReopenedMonths(t *testing.T) {
	t.Parallel()
	f := setupRebookFixture(t, 30)
	other := testpkg.CreateTestStaff(t, f.tc.db, "Rebooking", "Other")
	_, err := f.tc.db.NewRaw(`INSERT INTO active.staff_month_balance_snapshots
		(tenant_id, staff_id, year, month, closing_balance_minutes, closed_by, close_reason, reopened_at, reopened_by)
		VALUES (?, ?, 2026, 8, 0, ?, 'Reopened', NOW(), ?),
		       (?, ?, 2026, 8, 0, ?, 'Other staff', NULL, NULL),
		       (?, ?, 2026, 7, 0, ?, 'Earlier month', NULL, NULL)`,
		testpkg.Tenant(t), f.subjectID, f.subjectID, f.subjectID,
		testpkg.Tenant(t), other.ID, other.ID,
		testpkg.Tenant(t), f.subjectID, f.subjectID).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	for _, dryRun := range []bool{true, false} {
		rec := f.rebook(t, f.body("Open month", dryRun))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	}
}

func TestRebookingMonthValidationKeepsRequestOrder(t *testing.T) {
	t.Parallel()
	f := setupRebookFixture(t, 30)
	ctx := testpkg.Ctx(t)
	_, err := f.tc.db.NewRaw(`UPDATE active.staff_absences SET start_half_day = TRUE WHERE tenant_id = ? AND id = ?`, testpkg.Tenant(t), f.absences[0]).Exec(ctx)
	require.NoError(t, err)
	_, err = f.tc.db.NewRaw(`UPDATE active.staff_absences SET date_start = '2026-07-03', date_end = '2026-07-03' WHERE tenant_id = ? AND id = ?`, testpkg.Tenant(t), f.absences[1]).Exec(ctx)
	require.NoError(t, err)
	_, err = f.tc.db.NewRaw(`INSERT INTO active.staff_month_balance_snapshots
		(tenant_id, staff_id, year, month, closing_balance_minutes, closed_by, close_reason)
		VALUES (?, ?, 2026, 7, 0, ?, 'Closed July')`, testpkg.Tenant(t), f.subjectID, f.subjectID).Exec(ctx)
	require.NoError(t, err)
	for _, dryRun := range []bool{true, false} {
		body := f.body("Validation order", dryRun)
		rec := f.rebook(t, body)
		require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
		require.Contains(t, rec.Body.String(), `"code":"workforce.rebooking_half_day_edge"`)
		slices.Reverse(body["absence_ids"].([]string))
		rec = f.rebook(t, body)
		require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
		require.Contains(t, rec.Body.String(), `"code":"workforce.month_closed"`)
		require.Contains(t, rec.Body.String(), `"month":"Juli 2026"`)
	}
}
