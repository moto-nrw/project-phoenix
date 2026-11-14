package behavior_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

const operatorPasskeyOrigin = "http://operator.localhost"

// operatorPasskeyScene drives the operator ceremonies end to end, like
// accountPasskeyScene does for the school portals.
type operatorPasskeyScene struct {
	passkeys identityaccess.OperatorPasskeyFlows
	mfa      identityaccess.OperatorMFAFlows
	db       *bun.DB
}

func newOperatorPasskeyScene(t *testing.T, db *bun.DB) operatorPasskeyScene {
	t.Helper()
	module := newOperatorMFATestModule(t, db, withDeliveringMailer(nil)...)
	require.NotNil(t, module.OperatorPasskeys)
	return operatorPasskeyScene{passkeys: module.OperatorPasskeys, mfa: module.OperatorMFA, db: db}
}

// operator creates an operator with the second factor a registration is
// gated by.
func (s operatorPasskeyScene) operator(t *testing.T) int64 {
	t.Helper()
	operator := testpkg.CreateTestOperator(t, s.db)
	require.NoError(t, s.mfa.EnrollOperatorMFA(context.Background(), currentOperatorProof(t, s.db, operator.ID)))
	return operator.ID
}

// beginRegistration redeems a fresh e-mail code and starts the attestation
// ceremony. The code row is written directly: the send cap of three codes
// would end a test that registers more often.
func (s operatorPasskeyScene) beginRegistration(t *testing.T, operatorID int64) identityaccess.PasskeyCeremonyOptions {
	t.Helper()
	ctx := context.Background()
	code := fmt.Sprintf("%06d", 100000+passkeyCodeSequence.Add(1))
	_, err := s.db.NewRaw(`INSERT INTO platform.operator_mfa_email_challenges (operator_id, code_hash, expires_at, credential_generation)
 SELECT id, ?, ?, credential_generation FROM platform.operators WHERE id = ?`,
		testpkg.HashTestPassword(t, code), time.Now().Add(identityaccess.MFAChallengeTTL), operatorID).Exec(ctx)
	require.NoError(t, err)
	options, err := s.passkeys.BeginOperatorPasskeyRegistration(ctx, identityaccess.OperatorPasskeyRegistrationStart{
		OperatorID: operatorID, ExpectedOrigin: operatorPasskeyOrigin, Code: code,
	})
	require.NoError(t, err)
	return options
}

func (s operatorPasskeyScene) finishRegistration(ctx context.Context, t *testing.T, operatorID int64, options identityaccess.PasskeyCeremonyOptions, authenticator *passkeyAuthenticator, name string) (identityaccess.PasskeyCredentialSummary, error) {
	t.Helper()
	return s.passkeys.FinishOperatorPasskeyRegistration(ctx, identityaccess.OperatorPasskeyRegistrationFinish{
		OperatorID: operatorID, SessionID: options.SessionID, CredentialResponse: authenticator.Attest(t, options.Options, operatorPasskeyOrigin), Name: name,
	})
}

func (s operatorPasskeyScene) register(t *testing.T, operatorID int64, name string) (*passkeyAuthenticator, identityaccess.PasskeyCredentialSummary) {
	t.Helper()
	authenticator := newPasskeyAuthenticator(t)
	summary, err := s.finishRegistration(context.Background(), t, operatorID, s.beginRegistration(t, operatorID), authenticator, name)
	require.NoError(t, err)
	return authenticator, summary
}

// beginLogin starts a discoverable login; the ceremony names no operator, so
// the test owns its row.
func (s operatorPasskeyScene) beginLogin(ctx context.Context, t *testing.T) identityaccess.PasskeyCeremonyOptions {
	t.Helper()
	options, err := s.passkeys.BeginOperatorPasskeyLogin(ctx, operatorPasskeyOrigin)
	require.NoError(t, err)
	testpkg.OwnTestOperatorPasskeySession(t, s.db, options.SessionID)
	return options
}

func (s operatorPasskeyScene) login(ctx context.Context, t *testing.T, authenticator *passkeyAuthenticator) (identityaccess.PasskeyLoginResult, error) {
	t.Helper()
	options := s.beginLogin(ctx, t)
	return s.passkeys.FinishOperatorPasskeyLogin(ctx, identityaccess.OperatorPasskeyLoginFinish{
		SessionID: options.SessionID, CredentialResponse: authenticator.Assert(t, options.Options, operatorPasskeyOrigin),
	})
}

func (s operatorPasskeyScene) list(t *testing.T, operatorID int64) []identityaccess.PasskeyCredentialSummary {
	t.Helper()
	listed, err := s.passkeys.ListOperatorPasskeyCredentials(context.Background(), operatorID)
	require.NoError(t, err)
	return listed
}

func TestOperatorPasskeyCredentialLifecycle(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	scene := newOperatorPasskeyScene(t, db)
	operator, other := scene.operator(t), scene.operator(t)
	assert.Empty(t, scene.list(t, operator))
	assert.NotNil(t, scene.list(t, operator), "an empty listing is an empty slice")

	laptop, first := scene.register(t, operator, "Admin laptop")
	require.NotEmpty(t, first.ID)
	require.NotZero(t, first.CreatedAt)
	assert.Equal(t, "Admin laptop", first.Name)
	assert.Nil(t, first.LastUsedAt)
	_, second := scene.register(t, operator, "Phone")
	foreign, _ := scene.register(t, other, "Other laptop")

	listed := scene.list(t, operator)
	require.Len(t, listed, 2, "another operator's passkey is not listed")
	assert.Equal(t, first.ID, listed[0].ID, "the oldest registration comes first")
	assert.Equal(t, second.ID, listed[1].ID)

	ctx := context.Background()
	result, err := scene.login(ctx, t, laptop)
	require.NoError(t, err)
	assert.NotEmpty(t, result.AccessToken)
	assert.NotEmpty(t, result.RefreshToken)
	listed = scene.list(t, operator)
	require.NotNil(t, listed[0].LastUsedAt, "a login records the use")
	assert.WithinDuration(t, time.Now(), *listed[0].LastUsedAt, time.Minute)
	assert.Nil(t, listed[1].LastUsedAt)
	assert.Equal(t, 1, storedSignCount(t, db, "platform.operator_passkey_credentials", passkeyID(t, first)), "a login stores the credential state the assertion proved")

	impostor := *foreign
	impostor.UserHandle = laptop.UserHandle
	_, err = scene.login(ctx, t, &impostor)
	require.ErrorIs(t, err, identityaccess.ErrInvalidCredentials, "a credential only matches its own user handle")
	unknown := newPasskeyAuthenticator(t)
	unknown.UserHandle = laptop.UserHandle
	_, err = scene.login(ctx, t, unknown)
	require.ErrorIs(t, err, identityaccess.ErrInvalidCredentials, "an unknown credential does not sign in")

	require.ErrorIs(t, scene.passkeys.RevokeOperatorPasskeyCredential(ctx, other, passkeyID(t, first)), identityaccess.ErrPasskeyNotFound,
		"an operator cannot revoke another operator's passkey")
	require.NoError(t, scene.passkeys.RevokeOperatorPasskeyCredential(ctx, operator, passkeyID(t, first)))
	require.ErrorIs(t, scene.passkeys.RevokeOperatorPasskeyCredential(ctx, operator, passkeyID(t, first)), identityaccess.ErrPasskeyNotFound)
	_, err = scene.login(ctx, t, laptop)
	require.ErrorIs(t, err, identityaccess.ErrInvalidCredentials, "a revoked passkey never verifies")
	listed = scene.list(t, operator)
	require.Len(t, listed, 1)
	assert.Equal(t, second.ID, listed[0].ID)
}

// A credential ID is registered once platform-wide. The refused insert is no
// rejection of the ceremony: both writes roll back, so the same ceremony
// completes with another authenticator and is spent only then.
func TestOperatorPasskeyRegistrationRollsBackWhenTheCredentialIsTaken(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	scene := newOperatorPasskeyScene(t, db)
	operator, other := scene.operator(t), scene.operator(t)
	taken, _ := scene.register(t, other, "Taken")

	options := scene.beginRegistration(t, operator)
	ctx := context.Background()
	_, err := scene.finishRegistration(ctx, t, operator, options, taken, "Replay")
	require.ErrorContains(t, err, "uniq_operator_passkey_credentials_credential_id", "a credential ID is registered once platform-wide")
	assert.Empty(t, scene.list(t, operator), "the refused insert stores no credential")

	summary, err := scene.finishRegistration(ctx, t, operator, options, newPasskeyAuthenticator(t), "Fresh")
	require.NoError(t, err, "the retry finds the ceremony unconsumed")
	require.Len(t, scene.list(t, operator), 1)
	assert.Equal(t, "Fresh", summary.Name)
	_, err = scene.finishRegistration(ctx, t, operator, options, newPasskeyAuthenticator(t), "Late")
	require.ErrorIs(t, err, identityaccess.ErrPasskeySessionInvalid, "the committed ceremony is spent")
}

// A ceremony completes exactly once, only for its own purpose, only for the
// operator that started it and only before it expires.
func TestOperatorPasskeySessionIsConsumedOnce(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	scene := newOperatorPasskeyScene(t, db)
	operator, other := scene.operator(t), scene.operator(t)
	authenticator, _ := scene.register(t, operator, "Admin laptop")
	ctx := context.Background()

	registration := scene.beginRegistration(t, operator)
	_, err := scene.passkeys.FinishOperatorPasskeyLogin(ctx, identityaccess.OperatorPasskeyLoginFinish{
		SessionID: registration.SessionID, CredentialResponse: authenticator.Assert(t, scene.beginLogin(ctx, t).Options, operatorPasskeyOrigin),
	})
	require.ErrorIs(t, err, identityaccess.ErrPasskeySessionInvalid, "a registration ceremony cannot complete a login")
	_, err = scene.finishRegistration(ctx, t, operator, registration, newPasskeyAuthenticator(t), "Second")
	require.NoError(t, err, "the wrong purpose left the ceremony unspent")
	_, err = scene.finishRegistration(ctx, t, operator, registration, newPasskeyAuthenticator(t), "Replay")
	require.ErrorIs(t, err, identityaccess.ErrPasskeySessionInvalid, "a replayed completion is refused")

	foreign := scene.beginRegistration(t, other)
	_, err = scene.finishRegistration(ctx, t, operator, foreign, newPasskeyAuthenticator(t), "Stolen")
	require.ErrorIs(t, err, identityaccess.ErrPasskeySessionInvalid, "a ceremony completes only for the operator that started it")
	assert.Len(t, scene.list(t, operator), 2)

	login := scene.beginLogin(ctx, t)
	finish := identityaccess.OperatorPasskeyLoginFinish{SessionID: login.SessionID, CredentialResponse: authenticator.Assert(t, login.Options, operatorPasskeyOrigin)}
	_, err = scene.passkeys.FinishOperatorPasskeyLogin(ctx, finish)
	require.NoError(t, err, "a discoverable login names no operator and completes")
	_, err = scene.passkeys.FinishOperatorPasskeyLogin(ctx, finish)
	require.ErrorIs(t, err, identityaccess.ErrPasskeySessionInvalid, "a replayed login is refused")

	expired := scene.beginLogin(ctx, t)
	_, err = db.NewRaw("UPDATE platform.operator_passkey_sessions SET expires_at = now() - interval '1 minute' WHERE id = ?", expired.SessionID).Exec(ctx)
	require.NoError(t, err)
	_, err = scene.passkeys.FinishOperatorPasskeyLogin(ctx, identityaccess.OperatorPasskeyLoginFinish{
		SessionID: expired.SessionID, CredentialResponse: authenticator.Assert(t, expired.Options, operatorPasskeyOrigin),
	})
	require.ErrorIs(t, err, identityaccess.ErrPasskeySessionInvalid, "an expired ceremony is refused")
	_, err = scene.passkeys.FinishOperatorPasskeyLogin(ctx, identityaccess.OperatorPasskeyLoginFinish{
		SessionID: "unknown-" + expired.SessionID, CredentialResponse: authenticator.Assert(t, expired.Options, operatorPasskeyOrigin),
	})
	require.ErrorIs(t, err, identityaccess.ErrPasskeySessionInvalid)
}

// The operator passkey tables carry no tenant column: a tenant context must
// neither hide nor reveal rows, because every other table this owner touches
// is tenant-scoped and a reader could assume the same here.
func TestOperatorPasskeyRecordsAreVisibleFromEveryTenantContext(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	scene := newOperatorPasskeyScene(t, db)
	operator := scene.operator(t)
	authenticator, credential := scene.register(t, operator, "Admin laptop")
	first := testpkg.Tenant(t)
	second, _ := testpkg.CreateTestTenant(t, db)
	require.NotEqual(t, first, second)

	for name, tenantID := range map[string]int64{"own tenant": first, "foreign tenant": second} {
		t.Run(name, func(t *testing.T) {
			tenantCtx := testpkg.TenantContext(tenantID)
			listed, err := scene.passkeys.ListOperatorPasskeyCredentials(tenantCtx, operator)
			require.NoError(t, err)
			require.Len(t, listed, 1)
			assert.Equal(t, credential.ID, listed[0].ID)
			_, err = scene.login(tenantCtx, t, authenticator)
			require.NoError(t, err, "a ceremony started in a tenant context completes within it")

			started := scene.beginLogin(context.Background(), t)
			_, err = scene.passkeys.FinishOperatorPasskeyLogin(tenantCtx, identityaccess.OperatorPasskeyLoginFinish{
				SessionID: started.SessionID, CredentialResponse: authenticator.Assert(t, started.Options, operatorPasskeyOrigin),
			})
			require.NoError(t, err, "a ceremony started without this tenant completes within it")
			crossed := scene.beginLogin(tenantCtx, t)
			_, err = scene.passkeys.FinishOperatorPasskeyLogin(testpkg.TenantContext(first+second-tenantID), identityaccess.OperatorPasskeyLoginFinish{
				SessionID: crossed.SessionID, CredentialResponse: authenticator.Assert(t, crossed.Options, operatorPasskeyOrigin),
			})
			require.NoError(t, err, "a ceremony started in one tenant completes in the other")
		})
	}
}

// A passkey revoked while its login is being verified is not updated: the
// login sees the credential active, the revocation commits first, and the
// use then finds no active credential. The login fails instead of minting.
func TestOperatorPasskeyRevokedDuringALoginIsNotUpdated(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	scene := newOperatorPasskeyScene(t, db)
	operator := scene.operator(t)
	authenticator, credential := scene.register(t, operator, "Admin laptop")
	id := passkeyID(t, credential)

	options := scene.beginLogin(context.Background(), t)
	finish := identityaccess.OperatorPasskeyLoginFinish{SessionID: options.SessionID, CredentialResponse: authenticator.Assert(t, options.Options, operatorPasskeyOrigin)}
	loginErr := finishDuringRevocation(t, db, "platform.operator_passkey_credentials", id, func() error {
		_, err := scene.passkeys.FinishOperatorPasskeyLogin(context.Background(), finish)
		return err
	})
	require.ErrorIs(t, loginErr, identityaccess.ErrOperatorPasskeyNotFound)
	assert.Empty(t, scene.list(t, operator))
	assert.Zero(t, storedSignCount(t, db, "platform.operator_passkey_credentials", id), "the revoked credential keeps its state")
}
