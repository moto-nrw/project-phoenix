// group_access_changed emission tests for the Vertretungsplan (#2084).
//
// A Vertretung decides which groups a staff member may open in "Meine Gruppe".
// The substitute's own client never makes the write and no other event covers
// it, so every write path must announce group_access_changed to the tenant or
// an already-open tab keeps the stale group list until a manual reload.
//
// Driven through the production router even though the emit lives in
// services/education: the subtle part is that the broadcast fires only after
// the request's tenant transaction COMMITS, which only the full chain proves.
package substitutions_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/realtime"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// setupRecordingContext builds the production wiring with a recording
// broadcaster attached to the education service, mirroring the duck-typed
// SetBroadcaster block in services/factory.go.
func setupRecordingContext(t *testing.T) (*testContext, *testpkg.RecordingBroadcaster) {
	t.Helper()

	ctx := setupTestContext(t)
	broadcaster := testpkg.NewRecordingBroadcaster()

	aware, ok := ctx.services.Education.(interface {
		SetBroadcaster(realtime.Broadcaster)
	})
	require.True(t, ok, "education service must accept a broadcaster")
	aware.SetBroadcaster(broadcaster)

	return ctx, broadcaster
}

// assertGroupAccessChanged pins the tenant routing and privacy contract via the
// shared helper, then the emitting flow.
func assertGroupAccessChanged(t *testing.T, b *testpkg.RecordingBroadcaster, source string) {
	t.Helper()

	event := testpkg.AssertSingleTenantEvent(t, b, realtime.EventGroupAccessChanged,
		int64(testutil.DefaultTestClaims().TenantID))
	require.NotNil(t, event.Data.Source)
	assert.Equal(t, source, *event.Data.Source)
}

// substitutionFixtures creates the staff + group a Vertretung needs.
func substitutionFixtures(t *testing.T, ctx *testContext, label string) (staffID, groupID int64) {
	t.Helper()

	staff := testpkg.CreateTestStaff(t, ctx.db, label, "Substitute")
	t.Cleanup(func() { testpkg.CleanupActivityFixtures(t, ctx.db, staff.ID) })

	group := testpkg.CreateTestEducationGroup(t, ctx.db, "SSE"+label)
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, ctx.db, "education.groups", group.ID) })

	return staff.ID, group.ID
}

func TestCreateSubstitution_BroadcastsGroupAccessChanged(t *testing.T) {
	ctx, broadcaster := setupRecordingContext(t)
	staffID, groupID := substitutionFixtures(t, ctx, "Create")

	body := map[string]interface{}{
		"group_id":            groupID,
		"substitute_staff_id": staffID,
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

	assertGroupAccessChanged(t, broadcaster, "substitution_create")
}

func TestUpdateSubstitution_BroadcastsGroupAccessChanged(t *testing.T) {
	ctx, broadcaster := setupRecordingContext(t)
	staffID, groupID := substitutionFixtures(t, ctx, "Update")

	today := timezone.TodayDate()
	substitution := testpkg.CreateTestGroupSubstitution(t, ctx.db, groupID, nil, staffID, today, today.AddDays(3))
	defer cleanupSubstitution(t, ctx.db, substitution.ID)

	body := map[string]interface{}{
		"group_id":            groupID,
		"substitute_staff_id": staffID,
		"start_date":          today.AddDays(1).String(),
		"end_date":            today.AddDays(5).String(),
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/substitutions/%d", substitution.ID), body,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, testutil.DefaultTestClaims())),
	)

	rr := testutil.ExecuteRequest(ctx.router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	require.True(t, ok)
	createdAt, ok := data["created_at"].(string)
	require.True(t, ok)
	assert.Equal(t, substitution.CreatedAt.Format(time.RFC3339Nano), createdAt,
		"PUT must preserve the original creation timestamp")

	assertGroupAccessChanged(t, broadcaster, "substitution_update")
}

func TestUpdateSubstitution_AccessUnchanged_BroadcastsNothing(t *testing.T) {
	ctx, broadcaster := setupRecordingContext(t)
	staffID, groupID := substitutionFixtures(t, ctx, "UpdateReason")

	today := timezone.TodayDate()
	substitution := testpkg.CreateTestGroupSubstitution(t, ctx.db, groupID, nil, staffID, today, today.AddDays(3))
	defer cleanupSubstitution(t, ctx.db, substitution.ID)

	body := map[string]interface{}{
		"group_id":            groupID,
		"substitute_staff_id": staffID,
		"start_date":          today.String(),
		"end_date":            today.AddDays(3).String(),
		"reason":              "Updated reason only",
	}
	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/substitutions/%d", substitution.ID), body,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, testutil.DefaultTestClaims())),
	)

	rr := testutil.ExecuteRequest(ctx.router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	assert.False(t, broadcaster.HasEventType(realtime.EventGroupAccessChanged))
}

func TestDeleteSubstitution_BroadcastsGroupAccessChanged(t *testing.T) {
	ctx, broadcaster := setupRecordingContext(t)
	staffID, groupID := substitutionFixtures(t, ctx, "Delete")

	today := timezone.TodayDate()
	substitution := testpkg.CreateTestGroupSubstitution(t, ctx.db, groupID, nil, staffID, today, today.AddDays(3))
	defer cleanupSubstitution(t, ctx.db, substitution.ID)

	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/substitutions/%d", substitution.ID), nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, testutil.DefaultTestClaims())),
	)

	rr := testutil.ExecuteRequest(ctx.router, req)
	require.Equal(t, http.StatusNoContent, rr.Code)

	assertGroupAccessChanged(t, broadcaster, "substitution_delete")
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

	assert.False(t, broadcaster.HasEventType(realtime.EventGroupAccessChanged))
}
