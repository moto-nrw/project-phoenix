package migrations

import (
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestInvitationEmploymentActionStorage(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	role := testpkg.CreateTestRole(t, db, "invitation-action")
	creator := testpkg.CreateTestAccount(t, db, "invitation-action-creator")
	invitation := testpkg.CreateTestInvitationToken(t, db, "returning", role.ID, creator.ID, time.Now().Add(time.Hour))
	var action string
	require.NoError(t, db.NewRaw(`SELECT employment_action FROM auth.invitation_tokens WHERE id = ?`, invitation.ID).Scan(ctx, &action))
	require.Equal(t, "resume", action, "old callers and existing links continue the former employment")
	_, err := db.ExecContext(ctx, `UPDATE auth.invitation_tokens SET employment_action = 'new' WHERE id = ?`, invitation.ID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE auth.invitation_tokens SET employment_action = 'invalid' WHERE id = ?`, invitation.ID)
	require.Error(t, err, "the database rejects unknown decisions")
}
