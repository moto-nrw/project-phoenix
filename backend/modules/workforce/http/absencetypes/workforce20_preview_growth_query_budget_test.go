//go:build testhooks

package absencetypes_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

func TestWorkforce20AllowancePreviewGrowth(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	module, caller, err := api.NewAbsenceTypeTestModule(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	router := chi.NewRouter()
	router.Use(testpkg.TenantRuntimeMiddleware(t, db))
	router.Mount("/api/absence-types", api.NewAbsenceTypesRouterForBudgetTest(module.Administration, caller.Caller().StaffID))
	staff, account := testpkg.CreateTestStaffWithAccount(t, db, "Allowance", "Reader")
	typ, err := module.Catalog.CreateAbsenceType(testpkg.Ctx(t), workforce.CreateAbsenceType{Name: "Workforce20 yearly allowance", AllowanceEnabled: true})
	require.NoError(t, err)
	for weekday := range 5 {
		testpkg.CreateTestStaffWorkScheduleForTenant(t, db, testpkg.Tenant(t), staff.ID, weekday, 480, calendar.NewDate(2026, 1, 1))
	}
	createdYears := 0
	for _, n := range []int{3, 8} {
		for createdYears < n {
			year := 2026 + createdYears
			_, err := db.NewRaw("INSERT INTO active.staff_absence_type_allowances (tenant_id,staff_id,absence_type_id,year,entitled_days) VALUES (?,?,?,?,?)", testpkg.Tenant(t), staff.ID, typ.ID, year, 300).Exec(testpkg.Ctx(t))
			require.NoError(t, err)
			createdYears++
		}
		path := fmt.Sprintf("/api/absence-types/%d/allowances/%d/preview?date_start=2026-01-01&date_end=%d-12-31", typ.ID, staff.ID, 2025+n)
		claims := testutil.DefaultTestClaims()
		claims.ID = int(account.ID)
		claims.Roles = []string{"user"}
		claims.Permissions = []string{"time_tracking:manage"}
		token := testutil.MintTestJWT(t, claims)
		counter := testpkg.CaptureQueries(t, db)
		rec := testutil.ExecuteRequestForTest(t, router, testutil.NewAuthenticatedRequest(t, http.MethodGet, path, nil, testutil.WithJWTBearer(token)))
		counter.Stop()
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var body struct {
			Data struct {
				Years []struct {
					Year         int     `json:"year"`
					StaffID      string  `json:"staff_id"`
					BookingDays  float64 `json:"booking_days"`
					EntitledDays float64 `json:"entitled_days"`
				} `json:"years"`
				Blocked bool `json:"blocked"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		require.Len(t, body.Data.Years, n)
		require.False(t, body.Data.Blocked)
		for i, row := range body.Data.Years {
			year := 2026 + i
			require.Equal(t, year, row.Year)
			require.Equal(t, fmt.Sprint(staff.ID), row.StaffID)
			require.Equal(t, 300.0, row.EntitledDays)
			days := 0
			for d := calendar.NewDate(year, 1, 1); d.Year() == year; d = d.AddDays(1) {
				if d.Weekday() != time.Saturday && d.Weekday() != time.Sunday {
					days++
				}
			}
			require.Equal(t, float64(days), row.BookingDays)
		}
		querybudgets.AssertQueryBudget(t, "workforce.workforce20.allowance_preview", counter.Queries())
	}
}
