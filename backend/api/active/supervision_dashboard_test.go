package active_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// dashboardPerms is the permission set the aggregated endpoint serves fully —
// the union of what its constituent single endpoints required (#2096).
var dashboardPerms = []string{"groups:read", "schedules:read", "users:read"}

type dashboardEnvelope struct {
	Data struct {
		Groups []struct {
			ID        string  `json:"id"`
			RoomID    *string `json:"room_id"`
			RoomName  string  `json:"room_name"`
			RoomColor *string `json:"room_color"`
		} `json:"groups"`
		SelectedGroupID *string `json:"selected_group_id"`
		UnclaimedGroups []struct {
			ID       string `json:"id"`
			RoomName string `json:"room_name"`
		} `json:"unclaimed_groups"`
		CurrentStaffID    *string `json:"current_staff_id"`
		EducationalGroups []struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			RoomName string `json:"room_name"`
		} `json:"educational_groups"`
		Schulhof *struct {
			Exists bool `json:"exists"`
		} `json:"schulhof_status"`
		Capabilities struct {
			WebSpontaneousActivitiesEnabled bool `json:"web_spontaneous_activities_enabled"`
		} `json:"capabilities"`
		ActiveSessions []map[string]any `json:"active_sessions"`
		PlannedNow     []map[string]any `json:"planned_now"`
		Visits         []struct {
			StudentID         string  `json:"student_id"`
			StudentName       string  `json:"student_name"`
			SchoolClass       string  `json:"school_class"`
			GroupName         string  `json:"group_name"`
			ActiveGroupID     string  `json:"active_group_id"`
			ActualArrivalTime *string `json:"actual_arrival_time"`
			Sick              bool    `json:"sick"`
		} `json:"visits"`
		TrackingIndicators struct {
			Labels  []string          `json:"labels"`
			Results map[string][]bool `json:"results"`
		} `json:"tracking_indicators"`
		PickupTimes []struct {
			StudentID  string  `json:"student_id"`
			PickupTime *string `json:"pickup_time"`
		} `json:"pickup_times"`
		ArrivalTimes []struct {
			StudentID       string  `json:"student_id"`
			ExpectedArrival *string `json:"expected_arrival"`
		} `json:"arrival_times"`
	} `json:"data"`
}

func decodeDashboard(t *testing.T, body []byte) *dashboardEnvelope {
	t.Helper()
	var envelope dashboardEnvelope
	require.NoError(t, json.Unmarshal(body, &envelope))
	return &envelope
}

// setupDashboardContext wires the production active router with the real
// supervision dashboard service (assigned post-construction, as api/base.go
// does).
func setupDashboardContext(t *testing.T) (*testContext, chi.Router) {
	t.Helper()
	tc := setupTestContext(t)
	tc.resource.SupervisionDashboardService = tc.services.SupervisionDashboard
	return tc, mountActiveRouter(tc)
}

func dashboardExec(t *testing.T, router chi.Router, path string, accountID int64, perms []string) *dashboardEnvelope {
	t.Helper()
	rr := dashboardExecRaw(t, router, path, accountID, perms)
	require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())
	return decodeDashboard(t, rr.Body.Bytes())
}

func dashboardExecRaw(t *testing.T, router chi.Router, path string, accountID int64, perms []string) *httptest.ResponseRecorder {
	t.Helper()
	req := testutil.NewRequest("GET", path, nil)
	return testutil.ExecuteWithAuthPermissions(t, router, req, testutil.TeacherTestClaims(int(accountID)), perms)
}

func TestSupervisionDashboard_Aggregates(t *testing.T) {
	tc, router := setupDashboardContext(t)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "DashAgg", "Leader")
	room := testpkg.CreateTestRoom(t, tc.db, "DashAggRoom")
	activityGroup := testpkg.CreateTestActivityGroup(t, tc.db, "DashAggActivity")
	activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activityGroup.ID, room.ID)
	testpkg.CreateTestGroupSupervisor(t, tc.db, teacher.Staff.ID, activeGroup.ID, "supervisor")

	eduGroup := testpkg.CreateTestEducationGroup(t, tc.db, "DashAggEdu")
	testpkg.CreateTestGroupTeacher(t, tc.db, eduGroup.ID, teacher.ID)

	student := testpkg.CreateTestStudent(t, tc.db, "DashAgg", "Kind", "DA1")
	testpkg.AssignStudentToGroup(t, tc.db, student.ID, eduGroup.ID)

	device := testpkg.CreateTestDevice(t, tc.db, "DashAggDevice")
	checkIn := time.Now().Add(-1 * time.Hour)
	testpkg.CreateTestAttendance(t, tc.db, student.ID, teacher.Staff.ID, device.ID, checkIn, nil)
	testpkg.CreateTestVisit(t, tc.db, student.ID, activeGroup.ID, checkIn, nil)

	today := timezone.TodayDate()
	pickupException := testpkg.CreateTestPickupException(t, tc.db, student.ID, today, teacher.Staff.ID, "15:30", "Test")
	defer testpkg.CleanupScheduleFixturesB11(t, tc.db, nil, nil, nil, []int64{pickupException.ID}, nil, nil)
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID, eduGroup.ID, activeGroup.ID, activityGroup.ID, device.ID, room.ID, teacher.ID)

	settingsCtx := testpkg.TenantContext(1)
	require.NoError(t, tc.services.Settings.SetValue(settingsCtx, configModel.KeyTrackingIndicatorsEnabled, true, nil, nil))
	require.NoError(t, tc.services.Settings.SetValue(settingsCtx, configModel.KeyTrackingIndicator1, "Hausaufgaben", nil, nil))
	t.Cleanup(func() {
		_ = tc.services.Settings.ResetValue(settingsCtx, configModel.KeyTrackingIndicatorsEnabled, nil, nil)
		_ = tc.services.Settings.ResetValue(settingsCtx, configModel.KeyTrackingIndicator1, nil, nil)
	})

	envelope := dashboardExec(t, router, "/active/supervision-dashboard", account.ID, dashboardPerms)
	data := envelope.Data

	// Supervised session with bulk-loaded room info.
	require.Len(t, data.Groups, 1)
	assert.Equal(t, strconv.FormatInt(activeGroup.ID, 10), data.Groups[0].ID)
	require.NotNil(t, data.Groups[0].RoomID)
	assert.Equal(t, strconv.FormatInt(room.ID, 10), *data.Groups[0].RoomID)
	assert.Equal(t, room.Name, data.Groups[0].RoomName)

	require.NotNil(t, data.SelectedGroupID)
	assert.Equal(t, strconv.FormatInt(activeGroup.ID, 10), *data.SelectedGroupID)

	require.NotNil(t, data.CurrentStaffID)
	assert.Equal(t, strconv.FormatInt(teacher.Staff.ID, 10), *data.CurrentStaffID)

	foundEdu := false
	for _, group := range data.EducationalGroups {
		if group.ID == strconv.FormatInt(eduGroup.ID, 10) {
			foundEdu = true
			assert.Equal(t, eduGroup.Name, group.Name)
		}
	}
	assert.True(t, foundEdu, "educational groups must include the teacher's group")

	// Schulhof status is present (deterministic exists flag, no silent null).
	require.NotNil(t, data.Schulhof)

	// Open visit for the selected session with display fields and the
	// full-access actual arrival clock.
	require.Len(t, data.Visits, 1)
	visit := data.Visits[0]
	assert.Equal(t, strconv.FormatInt(student.ID, 10), visit.StudentID)
	assert.Equal(t, "DashAgg Kind", visit.StudentName)
	assert.Equal(t, "DA1", visit.SchoolClass)
	assert.Equal(t, eduGroup.Name, visit.GroupName)
	assert.Equal(t, strconv.FormatInt(activeGroup.ID, 10), visit.ActiveGroupID)
	assert.NotNil(t, visit.ActualArrivalTime)

	// Tracking indicators for the visit students.
	assert.Equal(t, []string{"Hausaufgaben"}, data.TrackingIndicators.Labels)

	// Today's pickup exception surfaces in the folded-in pickup times.
	require.Len(t, data.PickupTimes, 1)
	assert.Equal(t, strconv.FormatInt(student.ID, 10), data.PickupTimes[0].StudentID)
	require.NotNil(t, data.PickupTimes[0].PickupTime)
	assert.Equal(t, "15:30", *data.PickupTimes[0].PickupTime)

	// Full permission set resolves capabilities from settings defaults.
	assert.True(t, data.Capabilities.WebSpontaneousActivitiesEnabled)
}

// TestSupervisionDashboard_MinimalProjection pins the GDPR/payload contract:
// the aggregate must never carry the wide personal fields the supervision
// page does not render.
func TestSupervisionDashboard_MinimalProjection(t *testing.T) {
	tc, router := setupDashboardContext(t)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "DashSlim", "Leader")
	room := testpkg.CreateTestRoom(t, tc.db, "DashSlimRoom")
	activityGroup := testpkg.CreateTestActivityGroup(t, tc.db, "DashSlimActivity")
	activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activityGroup.ID, room.ID)
	testpkg.CreateTestGroupSupervisor(t, tc.db, teacher.Staff.ID, activeGroup.ID, "supervisor")

	student := testpkg.CreateTestStudent(t, tc.db, "DashSlim", "Kind", "DS1")
	testpkg.CreateTestVisit(t, tc.db, student.ID, activeGroup.ID, time.Now().Add(-30*time.Minute), nil)
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID, activeGroup.ID, activityGroup.ID, room.ID, teacher.ID)

	rr := dashboardExecRaw(t, router, "/active/supervision-dashboard", account.ID, dashboardPerms)
	require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())

	body := rr.Body.String()
	for _, forbidden := range []string{
		"guardian_name", "guardian_contact", "guardian_email", "guardian_phone",
		"address_street", "address_city", "address_postal_code",
		"health_info", "supervisor_notes", "extra_info", "birthday", "tag_id",
		"bus_days", "pickup_days", "departure_days", "allowed_departure_modes",
		"companion", "privacy", "person_id",
	} {
		assert.NotContains(t, body, forbidden,
			"aggregated dashboard response must not ship unrendered personal/internal field %q", forbidden)
	}
}

// dashboardQueryCounter counts every SQL statement issued through the hooked
// *bun.DB, including statements inside tenant transactions.
type dashboardQueryCounter struct {
	mu      sync.Mutex
	queries []string
}

func (h *dashboardQueryCounter) BeforeQuery(ctx context.Context, event *bun.QueryEvent) context.Context {
	h.mu.Lock()
	h.queries = append(h.queries, event.Query)
	h.mu.Unlock()
	return ctx
}

func (h *dashboardQueryCounter) AfterQuery(context.Context, *bun.QueryEvent) {}

func (h *dashboardQueryCounter) reset() {
	h.mu.Lock()
	h.queries = nil
	h.mu.Unlock()
}

func (h *dashboardQueryCounter) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.queries)
}

// TestSupervisionDashboard_QueryBudget guards the aggregate against
// per-student N+1 regressions: the query count must not grow with the number
// of checked-in students, and the total per request stays under a fixed
// budget.
func TestSupervisionDashboard_QueryBudget(t *testing.T) {
	tc, router := setupDashboardContext(t)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "DashBudget", "Leader")
	room := testpkg.CreateTestRoom(t, tc.db, "DashBudgetRoom")
	activityGroup := testpkg.CreateTestActivityGroup(t, tc.db, "DashBudgetActivity")
	activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activityGroup.ID, room.ID)
	testpkg.CreateTestGroupSupervisor(t, tc.db, teacher.Staff.ID, activeGroup.ID, "supervisor")

	var studentIDs []int64
	addStudents := func(n int) {
		for i := range n {
			student := testpkg.CreateTestStudent(t, tc.db, "DashBudget", fmt.Sprintf("Kind%d", len(studentIDs)+i), "DB1")
			testpkg.CreateTestVisit(t, tc.db, student.ID, activeGroup.ID, time.Now().Add(-30*time.Minute), nil)
			studentIDs = append(studentIDs, student.ID)
		}
	}
	defer func() {
		testpkg.CleanupActivityFixtures(t, tc.db, append(studentIDs, activeGroup.ID, activityGroup.ID, room.ID, teacher.ID)...)
	}()

	counter := &dashboardQueryCounter{}
	tc.db.AddQueryHook(counter)

	run := func() int {
		counter.reset()
		rr := dashboardExecRaw(t, router, "/active/supervision-dashboard", account.ID, dashboardPerms)
		require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())
		return counter.count()
	}

	addStudents(3)
	smallCount := run()

	addStudents(7)
	largeCount := run()

	t.Logf("query budget: 3 students → %d queries, 10 students → %d queries", smallCount, largeCount)

	assert.Equal(t, smallCount, largeCount,
		"query count must be independent of the checked-in student count (no per-student N+1)")

	// Fixed budget for the whole aggregate: identity resolution, session +
	// room bulk loads, schulhof status, unclaimed groups, educational groups,
	// planned-now + active sessions, visits + attendance, tracking, pickup /
	// arrival bulk loads, settings snapshot, tenant tx overhead. Raise only
	// with a written justification.
	// Measured at 29 flat; the cap leaves headroom for benign changes only.
	const maxQueries = 40
	assert.LessOrEqual(t, largeCount, maxQueries,
		"aggregated supervision dashboard request exceeded its query budget")
}

// TestSupervisionDashboard_PayloadBudget bounds the wire size for a
// production-sized session so the aggregate stays a fraction of the ~11
// responses it replaces.
func TestSupervisionDashboard_PayloadBudget(t *testing.T) {
	tc, router := setupDashboardContext(t)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "DashPayload", "Leader")
	room := testpkg.CreateTestRoom(t, tc.db, "DashPayloadRoom")
	activityGroup := testpkg.CreateTestActivityGroup(t, tc.db, "DashPayloadActivity")
	activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activityGroup.ID, room.ID)
	testpkg.CreateTestGroupSupervisor(t, tc.db, teacher.Staff.ID, activeGroup.ID, "supervisor")

	var studentIDs []int64
	const groupSize = 30
	for i := range groupSize {
		student := testpkg.CreateTestStudent(t, tc.db, "DashPayload", fmt.Sprintf("Produktionskind%02d", i), "DP1")
		testpkg.CreateTestVisit(t, tc.db, student.ID, activeGroup.ID, time.Now().Add(-30*time.Minute), nil)
		studentIDs = append(studentIDs, student.ID)
	}
	defer func() {
		testpkg.CleanupActivityFixtures(t, tc.db, append(studentIDs, activeGroup.ID, activityGroup.ID, room.ID, teacher.ID)...)
	}()

	rr := dashboardExecRaw(t, router, "/active/supervision-dashboard", account.ID, dashboardPerms)
	require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())

	size := rr.Body.Len()
	t.Logf("payload budget: %d students → %d bytes (%.0f bytes/student)", groupSize, size, float64(size)/groupSize)

	// Budget: ≤ 700 bytes per checked-in student (visit projection, tracking
	// row, pickup/arrival rows) plus 4 KB envelope/session/schulhof overhead.
	const maxBytes = 4096 + groupSize*700
	assert.LessOrEqual(t, size, maxBytes, "aggregated supervision dashboard payload exceeded its budget")
}

func TestSupervisionDashboard_ErrorContract(t *testing.T) {
	tc, router := setupDashboardContext(t)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "DashErr", "Leader")
	room := testpkg.CreateTestRoom(t, tc.db, "DashErrRoom")
	activityGroup := testpkg.CreateTestActivityGroup(t, tc.db, "DashErrActivity")
	activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activityGroup.ID, room.ID)
	testpkg.CreateTestGroupSupervisor(t, tc.db, teacher.Staff.ID, activeGroup.ID, "supervisor")

	otherTeacher, _ := testpkg.CreateTestTeacherWithAccount(t, tc.db, "DashErr", "Other")
	foreignRoom := testpkg.CreateTestRoom(t, tc.db, "DashErrForeignRoom")
	foreignActivityGroup := testpkg.CreateTestActivityGroup(t, tc.db, "DashErrForeignActivity")
	foreignActiveGroup := testpkg.CreateTestActiveGroup(t, tc.db, foreignActivityGroup.ID, foreignRoom.ID)
	testpkg.CreateTestGroupSupervisor(t, tc.db, otherTeacher.Staff.ID, foreignActiveGroup.ID, "supervisor")

	student := testpkg.CreateTestStudent(t, tc.db, "DashErr", "Kind", "DE1")
	testpkg.CreateTestVisit(t, tc.db, student.ID, activeGroup.ID, time.Now().Add(-30*time.Minute), nil)

	today := timezone.TodayDate()
	pickupException := testpkg.CreateTestPickupException(t, tc.db, student.ID, today, teacher.Staff.ID, "15:45", "Test")
	defer testpkg.CleanupScheduleFixturesB11(t, tc.db, nil, nil, nil, []int64{pickupException.ID}, nil, nil)
	defer testpkg.CleanupActivityFixtures(t, tc.db,
		student.ID, activeGroup.ID, activityGroup.ID, room.ID, teacher.ID,
		foreignActiveGroup.ID, foreignActivityGroup.ID, foreignRoom.ID, otherTeacher.ID)

	t.Run("invalid group_id is a 400", func(t *testing.T) {
		rr := dashboardExecRaw(t, router, "/active/supervision-dashboard?group_id=abc", account.ID, dashboardPerms)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("unsupervised group is a 403", func(t *testing.T) {
		rr := dashboardExecRaw(t, router, fmt.Sprintf("/active/supervision-dashboard?group_id=%d", foreignActiveGroup.ID), account.ID, dashboardPerms)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("missing schedules:read redacts the timetable sections", func(t *testing.T) {
		envelope := dashboardExec(t, router, "/active/supervision-dashboard", account.ID, []string{"groups:read", "users:read"})
		assert.Empty(t, envelope.Data.PlannedNow)
		assert.Empty(t, envelope.Data.ActiveSessions)
		assert.False(t, envelope.Data.Capabilities.WebSpontaneousActivitiesEnabled)
		// The roster itself stays complete.
		assert.NotEmpty(t, envelope.Data.Groups)
		assert.NotEmpty(t, envelope.Data.Visits)
	})

	t.Run("missing users:read redacts pickup and arrival times", func(t *testing.T) {
		envelope := dashboardExec(t, router, "/active/supervision-dashboard", account.ID, []string{"groups:read", "schedules:read"})
		assert.Empty(t, envelope.Data.PickupTimes)
		assert.Empty(t, envelope.Data.ArrivalTimes)
		assert.NotEmpty(t, envelope.Data.Visits)
	})

	t.Run("full permissions include the pickup exception", func(t *testing.T) {
		envelope := dashboardExec(t, router, "/active/supervision-dashboard", account.ID, dashboardPerms)
		require.Len(t, envelope.Data.PickupTimes, 1)
	})

	t.Run("no supervised groups is an explicit empty 200", func(t *testing.T) {
		lonely, lonelyAccount := testpkg.CreateTestTeacherWithAccount(t, tc.db, "DashErr", "Gruppenlos")
		defer testpkg.CleanupActivityFixtures(t, tc.db, lonely.ID)

		envelope := dashboardExec(t, router, "/active/supervision-dashboard", lonelyAccount.ID, dashboardPerms)
		assert.Empty(t, envelope.Data.Groups)
		assert.Nil(t, envelope.Data.SelectedGroupID)
		assert.Empty(t, envelope.Data.Visits)
		require.NotNil(t, envelope.Data.CurrentStaffID)
	})
}

func TestSupervisionDashboard_TenantIsolation(t *testing.T) {
	tc, router := setupDashboardContext(t)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "DashIso", "Leader")
	room := testpkg.CreateTestRoom(t, tc.db, "DashIsoRoom")
	activityGroup := testpkg.CreateTestActivityGroup(t, tc.db, "DashIsoActivity")
	activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activityGroup.ID, room.ID)
	testpkg.CreateTestGroupSupervisor(t, tc.db, teacher.Staff.ID, activeGroup.ID, "supervisor")

	testpkg.EnsureTestTenant(t, tc.db, 2)
	foreignRoom := testpkg.CreateTestRoomForTenant(t, tc.db, 2, "DashIsoForeignRoom")
	foreignActivityGroup := testpkg.CreateTestActivityGroupForTenant(t, tc.db, 2, "DashIsoForeignActivity")
	foreignActiveGroup := testpkg.CreateTestActiveGroupWithIDsForTenant(t, tc.db, 2, foreignActivityGroup.ID, foreignRoom.ID)

	defer testpkg.CleanupActivityFixtures(t, tc.db,
		activeGroup.ID, activityGroup.ID, room.ID, teacher.ID,
		foreignActiveGroup.ID, foreignActivityGroup.ID, foreignRoom.ID)

	rr := dashboardExecRaw(t, router, fmt.Sprintf("/active/supervision-dashboard?group_id=%d", foreignActiveGroup.ID), account.ID, dashboardPerms)
	assert.Equal(t, http.StatusForbidden, rr.Code,
		"a tenant-1 supervisor must never resolve a tenant-2 active group through the aggregate")
}
