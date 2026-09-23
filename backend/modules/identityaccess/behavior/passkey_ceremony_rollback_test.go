package behavior_test

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Completing a passkey ceremony writes twice: it consumes the ceremony row and
// then stores the credential. The rule the ceremony carries is that a
// *rejection* commits — the consumed row stays consumed, so a refused
// attestation cannot be retried against the same challenge — while any other
// failure rolls the pair back. The module pins the rule on a fake runtime that
// counts commits and rollbacks; this test pins the half that can be driven
// end to end on the same rows production writes, because a fake runtime cannot
// prove what Postgres committed (#3331; it replaces the DB-backed half of the
// retained TestPasskeyService_LoginCompletionTransaction).
func TestPasskeyRegistration_RefusedAttestationSpendsTheCeremony(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testpkg.SetupTestDB(t)
	module := newMFATestModule(t, db, withDeliveringMailer(nil)...)
	require.NotNil(t, module.Passkeys)

	account := testpkg.CreateTestAccount(t, db, "passkey-ceremony")
	tenantID, subdomain := testpkg.CreateTestTenant(t, db)
	testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)

	// The registration is gated on an e-mail code of this school's portal.
	const code = "246813"
	seedTenantChallenge(t, nativeMFARecords(t, db), account.ID, tenantID, code)

	options, err := module.Passkeys.BeginAccountPasskeyRegistration(ctx, identityaccess.AccountPasskeyRegistrationStart{
		AccountID:       account.ID,
		TenantID:        tenantID,
		TenantSubdomain: subdomain,
		ExpectedOrigin:  "http://" + subdomain + ".localhost",
		Code:            code,
		Name:            "Test key",
	})
	require.NoError(t, err, "a valid code starts the ceremony")
	require.NotEmpty(t, options.SessionID)

	// An attestation the relying party refuses. The ceremony is a rejection,
	// so its consumption must commit.
	_, err = module.Passkeys.FinishAccountPasskeyRegistration(ctx, identityaccess.AccountPasskeyRegistrationFinish{
		AccountID:          account.ID,
		SessionID:          options.SessionID,
		CredentialResponse: json.RawMessage(`{"id":"not-a-credential"}`),
		Name:               "Test key",
	})
	require.Error(t, err, "a refused attestation must not register a credential")

	// Proof against the database: the same ceremony cannot be replayed, and
	// no credential was stored.
	_, err = module.Passkeys.FinishAccountPasskeyRegistration(ctx, identityaccess.AccountPasskeyRegistrationFinish{
		AccountID:          account.ID,
		SessionID:          options.SessionID,
		CredentialResponse: json.RawMessage(`{"id":"not-a-credential"}`),
		Name:               "Test key",
	})
	require.ErrorIs(t, err, identityaccess.ErrPasskeySessionInvalid,
		"the refused ceremony stays spent — its consumption committed")

	credentials, err := module.Passkeys.ListAccountPasskeyCredentials(ctx, account.ID)
	require.NoError(t, err)
	assert.Empty(t, credentials, "a refused attestation stores no credential")
}

// seedTenantChallenge writes an active tenant-portal challenge with a known
// code, so a test can drive a flow that redeems one.
func seedTenantChallenge(t *testing.T, repo services.AccountMFARecords, accountID, tenantID int64, code string) identityaccess.AccountMFAChallenge {
	t.Helper()
	hash := testpkg.HashTestPassword(t, code)
	challenge := identityaccess.AccountMFAChallenge{
		AccountID: accountID,
		TenantID:  tenantID,
		Scope:     identityaccess.MFAChallengeScopeTenant,
		CodeHash:  hash,
		ExpiresAt: time.Now().Add(identityaccess.MFAChallengeTTL),
		IPAddress: net.ParseIP("203.0.113.11"),
	}
	stored, err := repo.CreateChallenge(context.Background(), challenge)
	require.NoError(t, err)
	return stored
}
