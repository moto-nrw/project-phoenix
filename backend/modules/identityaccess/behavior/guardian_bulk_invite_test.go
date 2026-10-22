package behavior_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/api"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The bulk invitation (#3378) against the real tables: one guardian of two
// siblings gets one invitation and one mail, a pickup-only contact is left
// alone, and a second run does not mail anybody again.
func TestBulkInviteToStudents_InvitesEachGuardianOnce(t *testing.T) {
	t.Parallel()

	outbox := api.NewCapturingOutbox()
	env := setupGuardianInvitationTest(t, func(_ *bun.DB, cfg *api.GuardianInvitationTestConfig) { cfg.Outbox = outbox })

	older := testpkg.CreateTestStudent(t, env.db, "Bulk", "Older", "3a")
	younger := testpkg.CreateTestStudent(t, env.db, "Bulk", "Younger", "1a")
	parent := testpkg.CreateTestGuardianProfileNamed(t, env.db, "Katharina", "Brenner", "bulk-parent")
	grandma := testpkg.CreateTestGuardianProfileNamed(t, env.db, "Oma", "Brenner", "bulk-grandma")
	testpkg.CreateTestStudentGuardianLink(t, env.db, older.ID, parent.ID, guardianRoleLegalGuardian)
	testpkg.CreateTestStudentGuardianLink(t, env.db, younger.ID, parent.ID, guardianRoleLegalGuardian)
	grandmaLink := testpkg.CreateTestStudentGuardianLink(t, env.db, older.ID, grandma.ID, guardianRolePickupOnly)

	ctx := testpkg.Ctx(t)
	req := identityaccess.BulkInviteRequest{StudentIDs: []int64{older.ID, younger.ID}, CreatedBy: env.inviterAccountID(t)}

	preview := req
	preview.DryRun = true
	counted, err := env.service.BulkInviteToStudents(ctx, preview)
	require.NoError(t, err)
	assert.Equal(t, 1, counted.Invited)
	assert.Empty(t, outbox.Requests(), "the preview mails nobody")

	result, err := env.service.BulkInviteToStudents(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Invited)
	assert.Equal(t, 1, result.SkippedRestricted)
	assert.Empty(t, result.Problems)
	require.Len(t, outbox.Requests(), 2, "two children, one guardian, one invitation and one welcome (#3534)")
	assert.Equal(t, emailKindGuardianInvitation, outbox.Requests()[0].Kind)
	assert.Equal(t, "guardian_welcome", outbox.Requests()[1].Kind)
	assert.Equal(t, parent.ID, outbox.Requests()[1].RelatedEntityID)
	assert.False(t, testpkg.StudentGuardianLinkGrantsPortalAccess(t, env.db, grandmaLink.ID), "the pickup-only contact is not upgraded")

	again, err := env.service.BulkInviteToStudents(ctx, req)
	require.NoError(t, err)
	assert.Zero(t, again.Invited)
	assert.Equal(t, 1, again.SkippedOpen)
	assert.Len(t, outbox.Requests(), 2, "a repeated run does not mail again")

	req.ResendOpen = true
	resent, err := env.service.BulkInviteToStudents(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, 1, resent.Resent)
	mailed := outbox.Requests()
	require.Len(t, mailed, 3, "a resend mails the link alone, without a second welcome")
	assert.Equal(t, emailKindGuardianInvitation, mailed[2].Kind)
	assert.Equal(t, mailed[0].RelatedEntityID, mailed[2].RelatedEntityID, "the same invitation is mailed again")
}
