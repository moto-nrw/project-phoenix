package behavior_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// accountPasskeyScene drives the school-portal ceremonies end to end: a
// software authenticator answers the options, the composed owner verifies
// them and Postgres keeps the rows.
type accountPasskeyScene struct {
	passkeys  identityaccess.AccountPasskeyFlows
	db        *bun.DB
	tenantID  int64
	subdomain string
	origin    string
}

var passkeyCodeSequence atomic.Int64

func newAccountPasskeyScene(t *testing.T, db *bun.DB) accountPasskeyScene {
	t.Helper()
	module := newMFATestModule(t, db, withDeliveringMailer(nil)...)
	require.NotNil(t, module.Passkeys)
	tenantID, subdomain := testpkg.CreateTestTenant(t, db)
	return accountPasskeyScene{passkeys: module.Passkeys, db: db, tenantID: tenantID, subdomain: subdomain, origin: "http://" + subdomain + ".localhost"}
}

// member creates an account that belongs to the scene's school.
func (s accountPasskeyScene) member(t *testing.T) int64 {
	t.Helper()
	account := testpkg.CreateTestAccount(t, s.db, "passkey-ceremony")
	testpkg.EnsureAccountTenant(t, s.db, account.ID, s.tenantID)
	return account.ID
}

// beginRegistration redeems a fresh e-mail code of the school's portal and
// starts the attestation ceremony.
func (s accountPasskeyScene) beginRegistration(t *testing.T, accountID int64) identityaccess.PasskeyCeremonyOptions {
	t.Helper()
	code := fmt.Sprintf("%06d", 100000+passkeyCodeSequence.Add(1))
	seedTenantChallenge(t, mfaRecordRows(t, s.db), accountID, s.tenantID, code)
	options, err := s.passkeys.BeginAccountPasskeyRegistration(context.Background(), identityaccess.AccountPasskeyRegistrationStart{
		AccountID: accountID, TenantID: s.tenantID, TenantSubdomain: s.subdomain, ExpectedOrigin: s.origin, Code: code,
	})
	require.NoError(t, err)
	return options
}

func (s accountPasskeyScene) finishRegistration(ctx context.Context, t *testing.T, accountID int64, options identityaccess.PasskeyCeremonyOptions, authenticator *passkeyAuthenticator, name string) (identityaccess.PasskeyCredentialSummary, error) {
	t.Helper()
	return s.passkeys.FinishAccountPasskeyRegistration(ctx, identityaccess.AccountPasskeyRegistrationFinish{
		AccountID: accountID, SessionID: options.SessionID, CredentialResponse: authenticator.Attest(t, options.Options, s.origin), Name: name,
	})
}

func (s accountPasskeyScene) register(t *testing.T, accountID int64, name string) (*passkeyAuthenticator, identityaccess.PasskeyCredentialSummary) {
	t.Helper()
	authenticator := newPasskeyAuthenticator(t)
	summary, err := s.finishRegistration(context.Background(), t, accountID, s.beginRegistration(t, accountID), authenticator, name)
	require.NoError(t, err)
	return authenticator, summary
}

// beginLogin starts a discoverable login on the portal of a school; the
// ceremony names no account, so the test owns its row.
func (s accountPasskeyScene) beginLogin(t *testing.T, tenantID int64, subdomain string) identityaccess.PasskeyCeremonyOptions {
	t.Helper()
	options, err := s.passkeys.BeginAccountPasskeyLogin(context.Background(), identityaccess.AccountPasskeyLoginStart{
		TenantID: tenantID, TenantSubdomain: subdomain, ExpectedOrigin: "http://" + subdomain + ".localhost",
	})
	require.NoError(t, err)
	testpkg.OwnTestAccountPasskeySession(t, s.db, options.SessionID)
	return options
}

func (s accountPasskeyScene) login(ctx context.Context, t *testing.T, authenticator *passkeyAuthenticator) (identityaccess.PasskeyLoginResult, error) {
	t.Helper()
	options := s.beginLogin(t, s.tenantID, s.subdomain)
	return s.passkeys.FinishAccountPasskeyLogin(ctx, identityaccess.AccountPasskeyLoginFinish{
		SessionID: options.SessionID, CredentialResponse: authenticator.Assert(t, options.Options, s.origin),
	})
}

func (s accountPasskeyScene) list(t *testing.T, accountID int64) []identityaccess.PasskeyCredentialSummary {
	t.Helper()
	listed, err := s.passkeys.ListAccountPasskeyCredentials(context.Background(), accountID)
	require.NoError(t, err)
	return listed
}

func passkeyID(t *testing.T, summary identityaccess.PasskeyCredentialSummary) int64 {
	t.Helper()
	var id int64
	_, err := fmt.Sscan(summary.ID, &id)
	require.NoError(t, err)
	return id
}

// storedSignCount reads the signature counter the relying party keeps in a
// credential's state.
func storedSignCount(t *testing.T, db *bun.DB, table string, credentialID int64) int {
	t.Helper()
	var count int
	require.NoError(t, db.NewRaw("SELECT (credential_json->'authenticator'->>'signCount')::int FROM "+table+" WHERE id = ?", credentialID).Scan(context.Background(), &count))
	return count
}

func TestAccountPasskeyCredentialLifecycle(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	scene := newAccountPasskeyScene(t, db)
	account, other := scene.member(t), scene.member(t)
	assert.Empty(t, scene.list(t, account))
	assert.NotNil(t, scene.list(t, account), "an empty listing is an empty slice")

	laptop, first := scene.register(t, account, "Laptop")
	require.NotEmpty(t, first.ID)
	require.NotZero(t, first.CreatedAt)
	assert.Equal(t, "Laptop", first.Name)
	assert.Nil(t, first.LastUsedAt)
	_, second := scene.register(t, account, "Phone")
	foreign, _ := scene.register(t, other, "Other laptop")

	listed := scene.list(t, account)
	require.Len(t, listed, 2, "another account's passkey is not listed")
	assert.Equal(t, first.ID, listed[0].ID, "the oldest registration comes first")
	assert.Equal(t, second.ID, listed[1].ID)

	result, err := scene.login(context.Background(), t, laptop)
	require.NoError(t, err)
	assert.NotEmpty(t, result.AccessToken)
	assert.NotEmpty(t, result.RefreshToken)
	listed = scene.list(t, account)
	require.NotNil(t, listed[0].LastUsedAt, "a login records the use")
	assert.WithinDuration(t, time.Now(), *listed[0].LastUsedAt, time.Minute)
	assert.Nil(t, listed[1].LastUsedAt)
	assert.Equal(t, 1, storedSignCount(t, db, "auth.passkey_credentials", passkeyID(t, first)), "a login stores the credential state the assertion proved")

	impostor := *foreign
	impostor.UserHandle = laptop.UserHandle
	_, err = scene.login(context.Background(), t, &impostor)
	require.ErrorIs(t, err, identityaccess.ErrInvalidCredentials, "a credential only matches its own user handle")
	unknown := newPasskeyAuthenticator(t)
	unknown.UserHandle = laptop.UserHandle
	_, err = scene.login(context.Background(), t, unknown)
	require.ErrorIs(t, err, identityaccess.ErrInvalidCredentials, "an unknown credential does not sign in")

	ctx := context.Background()
	require.ErrorIs(t, scene.passkeys.RevokeAccountPasskeyCredential(ctx, other, passkeyID(t, first)), identityaccess.ErrPasskeyNotFound,
		"an account cannot revoke another account's passkey")
	require.NoError(t, scene.passkeys.RevokeAccountPasskeyCredential(ctx, account, passkeyID(t, first)))
	require.ErrorIs(t, scene.passkeys.RevokeAccountPasskeyCredential(ctx, account, passkeyID(t, first)), identityaccess.ErrPasskeyNotFound)
	_, err = scene.login(ctx, t, laptop)
	require.ErrorIs(t, err, identityaccess.ErrInvalidCredentials, "a revoked passkey never verifies")
	listed = scene.list(t, account)
	require.Len(t, listed, 1)
	assert.Equal(t, second.ID, listed[0].ID)
}

// A credential ID is registered once platform-wide. The refused insert is no
// rejection of the ceremony: both writes roll back, so the same ceremony
// completes with another authenticator and is spent only then.
func TestAccountPasskeyRegistrationRollsBackWhenTheCredentialIsTaken(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	scene := newAccountPasskeyScene(t, db)
	account, other := scene.member(t), scene.member(t)
	taken, _ := scene.register(t, other, "Taken")

	options := scene.beginRegistration(t, account)
	ctx := context.Background()
	_, err := scene.finishRegistration(ctx, t, account, options, taken, "Replay")
	require.ErrorContains(t, err, "uniq_passkey_credentials_credential_id", "a credential ID is registered once platform-wide")
	assert.Empty(t, scene.list(t, account), "the refused insert stores no credential")

	fresh := newPasskeyAuthenticator(t)
	summary, err := scene.finishRegistration(ctx, t, account, options, fresh, "Fresh")
	require.NoError(t, err, "the retry finds the ceremony unconsumed")
	require.Len(t, scene.list(t, account), 1)
	assert.Equal(t, "Fresh", summary.Name)
	_, err = scene.finishRegistration(ctx, t, account, options, newPasskeyAuthenticator(t), "Late")
	require.ErrorIs(t, err, identityaccess.ErrPasskeySessionInvalid, "the committed ceremony is spent")
}

// A ceremony completes exactly once, only for its own purpose, only for the
// account that started it and only before it expires.
func TestAccountPasskeySessionIsConsumedOnce(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	scene := newAccountPasskeyScene(t, db)
	account, other := scene.member(t), scene.member(t)
	authenticator, _ := scene.register(t, account, "Laptop")
	ctx := context.Background()

	registration := scene.beginRegistration(t, account)
	_, err := scene.passkeys.FinishAccountPasskeyLogin(ctx, identityaccess.AccountPasskeyLoginFinish{
		SessionID: registration.SessionID, CredentialResponse: authenticator.Assert(t, scene.beginLogin(t, scene.tenantID, scene.subdomain).Options, scene.origin),
	})
	require.ErrorIs(t, err, identityaccess.ErrPasskeySessionInvalid, "a registration ceremony cannot complete a login")
	_, err = scene.finishRegistration(ctx, t, account, registration, newPasskeyAuthenticator(t), "Second")
	require.NoError(t, err, "the wrong purpose left the ceremony unspent")
	_, err = scene.finishRegistration(ctx, t, account, registration, newPasskeyAuthenticator(t), "Replay")
	require.ErrorIs(t, err, identityaccess.ErrPasskeySessionInvalid, "a replayed completion is refused")

	foreign := scene.beginRegistration(t, other)
	_, err = scene.finishRegistration(ctx, t, account, foreign, newPasskeyAuthenticator(t), "Stolen")
	require.ErrorIs(t, err, identityaccess.ErrPasskeySessionInvalid, "a ceremony completes only for the account that started it")
	assert.Len(t, scene.list(t, account), 2)

	login := scene.beginLogin(t, scene.tenantID, scene.subdomain)
	finish := identityaccess.AccountPasskeyLoginFinish{SessionID: login.SessionID, CredentialResponse: authenticator.Assert(t, login.Options, scene.origin)}
	_, err = scene.passkeys.FinishAccountPasskeyLogin(ctx, finish)
	require.NoError(t, err, "a discoverable login names no account and completes")
	_, err = scene.passkeys.FinishAccountPasskeyLogin(ctx, finish)
	require.ErrorIs(t, err, identityaccess.ErrPasskeySessionInvalid, "a replayed login is refused")

	expired := scene.beginLogin(t, scene.tenantID, scene.subdomain)
	_, err = db.NewRaw("UPDATE auth.passkey_sessions SET expires_at = now() - interval '1 minute' WHERE id = ?", expired.SessionID).Exec(ctx)
	require.NoError(t, err)
	_, err = scene.passkeys.FinishAccountPasskeyLogin(ctx, identityaccess.AccountPasskeyLoginFinish{
		SessionID: expired.SessionID, CredentialResponse: authenticator.Assert(t, expired.Options, scene.origin),
	})
	require.ErrorIs(t, err, identityaccess.ErrPasskeySessionInvalid, "an expired ceremony is refused")
	_, err = scene.passkeys.FinishAccountPasskeyLogin(ctx, identityaccess.AccountPasskeyLoginFinish{
		SessionID: "unknown-" + expired.SessionID, CredentialResponse: authenticator.Assert(t, expired.Options, scene.origin),
	})
	require.ErrorIs(t, err, identityaccess.ErrPasskeySessionInvalid)
}

// A passkey belongs to the account, not to a school: a tenant context must
// neither hide nor reveal one. A ceremony keeps the school whose portal
// started it, and the login decides the school binding from that row, not
// from the connection.
func TestAccountPasskeyRecordsAreVisibleFromEveryTenantContext(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	scene := newAccountPasskeyScene(t, db)
	account := scene.member(t)
	authenticator, credential := scene.register(t, account, "Laptop")
	foreignTenant, foreignSubdomain := testpkg.CreateTestTenant(t, db)

	for name, tenantID := range map[string]int64{"own tenant": scene.tenantID, "foreign tenant": foreignTenant} {
		t.Run(name, func(t *testing.T) {
			tenantCtx := testpkg.TenantContext(tenantID)
			listed, err := scene.passkeys.ListAccountPasskeyCredentials(tenantCtx, account)
			require.NoError(t, err)
			require.Len(t, listed, 1)
			assert.Equal(t, credential.ID, listed[0].ID)
			_, err = scene.login(tenantCtx, t, authenticator)
			require.NoError(t, err, "a ceremony of the account's school completes in any tenant context")
		})
	}

	// The same passkey at the portal of a school the account does not belong
	// to: the ceremony carries that school and refuses, whatever the context.
	options := scene.beginLogin(t, foreignTenant, foreignSubdomain)
	_, err := scene.passkeys.FinishAccountPasskeyLogin(testpkg.TenantContext(scene.tenantID), identityaccess.AccountPasskeyLoginFinish{
		SessionID: options.SessionID, CredentialResponse: authenticator.Assert(t, options.Options, "http://"+foreignSubdomain+".localhost"),
	})
	require.ErrorIs(t, err, identityaccess.ErrTenantAccessDenied, "the recorded school is the one that started the ceremony")
}

// A passkey revoked while its login is being verified is not updated: the
// login sees the credential active, the revocation commits first, and the
// use then finds no active credential. The login fails instead of minting.
func TestAccountPasskeyRevokedDuringALoginIsNotUpdated(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	scene := newAccountPasskeyScene(t, db)
	account := scene.member(t)
	authenticator, credential := scene.register(t, account, "Laptop")
	id := passkeyID(t, credential)

	options := scene.beginLogin(t, scene.tenantID, scene.subdomain)
	finish := identityaccess.AccountPasskeyLoginFinish{SessionID: options.SessionID, CredentialResponse: authenticator.Assert(t, options.Options, scene.origin)}
	loginErr := finishDuringRevocation(t, db, "auth.passkey_credentials", id, func() error {
		_, err := scene.passkeys.FinishAccountPasskeyLogin(context.Background(), finish)
		return err
	})
	require.ErrorIs(t, loginErr, identityaccess.ErrAccountPasskeyNotFound)
	assert.Empty(t, scene.list(t, account))
	assert.Zero(t, storedSignCount(t, db, "auth.passkey_credentials", id), "the revoked credential keeps its state")
}

// finishDuringRevocation revokes a credential in a transaction that commits
// only once finish waits for that transaction's row lock, and returns what
// finish answered. finish runs on its own goroutine and must not use t.
func finishDuringRevocation(t *testing.T, db *bun.DB, table string, credentialID int64, finish func() error) error {
	t.Helper()
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	var revoker int
	require.NoError(t, tx.NewRaw("UPDATE "+table+" SET revoked_at = now() WHERE id = ? RETURNING pg_backend_pid()", credentialID).Scan(ctx, &revoker))

	done := make(chan error, 1)
	go func() { done <- finish() }()
	require.Eventually(t, func() bool {
		var waiting int
		err := db.NewRaw("SELECT count(*) FROM pg_stat_activity WHERE ? = ANY(pg_blocking_pids(pid))", revoker).Scan(ctx, &waiting)
		return err == nil && waiting > 0
	}, 10*time.Second, 10*time.Millisecond, "the login must reach the credential row")
	require.NoError(t, tx.Commit())
	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		t.Fatal("the login did not finish after the revocation committed")
		return nil
	}
}
