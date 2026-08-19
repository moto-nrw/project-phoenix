// Package checkin_test holds the DB-backed and stub-based unit tests for the
// extracted CheckinService (issue #575 B8). These live in the EXTERNAL
// checkin_test package (not package checkin) because they import the services
// factory via api/testutil.SetupAPITest — importing services from an internal
// checkin test would recreate the services ↔ services/iot/checkin cycle.
package checkin_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	checkin "github.com/moto-nrw/project-phoenix/services/iot/checkin"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// svcTestContext holds shared dependencies for DB-backed CheckinService tests.
type svcTestContext struct {
	svc *checkin.CheckinService
	db  *bun.DB
}

// setupCheckinServiceTest wires a real CheckinService (via the services factory)
// against the test database. It replaces the former setupInternalTestResource
// helper that built an api/iot/checkin Resource.
func setupCheckinServiceTest(t *testing.T) *svcTestContext {
	t.Helper()

	db, svc := testutil.SetupAPITest(t)

	// Belt-and-suspenders: ensure the FK target row for tenant_id=1 exists on
	// the exact connection pool used by the services.
	testpkg.EnsureTestTenant(t, db, 1)

	return &svcTestContext{svc: svc.Checkin, db: db}
}

// createTestActiveGroupWithDevice creates an active group linked to a device.
func createTestActiveGroupWithDevice(t *testing.T, db *bun.DB, activityGroupID, roomID, deviceID int64) *active.Group {
	t.Helper()

	now := time.Now()
	group := &active.Group{
		Model:          base.Model{ID: 0},
		GroupID:        &activityGroupID,
		RoomID:         roomID,
		DeviceID:       &deviceID,
		StartTime:      now,
		LastActivity:   now,
		TimeoutMinutes: 30,
	}
	group.SetTenantID(1)

	_, err := db.NewInsert().
		Model(group).
		ModelTableExpr(`active.groups AS "group"`).
		Exec(testpkg.TenantContext(1))
	require.NoError(t, err, "Failed to create test active group with device")

	return group
}

// =============================================================================
// GetDeviceActiveGroupInRoom Tests (DB-backed)
// =============================================================================

func TestGetDeviceActiveGroupInRoom_ReturnsMatchingGroup(t *testing.T) {
	tc := setupCheckinServiceTest(t)

	ctx := testpkg.TenantContext(1)

	activity := testpkg.CreateTestActivityGroup(t, tc.db, "device-group-match")
	room := testpkg.CreateTestRoom(t, tc.db, "DeviceGroupMatchRoom")
	device := testpkg.CreateTestDevice(t, tc.db, "dev-match-001")
	activeGroup := createTestActiveGroupWithDevice(t, tc.db, activity.ID, room.ID, device.ID)

	result := tc.svc.GetDeviceActiveGroupInRoom(ctx, room.ID, device.ID)

	require.NotNil(t, result, "Should find the active group for the device")
	assert.Equal(t, activeGroup.ID, result.ID)
}

func TestGetDeviceActiveGroupInRoom_NoMatchingDevice(t *testing.T) {
	tc := setupCheckinServiceTest(t)

	ctx := testpkg.TenantContext(1)

	activity := testpkg.CreateTestActivityGroup(t, tc.db, "device-group-nomatch")
	room := testpkg.CreateTestRoom(t, tc.db, "DeviceGroupNoMatchRoom")
	device := testpkg.CreateTestDevice(t, tc.db, "dev-nomatch-001")
	_ = createTestActiveGroupWithDevice(t, tc.db, activity.ID, room.ID, device.ID)

	// Use a different device ID
	result := tc.svc.GetDeviceActiveGroupInRoom(ctx, room.ID, 999999)

	assert.Nil(t, result, "Should return nil when no group matches the device")
}

func TestGetDeviceActiveGroupInRoom_NoGroupsInRoom(t *testing.T) {
	tc := setupCheckinServiceTest(t)

	ctx := context.Background()

	result := tc.svc.GetDeviceActiveGroupInRoom(ctx, 999999, 1)

	assert.Nil(t, result, "Should return nil when no groups exist in the room")
}

// =============================================================================
// GetActiveStudentCountForRoom Tests (DB-backed)
// =============================================================================

func TestGetActiveStudentCountForRoom_ReturnsCount(t *testing.T) {
	tc := setupCheckinServiceTest(t)

	ctx := context.Background()

	activity := testpkg.CreateTestActivityGroup(t, tc.db, "count-students")
	room := testpkg.CreateTestRoom(t, tc.db, "CountStudentsRoom")
	activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activity.ID, room.ID)
	student1 := testpkg.CreateTestStudent(t, tc.db, "Count1", "Student", "1a")
	student2 := testpkg.CreateTestStudent(t, tc.db, "Count2", "Student", "1a")
	_ = testpkg.CreateTestVisit(t, tc.db, student1.ID, activeGroup.ID, time.Now(), nil)
	_ = testpkg.CreateTestVisit(t, tc.db, student2.ID, activeGroup.ID, time.Now(), nil)

	result := tc.svc.GetActiveStudentCountForRoom(ctx, room.ID)

	require.NotNil(t, result, "Should return a count")
	assert.Equal(t, 2, *result)
}

func TestGetActiveStudentCountForRoom_EmptyRoom(t *testing.T) {
	tc := setupCheckinServiceTest(t)

	ctx := context.Background()

	room := testpkg.CreateTestRoom(t, tc.db, "EmptyCountRoom")

	result := tc.svc.GetActiveStudentCountForRoom(ctx, room.ID)

	require.NotNil(t, result, "Should return a count even for empty rooms")
	assert.Equal(t, 0, *result)
}

// =============================================================================
// UpdateSessionActivity Tests (DB-backed)
// =============================================================================

func TestUpdateSessionActivity_Success(t *testing.T) {
	tc := setupCheckinServiceTest(t)

	ctx := context.Background()

	activity := testpkg.CreateTestActivityGroup(t, tc.db, "session-activity")
	room := testpkg.CreateTestRoom(t, tc.db, "SessionActivityRoom")
	activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activity.ID, room.ID)

	// Should not panic or error
	tc.svc.UpdateSessionActivity(ctx, activeGroup.ID)
}

func TestUpdateSessionActivity_NonExistentGroup(t *testing.T) {
	tc := setupCheckinServiceTest(t)

	ctx := context.Background()

	// Should log warning but not panic
	tc.svc.UpdateSessionActivity(ctx, 999999)
}

// =============================================================================
// CountActiveGroupOccupancy Tests (DB-backed)
// =============================================================================

func TestCountActiveGroupOccupancy_WithActiveVisits(t *testing.T) {
	tc := setupCheckinServiceTest(t)

	ctx := context.Background()

	activity := testpkg.CreateTestActivityGroup(t, tc.db, "occ-count")
	room := testpkg.CreateTestRoom(t, tc.db, "OccCountRoom")
	activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activity.ID, room.ID)
	student := testpkg.CreateTestStudent(t, tc.db, "Occ", "Student", "1a")
	_ = testpkg.CreateTestVisit(t, tc.db, student.ID, activeGroup.ID, time.Now(), nil)

	count, err := tc.svc.CountActiveGroupOccupancyForTest(ctx, activeGroup.ID)

	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestCountActiveGroupOccupancy_EmptyGroup(t *testing.T) {
	tc := setupCheckinServiceTest(t)

	ctx := context.Background()

	activity := testpkg.CreateTestActivityGroup(t, tc.db, "occ-empty")
	room := testpkg.CreateTestRoom(t, tc.db, "OccEmptyRoom")
	activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activity.ID, room.ID)

	count, err := tc.svc.CountActiveGroupOccupancyForTest(ctx, activeGroup.ID)

	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestCountActiveGroupOccupancy_ExcludesExitedVisits(t *testing.T) {
	tc := setupCheckinServiceTest(t)

	ctx := context.Background()

	activity := testpkg.CreateTestActivityGroup(t, tc.db, "occ-exited")
	room := testpkg.CreateTestRoom(t, tc.db, "OccExitedRoom")
	activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activity.ID, room.ID)
	student1 := testpkg.CreateTestStudent(t, tc.db, "Active", "Student", "1a")
	student2 := testpkg.CreateTestStudent(t, tc.db, "Exited", "Student", "1a")
	entryTime := time.Now().Add(-10 * time.Minute)
	exitTime := time.Now()
	_ = testpkg.CreateTestVisit(t, tc.db, student1.ID, activeGroup.ID, time.Now(), nil)      // active
	_ = testpkg.CreateTestVisit(t, tc.db, student2.ID, activeGroup.ID, entryTime, &exitTime) // exited

	count, err := tc.svc.CountActiveGroupOccupancyForTest(ctx, activeGroup.ID)

	require.NoError(t, err)
	assert.Equal(t, 1, count, "Should only count active visits (exit_time IS NULL)")
}

// =============================================================================
// LoadCurrentVisitWithRoom Tests (DB-backed)
// =============================================================================

func TestLoadCurrentVisitWithRoom_NoVisit(t *testing.T) {
	tc := setupCheckinServiceTest(t)

	ctx := context.Background()

	// Non-existent student
	result := tc.svc.LoadCurrentVisitWithRoom(ctx, 999999)
	assert.Nil(t, result, "Should return nil for a student with no current visit")
}

func TestLoadCurrentVisitWithRoom_WithVisit(t *testing.T) {
	tc := setupCheckinServiceTest(t)

	ctx := context.Background()

	activity := testpkg.CreateTestActivityGroup(t, tc.db, "load-visit-room")
	room := testpkg.CreateTestRoom(t, tc.db, "LoadVisitRoom")
	activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activity.ID, room.ID)
	student := testpkg.CreateTestStudent(t, tc.db, "Load", "Visit", "1a")
	visit := testpkg.CreateTestVisit(t, tc.db, student.ID, activeGroup.ID, time.Now(), nil)

	result := tc.svc.LoadCurrentVisitWithRoom(ctx, student.ID)

	require.NotNil(t, result, "Should return the current visit")
	assert.Equal(t, visit.ID, result.ID)
	require.NotNil(t, result.ActiveGroup, "Should have ActiveGroup loaded")
	require.NotNil(t, result.ActiveGroup.Room, "Should have Room loaded on ActiveGroup")
	assert.Contains(t, result.ActiveGroup.Room.Name, "LoadVisitRoom")
}

// =============================================================================
// RoomNameByID / RoomNameForResponse Tests (DB fallback paths)
// =============================================================================

func TestRoomNameByID_FallbackToLookup(t *testing.T) {
	tc := setupCheckinServiceTest(t)

	room := testpkg.CreateTestRoom(t, tc.db, "LookupRoom")

	// Pass nil room to force lookup by ID
	name := tc.svc.RoomNameByIDForTest(context.Background(), nil, room.ID)
	assert.Contains(t, name, "LookupRoom")
}

func TestRoomNameByID_FallbackToFormattedID(t *testing.T) {
	tc := setupCheckinServiceTest(t)

	// Use a non-existent room ID
	name := tc.svc.RoomNameByIDForTest(context.Background(), nil, 999999)
	assert.Equal(t, "Room 999999", name)
}

func TestRoomNameForResponse_WithRoomID_NoVisit(t *testing.T) {
	tc := setupCheckinServiceTest(t)

	room := testpkg.CreateTestRoom(t, tc.db, "ResponseRoom")

	roomID := room.ID
	name := tc.svc.RoomNameForResponseForTest(context.Background(), nil, &roomID)
	assert.Contains(t, name, "ResponseRoom")
}

func TestRoomNameForResponse_VisitWithoutRoom_FallbackToRoomID(t *testing.T) {
	tc := setupCheckinServiceTest(t)

	room := testpkg.CreateTestRoom(t, tc.db, "FallbackRoom")

	// Visit without ActiveGroup.Room loaded
	visit := &active.Visit{ActiveGroup: &active.Group{}}
	roomID := room.ID
	name := tc.svc.RoomNameForResponseForTest(context.Background(), visit, &roomID)
	assert.Contains(t, name, "FallbackRoom")
}

// TestResolveStudentFromPerson_RejectsAlumnus covers the #405 P1 primary
// check-in guard: a graduated (alumnus) student resolves to nil so the kiosk
// treats the scan like an unknown tag instead of opening a room visit or an
// attendance row.
func TestResolveStudentFromPerson_RejectsAlumnus(t *testing.T) {
	tc := setupCheckinServiceTest(t)
	ctx := testpkg.TenantContext(1)

	activeStudent := testpkg.CreateTestStudent(t, tc.db, "Primary", "Active", "1a")
	alumStudent := testpkg.CreateTestStudent(t, tc.db, "Primary", "Alumnus", "4a")

	_, err := tc.db.NewUpdate().
		TableExpr("users.students").
		Set("status = ?", "alumnus").
		Where("id = ?", alumStudent.ID).
		Exec(ctx)
	require.NoError(t, err)

	// An active student resolves normally.
	got, err := tc.svc.ResolveStudentFromPerson(ctx, activeStudent.PersonID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, activeStudent.ID, got.ID)

	// A graduated alumnus resolves to nil (treated as "no active student").
	gotAlum, err := tc.svc.ResolveStudentFromPerson(ctx, alumStudent.PersonID)
	require.NoError(t, err)
	assert.Nil(t, gotAlum)
}

// TestProcessStudentCheckin_RoomRejectsGraduatedRace covers the #405 fix for the
// detailed room check-in: when graduation commits AFTER the student was resolved
// as active but BEFORE the visit is created, CreateVisit returns
// ErrStudentGraduated. The service must surface that as the same 404
// "person is not a student" an unknown/absent student gets — not the generic
// 500 "failed to create visit record" that tells PyrePortal to retry.
func TestProcessStudentCheckin_RoomRejectsGraduatedRace(t *testing.T) {
	tc := setupCheckinServiceTest(t)
	ctx := testpkg.TenantContext(1)

	activity := testpkg.CreateTestActivityGroup(t, tc.db, "grad-race-group")
	room := testpkg.CreateTestRoom(t, tc.db, "GradRaceRoom")
	device := testpkg.CreateTestDevice(t, tc.db, "grad-race-dev-001")
	_ = createTestActiveGroupWithDevice(t, tc.db, activity.ID, room.ID, device.ID)
	student := testpkg.CreateTestStudent(t, tc.db, "Race", "Graduate", "4a")

	// Simulate the race: student resolved as active, then a concurrent
	// grade-transition apply graduated them before the visit write.
	_, err := tc.db.NewUpdate().
		TableExpr("users.students").
		Set("status = ?", "alumnus").
		Where("id = ?", student.ID).
		Exec(ctx)
	require.NoError(t, err)

	person := &users.Person{FirstName: "Race", LastName: "Graduate"}
	person.ID = student.PersonID

	_, err = tc.svc.ProcessStudentCheckin(ctx, student, person, &checkin.CheckinProcessingInput{
		RoomID:   &room.ID,
		DeviceID: device.ID,
	})
	require.Error(t, err)

	var ce *checkin.CheckinError
	require.ErrorAs(t, err, &ce, "graduated race must surface a classified CheckinError, not a raw active error")
	assert.Equal(t, "person is not a student", ce.Error(),
		"detailed room check-in must map the graduated sentinel to the 404 unknown-student wire error, not a 500")
}
