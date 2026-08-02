// Substitution SSE emission tests (#2084).
//
// A Vertretung decides which groups a staff member may open in "Meine Gruppe".
// The substitute's own client never makes the write, and no other event covers
// it, so every write path must announce substitution_changed to the tenant or
// an already-open tab keeps the stale group list until a manual reload.
package substitutions_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/realtime"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// setupRecordingContext builds the production resource wired to a recording
// broadcaster. Router() captures the resource pointer, so assigning the
// broadcaster after the mount is picked up by every handler call.
func setupRecordingContext(t *testing.T) (*testContext, *testpkg.RecordingBroadcaster) {
	t.Helper()

	ctx := setupTestContext(t)
	broadcaster := testpkg.NewRecordingBroadcaster()
	ctx.resource.Broadcaster = broadcaster

	return ctx, broadcaster
}

// assertSubstitutionChanged asserts that exactly one tenant-wide
// substitution_changed event was emitted, carrying the expected source and no
// staff identity (the event reaches every staff client of the tenant).
func assertSubstitutionChanged(t *testing.T, broadcaster *testpkg.RecordingBroadcaster, source string) {
	t.Helper()

	events := broadcaster.EventsOfType(realtime.EventSubstitutionChanged)
	require.Len(t, events, 1, "expected exactly one substitution_changed event")

	require.NotNil(t, events[0].Data.Source)
	assert.Equal(t, source, *events[0].Data.Source)
	assert.Nil(t, events[0].Data.StudentID, "substitution_changed must not carry student identity")
	assert.Empty(t, events[0].ActiveGroupID, "substitution_changed is tenant-wide, not group-scoped")

	// Routed to the caller's tenant, never broadcast school-wide across tenants.
	calls := broadcaster.CallsByMethod("tenant")
	require.NotEmpty(t, calls)
	assert.Equal(t, int64(testutil.DefaultTestClaims().TenantID), calls[0].TenantID)
}

func TestCreateSubstitution_BroadcastsSubstitutionChanged(t *testing.T) {
	ctx, broadcaster := setupRecordingContext(t)

	staff := testpkg.CreateTestStaff(t, ctx.db, "SSECreate", "Substitute")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, staff.ID)

	group := testpkg.CreateTestEducationGroup(t, ctx.db, "SSESubstitutionCreate")
	defer testpkg.CleanupTableRecords(t, ctx.db, "education.groups", group.ID)

	body := map[string]interface{}{
		"group_id":            group.ID,
		"substitute_staff_id": staff.ID,
		"start_date":          timezone.TodayDate().AddDays(1).String(),
		"end_date":            timezone.TodayDate().AddDays(7).String(),
		"reason":              "SSE test",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/substitutions", body,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, testutil.DefaultTestClaims())),
	)

	rr := testutil.ExecuteRequest(ctx.router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	require.True(t, ok)
	if id, ok := data["id"].(float64); ok {
		defer cleanupSubstitution(t, ctx.db, int64(id))
	}

	assertSubstitutionChanged(t, broadcaster, "substitution_create")
}

func TestUpdateSubstitution_BroadcastsSubstitutionChanged(t *testing.T) {
	ctx, broadcaster := setupRecordingContext(t)

	staff := testpkg.CreateTestStaff(t, ctx.db, "SSEUpdate", "Substitute")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, staff.ID)

	group := testpkg.CreateTestEducationGroup(t, ctx.db, "SSESubstitutionUpdate")
	defer testpkg.CleanupTableRecords(t, ctx.db, "education.groups", group.ID)

	today := timezone.TodayDate()
	substitution := testpkg.CreateTestGroupSubstitution(t, ctx.db, group.ID, nil, staff.ID, today, today.AddDays(3))
	defer cleanupSubstitution(t, ctx.db, substitution.ID)

	body := map[string]interface{}{
		"group_id":            group.ID,
		"substitute_staff_id": staff.ID,
		"start_date":          today.AddDays(1).String(),
		"end_date":            today.AddDays(5).String(),
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/substitutions/%d", substitution.ID), body,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, testutil.DefaultTestClaims())),
	)

	rr := testutil.ExecuteRequest(ctx.router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	assertSubstitutionChanged(t, broadcaster, "substitution_update")
}

func TestDeleteSubstitution_BroadcastsSubstitutionChanged(t *testing.T) {
	ctx, broadcaster := setupRecordingContext(t)

	staff := testpkg.CreateTestStaff(t, ctx.db, "SSEDelete", "Substitute")
	defer testpkg.CleanupActivityFixtures(t, ctx.db, staff.ID)

	group := testpkg.CreateTestEducationGroup(t, ctx.db, "SSESubstitutionDelete")
	defer testpkg.CleanupTableRecords(t, ctx.db, "education.groups", group.ID)

	today := timezone.TodayDate()
	substitution := testpkg.CreateTestGroupSubstitution(t, ctx.db, group.ID, nil, staff.ID, today, today.AddDays(3))
	defer cleanupSubstitution(t, ctx.db, substitution.ID)

	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/substitutions/%d", substitution.ID), nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, testutil.DefaultTestClaims())),
	)

	rr := testutil.ExecuteRequest(ctx.router, req)
	require.Equal(t, http.StatusNoContent, rr.Code)

	assertSubstitutionChanged(t, broadcaster, "substitution_delete")
}

// A rejected write must stay silent: a client that refetches on a phantom
// event pays a full round trip for a change that never happened.
func TestCreateSubstitution_Rejected_BroadcastsNothing(t *testing.T) {
	ctx, broadcaster := setupRecordingContext(t)

	// Missing group_id — rejected before any write.
	body := map[string]interface{}{
		"substitute_staff_id": 1,
		"start_date":          timezone.TodayDate().AddDays(1).String(),
		"end_date":            timezone.TodayDate().AddDays(7).String(),
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/substitutions", body,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, testutil.DefaultTestClaims())),
	)

	rr := testutil.ExecuteRequest(ctx.router, req)
	testutil.AssertBadRequest(t, rr)

	assert.False(t, broadcaster.HasEventType(realtime.EventSubstitutionChanged))
}
