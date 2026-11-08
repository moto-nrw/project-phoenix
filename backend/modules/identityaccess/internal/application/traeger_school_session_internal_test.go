package application

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// admittingTraegerSchool is an admitting Träger school guard, so the tests
// reach the proof re-check of the Absprung.
func admittingTraegerSchool() map[domain.SessionKind]sessionPolicy {
	return admitting(domain.SessionKindTraegerSchool, func(context.Context, domain.LoginAccount) (*domain.AccountClaimsPayload, error) {
		return &domain.AccountClaimsPayload{TenantID: 10, OrgID: 7, ViaTraeger: true}, nil
	})
}

// runTraegerSchoolHandoffGuard mints the Absprung session of the fixture.
func runTraegerSchoolHandoffGuard(t *testing.T, f *authFixture, familyID string, proof bool) error {
	t.Helper()
	f.store.addAccount(1, "buero@traeger.example", "hash:secret", true)
	_, err := f.auth.mintAccountSession(context.Background(), mintRequest{
		portal: domain.PortalTenant, account: f.store.accounts[1], tenantID: 10, orgID: 7,
		proof:          mintProof{kind: proofTraegerSession, traegerFamilyID: familyID, traegerVerified: proof},
		sourceFamilyID: familyID, lifetime: domain.TraegerSchoolHandoffTTL,
	})
	return err
}

// TestTraegerSchoolHandoffGuard_RefusesARevokedTraegerFamilyWithoutMFA pins
// the Absprung side of #5422: without a configured second factor the
// missing proof never compared, so a Träger session logged out between
// the request and the mint still opened a school session.
func TestTraegerSchoolHandoffGuard_RefusesARevokedTraegerFamilyWithoutMFA(t *testing.T) {
	t.Parallel()
	f := newAuthFixture(t)
	require.False(t, f.mfa.Configured(), "the fixture runs without a second factor")

	f.auth.policies = admittingTraegerSchool()
	err := runTraegerSchoolHandoffGuard(t, f, "logged-out-traeger-family", false)

	require.ErrorIs(t, err, identityaccess.ErrInvalidToken, "no school session out of a revoked Träger family")
}

// TestTraegerSchoolHandoffGuard_AdmitsALiveTraegerFamily keeps the
// Absprung of a live Träger session open.
func TestTraegerSchoolHandoffGuard_AdmitsALiveTraegerFamily(t *testing.T) {
	t.Parallel()
	f := newAuthFixture(t)
	f.store.mu.Lock()
	f.store.sessions[50] = domain.AccountSession{
		ID: 50, AccountID: 1, OrganizationID: 7, Token: "traeger-token", FamilyID: "live-traeger-family",
		PortalScope: domain.PortalOrg.Name, Expiry: time.Now().Add(time.Hour),
	}
	f.store.mu.Unlock()

	f.auth.policies = admittingTraegerSchool()
	err := runTraegerSchoolHandoffGuard(t, f, "live-traeger-family", false)

	assert.NoError(t, err)
}
