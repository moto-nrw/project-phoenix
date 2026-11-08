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

// runTraegerSchoolHandoffGuard mints the Absprung session of the fixture.
func runTraegerSchoolHandoffGuard(t *testing.T, f *authFixture, familyID string, proof bool) error {
	t.Helper()
	f.store.addAccount(1, "buero@traeger.example", "hash:secret", true)
	f.schools.add(10, 7, "school-10", true, false)
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

	f.auth.store, f.auth.traeger = admittedStore{fakeStore: f.store}, validationMemberships{}
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

	f.auth.store, f.auth.traeger = admittedStore{fakeStore: f.store}, validationMemberships{}
	err := runTraegerSchoolHandoffGuard(t, f, "live-traeger-family", false)

	assert.NoError(t, err)
}
