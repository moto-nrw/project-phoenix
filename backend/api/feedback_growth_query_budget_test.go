package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/feedback"
	"github.com/moto-nrw/project-phoenix/modules/settings"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

func TestFeedbackListsPositiveGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	module, config, err := NewFeedbackTestModuleWithSettings(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	require.NoError(t, config.Settings.SetValue(ctx, settings.KeyFeedbackEnabled, true, nil, nil))
	student := testpkg.CreateTestStudent(t, db, "Feedback", "ListGrowth", "1a")
	_, account := testpkg.CreateTestStaffWithAccount(t, db, "Feedback", "Reader")
	token := testutil.MintTestJWT(t, testutil.AdminTestClaimsForTenant(int(account.ID), testpkg.Tenant(t)))
	foreign := testpkg.NewTenantScope(t, db)
	other := testpkg.CreateTestStudentForTenant(t, db, foreign.TenantID, "Foreign", "Feedback", "1a")
	_, err = db.NewRaw(`INSERT INTO feedback.entries (tenant_id, student_id, value, day, time, is_mensa_feedback) VALUES (?, ?, 'negative', '2026-08-31', '12:34:59', true)`, foreign.TenantID, other.ID).Exec(ctx)
	require.NoError(t, err)
	router := chi.NewRouter()
	router.Use(testpkg.TenantRuntimeMiddleware(t, db))
	router.Mount("/feedback", newFeedbackResource(module.Feedback).Router())
	entries := []feedback.Entry{}
	counts := map[string]int{}
	for _, n := range []int{3, 8} {
		for len(entries) < n {
			entry, err := module.Feedback.Submit(ctx, feedback.CreateEntry{Value: feedback.ValuePositive, Day: "2026-08-31", Time: fmt.Sprintf("12:34:%02d", len(entries)), StudentID: student.ID, IsMensaFeedback: true})
			require.NoError(t, err)
			entries = append(entries, entry)
		}
		for _, path := range []string{"/", fmt.Sprintf("/student/%d", student.ID), "/date/2026-08-31", "/mensa", "/date-range?start_date=2026-08-01&end_date=2026-08-31"} {
			request := testutil.NewAuthenticatedRequest(t, http.MethodGet, "/feedback"+path, nil, testutil.WithJWTBearer(token))
			counter := testpkg.CaptureQueries(t, db)
			response := testutil.ExecuteRequest(testpkg.MountedQueryBudgetRouter("/api", router), request)
			counter.Stop()
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			var body struct {
				Data []struct {
					ID              int64  `json:"id"`
					StudentID       int64  `json:"student_id"`
					Value           string `json:"value"`
					Day             string `json:"day"`
					Time            string `json:"time"`
					IsMensaFeedback bool   `json:"is_mensa_feedback"`
				} `json:"data"`
			}
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
			require.Len(t, body.Data, n)
			for i, row := range body.Data {
				want := entries[n-1-i]
				require.Equal(t, want.ID, row.ID)
				require.Equal(t, student.ID, row.StudentID)
				require.Equal(t, want.Value, row.Value)
				require.Equal(t, string(want.Day), row.Day)
				require.Equal(t, want.Time, row.Time)
				require.True(t, row.IsMensaFeedback)
			}
			if n == 3 {
				counts[path] = counter.Total()
			} else {
				require.Equal(t, counts[path], counter.Total(), path)
			}
			t.Logf("path=/feedback%s N=%d statements=%d scope=full-resource-with-real-settings-permission-and-tenant-transaction", path, n, counter.Total())
			switch path {
			case "/":
				querybudgets.AssertQueryBudget(t, "api.feedback.list.positive_growth", counter.Measurement(counter.Queries()))
			case "/date/2026-08-31":
				querybudgets.AssertQueryBudget(t, "api.feedback.date.positive_growth", counter.Measurement(counter.Queries()))
			case "/mensa":
				querybudgets.AssertQueryBudget(t, "api.feedback.mensa.positive_growth", counter.Measurement(counter.Queries()))
			case "/date-range?start_date=2026-08-01&end_date=2026-08-31":
				querybudgets.AssertQueryBudget(t, "api.feedback.date_range.positive_growth", counter.Measurement(counter.Queries()))
			default:
				querybudgets.AssertQueryBudget(t, "api.feedback.student.positive_growth", counter.Measurement(counter.Queries()))
			}
		}
	}
}
