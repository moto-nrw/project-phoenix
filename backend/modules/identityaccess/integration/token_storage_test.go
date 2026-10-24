package integration

// Coverage for #4902: session, invitation and e-mail-change tokens are
// stored as their hex SHA-256 only. A reader of the database or a backup
// never sees the value a link or refresh JWT carries; lookups still find the
// row by the presented value.

import (
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func storedColumn(t *testing.T, db *bun.DB, table, column string, id int64) string {
	t.Helper()
	var value string
	require.NoError(t, db.NewRaw(`SELECT `+column+` FROM `+table+` WHERE id = ?`, id).Scan(testpkg.Ctx(t), &value))
	return value
}

func TestAccountSessionTokensAreStoredAsHashes(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	access := newAccountSessionAccess(t, db)
	ctx := testpkg.Ctx(t)
	account := testpkg.CreateTestAccount(t, db, "session-hash")
	familyID := newFamilyID()

	predecessor, err := access.CreateAccountSession(ctx, newAccountSession(account.ID, "tenant", familyID, 0))
	require.NoError(t, err)
	successor, err := access.CreateAccountSession(ctx, newAccountSession(account.ID, "tenant", familyID, 1))
	require.NoError(t, err)
	require.NoError(t, access.MarkAccountSessionRotated(ctx, predecessor.ID, successor.Token, []byte{1}, time.Now()))

	assert.Equal(t, testpkg.TokenHash(t, db, predecessor.Token), storedColumn(t, db, "auth.tokens", "token", predecessor.ID))
	assert.Equal(t, testpkg.TokenHash(t, db, successor.Token), storedColumn(t, db, "auth.tokens", "replacement_token", predecessor.ID))
	assert.Equal(t, testpkg.TokenHash(t, db, predecessor.Token), predecessor.TokenHash)

	found, err := access.FindAccountSession(ctx, predecessor.Token)
	require.NoError(t, err, "the presented value still finds its session")
	assert.Equal(t, predecessor.ID, found.ID)
	assert.Equal(t, predecessor.Token, found.Token, "a lookup returns the value it was given")
	require.NotNil(t, found.ReplacementTokenHash)
	assert.Equal(t, testpkg.TokenHash(t, db, successor.Token), *found.ReplacementTokenHash)

	_, err = access.FindAccountSession(ctx, testpkg.TokenHash(t, db, predecessor.Token))
	require.ErrorIs(t, err, identityaccess.ErrAccountSessionNotFound, "the stored hash is no credential")

	listed, err := access.ListAccountSessions(ctx, identityaccess.AccountSessionFilter{FamilyID: familyID})
	require.NoError(t, err)
	require.Len(t, listed, 2)
	for _, session := range listed {
		assert.Empty(t, session.Token, "a listing cannot know the raw value")
		assert.Len(t, session.TokenHash, 64)
	}
}

func TestOperatorTokensAreStoredAsHashes(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := newOperatorTokens(t, db)
	ctx := testpkg.Ctx(t)
	operator := testpkg.CreateTestOperator(t, db)

	familyID := newFamilyID()
	predecessor, err := module.CreateOperatorSession(ctx, identityaccess.OperatorSession{
		OperatorID: operator.ID, Token: uuid.Must(uuid.NewV4()).String(), Expiry: time.Now().Add(time.Hour), FamilyID: familyID,
	})
	require.NoError(t, err)
	successor, err := module.CreateOperatorSession(ctx, identityaccess.OperatorSession{
		OperatorID: operator.ID, Token: uuid.Must(uuid.NewV4()).String(), Expiry: time.Now().Add(time.Hour), FamilyID: familyID, Generation: 1,
	})
	require.NoError(t, err)
	require.NoError(t, module.MarkOperatorSessionRotated(ctx, predecessor.ID, successor.Token, []byte{1}, time.Now()))
	assert.Equal(t, testpkg.TokenHash(t, db, predecessor.Token), storedColumn(t, db, "platform.operator_refresh_tokens", "token", predecessor.ID))
	assert.Equal(t, testpkg.TokenHash(t, db, successor.Token), storedColumn(t, db, "platform.operator_refresh_tokens", "replacement_token", predecessor.ID))

	invitation, err := module.CreateOperatorInvitation(ctx, newInvitation(operator.ID, inviteeEmail()))
	require.NoError(t, err)
	assert.NotEmpty(t, invitation.Token, "the created link keeps the value the mail carries")
	assert.Equal(t, testpkg.TokenHash(t, db, invitation.Token), storedColumn(t, db, "platform.operator_invitation_tokens", "token", invitation.ID))
	validated, err := module.FindRedeemableOperatorInvitation(ctx, invitation.Token)
	require.NoError(t, err)
	assert.Equal(t, invitation.ID, validated.ID)
	_, err = module.FindRedeemableOperatorInvitation(ctx, testpkg.TokenHash(t, db, invitation.Token))
	require.Error(t, err, "the stored hash is no credential")

	change, err := module.CreateOperatorEmailChange(ctx, newEmailChange(operator.ID))
	require.NoError(t, err)
	assert.Equal(t, testpkg.TokenHash(t, db, change.Token), storedColumn(t, db, "platform.operator_email_change_tokens", "token", change.ID))
	redeemed, err := module.RedeemOperatorEmailChange(ctx, change.Token)
	require.NoError(t, err)
	assert.Equal(t, change.ID, redeemed.ID)
}
