// Package timetable — DB-backed handler tests for the school-year bootstrap
// (WP-B1) and the advisory overlap warnings on the period create path (WP-B2).
package timetable

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newPeriodsTestServer wires the periods handlers against the real service +
// repository on a fresh, unique tenant and returns a router that injects that
// tenant into every request context (mirroring jwt.TenantMiddleware). A fresh
// tenant keeps the assertions exact — the shared tenant 1 accumulates
// calendar periods from parallel test packages.
func newPeriodsTestServer(t *testing.T) chi.Router {
	t.Helper()
	db := testpkg.SetupTestDB(t)

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	t.Cleanup(func() {
		_, _ = db.NewDelete().
			Table("schedule.calendar_periods").
			Where("tenant_id = ?", tenantID).
			Exec(context.Background())
	})

	res := NewResource(Dependencies{
		CalendarPeriodService: scheduleSvc.NewCalendarPeriodServiceWithConfig(scheduleSvc.CalendarPeriodServiceConfig{
			Repo: repositories.NewFactory(db).CalendarPeriod,
		}),
		DB: db,
	})

	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(tenant.WithTenantID(testpkg.WithPackageTenantRuntime(req.Context()), tenantID)))
		})
	})
	r.Post("/periods", res.createPeriod)
	r.Post("/periods/bootstrap", res.bootstrapPeriods)
	return r
}

// TestBootstrapPeriodsEndToEnd drives POST /periods/bootstrap twice against
// the real service + repository: the first call creates the default school
// year (created=true), the second is a clean no-op (created=false) — both
// answer 200, never 409.
func TestBootstrapPeriodsEndToEnd(t *testing.T) {
	t.Parallel()

	router := newPeriodsTestServer(t)

	first := executeRequest(router, http.MethodPost, "/periods/bootstrap", nil)
	require.Equal(t, http.StatusOK, first.Code, "body=%s", first.Body.String())
	firstData := decodeTemplateData[bootstrapPeriodsResponse](t, first)
	assert.True(t, firstData.Created)
	require.Len(t, firstData.Periods, 1)
	created := firstData.Periods[0]
	assert.Contains(t, created.Name, "Schuljahr ")
	assert.Equal(t, "school_year", created.PeriodType)
	assert.True(t, created.IsActive)
	assert.Equal(t, 1, created.WeekCycleLength)

	second := executeRequest(router, http.MethodPost, "/periods/bootstrap", nil)
	require.Equal(t, http.StatusOK, second.Code,
		"repeated bootstrap must stay 200, never 409; body=%s", second.Body.String())
	secondData := decodeTemplateData[bootstrapPeriodsResponse](t, second)
	assert.False(t, secondData.Created)
	require.Len(t, secondData.Periods, 1)
	assert.Equal(t, created.ID, secondData.Periods[0].ID, "no second period may appear")
}

// TestCreatePeriodOverlapWarningsEndToEnd verifies the full warning wire
// shape against the real overlap query: an overlapping active period yields
// 201 + warnings, a disjoint one yields 201 without the warnings field.
func TestCreatePeriodOverlapWarningsEndToEnd(t *testing.T) {
	t.Parallel()

	router := newPeriodsTestServer(t)
	suffix := time.Now().UnixNano()

	seed := CalendarPeriodRequest{
		Name:       fmt.Sprintf("Warn-Basis-%d", suffix),
		PeriodType: "school_year",
		StartDate:  "2030-08-01",
		EndDate:    "2031-07-31",
		IsActive:   true,
	}
	seedW := executeRequest(router, http.MethodPost, "/periods", seed)
	require.Equal(t, http.StatusCreated, seedW.Code, "body=%s", seedW.Body.String())
	seedResp := decodeTemplateData[CalendarPeriodResponse](t, seedW)
	assert.Empty(t, seedResp.Warnings, "first period has nothing to overlap with")

	overlapping := CalendarPeriodRequest{
		Name:       fmt.Sprintf("Warn-Kollision-%d", suffix),
		PeriodType: "semester",
		StartDate:  "2031-01-01",
		EndDate:    "2031-12-31",
		IsActive:   true,
	}
	overlapW := executeRequest(router, http.MethodPost, "/periods", overlapping)
	require.Equal(t, http.StatusCreated, overlapW.Code,
		"overlap is advisory, the save must succeed; body=%s", overlapW.Body.String())
	overlapResp := decodeTemplateData[CalendarPeriodResponse](t, overlapW)
	require.Len(t, overlapResp.Warnings, 1)
	warning := overlapResp.Warnings[0]
	assert.Equal(t, "overlapping_active_periods", warning.Code)
	assert.Equal(t, []int64{seedResp.ID}, warning.OverlappingPeriodIDs)
	assert.Equal(t, []string{seed.Name}, warning.OverlappingPeriodNames)
	assert.Contains(t, warning.Message, seed.Name)
	assert.Contains(t, warning.Message, "01.08.2030 – 31.07.2031")

	disjoint := CalendarPeriodRequest{
		Name:       fmt.Sprintf("Warn-Frei-%d", suffix),
		PeriodType: "custom",
		StartDate:  "2035-08-01",
		EndDate:    "2036-07-31",
		IsActive:   true,
	}
	disjointW := executeRequest(router, http.MethodPost, "/periods", disjoint)
	require.Equal(t, http.StatusCreated, disjointW.Code, "body=%s", disjointW.Body.String())
	assert.NotContains(t, disjointW.Body.String(), `"warnings"`,
		"non-overlapping create must not carry a warnings field")
}
