package application

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// legacyCapAuth builds the fixture's authentication with the given Träger
// memberships of the account.
func legacyCapAuth(t *testing.T, f *authFixture, organizations []identityaccess.TraegerOrganization) *AccountAuthentication {
	t.Helper()
	auth, err := NewAccountAuthentication(f.sessions, AccountAuthenticationDependencies{
		Store: f.store, Schools: f.schools, Persons: f.persons, Passwords: fakePasswords{}, PasswordPolicy: testPasswordLoginPolicy, Codec: fakeCodec{}, PortalModules: f.portals,
		MFA: f.mfa, MFALock: f.mfaLock, Audit: f.audit, Push: f.push, Runtime: f.runtime, Rotation: fakeRotation{},
		Traeger: traegerMembershipsStub{organizations: organizations},
		Logger:  slog.New(slog.NewTextHandler(testWriter{t}, nil)),
	})
	require.NoError(t, err)
	return auth
}

// TestCapLegacyLineage pins which families issued before #5422 keep their
// deadline: those that may descend from a school switch or an Absprung.
func TestCapLegacyLineage(t *testing.T) {
	t.Parallel()
	expiry := time.Now().Add(time.Hour)
	traeger := []identityaccess.TraegerOrganization{{ID: 7, Name: "Träger"}}
	for _, tc := range []struct {
		name          string
		session       domain.AccountSession
		mappings      int
		organizations []identityaccess.TraegerOrganization
		capped        bool
	}{
		{name: "multi-school account", session: domain.AccountSession{PortalScope: domain.PortalScopeTenant}, mappings: 2, capped: true},
		{name: "school portal of a multi-school account", session: domain.AccountSession{PortalScope: domain.PortalScopeSchool}, mappings: 2, capped: true},
		{name: "Träger school session", session: domain.AccountSession{PortalScope: domain.PortalScopeTenant, OrganizationID: 7}, mappings: 0, capped: true},
		{name: "Träger member with one own school", session: domain.AccountSession{PortalScope: domain.PortalScopeTenant}, mappings: 1, organizations: traeger, capped: true},
		{name: "single-school account", session: domain.AccountSession{PortalScope: domain.PortalScopeTenant}, mappings: 1},
		{name: "parent portal", session: domain.AccountSession{PortalScope: domain.PortalScopeParent}, mappings: 2},
		{name: "Träger portal", session: domain.AccountSession{PortalScope: domain.PortalScopeOrg}, organizations: traeger},
		{name: "family issued with its origin", session: domain.AccountSession{PortalScope: domain.PortalScopeTenant, SwitchedFromFamilyID: "family"}, mappings: 2},
		{name: "linked switch family", session: domain.AccountSession{PortalScope: domain.PortalScopeTenant, SwitchedFromFamilyID: "origin"}, mappings: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newAuthFixture(t)
			f.store.addAccount(1, "lehrkraft@example.com", "hash:secret", true)
			for school := int64(10); school < 10+int64(tc.mappings); school++ {
				f.store.addMapping(1, school)
			}
			session := tc.session
			session.AccountID, session.FamilyID, session.Expiry = 1, "family", expiry

			capped, err := legacyCapAuth(t, f, tc.organizations).capLegacyLineage(context.Background(), &session)

			require.NoError(t, err)
			assert.Equal(t, tc.capped, capped)
			if !tc.capped {
				assert.Nil(t, session.FamilyExpiryCap)
				assert.NotEmpty(t, session.SwitchedFromFamilyID, "a family that cannot stem from a switch names an origin and is checked once")
				return
			}
			assert.Empty(t, session.SwitchedFromFamilyID, "a capped family stays unlinked")
			require.NotNil(t, session.FamilyExpiryCap)
			assert.True(t, session.FamilyExpiryCap.Equal(expiry), "the family keeps its current expiry as its deadline")
		})
	}
}

// TestCapLegacyLineage_KeepsAnEarlierDeadline leaves a retired family's
// shorter deadline in place.
func TestCapLegacyLineage_KeepsAnEarlierDeadline(t *testing.T) {
	t.Parallel()
	f := newAuthFixture(t)
	f.store.addAccount(1, "lehrkraft@example.com", "hash:secret", true)
	f.store.addMapping(1, 10)
	f.store.addMapping(1, 11)
	retired := time.Now().Add(time.Minute)
	session := domain.AccountSession{AccountID: 1, FamilyID: "family", PortalScope: domain.PortalScopeTenant, Expiry: retired.Add(time.Hour), FamilyExpiryCap: &retired}

	_, err := f.auth.capLegacyLineage(context.Background(), &session)
	require.NoError(t, err)

	assert.True(t, session.FamilyExpiryCap.Equal(retired))
}
