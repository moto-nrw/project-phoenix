package migrations

import (
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestInvitationEmploymentResetStorage(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	role := testpkg.CreateTestRole(t, db, "invitation-resets")
	creator := testpkg.CreateTestAccount(t, db, "invitation-reset-creator")
	invitation := testpkg.CreateTestInvitationToken(t, db, "returning-reset", role.ID, creator.ID, time.Now().Add(time.Hour))
	var stored struct{ ResetMasterData, ResetQualifications, ResetWorkTime bool }
	require.NoError(t, db.NewRaw(`SELECT reset_master_data, reset_qualifications, reset_work_time FROM auth.invitation_tokens WHERE id = ?`, invitation.ID).Scan(ctx, &stored))
	require.False(t, stored.ResetMasterData)
	require.False(t, stored.ResetQualifications)
	require.False(t, stored.ResetWorkTime)
	_, err := db.ExecContext(ctx, `UPDATE auth.invitation_tokens SET reset_master_data = true, reset_qualifications = true, reset_work_time = true WHERE id = ?`, invitation.ID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE auth.invitation_tokens SET employment_action = 'new' WHERE id = ?`, invitation.ID)
	require.Error(t, err, "a new employment cannot reset the old one's data")
	_, err = db.ExecContext(ctx, `UPDATE auth.invitation_tokens SET reset_master_data = false, reset_qualifications = false, reset_work_time = false, employment_action = 'new' WHERE id = ?`, invitation.ID)
	require.NoError(t, err)
}
