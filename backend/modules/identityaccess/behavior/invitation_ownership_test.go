package behavior_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"

	authModels "github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/authmodels"
	authjwt "github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestInvitationTokenCannotTakeOverExistingAccount(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := setupInvitationService(t, db)
	repos, compositionErr := repositories.NewInvitationPersistence(db)
	require.NoError(t, compositionErr)
	owner := testpkg.CreateTestAccountWithPassword(t, db, fmt.Sprintf("invitation-owner-%d@example.com", testpkg.Tenant(t)), testPassword)
	original, err := repos.Account.FindByID(context.Background(), owner.ID)
	require.NoError(t, err)
	schoolA := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, schoolA)
	role := testpkg.CreateTestRoleForTenant(t, db, "invited-staff", schoolA)
	creator := testpkg.CreateTestAccount(t, db, "invitation-creator")
	invitation, err := service.CreateSchoolInvitation(testpkg.TenantContext(schoolA), identityaccess.SchoolInvitationRequest{
		Email: owner.Email, RoleID: role.ID, CreatedBy: creator.ID,
		FirstName: testpkg.StrPtr("Invited"), LastName: testpkg.StrPtr("Owner"),
	})
	require.NoError(t, err)
	_, acceptErr := service.AcceptSchoolInvitation(context.Background(), invitation.Token, identityaccess.InvitationRegistration{
		Password: testNewPassword, ConfirmPassword: testNewPassword,
	})
	stored, err := repos.Account.FindByID(context.Background(), owner.ID)
	require.NoError(t, err)
	require.Equal(t, original.PasswordHash, stored.PasswordHash, "invitation must not replace global credentials")
	require.Equal(t, original.Active, stored.Active)
	require.Error(t, acceptErr, "existing accounts require authenticated ownership")
	exists, err := testpkg.ActiveAccountTenantExists(context.Background(), db, owner.ID, schoolA)
	require.NoError(t, err)
	require.False(t, exists)
	_, err = service.ValidateSchoolInvitation(context.Background(), invitation.Token)
	require.NoError(t, err, "rejection must not consume the invitation")
}

func TestInvitationExistingOwnerProof(t *testing.T) {
	t.Parallel()
	for _, scope := range []string{"", "tenant", "org", "parent", "school", "platform", "unknown"} {
		t.Run("scope="+scope, func(t *testing.T) {
			testpkg.OwnTenant(t)
			db := testpkg.SetupTestDB(t)
			service := setupInvitationService(t, db)
			repos, compositionErr := repositories.NewInvitationPersistence(db)
			require.NoError(t, compositionErr)
			owner := testpkg.CreateTestAccountWithPassword(t, db, fmt.Sprintf("owner-%d@example.com", testpkg.Tenant(t)), testPassword)
			records := nativeMFARecords(t, db)
			require.NoError(t, records.CreateCredential(context.Background(), identityaccess.AccountMFACredential{AccountID: owner.ID, Method: "email"}))
			credential, found, err := records.FindCredential(context.Background(), owner.ID)
			require.NoError(t, err)
			require.True(t, found)
			schoolA := testpkg.UniqueTestTenantID(t)
			testpkg.EnsureTestTenant(t, db, schoolA)
			role := testpkg.CreateTestRoleForTenant(t, db, "invited-staff", schoolA)
			creator := testpkg.CreateTestAccount(t, db, "invitation-creator")
			invitation, err := service.CreateSchoolInvitation(testpkg.TenantContext(schoolA), identityaccess.SchoolInvitationRequest{
				Email: owner.Email, RoleID: role.ID, CreatedBy: creator.ID,
				FirstName: testpkg.StrPtr("Invited"), LastName: testpkg.StrPtr("Owner"),
			})
			require.NoError(t, err)
			signer, err := authjwt.NewTokenAuthWithDurations(authTestFactoryConfig(false).JWTSecret, 15*time.Minute, 24*time.Hour)
			require.NoError(t, err)
			accessToken, err := signer.CreateJWT(authjwt.AppClaims{
				ID: int(owner.ID), Sub: owner.Email, Roles: []string{}, Scope: scope,
				TenantID: testpkg.Tenant(t),
			})
			require.NoError(t, err)
			accepted, acceptErr := service.AcceptSchoolInvitation(context.Background(), invitation.Token, identityaccess.InvitationRegistration{OwnerAccessToken: accessToken})
			allowed := scope != "platform" && scope != "unknown"
			if allowed {
				require.NoError(t, acceptErr)
				require.Equal(t, owner.ID, accepted.ID)
				_, replayErr := service.AcceptSchoolInvitation(context.Background(), invitation.Token, identityaccess.InvitationRegistration{OwnerAccessToken: accessToken})
				require.ErrorIs(t, replayErr, identityaccess.ErrInvitationUsed)
			} else {
				require.ErrorIs(t, acceptErr, identityaccess.ErrInvitationOwnerRequired)
			}
			stored, err := repos.Account.FindByID(context.Background(), owner.ID)
			require.NoError(t, err)
			require.Equal(t, owner.PasswordHash, stored.PasswordHash)
			require.True(t, stored.Active)
			storedCredential, found, err := records.FindCredential(context.Background(), owner.ID)
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, credential, storedCredential)
			joined, err := testpkg.ActiveAccountTenantExists(context.Background(), db, owner.ID, schoolA)
			require.NoError(t, err)
			require.Equal(t, allowed, joined)
			originalMembership, err := testpkg.ActiveAccountTenantExists(context.Background(), db, owner.ID, testpkg.Tenant(t))
			require.NoError(t, err)
			require.True(t, originalMembership)
		})
	}
}

func TestInvitationRejectsInvalidOwnerProofWithoutWrites(t *testing.T) {
	t.Parallel()
	cases := []string{"missing", "forged", "expired", "no-expiry", "mfa-pending", "mfa-enrollment", "preview", "acting-admin", "preview-id", "disabled"}
	for _, scope := range []string{"", "tenant", "org", "parent", "school"} {
		cases = append(cases, "wrong-owner:"+scope)
	}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			db := testpkg.SetupTestDB(t)
			service := setupInvitationService(t, db)
			repos, compositionErr := repositories.NewInvitationPersistence(db)
			require.NoError(t, compositionErr)
			owner := testpkg.CreateTestAccountWithPassword(t, db, fmt.Sprintf("rejected-owner-%d@example.com", testpkg.Tenant(t)), testPassword)
			schoolA := testpkg.UniqueTestTenantID(t)
			testpkg.EnsureTestTenant(t, db, schoolA)
			role := testpkg.CreateTestRoleForTenant(t, db, "invited-staff", schoolA)
			invitation := &testpkg.InvitationTokenFixture{
				Email: owner.Email, RoleID: role.ID, Token: fmt.Sprintf("owner-proof-%d", schoolA),
				FirstName: testpkg.StrPtr("Invited"), LastName: testpkg.StrPtr("Owner"), ExpiresAt: time.Now().Add(time.Hour),
			}
			invitation.TenantID = schoolA
			testpkg.InsertTestInvitationToken(t, db, invitation)
			claims := map[string]any{"id": owner.ID, "sub": owner.Email, "roles": []string{}, "tenant_id": testpkg.Tenant(t), "exp": time.Now().Add(time.Hour).Unix()}
			wantErr := identityaccess.ErrInvitationOwnerRequired
			signer := schoolTokenAuth(t)
			switch name {
			case "expired":
				claims["exp"] = time.Now().Add(-time.Hour).Unix()
			case "no-expiry":
				delete(claims, "exp")
			case "mfa-pending":
				claims["mfa_pending"] = true
			case "mfa-enrollment":
				claims["mfa_enrollment_pending"] = true
			case "preview":
				claims["read_only"] = true
			case "acting-admin":
				claims["acting_admin_id"] = owner.ID
			case "preview-id":
				claims["preview_id"] = "preview-proof"
			case "disabled":
				owner.Active = false
				updateErr := repos.Account.Update(context.Background(), owner)
				require.NoError(t, updateErr)
				wantErr = identityaccess.ErrAccountInactive
			case "forged":
				var err error
				signer, err = authjwt.NewTokenAuthWithSecret("different-test-signing-secret-not-authorized")
				require.NoError(t, err)
			}
			if strings.HasPrefix(name, "wrong-owner:") {
				other := testpkg.CreateTestAccount(t, db, "different-owner")
				claims["id"] = other.ID
				claims["scope"] = strings.TrimPrefix(name, "wrong-owner:")
				wantErr = identityaccess.ErrInvitationOwnerMismatch
			}
			_, proof, err := signer.JwtAuth.Encode(claims)
			require.NoError(t, err)
			if name == "missing" {
				proof = ""
			}
			_, err = service.AcceptSchoolInvitation(context.Background(), invitation.Token, identityaccess.InvitationRegistration{OwnerAccessToken: proof, Password: testNewPassword, ConfirmPassword: testNewPassword})
			require.ErrorIs(t, err, wantErr)
			stored, err := repos.Account.FindByID(context.Background(), owner.ID)
			require.NoError(t, err)
			require.Equal(t, owner.PasswordHash, stored.PasswordHash)
			require.Equal(t, name != "disabled", stored.Active)
			joined, err := testpkg.ActiveAccountTenantExists(context.Background(), db, owner.ID, schoolA)
			require.NoError(t, err)
			require.False(t, joined)
			_, err = service.ValidateSchoolInvitation(context.Background(), invitation.Token)
			require.NoError(t, err)
		})
	}
}

func TestInvitationLifecycleAndImportedSchoolIdentity(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"new", "new-school", "existing-school", "expired-new", "expired-existing", "revoked-new", "revoked-existing"} {
		t.Run(name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			db := testpkg.SetupTestDB(t)
			service := setupInvitationService(t, db)
			repos, compositionErr := repositories.NewInvitationPersistence(db)
			require.NoError(t, compositionErr)
			ctx := testpkg.Ctx(t)
			roleID := testpkg.CreateTestRole(t, db, "invited-staff").ID
			if strings.Contains(name, "school") {
				roleID = systemRoleID(t, db, "lehrkraft")
			}
			email := fmt.Sprintf("lifecycle-%d@example.com", testpkg.Tenant(t))
			var owner *authModels.Account
			var proof string
			if strings.Contains(name, "existing") {
				owner = testpkg.CreateTestAccountWithPassword(t, db, email, testPassword)
				var err error
				proof, err = schoolTokenAuth(t).CreateJWT(authjwt.AppClaims{ID: int(owner.ID), Sub: owner.Email, Roles: []string{}, Scope: "school", TenantID: testpkg.Tenant(t)})
				require.NoError(t, err)
			}
			// The import creates a person before sending the invitation.
			person := testpkg.CreateTestPerson(t, db, "Imported", "Staff")
			invitation := &testpkg.InvitationTokenFixture{
				Email: email, RoleID: roleID, Token: fmt.Sprintf("lifecycle-%d", testpkg.Tenant(t)), PersonID: &person.ID,
				FirstName: &person.FirstName, LastName: &person.LastName, ExpiresAt: time.Now().Add(time.Hour),
			}
			invitation.TenantID = testpkg.Tenant(t)
			if strings.HasPrefix(name, "expired") {
				invitation.ExpiresAt = time.Now().Add(-time.Hour)
			}
			testpkg.InsertTestInvitationToken(t, db, invitation)
			if strings.HasPrefix(name, "revoked") {
				creator := testpkg.CreateTestAccount(t, db, "revoking-creator")
				require.NoError(t, service.RevokeSchoolInvitation(ctx, invitation.ID, creator.ID))
			}
			data := identityaccess.InvitationRegistration{OwnerAccessToken: proof, Password: testPassword, ConfirmPassword: testPassword}
			account, err := service.AcceptSchoolInvitation(context.Background(), invitation.Token, data)
			if strings.HasPrefix(name, "expired") || strings.HasPrefix(name, "revoked") {
				if strings.HasPrefix(name, "expired") {
					require.ErrorIs(t, err, identityaccess.ErrInvitationExpired)
				} else {
					require.ErrorIs(t, err, identityaccess.ErrInvitationUsed)
				}
				require.Zero(t, account.ID, "a refused acceptance hands back no account")
				storedPerson, findErr := repos.Person.FindByID(ctx, person.ID)
				require.NoError(t, findErr)
				require.Nil(t, storedPerson.AccountID)
				if owner == nil {
					_, findErr = repos.Account.FindByEmail(context.Background(), email)
					require.Error(t, findErr)
				}
				return
			}
			require.NoError(t, err)
			testpkg.OwnTestAccount(t, db, account.ID)
			storedPerson, err := repos.Person.FindByID(ctx, person.ID)
			require.NoError(t, err)
			require.NotNil(t, storedPerson.AccountID)
			require.Equal(t, account.ID, *storedPerson.AccountID)
			if owner != nil {
				// The acceptance grants membership, never authority over the
				// account's global credential. The owner's stored hash is the
				// evidence; the answer itself carries no credential.
				stored, storedErr := repos.Account.FindByID(context.Background(), owner.ID)
				require.NoError(t, storedErr)
				require.Equal(t, owner.PasswordHash, stored.PasswordHash)
				require.Equal(t, owner.ID, account.ID)
			}
			_, err = service.AcceptSchoolInvitation(context.Background(), invitation.Token, data)
			require.ErrorIs(t, err, identityaccess.ErrInvitationUsed)
		})
	}
}

func TestInvitationConcurrentOwnerAcceptanceIsSingleUse(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := setupInvitationService(t, db)
	repos, compositionErr := repositories.NewInvitationPersistence(db)
	require.NoError(t, compositionErr)
	owner := testpkg.CreateTestAccountWithPassword(t, db, fmt.Sprintf("concurrent-owner-%d@example.com", testpkg.Tenant(t)), testPassword)
	schoolA := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, schoolA)
	role := testpkg.CreateTestRoleForTenant(t, db, "concurrent-staff", schoolA)
	invitation := &testpkg.InvitationTokenFixture{
		Email: owner.Email, RoleID: role.ID, Token: fmt.Sprintf("concurrent-%d", schoolA),
		FirstName: testpkg.StrPtr("Invited"), LastName: testpkg.StrPtr("Owner"), ExpiresAt: time.Now().Add(time.Hour),
	}
	invitation.TenantID = schoolA
	testpkg.InsertTestInvitationToken(t, db, invitation)
	proof, err := schoolTokenAuth(t).CreateJWT(authjwt.AppClaims{ID: int(owner.ID), Sub: owner.Email, Roles: []string{}, TenantID: testpkg.Tenant(t)})
	require.NoError(t, err)
	start := make(chan struct{})
	results := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			_, acceptErr := service.AcceptSchoolInvitation(context.Background(), invitation.Token, identityaccess.InvitationRegistration{OwnerAccessToken: proof})
			results <- acceptErr
		}()
	}
	close(start)
	successes := 0
	for range 2 {
		if <-results == nil {
			successes++
		}
	}
	require.Equal(t, 1, successes)
	var assignments int
	require.NoError(t, db.NewSelect().TableExpr("auth.account_roles").ColumnExpr("COUNT(*)").Where("account_id = ? AND tenant_id = ?", owner.ID, schoolA).Scan(context.Background(), &assignments))
	require.Equal(t, 1, assignments)
	stored, err := repos.Account.FindByID(context.Background(), owner.ID)
	require.NoError(t, err)
	require.Equal(t, owner.PasswordHash, stored.PasswordHash)
}
