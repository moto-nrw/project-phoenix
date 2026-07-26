// Tests for GET /deviations/history (#1886): range/slot filtering, resolved
// display names, and the missing-person fallback.
package timetable

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func historyRouter(s *devSetup) chi.Router {
	tenantID := tenant.FromContext(s.ctx)
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(tenant.WithTenantID(req.Context(), tenantID)))
		})
	})
	r.Get("/deviations/history", s.res.getDeviationHistory)
	return r
}

func TestDeviationHistory_ReturnsEventsWithResolvedNames(t *testing.T) {
	s := buildDevSetup(t)
	router := devRouter(s.ctx, s.res)
	_, date := futureSubDate(1)

	inst := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{Title: "Verlauf"})
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances", inst.ID) })
	t.Cleanup(func() { cleanupEventsForInstances(t, s.db, inst.ID) })

	rowA := testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.staffA, testpkg.InstanceStaffOpts{})
	rowB := testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.staffB, testpkg.InstanceStaffOpts{})
	t.Cleanup(func() { testpkg.CleanupInstanceStaffFixtures(t, s.db, rowA.ID, rowB.ID) })

	w := doDev(t, router, inst.ID, map[string]any{
		"substitutions": []map[string]any{{"absent_staff_id": s.staffA, "substitute_staff_id": s.staffY, "reason": "krank"}},
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	hr := historyRouter(s)
	req := httptest.NewRequest(http.MethodGet,
		fmt.Sprintf("/deviations/history?date=%s&date_to=%s", date.String(), date.String()), nil)
	rec := httptest.NewRecorder()
	hr.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())

	var resp struct {
		Data DeviationHistoryResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	// Other tests share tenant 1; narrow to this instance's events.
	var mine []DeviationHistoryEvent
	for _, ev := range resp.Data.Events {
		if ev.InstanceID != nil && *ev.InstanceID == inst.ID {
			mine = append(mine, ev)
		}
	}
	require.Len(t, mine, 1)
	ev := mine[0]
	assert.Equal(t, auditModels.DeviationEventSubstitution, ev.EventType)
	require.NotNil(t, ev.SubjectStaffName)
	assert.Contains(t, *ev.SubjectStaffName, "Planned", "subject staff display name resolved")
	require.NotNil(t, ev.RelatedStaffName)
	assert.Contains(t, *ev.RelatedStaffName, "Replacement", "substitute display name resolved")
	assert.Nil(t, ev.ActorName, "no JWT claims in the test request → actor unknown")
	require.NotNil(t, ev.Reason)
	assert.Equal(t, "krank", *ev.Reason)
	assert.Equal(t, date.String(), ev.OccurrenceDate)
}

func TestDeviationHistory_RejectsInvalidRange(t *testing.T) {
	s := buildDevSetup(t)
	hr := historyRouter(s)

	for _, path := range []string{
		"/deviations/history", // missing date
		"/deviations/history?date=2026-05-10&date_to=2026-05-01",    // inverted
		"/deviations/history?date=2026-01-01&date_to=2026-12-31",    // too large
		"/deviations/history?date=2026-05-01&activity_group_id=abc", // bad id
		"/deviations/history?date=2026-05-01&start_time=nope",       // bad time
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		hr.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "path=%s body=%s", path, rec.Body.String())
	}
}
