package students_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// ogsLivePerms is the permission set the aggregated endpoint gates on — the
// union of what its constituent single endpoints required.
var ogsLivePerms = []string{"users:read", "groups:read"}

type ogsLiveEnvelope struct {
	Data struct {
		Groups []struct {
			ID              int64  `json:"id"`
			Name            string `json:"name"`
			RoomID          *int64 `json:"room_id"`
			RoomName        string `json:"room_name"`
			ViaSubstitution bool   `json:"via_substitution"`
		} `json:"groups"`
		GroupID    *int64           `json:"group_id"`
		Students   []map[string]any `json:"students"`
		RoomStatus map[string]struct {
			InGroupRoom   bool   `json:"in_group_room"`
			CurrentRoomID *int64 `json:"current_room_id"`
		} `json:"room_status"`
		PickupTimes        []map[string]any `json:"pickup_times"`
		TrackingIndicators struct {
			Labels  []string          `json:"labels"`
			Results map[string][]bool `json:"results"`
		} `json:"tracking_indicators"`
		Transfers []struct {
			ID                int64  `json:"id"`
			GroupID           int64  `json:"group_id"`
			SubstituteStaffID int64  `json:"substitute_staff_id"`
			SubstituteName    string `json:"substitute_name"`
			EndDate           string `json:"end_date"`
		} `json:"transfers"`
	} `json:"data"`
}

func decodeOGSLive(t *testing.T, body []byte) *ogsLiveEnvelope {
	t.Helper()
	var envelope ogsLiveEnvelope
	require.NoError(t, json.Unmarshal(body, &envelope))
	return &envelope
}

func assignRoomToEducationGroup(t *testing.T, db *bun.DB, groupID, roomID int64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.NewUpdate().
		Table("education.groups").
		Set("room_id = ?", roomID).
		Where("id = ?", groupID).
		Exec(ctx)
	require.NoError(t, err, "failed to assign room to education group")
}

func TestOGSGroupLive_AggregatesGroupData(t *testing.T) {
	tc := setupTestContext(t)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "OGSLive", "Leader")
	group := testpkg.CreateTestEducationGroup(t, tc.db, "OGSLiveGroupA")
	room := testpkg.CreateTestRoom(t, tc.db, "OGSLiveRoom")
	assignRoomToEducationGroup(t, tc.db, group.ID, room.ID)

	inRoom := testpkg.CreateTestStudent(t, tc.db, "OGSLive", "Drinnen", "OL1")
	atHome := testpkg.CreateTestStudent(t, tc.db, "OGSLive", "Zuhause", "OL1")
	defer testpkg.CleanupActivityFixtures(t, tc.db, inRoom.ID, atHome.ID, group.ID, room.ID, teacher.ID)

	testpkg.AssignStudentToGroup(t, tc.db, inRoom.ID, group.ID)
	testpkg.AssignStudentToGroup(t, tc.db, atHome.ID, group.ID)
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)

	// Check the first student into an active group running in the group's own
	// room: attendance row (checked in) + open visit.
	device := testpkg.CreateTestDevice(t, tc.db, "OGSLiveDevice")
	activityGroup := testpkg.CreateTestActivityGroup(t, tc.db, "OGSLiveActivity")
	activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activityGroup.ID, room.ID)
	testpkg.CreateTestAttendance(t, tc.db, inRoom.ID, teacher.Staff.ID, device.ID, time.Now().Add(-1*time.Hour), nil)
	testpkg.CreateTestVisit(t, tc.db, inRoom.ID, activeGroup.ID, time.Now().Add(-1*time.Hour), nil)
	defer testpkg.CleanupActivityFixtures(t, tc.db, activeGroup.ID, activityGroup.ID, device.ID)

	// Same-day transfer: substitution with unassigned regular staff slot.
	substitute := testpkg.CreateTestStaff(t, tc.db, "OGSLive", "Vertretung")
	today := timezone.TodayDate()
	sub := testpkg.CreateTestGroupSubstitution(t, tc.db, group.ID, nil, substitute.ID, today, today)
	defer testpkg.CleanupActivityFixtures(t, tc.db, sub.ID, substitute.ID)

	// Tracking indicators: enable the feature with one label so the aggregate
	// exercises the settings-gated ActiveService path.
	settingsCtx := testpkg.TenantContext(1)
	require.NoError(t, tc.services.Settings.SetValue(settingsCtx, configModel.KeyTrackingIndicatorsEnabled, true, nil, nil))
	require.NoError(t, tc.services.Settings.SetValue(settingsCtx, configModel.KeyTrackingIndicator1, "Hausaufgaben", nil, nil))
	t.Cleanup(func() {
		_ = tc.services.Settings.ResetValue(settingsCtx, configModel.KeyTrackingIndicatorsEnabled, nil, nil)
		_ = tc.services.Settings.ResetValue(settingsCtx, configModel.KeyTrackingIndicator1, nil, nil)
	})

	req := testutil.NewRequest("GET", "/ogs-group-live", nil)
	rr := authExec(t, tc, req, testutil.TeacherTestClaims(int(account.ID)), ogsLivePerms)
	require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())

	envelope := decodeOGSLive(t, rr.Body.Bytes())
	data := envelope.Data

	require.NotNil(t, data.GroupID)
	assert.Equal(t, group.ID, *data.GroupID)
	require.Len(t, data.Groups, 1)
	assert.Equal(t, group.Name, data.Groups[0].Name)
	require.NotNil(t, data.Groups[0].RoomID)
	assert.Equal(t, room.ID, *data.Groups[0].RoomID)
	assert.Equal(t, room.Name, data.Groups[0].RoomName)
	assert.False(t, data.Groups[0].ViaSubstitution)

	require.Len(t, data.Students, 2)

	// Room status: the checked-in student is in the group room; the absent one
	// carries no current room.
	inRoomStatus, ok := data.RoomStatus[fmt.Sprintf("%d", inRoom.ID)]
	require.True(t, ok)
	assert.True(t, inRoomStatus.InGroupRoom)
	require.NotNil(t, inRoomStatus.CurrentRoomID)
	assert.Equal(t, room.ID, *inRoomStatus.CurrentRoomID)

	atHomeStatus, ok := data.RoomStatus[fmt.Sprintf("%d", atHome.ID)]
	require.True(t, ok)
	assert.False(t, atHomeStatus.InGroupRoom)
	assert.Nil(t, atHomeStatus.CurrentRoomID)

	// Transfer projection carries the substitute's name and end date.
	require.Len(t, data.Transfers, 1)
	assert.Equal(t, group.ID, data.Transfers[0].GroupID)
	assert.Equal(t, substitute.ID, data.Transfers[0].SubstituteStaffID)
	assert.NotEmpty(t, data.Transfers[0].SubstituteName)
	assert.Equal(t, today.String(), data.Transfers[0].EndDate)

	// Tracking indicators: enabled with one configured label; every student
	// gets a result row.
	assert.Equal(t, []string{"Hausaufgaben"}, data.TrackingIndicators.Labels)
}

// TestOGSGroupLive_MinimalProjection is the payload guard for #2056: the
// production student list averaged ~7.4x staging size (up to ~198 KB) because
// it ships the full student projection. The aggregate must never carry the
// wide personal fields the OGS page does not render.
func TestOGSGroupLive_MinimalProjection(t *testing.T) {
	tc := setupTestContext(t)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "OGSSlim", "Leader")
	group := testpkg.CreateTestEducationGroup(t, tc.db, "OGSSlimGroup")
	student := testpkg.CreateTestStudent(t, tc.db, "OGSSlim", "Kind", "OS1")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID, group.ID, teacher.ID)

	testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)

	req := testutil.NewRequest("GET", fmt.Sprintf("/ogs-group-live?group_id=%d", group.ID), nil)
	rr := authExec(t, tc, req, testutil.TeacherTestClaims(int(account.ID)), ogsLivePerms)
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
			"aggregated OGS response must not ship unrendered personal/internal field %q", forbidden)
	}

	// Fields the page renders must be present on the student projection.
	envelope := decodeOGSLive(t, rr.Body.Bytes())
	require.Len(t, envelope.Data.Students, 1)
	studentJSON := envelope.Data.Students[0]
	for _, required := range []string{"id", "first_name", "last_name", "school_class", "current_location", "sick", "excused", "class_trip"} {
		assert.Contains(t, studentJSON, required, "student projection must include %q", required)
	}
}

// queryCounter counts every SQL statement issued through the hooked *bun.DB,
// including statements inside tenant transactions.
type queryCounter struct {
	mu      sync.Mutex
	queries []string
}

func (h *queryCounter) BeforeQuery(ctx context.Context, event *bun.QueryEvent) context.Context {
	h.mu.Lock()
	h.queries = append(h.queries, event.Query)
	h.mu.Unlock()
	return ctx
}

func (h *queryCounter) AfterQuery(context.Context, *bun.QueryEvent) {}

func (h *queryCounter) reset() {
	h.mu.Lock()
	h.queries = nil
	h.mu.Unlock()
}

func (h *queryCounter) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.queries)
}

// TestOGSGroupLive_QueryBudget guards the aggregate against per-student N+1
// regressions: the query count must not grow with group size, and the total
// per request stays under a fixed budget.
func TestOGSGroupLive_QueryBudget(t *testing.T) {
	tc := setupTestContext(t)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "OGSBudget", "Leader")
	group := testpkg.CreateTestEducationGroup(t, tc.db, "OGSBudgetGroup")
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)

	var studentIDs []int64
	addStudents := func(n int) {
		for i := range n {
			student := testpkg.CreateTestStudent(t, tc.db, "OGSBudget", fmt.Sprintf("Kind%d", len(studentIDs)+i), "OB1")
			testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)
			studentIDs = append(studentIDs, student.ID)
		}
	}
	// Students before groups: a group still referenced by a student cannot go.
	defer func() {
		testpkg.CleanupActivityFixtures(t, tc.db, append(studentIDs, group.ID, teacher.ID)...)
	}()

	counter := &queryCounter{}
	tc.db.AddQueryHook(counter)

	run := func() int {
		counter.reset()
		req := testutil.NewRequest("GET", fmt.Sprintf("/ogs-group-live?group_id=%d", group.ID), nil)
		rr := authExec(t, tc, req, testutil.TeacherTestClaims(int(account.ID)), ogsLivePerms)
		require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())
		return counter.count()
	}

	addStudents(3)
	smallCount := run()

	addStudents(7)
	largeCount := run()

	t.Logf("query budget: 3 students → %d queries, 10 students → %d queries", smallCount, largeCount)

	assert.Equal(t, smallCount, largeCount,
		"query count must be independent of group size (no per-student N+1)")

	// Fixed budget: identity resolution (~6), group list + room (~4), students +
	// snapshot (~6), status days + day planning (~6), transfers + settings (~4),
	// tenant tx overhead (~6). Raise only with a written justification.
	const maxQueries = 40
	assert.LessOrEqual(t, largeCount, maxQueries,
		"aggregated OGS request exceeded its query budget")
}

// TestOGSGroupLive_PayloadBudget bounds the wire size for a production-sized
// group: the projection must stay far below the ~198 KB full student list
// observed in production (#2056).
func TestOGSGroupLive_PayloadBudget(t *testing.T) {
	tc := setupTestContext(t)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "OGSPayload", "Leader")
	group := testpkg.CreateTestEducationGroup(t, tc.db, "OGSPayloadGroup")
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)

	var studentIDs []int64
	const groupSize = 30
	for i := range groupSize {
		student := testpkg.CreateTestStudent(t, tc.db, "OGSPayload", fmt.Sprintf("Produktionskind%02d", i), "OP1")
		testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)
		studentIDs = append(studentIDs, student.ID)
	}
	// Students before groups: a group still referenced by a student cannot go.
	defer func() {
		testpkg.CleanupActivityFixtures(t, tc.db, append(studentIDs, group.ID, teacher.ID)...)
	}()

	req := testutil.NewRequest("GET", fmt.Sprintf("/ogs-group-live?group_id=%d", group.ID), nil)
	rr := authExec(t, tc, req, testutil.TeacherTestClaims(int(account.ID)), ogsLivePerms)
	require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())

	size := rr.Body.Len()
	t.Logf("payload budget: %d students → %d bytes (%.0f bytes/student)", groupSize, size, float64(size)/groupSize)

	// Budget: ≤ 600 bytes per student for the projection (name, location,
	// status flags, planning fields, room status) plus 2 KB envelope/group
	// overhead. A production group of 30 must stay under ~20 KB — roughly a
	// tenth of the observed full-projection payloads.
	const maxBytes = 2048 + groupSize*600
	assert.LessOrEqual(t, size, maxBytes, "aggregated OGS payload exceeded its budget")
}

func TestOGSGroupLive_ErrorContract(t *testing.T) {
	tc := setupTestContext(t)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "OGSErr", "Leader")
	group := testpkg.CreateTestEducationGroup(t, tc.db, "OGSErrGroup")
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)

	otherTeacher, _ := testpkg.CreateTestTeacherWithAccount(t, tc.db, "OGSErr", "Other")
	foreignGroup := testpkg.CreateTestEducationGroup(t, tc.db, "OGSErrForeign")
	testpkg.CreateTestGroupTeacher(t, tc.db, foreignGroup.ID, otherTeacher.ID)

	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID, foreignGroup.ID, teacher.ID, otherTeacher.ID)

	t.Run("invalid group_id is a 400", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/ogs-group-live?group_id=abc", nil)
		rr := authExec(t, tc, req, testutil.TeacherTestClaims(int(account.ID)), ogsLivePerms)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("unsupervised group is a 403", func(t *testing.T) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/ogs-group-live?group_id=%d", foreignGroup.ID), nil)
		rr := authExec(t, tc, req, testutil.TeacherTestClaims(int(account.ID)), ogsLivePerms)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("missing groups:read keeps the roster but redacts group-derived sections", func(t *testing.T) {
		// Parity with the replaced single endpoints: the roster needed only
		// users:read, while room status, transfers, and tracking indicators
		// were gated on groups:read. Without groups:read those sections are
		// deterministically empty — never a 403 for the whole roster, never
		// silently-degraded data for callers who do hold the permission.
		req := testutil.NewRequest("GET", "/ogs-group-live", nil)
		rr := authExec(t, tc, req, testutil.TeacherTestClaims(int(account.ID)), []string{"users:read"})
		require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())

		envelope := decodeOGSLive(t, rr.Body.Bytes())
		assert.NotEmpty(t, envelope.Data.Groups)
		assert.Empty(t, envelope.Data.RoomStatus)
		assert.Empty(t, envelope.Data.Transfers)
		assert.Empty(t, envelope.Data.TrackingIndicators.Labels)
	})

	t.Run("no supervised groups is an explicit empty 200", func(t *testing.T) {
		lonely, lonelyAccount := testpkg.CreateTestTeacherWithAccount(t, tc.db, "OGSErr", "Gruppenlos")
		defer testpkg.CleanupActivityFixtures(t, tc.db, lonely.ID)

		req := testutil.NewRequest("GET", "/ogs-group-live", nil)
		rr := authExec(t, tc, req, testutil.TeacherTestClaims(int(lonelyAccount.ID)), ogsLivePerms)
		require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())

		envelope := decodeOGSLive(t, rr.Body.Bytes())
		assert.Empty(t, envelope.Data.Groups)
		assert.Nil(t, envelope.Data.GroupID)
		assert.Empty(t, envelope.Data.Students)
	})
}

func TestOGSGroupLive_TenantIsolation(t *testing.T) {
	tc := setupTestContext(t)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "OGSIso", "Leader")
	ownGroup := testpkg.CreateTestEducationGroup(t, tc.db, "OGSIsoOwn")
	testpkg.CreateTestGroupTeacher(t, tc.db, ownGroup.ID, teacher.ID)

	testpkg.EnsureTestTenant(t, tc.db, 2)
	otherTenantGroup := testpkg.CreateTestEducationGroupForTenant(t, tc.db, 2, "OGSIsoForeignTenant")
	defer testpkg.CleanupActivityFixtures(t, tc.db, ownGroup.ID, otherTenantGroup.ID, teacher.ID)

	req := testutil.NewRequest("GET", fmt.Sprintf("/ogs-group-live?group_id=%d", otherTenantGroup.ID), nil)
	rr := authExec(t, tc, req, testutil.TeacherTestClaims(int(account.ID)), ogsLivePerms)
	assert.Equal(t, http.StatusForbidden, rr.Code,
		"a tenant-1 supervisor must never resolve a tenant-2 group through the aggregate")
}
