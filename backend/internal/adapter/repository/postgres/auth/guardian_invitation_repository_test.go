package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres"
	"github.com/moto-nrw/project-phoenix/models/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// GuardianInvitationRepository CRUD Tests
// ============================================================================

func TestGuardianInvitationRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GuardianInvitation
	ctx := context.Background()

	t.Run("creates invitation with valid data", func(t *testing.T) {
		// Create a guardian profile first
		guardian := testpkg.CreateTestGuardianProfile(t, db, "invite-test")
		defer testpkg.CleanupActivityFixtures(t, db, guardian.ID)

		invitation := &auth.GuardianInvitation{
			GuardianProfileID: guardian.ID,
			Token:             uuid.Must(uuid.NewV4()).String(),
			ExpiresAt:         time.Now().Add(48 * time.Hour),
			CreatedBy:         1,
		}

		err := repo.Create(ctx, invitation)
		require.NoError(t, err)
		assert.NotZero(t, invitation.ID)

		// Cleanup
		testpkg.CleanupTableRecords(t, db, "auth.guardian_invitations", invitation.ID)
	})

	t.Run("returns error for invalid invitation", func(t *testing.T) {
		invitation := &auth.GuardianInvitation{
			// Missing required fields
		}

		err := repo.Create(ctx, invitation)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "validation failed")
	})
}

func TestGuardianInvitationRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GuardianInvitation
	ctx := context.Background()

	t.Run("finds existing invitation", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "FindByID")
		defer testpkg.CleanupActivityFixtures(t, db, guardian.ID)

		invitation := &auth.GuardianInvitation{
			GuardianProfileID: guardian.ID,
			Token:             uuid.Must(uuid.NewV4()).String(),
			ExpiresAt:         time.Now().Add(48 * time.Hour),
			CreatedBy:         1,
		}
		err := repo.Create(ctx, invitation)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "auth.guardian_invitations", invitation.ID)

		found, err := repo.FindByID(ctx, invitation.ID)
		require.NoError(t, err)
		assert.Equal(t, invitation.ID, found.ID)
		assert.Equal(t, invitation.Token, found.Token)
	})

	t.Run("returns error for non-existent ID", func(t *testing.T) {
		_, err := repo.FindByID(ctx, 999999)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestGuardianInvitationRepository_FindByToken(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GuardianInvitation
	ctx := context.Background()

	t.Run("finds invitation by token", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "FindByToken")
		defer testpkg.CleanupActivityFixtures(t, db, guardian.ID)

		token := uuid.Must(uuid.NewV4()).String()
		invitation := &auth.GuardianInvitation{
			GuardianProfileID: guardian.ID,
			Token:             token,
			ExpiresAt:         time.Now().Add(48 * time.Hour),
			CreatedBy:         1,
		}
		err := repo.Create(ctx, invitation)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "auth.guardian_invitations", invitation.ID)

		found, err := repo.FindByToken(ctx, token)
		require.NoError(t, err)
		assert.Equal(t, invitation.ID, found.ID)
		assert.Equal(t, token, found.Token)
	})

	t.Run("returns error for invalid token", func(t *testing.T) {
		_, err := repo.FindByToken(ctx, "invalid-token")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestGuardianInvitationRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GuardianInvitation
	ctx := context.Background()

	t.Run("updates existing invitation", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "Update")
		defer testpkg.CleanupActivityFixtures(t, db, guardian.ID)

		invitation := &auth.GuardianInvitation{
			GuardianProfileID: guardian.ID,
			Token:             uuid.Must(uuid.NewV4()).String(),
			ExpiresAt:         time.Now().Add(48 * time.Hour),
			CreatedBy:         1,
		}
		err := repo.Create(ctx, invitation)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "auth.guardian_invitations", invitation.ID)

		// Update expiry
		newExpiry := time.Now().Add(72 * time.Hour)
		invitation.ExpiresAt = newExpiry
		err = repo.Update(ctx, invitation)
		require.NoError(t, err)

		// Verify update
		found, err := repo.FindByID(ctx, invitation.ID)
		require.NoError(t, err)
		assert.WithinDuration(t, newExpiry, found.ExpiresAt, time.Second)
	})

	t.Run("returns error for non-existent invitation", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "UpdateNonExist")
		defer testpkg.CleanupActivityFixtures(t, db, guardian.ID)

		invitation := &auth.GuardianInvitation{
			GuardianProfileID: guardian.ID,
			Token:             uuid.Must(uuid.NewV4()).String(),
			ExpiresAt:         time.Now().Add(48 * time.Hour),
			CreatedBy:         1,
		}
		invitation.ID = 999999 // Set ID on embedded Model

		err := repo.Update(ctx, invitation)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestGuardianInvitationRepository_FindByGuardianProfileID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GuardianInvitation
	ctx := context.Background()

	t.Run("finds invitations by guardian profile ID", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "FindByProfile")
		defer testpkg.CleanupActivityFixtures(t, db, guardian.ID)

		invitation := &auth.GuardianInvitation{
			GuardianProfileID: guardian.ID,
			Token:             uuid.Must(uuid.NewV4()).String(),
			ExpiresAt:         time.Now().Add(48 * time.Hour),
			CreatedBy:         1,
		}
		err := repo.Create(ctx, invitation)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "auth.guardian_invitations", invitation.ID)

		invitations, err := repo.FindByGuardianProfileID(ctx, guardian.ID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(invitations), 1)
	})

	t.Run("returns empty list for non-existent guardian", func(t *testing.T) {
		invitations, err := repo.FindByGuardianProfileID(ctx, 999999)
		require.NoError(t, err)
		assert.Empty(t, invitations)
	})
}

func TestGuardianInvitationRepository_FindPending(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GuardianInvitation
	ctx := context.Background()

	t.Run("finds pending invitations", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "FindPending")
		defer testpkg.CleanupActivityFixtures(t, db, guardian.ID)

		// Create a pending invitation (not accepted, not expired)
		invitation := &auth.GuardianInvitation{
			GuardianProfileID: guardian.ID,
			Token:             uuid.Must(uuid.NewV4()).String(),
			ExpiresAt:         time.Now().Add(48 * time.Hour),
			CreatedBy:         1,
			// AcceptedAt is nil (pending)
		}
		err := repo.Create(ctx, invitation)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "auth.guardian_invitations", invitation.ID)

		pending, err := repo.FindPending(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(pending), 1)
	})
}

func TestGuardianInvitationRepository_FindExpired(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GuardianInvitation
	ctx := context.Background()

	t.Run("finds expired invitations", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "FindExpired")
		defer testpkg.CleanupActivityFixtures(t, db, guardian.ID)

		// Create an expired invitation - bypass validation by inserting directly
		token := uuid.Must(uuid.NewV4()).String()
		_, err := db.NewInsert().
			Model(&auth.GuardianInvitation{
				GuardianProfileID: guardian.ID,
				Token:             token,
				ExpiresAt:         time.Now().Add(-1 * time.Hour), // Expired 1 hour ago
				CreatedBy:         1,
			}).
			ModelTableExpr(`auth.guardian_invitations`).
			Exec(ctx)
		require.NoError(t, err)

		// Find the created invitation
		var created auth.GuardianInvitation
		err = db.NewSelect().
			Model(&created).
			ModelTableExpr(`auth.guardian_invitations AS "guardian_invitation"`).
			Where(`"guardian_invitation".token = ?`, token).
			Scan(ctx)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "auth.guardian_invitations", created.ID)

		expired, err := repo.FindExpired(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(expired), 1)
	})
}

func TestGuardianInvitationRepository_MarkAsAccepted(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GuardianInvitation
	ctx := context.Background()

	t.Run("marks invitation as accepted", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "MarkAccepted")
		defer testpkg.CleanupActivityFixtures(t, db, guardian.ID)

		invitation := &auth.GuardianInvitation{
			GuardianProfileID: guardian.ID,
			Token:             uuid.Must(uuid.NewV4()).String(),
			ExpiresAt:         time.Now().Add(48 * time.Hour),
			CreatedBy:         1,
		}
		err := repo.Create(ctx, invitation)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "auth.guardian_invitations", invitation.ID)

		err = repo.MarkAsAccepted(ctx, invitation.ID)
		require.NoError(t, err)

		// Verify it's accepted
		found, err := repo.FindByID(ctx, invitation.ID)
		require.NoError(t, err)
		assert.NotNil(t, found.AcceptedAt)
	})

	t.Run("returns error for non-existent invitation", func(t *testing.T) {
		err := repo.MarkAsAccepted(ctx, 999999)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestGuardianInvitationRepository_UpdateEmailStatus(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GuardianInvitation
	ctx := context.Background()

	t.Run("updates email status", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "EmailStatus")
		defer testpkg.CleanupActivityFixtures(t, db, guardian.ID)

		invitation := &auth.GuardianInvitation{
			GuardianProfileID: guardian.ID,
			Token:             uuid.Must(uuid.NewV4()).String(),
			ExpiresAt:         time.Now().Add(48 * time.Hour),
			CreatedBy:         1,
		}
		err := repo.Create(ctx, invitation)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "auth.guardian_invitations", invitation.ID)

		sentAt := time.Now()
		err = repo.UpdateEmailStatus(ctx, invitation.ID, &sentAt, nil, 0)
		require.NoError(t, err)

		// Verify status
		found, err := repo.FindByID(ctx, invitation.ID)
		require.NoError(t, err)
		assert.NotNil(t, found.EmailSentAt)
	})

	t.Run("updates email error status", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "EmailError")
		defer testpkg.CleanupActivityFixtures(t, db, guardian.ID)

		invitation := &auth.GuardianInvitation{
			GuardianProfileID: guardian.ID,
			Token:             uuid.Must(uuid.NewV4()).String(),
			ExpiresAt:         time.Now().Add(48 * time.Hour),
			CreatedBy:         1,
		}
		err := repo.Create(ctx, invitation)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "auth.guardian_invitations", invitation.ID)

		errorMsg := "SMTP connection failed"
		err = repo.UpdateEmailStatus(ctx, invitation.ID, nil, &errorMsg, 1)
		require.NoError(t, err)

		// Verify error status
		found, err := repo.FindByID(ctx, invitation.ID)
		require.NoError(t, err)
		assert.NotNil(t, found.EmailError)
		assert.Equal(t, 1, found.EmailRetryCount)
	})

	t.Run("returns error for non-existent invitation", func(t *testing.T) {
		sentAt := time.Now()
		err := repo.UpdateEmailStatus(ctx, 999999, &sentAt, nil, 0)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestGuardianInvitationRepository_DeleteExpired(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GuardianInvitation
	ctx := context.Background()

	t.Run("deletes expired invitations", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "DeleteExpired")
		defer testpkg.CleanupActivityFixtures(t, db, guardian.ID)

		// Create an expired invitation - bypass validation by inserting directly
		token := uuid.Must(uuid.NewV4()).String()
		_, err := db.NewInsert().
			Model(&auth.GuardianInvitation{
				GuardianProfileID: guardian.ID,
				Token:             token,
				ExpiresAt:         time.Now().Add(-1 * time.Hour), // Expired
				CreatedBy:         1,
			}).
			ModelTableExpr(`auth.guardian_invitations`).
			Exec(ctx)
		require.NoError(t, err)

		// Find the ID for verification
		var created auth.GuardianInvitation
		err = db.NewSelect().
			Model(&created).
			ModelTableExpr(`auth.guardian_invitations AS "guardian_invitation"`).
			Where(`"guardian_invitation".token = ?`, token).
			Scan(ctx)
		require.NoError(t, err)

		// Delete expired
		count, err := repo.DeleteExpired(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 1)

		// Verify deleted
		_, err = repo.FindByID(ctx, created.ID)
		require.Error(t, err)
	})
}

func TestGuardianInvitationRepository_Count(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GuardianInvitation
	ctx := context.Background()

	t.Run("counts invitations", func(t *testing.T) {
		count, err := repo.Count(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 0)
	})
}
