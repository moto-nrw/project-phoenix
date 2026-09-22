package behavior_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/services"
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

	outbox := testpkg.NewCapturingOutbox()
	env := setupGuardianInvitationTest(t, func(_ *bun.DB, cfg *services.GuardianInvitationTestConfig) { cfg.Outbox = outbox })
	defer env.cleanup()

	older := testpkg.CreateTestStudent(t, env.db, "Bulk", "Older", "3a")
	younger := testpkg.CreateTestStudent(t, env.db, "Bulk", "Younger", "1a")
	defer env.deleteStudentGuardianLinks(older.ID)
	defer env.deleteStudentGuardianLinks(younger.ID)
	parent := testpkg.CreateTestGuardianProfileNamed(t, env.db, "Katharina", "Brenner", "bulk-parent")
	grandma := testpkg.CreateTestGuardianProfileNamed(t, env.db, "Oma", "Brenner", "bulk-grandma")
	testpkg.CreateTestStudentGuardianLink(t, env.db, older.ID, parent.ID, guardianRoleLegalGuardian)
	testpkg.CreateTestStudentGuardianLink(t, env.db, younger.ID, parent.ID, guardianRoleLegalGuardian)
	grandmaLink := testpkg.CreateTestStudentGuardianLink(t, env.db, older.ID, grandma.ID, guardianRolePickupOnly)
	defer func() {
		_, _ = env.db.NewDelete().TableExpr("auth.guardian_invitations").
			Where("guardian_profile_id IN (?)", bun.List([]int64{parent.ID, grandma.ID})).Exec(context.Background())
		_, _ = env.db.NewDelete().TableExpr("users.guardian_profiles").
			Where("id IN (?)", bun.List([]int64{parent.ID, grandma.ID})).Exec(context.Background())
	}()

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
	require.Len(t, outbox.Requests(), 1, "two children, one guardian, one mail")
	assert.Equal(t, emailKindGuardianInvitation, outbox.Requests()[0].Kind)
	assert.False(t, testpkg.StudentGuardianLinkGrantsPortalAccess(t, env.db, grandmaLink.ID), "the pickup-only contact is not upgraded")

	again, err := env.service.BulkInviteToStudents(ctx, req)
	require.NoError(t, err)
	assert.Zero(t, again.Invited)
	assert.Equal(t, 1, again.SkippedOpen)
	assert.Len(t, outbox.Requests(), 1, "a repeated run does not mail again")

	req.ResendOpen = true
	resent, err := env.service.BulkInviteToStudents(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, 1, resent.Resent)
	mailed := outbox.Requests()
	require.Len(t, mailed, 2)
	assert.Equal(t, mailed[0].RelatedEntityID, mailed[1].RelatedEntityID, "the same invitation is mailed again")
}
