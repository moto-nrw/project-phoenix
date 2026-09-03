package api

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Staff invite endpoint goldens for the restricted-contact upgrade flow
// (#2172): full stack through the composed router with a signed admin JWT.

func TestGuardianComposition_InviteRestrictedContactRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	studentID, _ := ctx.createStudent("Staff", "Invite", "8a")
	guardianID, email := ctx.createGuardian("staff-restricted")
	linkID := ctx.link(studentID, guardianID, "emergency_contact")
	path := fmt.Sprintf("/students/%d/invite", studentID)

	// Without confirmation: detection outcome, no upgrade, existing_role set.
	rr := ctx.do(t, testutil.DefaultTestClaims(), http.MethodPost, path, map[string]any{"email": email})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), `"outcome":"existing_contact_restricted"`)
	assert.Contains(t, rr.Body.String(), `"existing_role":"emergency_contact"`)
	assert.Equal(t, "emergency_contact", ctx.persistedLink(linkID).Role, "unconfirmed invite must not change the link")

	// With confirmation: the resolve proceeds and the link is upgraded.
	rr = ctx.do(t, testutil.DefaultTestClaims(), http.MethodPost, path, map[string]any{"email": email, "confirm_role_upgrade": true})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), `"outcome":"invited"`)
	state := ctx.persistedLink(linkID)
	assert.Equal(t, "legal_guardian", state.Role)
	assert.True(t, state.PortalAccess)
}

func TestGuardianComposition_InviteSocialWorkerLinkRefused(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	studentID, _ := ctx.createStudent("Staff", "SocialWorker", "8b")
	guardianID, email := ctx.createGuardian("staff-social-worker")
	linkID := ctx.link(studentID, guardianID, "social_worker")

	rr := ctx.do(t, testutil.DefaultTestClaims(), http.MethodPost, fmt.Sprintf("/students/%d/invite", studentID),
		map[string]any{"email": email, "confirm_role_upgrade": true})
	require.Equal(t, http.StatusForbidden, rr.Code, rr.Body.String())
	assert.Equal(t, "social_worker", ctx.persistedLink(linkID).Role, "link must be untouched")
}

func TestGuardianComposition_InviteGates(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	studentID, _ := ctx.createStudent("Invite", "Gate", "8c")

	testutil.AssertBadRequest(t, ctx.do(t, testutil.DefaultTestClaims(), http.MethodPost, "/students/invalid/invite", map[string]any{"email": "x@example.test"}))
	testutil.AssertForbidden(t, ctx.do(t, nonStaffClaims(t, "users:create"), http.MethodPost, fmt.Sprintf("/students/%d/invite", studentID), map[string]any{"email": "x@example.test"}))

	// The approval queue is admin-only (users:manage), not users:read.
	testutil.AssertForbidden(t, ctx.do(t, withPerms(testutil.TeacherTestClaims(1), "users:read"), http.MethodGet, "/invitations/pending-approval", nil))
	rr := ctx.do(t, testutil.AdminTestClaims(999), http.MethodGet, "/invitations/pending-approval", nil)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	dataArray(t, rr.Body.Bytes())

	testutil.AssertBadRequest(t, ctx.do(t, testutil.AdminTestClaims(999), http.MethodPost, "/invitations/99999/approve", nil))
	testutil.AssertBadRequest(t, ctx.do(t, testutil.AdminTestClaims(999), http.MethodPost, "/invitations/99999/reject", nil))
}
