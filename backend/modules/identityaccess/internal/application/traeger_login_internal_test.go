package application

import (
	"context"
	"log/slog"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The Träger sign-in checks the credentials in a committed unit before it
// mints. The mint guard locks and re-reads the account, so an account
// switched off in between gets no Träger session (#4649).

type traegerMembershipsStub struct {
	organizations []identityaccess.TraegerOrganization
}

func (s traegerMembershipsStub) ListAccountTraegerOrganizations(context.Context, int64) ([]identityaccess.TraegerOrganization, error) {
	return s.organizations, nil
}

// runTraegerMintGuard runs the guard of an authentication built over the
// fixture's store, with one Träger membership of the account.
func runTraegerMintGuard(t *testing.T, f *authFixture, presented domain.LoginAccount) (*domain.AccountClaimsPayload, error) {
	t.Helper()
	auth, err := NewAccountAuthentication(f.sessions, AccountAuthenticationDependencies{
		Store: f.store, Schools: f.schools, Persons: f.persons, Passwords: fakePasswords{}, PasswordPolicy: testPasswordLoginPolicy, Codec: fakeCodec{}, PortalModules: f.portals,
		MFA: f.mfa, MFALock: f.mfaLock, Audit: f.audit, Push: f.push, Runtime: f.runtime, Rotation: fakeRotation{},
		Traeger: traegerMembershipsStub{organizations: []identityaccess.TraegerOrganization{{ID: 7, Name: "Träger"}}},
		Logger:  slog.New(slog.NewTextHandler(testWriter{t}, nil)),
	})
	require.NoError(t, err)
	var claims *domain.AccountClaimsPayload
	err = f.runtime.WithAdminTx(context.Background(), func(txCtx context.Context) error {
		return auth.traegerMintGuard(7, &claims)(txCtx, presented)
	})
	return claims, err
}

func TestTraegerMintGuard_MintsForALiveMember(t *testing.T) {
	t.Parallel()
	f := newAuthFixture(t)
	f.store.addAccount(1, "buero@traeger.example", "hash:secret", true)

	claims, err := runTraegerMintGuard(t, f, f.store.accounts[1])
	require.NoError(t, err)
	require.NotNil(t, claims)
	assert.Equal(t, domain.ScopeOrg, claims.Scope)
	assert.Equal(t, int64(7), claims.OrgID)
	assert.Contains(t, f.store.recorded(), "FindLoginAccount(forUpdate)", "the guard locks the account row")
}

func TestTraegerMintGuard_RefusesAnAccountSwitchedOffAfterTheCredentialCheck(t *testing.T) {
	t.Parallel()
	f := newAuthFixture(t)
	f.store.addAccount(1, "buero@traeger.example", "hash:secret", true)
	presented := f.store.accounts[1]
	f.store.addAccount(1, "buero@traeger.example", "hash:secret", false)

	claims, err := runTraegerMintGuard(t, f, presented)
	require.ErrorIs(t, err, identityaccess.ErrAccountInactive)
	assert.Nil(t, claims)
}
