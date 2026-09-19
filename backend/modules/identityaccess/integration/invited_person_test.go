package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestInvitedPersonLookupPreservesExpiredLinksAndTenantBoundary(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	role := testpkg.CreateTestRole(t, db, "invited-person-role")
	person := testpkg.CreateTestPerson(t, db, "Invited", "Person")
	invitation := testpkg.CreateTestInvitationToken(t, db, "invited-person", role.ID, 0, time.Now().Add(-time.Hour))
	access, err := identityCompose.New(identityCompose.Dependencies{DB: db, Observe: func(identityCompose.Observation) {}})
	require.NoError(t, err)
	ids, err := access.FindInvitedPersonIDs(testpkg.Ctx(t), invitation.Email)
	require.NoError(t, err)
	require.Empty(t, ids, "invitations without a person are not import identities")

	_, err = db.NewRaw(`UPDATE auth.invitation_tokens SET person_id = ? WHERE id = ? AND tenant_id = ?`, person.ID, invitation.ID, testpkg.Tenant(t)).Exec(context.Background())
	require.NoError(t, err)
	ids, err = access.FindInvitedPersonIDs(testpkg.Ctx(t), strings.ToUpper(invitation.Email))
	require.NoError(t, err)
	require.Equal(t, []int64{person.ID}, ids, "expiry does not remove the imported person's email match")

	otherTenant, _ := testpkg.CreateTestTenant(t, db)
	ids, err = access.FindInvitedPersonIDs(testpkg.ContextForTenant(testpkg.Ctx(t), otherTenant), invitation.Email)
	require.NoError(t, err)
	require.Empty(t, ids)
	_, err = access.FindInvitedPersonIDs(context.Background(), invitation.Email)
	require.ErrorIs(t, err, identityaccess.ErrTenantRequired)

	_, err = db.NewRaw(`UPDATE auth.invitation_tokens SET used_at = NOW() WHERE id = ? AND tenant_id = ?`, invitation.ID, testpkg.Tenant(t)).Exec(context.Background())
	require.NoError(t, err)
	ids, err = access.FindInvitedPersonIDs(testpkg.Ctx(t), invitation.Email)
	require.NoError(t, err)
	require.Empty(t, ids, "used or revoked invitations no longer identify a pending invitee")
}
