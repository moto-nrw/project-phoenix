package behavior_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestSchoolSwitchRequiresLiveSameAccountMFAProof(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	for _, scenario := range []string{"verified", "unverified", "missing", "other-account", "other-portal", "revoked", "expired", "expired-cap", "mfa-reset"} {
		t.Run(scenario, func(t *testing.T) {
			testpkg.OwnTenant(t)
			ctx := context.Background()
			module := newMFATestModule(t, db, withDeliveringMailer(nil)...)
			service := newFixtureAuthService(t, db, module.Auth)
			source, _ := newSchoolTenant(t, db)
			target, targetSlug := newSchoolTenant(t, db)
			email, accountID := createLehrkraftAccount(t, db, service, "school-switch-proof", source)
			testpkg.EnsureAccountTenant(t, db, accountID, target)
			testpkg.AssignLehrkraftSystemRole(t, db, accountID, target)
			testpkg.GrantTestPermission(t, db, target, accountID, "staff:financial")
			var access, refresh string
			var err error
			switch scenario {
			case "unverified":
				login, loginErr := service.LoginSchoolWithMFAGate(ctx, email, testPassword, "", "", "")
				require.NoError(t, loginErr)
				refresh = login.RefreshToken
			case "other-account":
				_, otherID := createLehrkraftAccount(t, db, service, "other-school-switch", source)
				_, refresh, err = service.IssueSchoolTokensForAuthenticatedAccount(ctx, otherID, source, "", "")
			default:
				_, refresh, err = service.IssueSchoolTokensForAuthenticatedAccount(ctx, accountID, source, "", "")
			}
			require.NoError(t, err)
			// Rotate before switching: proof belongs to the latest stored generation.
			access, refresh, err = service.RefreshTokenWithAudit(ctx, refresh, "", "", "school")
			require.NoError(t, err)
			claims, err := service.ValidateSessionTokens(ctx, access, refresh, "school")
			require.NoError(t, err)
			family := claims.FamilyID
			switch scenario {
			case "missing":
				family = ""
			case "revoked":
				require.NoError(t, service.LogoutWithAudit(ctx, refresh, "", ""))
			case "expired":
				_, err := db.ExecContext(ctx, `UPDATE auth.tokens SET expiry = NOW() - INTERVAL '1 second' WHERE family_id = ?`, family)
				require.NoError(t, err)
			case "expired-cap":
				_, err := db.ExecContext(ctx, `UPDATE auth.tokens SET family_expiry_cap = NOW() - INTERVAL '1 second' WHERE family_id = ?`, family)
				require.NoError(t, err)
			case "other-portal":
				_, err := db.ExecContext(ctx, `UPDATE auth.tokens SET portal_scope = 'tenant' WHERE family_id = ?`, family)
				require.NoError(t, err)
			case "mfa-reset":
				require.NoError(t, module.MFA.DisableMFA(testpkg.TenantContext(source), accountID))
			}
			access, refresh, err = service.SwitchSchool(ctx, accountID, targetSlug, switchIP, switchUserAgent, family)
			if scenario != "verified" {
				require.ErrorIs(t, err, identityaccess.ErrInvalidToken)
				require.Empty(t, access)
				require.Empty(t, refresh)
				return
			}
			require.NoError(t, err)
			claims, err = service.ValidateSessionTokens(ctx, access, refresh, "school")
			require.NoError(t, err)
			require.Equal(t, target, claims.TenantID)
			_, _, err = service.RefreshTokenWithAudit(ctx, refresh, "", "", "school")
			require.NoError(t, err, "the switched school session must retain MFA proof")
		})
	}
}
