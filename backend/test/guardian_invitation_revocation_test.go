package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	authRepo "github.com/moto-nrw/project-phoenix/database/repositories/auth"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// seedGuardianProfile inserts a guardian profile owned by the test tenant, so
// CleanupTenantTestData removes it together with everything else.
func seedGuardianProfile(t *testing.T, db *bun.DB, tenantID int64, label string) *users.GuardianProfile {
	t.Helper()

	email := fmt.Sprintf("%s-%d@revocation.test", label, time.Now().UnixNano())
	profile := &users.GuardianProfile{
		FirstName:              "Revocation",
		LastName:               "Test",
		Email:                  &email,
		PreferredContactMethod: "email",
		LanguagePreference:     "de",
	}
	profile.SetTenantID(tenantID)

	_, err := db.NewInsert().
		Model(profile).
		ModelTableExpr(`users.guardian_profiles`).
		Exec(context.Background())
	require.NoError(t, err)
	return profile
}

// seedGuardianInvitation inserts an invitation for a guardian profile.
func seedGuardianInvitation(t *testing.T, db *bun.DB, tenantID, profileID int64, accepted bool) *authModels.GuardianInvitation {
	t.Helper()

	invitation := &authModels.GuardianInvitation{
		Token:             fmt.Sprintf("tok-%d-%d", profileID, time.Now().UnixNano()),
		GuardianProfileID: profileID,
		CreatedBy:         1,
		ExpiresAt:         time.Now().Add(72 * time.Hour),
		ApprovalStatus:    authModels.GuardianInvitationApprovalNotRequired,
	}
	if accepted {
		now := time.Now()
		invitation.AcceptedAt = &now
	}
	invitation.SetTenantID(tenantID)

	_, err := db.NewInsert().
		Model(invitation).
		ModelTableExpr(`auth.guardian_invitations AS "guardian_invitation"`).
		Exec(context.Background())
	require.NoError(t, err)
	return invitation
}

// A token minted for a mistyped address is a live credential sitting in a
// stranger's inbox: if that typo resolves to a real mailbox, accepting the
// invitation grants access to the family's child. Revocation is the fix, and
// this test is its guard.
func TestRevokeOpenByGuardianProfileID_InvalidatesOpenTokensOnly(t *testing.T) {
	db := SetupTestDB(t)
	defer func() { _ = db.Close() }()

	scope := NewTenantScope(t, db)
	defer CleanupTenantTestData(t, db, scope.TenantID)

	profile := seedGuardianProfile(t, db, scope.TenantID, "revocation")

	open := seedGuardianInvitation(t, db, scope.TenantID, profile.ID, false)
	acceptedInvite := seedGuardianInvitation(t, db, scope.TenantID, profile.ID, true)

	repo := authRepo.NewGuardianInvitationRepository(db)
	ctx := TenantContext(scope.TenantID)

	ids, err := repo.RevokeOpenByGuardianProfileID(ctx, profile.ID, "guardian email changed")
	require.NoError(t, err)

	require.Len(t, ids, 1, "only the open invitation may be revoked")
	assert.Equal(t, open.ID, ids[0])

	revoked, err := repo.FindByID(ctx, open.ID)
	require.NoError(t, err)
	require.NotNil(t, revoked.RevokedAt, "the open token must carry a revocation timestamp")
	require.NotNil(t, revoked.RevokedReason)
	assert.Equal(t, "guardian email changed", *revoked.RevokedReason)

	// An already-accepted invitation is history, not a live credential —
	// rewriting it would corrupt the audit trail for no benefit.
	untouched, err := repo.FindByID(ctx, acceptedInvite.ID)
	require.NoError(t, err)
	assert.Nil(t, untouched.RevokedAt, "an accepted invitation must not be revoked")
}

// Revocation must be idempotent: a second address correction must not
// re-stamp (and thus re-date) an already-revoked token.
func TestRevokeOpenByGuardianProfileID_IsIdempotent(t *testing.T) {
	db := SetupTestDB(t)
	defer func() { _ = db.Close() }()

	scope := NewTenantScope(t, db)
	defer CleanupTenantTestData(t, db, scope.TenantID)

	profile := seedGuardianProfile(t, db, scope.TenantID, "revocation-twice")

	seedGuardianInvitation(t, db, scope.TenantID, profile.ID, false)

	repo := authRepo.NewGuardianInvitationRepository(db)
	ctx := TenantContext(scope.TenantID)

	first, err := repo.RevokeOpenByGuardianProfileID(ctx, profile.ID, "first change")
	require.NoError(t, err)
	require.Len(t, first, 1)

	second, err := repo.RevokeOpenByGuardianProfileID(ctx, profile.ID, "second change")
	require.NoError(t, err)
	assert.Empty(t, second, "an already-revoked token must not be revoked again")
}
