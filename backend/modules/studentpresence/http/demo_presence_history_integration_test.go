package studentpresencehttp_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	studentpresencehttp "github.com/moto-nrw/project-phoenix/modules/studentpresence/http"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The public demo's route (ADR 0127) writes the days in the school of its
// path, through the server's request runtime, and nowhere else.
func TestDemoPresenceHistoryRouteWritesInTheSchoolOfThePath(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	router := chi.NewRouter()
	router.Use(testpkg.TenantRuntimeMiddleware(t, db))
	router.Post("/schools/{id}/demo-presence-history",
		studentpresencehttp.DemoPresenceHistoryHandler(compose.NewPresenceHistory(compose.Dependencies{DB: db})))

	device := testpkg.CreateTestDevice(t, db, "demo-presence-route")
	activity := testpkg.CreateTestActivityGroup(t, db, "Demo presence route")
	room := testpkg.CreateTestRoom(t, db, "Demo presence route")
	student := testpkg.CreateTestStudent(t, db, "Route", "History", "2b")
	day := calendar.NewDate(2026, time.March, 11)
	at := func(hour, minute int) time.Time {
		return day.BerlinMidnight().Add(time.Duration(hour)*time.Hour + time.Duration(minute)*time.Minute)
	}
	body, err := json.Marshal(map[string]any{"days": []map[string]any{{
		"student_id": student.ID, "date": day.String(), "device_id": device.ID,
		"check_in": at(12, 0), "check_out": at(15, 30),
		"visit": map[string]any{"activity_group_id": activity.ID, "room_id": room.ID,
			"session_start": at(12, 0), "session_end": at(16, 0), "entry": at(12, 10), "exit": at(15, 20)},
	}}})
	require.NoError(t, err)
	post := func(schoolID int64) *httptest.ResponseRecorder {
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, fmt.Sprintf("/schools/%d/demo-presence-history", schoolID), bytes.NewReader(body)))
		return rr
	}

	rr := post(testpkg.Tenant(t))
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	attendance, err := module.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{student.ID}, FromDate: day.String(), UntilDate: day.String()})
	require.NoError(t, err)
	require.Len(t, attendance, 1)
	assert.True(t, attendance[0].CheckInTime.Equal(at(12, 0)))
	visits, err := module.ListVisits(ctx, studentpresence.VisitFilter{StudentIDs: []int64{student.ID}})
	require.NoError(t, err)
	require.Len(t, visits, 1)

	// The tenant-composite foreign keys refuse a child or kiosk of another
	// school, so a wrong path writes nothing.
	other := testpkg.NewTenantScope(t, db)
	rr = post(other.TenantID)
	assert.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
	foreign, err := module.ListAttendance(other.Context(), studentpresence.AttendanceFilter{StudentIDs: []int64{student.ID}})
	require.NoError(t, err)
	assert.Empty(t, foreign)
}
