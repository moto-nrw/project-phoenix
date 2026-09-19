package timetable

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/timetable/legacy/timetableplanning"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestListPeriodsQueryBudget pins the statement count of GET /periods (#3124).
// The list runs the period read plus the usage counts, which cross two owner
// boundaries (Enrollment phases, Timetable planning tables) at one round trip
// each. The count must stay flat as periods and their references grow, and
// the exact register entry makes the next owner-boundary move fail here
// instead of at a runtime checkpoint (#3020).
func TestListPeriodsQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.Ctx(t)

	res := NewResource(Dependencies{
		CalendarPeriodService: timetableplanning.NewCalendarPeriodServiceWithConfig(timetableplanning.CalendarPeriodServiceConfig{
			Repo: mustTimetableTestRepositories(db).CalendarPeriod,
		}),
		DB: db,
	})
	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(tenant.WithTenantID(testpkg.WithPackageTenantRuntime(req.Context()), tenant.FromContext(ctx))))
		})
	})
	// Production wraps the list in the tenant transaction; the owner queries
	// join that transaction instead of opening their own.
	router.Use(testpkg.TenantTxMiddleware(db))
	router.Get("/periods", res.listPeriods)

	created := 0
	addPeriods := func(n int) {
		for range n {
			year := 2040 + created
			period := testpkg.CreateTestCalendarPeriod(t, db, fmt.Sprintf("Budget-%d", created),
				testpkg.ScheduleDate(year, time.August, 1), testpkg.ScheduleDate(year+1, time.July, 31))
			testpkg.CreateTestEnrollmentPhaseForCalendarPeriod(t, db, period.ID)
			group := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("Budget-AG-%d", created))
			_, err := db.NewUpdate().TableExpr("activities.groups").
				Set("calendar_period_id = ?", period.ID).
				Where("id = ?", group.ID).
				Exec(ctx)
			require.NoError(t, err)
			created++
		}
	}

	counter := testpkg.CaptureQueries(t, db)
	run := func() int {
		counter.Reset()
		w := executeRequest(router, http.MethodGet, "/periods", nil)
		require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
		periods := decodeTemplateData[[]CalendarPeriodResponse](t, w)
		require.Len(t, periods, created)
		for _, p := range periods {
			assert.Equal(t, 1, p.EnrollmentPhaseCount, "period %d", p.ID)
			assert.Equal(t, 1, p.ActivityGroupCount, "period %d", p.ID)
		}
		return counter.Total()
	}

	addPeriods(3)
	smallCount := run()

	addPeriods(3)
	largeCount := run()

	t.Logf("query budget: 3 periods → %d queries, 6 periods → %d queries", smallCount, largeCount)
	assert.Equal(t, smallCount, largeCount,
		"query count must be independent of the period count (no per-period N+1)")
	testpkg.AssertQueryBudget(t, "api.timetable.periods.list", counter.Queries())
}
