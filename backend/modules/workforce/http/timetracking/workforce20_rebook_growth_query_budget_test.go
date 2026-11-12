package timetracking_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/workforce/workforcetest"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

func TestWorkforce20RebookGrowth(t *testing.T) {
	t.Parallel()
	for _, n := range []int{3, 8} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			testpkg.OwnTenant(t)
			testpkg.SetupIsolatedTestDB(t)
			f := workforcetest.RebookGrowthFixture(t, n)
			before := f.AugustBalance(t)
			for _, dryRun := range []bool{true, false} {
				counter := testpkg.CaptureQueries(t, f.DB)
				rec := f.Rebook(t, f.Body("Workforce20 rebooking", dryRun))
				counter.Stop()
				require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
				var result workforce20RebookResponse
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result), rec.Body.String())
				require.Equal(t, !dryRun, result.Data.Applied)
				require.Equal(t, float64(n), result.Data.Days)
				require.Equal(t, n*480, result.Data.BalanceDeltaMinutes)
				require.False(t, result.Data.AllowanceExceeded)
				require.Len(t, result.Data.Absences, n)
				require.Len(t, result.Data.Allowances, 1)
				require.Equal(t, float64(n), result.Data.Allowances[0].BookingDays)
				for i, row := range result.Data.Absences {
					require.Equal(t, f.Absences[i], row.ID)
					require.Equal(t, "other", row.AbsenceType)
					require.Equal(t, strconv.FormatInt(f.TypeID, 10), row.AbsenceTypeID)
				}
				for _, stored := range f.StoredTypes(t) {
					want := "comp_time:"
					if !dryRun {
						want = "other:" + strconv.FormatInt(f.TypeID, 10)
					}
					require.Equal(t, want, stored)
				}
				var auditCount int
				require.NoError(t, f.DB.NewRaw("SELECT count(*) FROM audit.time_tracking_audit_log WHERE tenant_id = ? AND staff_id = ? AND reason = ?", testpkg.Tenant(t), f.SubjectID, "Workforce20 rebooking").Scan(testpkg.Ctx(t), &auditCount))
				wantAudit := 0
				if !dryRun {
					wantAudit = n
				}
				require.Equal(t, wantAudit, auditCount)
				if n == 3 {
					if dryRun {
						querybudgets.AssertQueryBudget(t, "workforce.workforce20.rebook_preview_n3", counter.Measurement(counter.Queries()))
					} else {
						querybudgets.AssertQueryBudget(t, "workforce.workforce20.rebook_apply_n3", counter.Measurement(counter.Queries()))
					}
				} else {
					if dryRun {
						querybudgets.AssertQueryBudget(t, "workforce.workforce20.rebook_preview_n8", counter.Measurement(counter.Queries()))
					} else {
						querybudgets.AssertQueryBudget(t, "workforce.workforce20.rebook_apply_n8", counter.Measurement(counter.Queries()))
					}
				}
			}
			require.Equal(t, before+n*480, f.AugustBalance(t))
			rec := httptest.NewRecorder()
			f.Router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, fmt.Sprintf("/staff/%d/absences/rebook", f.SubjectID), nil))
			require.Equal(t, http.StatusUnauthorized, rec.Code)
		})
	}
}

type workforce20RebookResponse struct {
	Data struct {
		Absences []struct {
			ID               int64  `json:"id"`
			AbsenceType      string `json:"absence_type"`
			AbsenceTypeID    string `json:"absence_type_id"`
			AbsenceTypeLabel string `json:"absence_type_label"`
		} `json:"absences"`
		Days                float64 `json:"days"`
		BalanceDeltaMinutes int     `json:"balance_delta_minutes"`
		Allowances          []struct {
			Year          int     `json:"year"`
			RemainingDays float64 `json:"remaining_days"`
			BookingDays   float64 `json:"booking_days"`
		} `json:"allowances"`
		AllowanceExceeded bool `json:"allowance_exceeded"`
		Applied           bool `json:"applied"`
	} `json:"data"`
}
