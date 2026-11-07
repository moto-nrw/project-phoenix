package students_test

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	rootAPI "github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

func TestWorkforce20BirthdaysGrowth(t *testing.T) {
	t.Parallel()
	for _, export := range []bool{false, true} {
		t.Run(fmt.Sprint(export), func(t *testing.T) {
			testpkg.OwnTenant(t)
			testpkg.SetupIsolatedTestDB(t)
			today := calendar.NewDate(2026, time.November, 6)
			tc := setupBirthdaysRoute(t, func() time.Time { return today.BerlinMidnight().Add(12 * time.Hour) })
			testpkg.SetTestTenantSetting(t, tc.db, testpkg.Tenant(t), keyDisplayEnabled, `true`)
			testpkg.SetTestTenantSetting(t, tc.db, testpkg.Tenant(t), keyIncludeStaff, `true`)
			_, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "Birthday", "Reader")
			router := chi.NewRouter()
			router.Use(testpkg.TenantRuntimeMiddleware(t, tc.db))
			router.Mount("/birthdays", tc.resource.Router(rootAPI.ProtectedTenantRoutes))
			foreign := testpkg.NewTenantScope(t, tc.db)
			foreignStaff := testpkg.CreateTestStaffForTenant(t, tc.db, foreign.TenantID, "Foreign", "Birthday")
			setPersonBirthday(t, tc.db, foreignStaff.PersonID, calendar.NewDate(1986, time.November, 6))
			counts := []int{}
			created := 0
			for _, n := range []int{3, 8} {
				for created < n {
					i := created
					staff := testpkg.CreateTestStaff(t, tc.db, "Birthday", fmt.Sprintf("Member%02d", 8-i))
					setPersonBirthday(t, tc.db, staff.PersonID, calendar.NewDate(1986, time.November, 6))
					created++
				}
				// Name order is asserted from the complete accumulated fixture, rather
				// than assuming database insertion order.
				method, path, body := http.MethodGet, "/birthdays/?week_start=2026-11-06", ""
				perms := []string{"users:read"}
				if export {
					method, path, body = http.MethodPost, "/birthdays/staff-export", `{"format":"csv","months":["11"]}`
					perms = []string{"time_tracking:manage"}
				}
				req := httptest.NewRequest(method, path, strings.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				claims := claimsFor(t, account.ID)
				claims.Permissions = perms
				req.Header.Set("Authorization", "Bearer "+testutil.MintTestJWT(t, claims))
				counter := testpkg.CaptureQueries(t, tc.db)
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, req)
				counter.Stop()
				require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
				require.NotContains(t, rec.Body.String(), "Foreign")
				if export {
					reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(rec.Body.String(), "\xef\xbb\xbf")))
					reader.Comma = ';'
					rows, err := reader.ReadAll()
					require.NoError(t, err)
					for i := 0; i < n; i++ {
						require.Equal(t, []string{fmt.Sprintf("Birthday Member%02d", 9-n+i), "06.11.1986"}, rows[i+1])
					}
					require.Len(t, rows, n+1)
				} else {
					payload := decodeOverview(t, rec)
					require.True(t, payload.Data.Enabled)
					require.True(t, payload.Data.IncludeStaff)
					require.Len(t, payload.Data.Celebrations, n)
					for i, row := range payload.Data.Celebrations {
						require.Equal(t, fmt.Sprintf("Birthday Member%02d", 9-n+i), row.Name)
						require.Equal(t, "staff", row.Kind)
						require.Equal(t, "2026-11-06", row.Date)
						require.Zero(t, row.Age, "staff overview does not disclose age")
						if i > 0 {
							require.LessOrEqual(t, payload.Data.Celebrations[i-1].Name, row.Name)
						}
					}
				}
				if export {
					querybudgets.AssertQueryBudget(t, "workforce.workforce20.birthday_staff_export", counter.Queries())
				} else {
					querybudgets.AssertQueryBudget(t, "workforce.workforce20.birthday_overview", counter.Queries())
				}
				counts = append(counts, len(counter.Queries()))
				denied := httptest.NewRequest(method, path, strings.NewReader(body))
				claims.Permissions = nil
				denied.Header.Set("Authorization", "Bearer "+testutil.MintTestJWT(t, claims))
				rr := httptest.NewRecorder()
				router.ServeHTTP(rr, denied)
				require.Equal(t, http.StatusForbidden, rr.Code, rr.Body.String())
			}
			require.Equal(t, counts[0], counts[1])
		})
	}
}
