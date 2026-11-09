package behavior_test

import (
	"context"
	"fmt"
	"testing"
	"time"

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

// #6294: a link that works for nobody does not count as an open invitation.
// A legacy link without a recorded recipient (ended by 1.15.846) and a link
// mailed to an address since corrected both lead to a new link for the
// address on file, also without ResendOpen.
func TestBulkInviteToStudents_DeadLinksGetANewInvitation(t *testing.T) {
	t.Parallel()
	outbox := api.NewCapturingOutbox()
	env := setupGuardianInvitationTest(t, func(_ *bun.DB, cfg *api.GuardianInvitationTestConfig) { cfg.Outbox = outbox })
	creator := env.inviterAccountID(t)
	ctx := testpkg.Ctx(t)

	for _, tc := range []struct {
		name  string
		plant func(t *testing.T, profileID int64)
	}{
		{"legacy link without recipient", func(t *testing.T, profileID int64) {
			legacy := testpkg.InsertTestGuardianInvitation(t, env.db, &testpkg.GuardianInvitation{
				Token: fmt.Sprintf("bulk-legacy-%d", profileID), GuardianProfileID: profileID, CreatedBy: creator,
				ExpiresAt: time.Now().Add(-time.Hour),
			})
			_, err := env.db.NewRaw(`UPDATE auth.guardian_invitations SET email = NULL WHERE id = ?`, legacy.ID).Exec(context.Background())
			require.NoError(t, err)
		}},
		{"address corrected after the link was mailed", func(t *testing.T, profileID int64) {
			_, err := env.service.CreateGuardianInvitation(ctx, identityaccess.GuardianInvitationRequest{GuardianProfileID: profileID, CreatedBy: creator})
			require.NoError(t, err)
			env.changeGuardianEmail(t, profileID, fmt.Sprintf("bulk-corrected-%d@example.test", time.Now().UnixNano()))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			student := testpkg.CreateTestStudent(t, env.db, "Bulk", "Dead", "2c")
			profile := testpkg.CreateTestGuardianProfile(t, env.db, "bulk-dead-link")
			testpkg.CreateTestStudentGuardianLink(t, env.db, student.ID, profile.ID, guardianRoleLegalGuardian)
			tc.plant(t, profile.ID)
			current, err := testpkg.TenantQuery1(t, ctx, env.db, env.repos.GuardianProfile.FindByID, profile.ID)
			require.NoError(t, err)

			result, err := env.service.BulkInviteToStudents(ctx, identityaccess.BulkInviteRequest{StudentIDs: []int64{student.ID}, CreatedBy: creator})
			require.NoError(t, err)
			assert.Zero(t, result.SkippedOpen, "a dead link is no open invitation")
			assert.Equal(t, 1, result.Invited)
			token := mailedToken(t, outbox, *current.Email)
			preview, err := env.service.ValidateGuardianInvitation(context.Background(), token)
			require.NoError(t, err, "the new link works for the address on file")
			assert.Equal(t, *current.Email, preview.Email)
		})
	}
}
