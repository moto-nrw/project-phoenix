package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

// ============================================================================
// GuardianInvitationRepository CRUD Tests
// ============================================================================

func TestGuardianInvitationRepository_Create(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GuardianInvitation
	ctx := testpkg.Ctx(t)

	t.Run("creates invitation with valid data", func(t *testing.T) {
		// Create a guardian profile first
		guardian := testpkg.CreateTestGuardianProfile(t, db, "invite-test")

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GuardianInvitation
	ctx := testpkg.Ctx(t)

	t.Run("finds existing invitation", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "FindByID")

		invitation := &auth.GuardianInvitation{
			GuardianProfileID: guardian.ID,
			Token:             uuid.Must(uuid.NewV4()).String(),
			ExpiresAt:         time.Now().Add(48 * time.Hour),
			CreatedBy:         1,
		}
		err := repo.Create(ctx, invitation)
		require.NoError(t, err)

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GuardianInvitation
	ctx := testpkg.Ctx(t)

	t.Run("finds invitation by token", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "FindByToken")

		token := uuid.Must(uuid.NewV4()).String()
		invitation := &auth.GuardianInvitation{
			GuardianProfileID: guardian.ID,
			Token:             token,
			ExpiresAt:         time.Now().Add(48 * time.Hour),
			CreatedBy:         1,
		}
		err := repo.Create(ctx, invitation)
		require.NoError(t, err)

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GuardianInvitation
	ctx := testpkg.Ctx(t)

	t.Run("updates existing invitation", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "Update")

		invitation := &auth.GuardianInvitation{
			GuardianProfileID: guardian.ID,
			Token:             uuid.Must(uuid.NewV4()).String(),
			ExpiresAt:         time.Now().Add(48 * time.Hour),
			CreatedBy:         1,
		}
		err := repo.Create(ctx, invitation)
		require.NoError(t, err)

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GuardianInvitation
	ctx := testpkg.Ctx(t)

	t.Run("finds invitations by guardian profile ID", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "FindByProfile")

		invitation := &auth.GuardianInvitation{
			GuardianProfileID: guardian.ID,
			Token:             uuid.Must(uuid.NewV4()).String(),
			ExpiresAt:         time.Now().Add(48 * time.Hour),
			CreatedBy:         1,
		}
		err := repo.Create(ctx, invitation)
		require.NoError(t, err)

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GuardianInvitation
	ctx := testpkg.Ctx(t)

	t.Run("finds pending invitations", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "FindPending")

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

		pending, err := repo.FindPending(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(pending), 1)
	})
}

func TestGuardianInvitationRepository_MarkAsAccepted(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GuardianInvitation
	ctx := testpkg.Ctx(t)

	t.Run("marks invitation as accepted", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "MarkAccepted")

		invitation := &auth.GuardianInvitation{
			GuardianProfileID: guardian.ID,
			Token:             uuid.Must(uuid.NewV4()).String(),
			ExpiresAt:         time.Now().Add(48 * time.Hour),
			CreatedBy:         1,
		}
		err := repo.Create(ctx, invitation)
		require.NoError(t, err)

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GuardianInvitation
	ctx := testpkg.Ctx(t)

	t.Run("updates email status", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "EmailStatus")

		invitation := &auth.GuardianInvitation{
			GuardianProfileID: guardian.ID,
			Token:             uuid.Must(uuid.NewV4()).String(),
			ExpiresAt:         time.Now().Add(48 * time.Hour),
			CreatedBy:         1,
		}
		err := repo.Create(ctx, invitation)
		require.NoError(t, err)

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

		invitation := &auth.GuardianInvitation{
			GuardianProfileID: guardian.ID,
			Token:             uuid.Must(uuid.NewV4()).String(),
			ExpiresAt:         time.Now().Add(48 * time.Hour),
			CreatedBy:         1,
		}
		err := repo.Create(ctx, invitation)
		require.NoError(t, err)

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

func TestGuardianInvitationRepository_UpdateEmailStatusPreservesRowsAffectedError(t *testing.T) {
	t.Parallel()

	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	db := bun.NewDB(sqlDB, pgdialect.New())
	repo := repositories.NewFactory(db).GuardianInvitation
	rowsErr := errors.New("rows affected failed")
	mock.ExpectExec(`UPDATE "auth"\."guardian_invitations" AS "guardian_invitation"`).
		WillReturnResult(sqlmock.NewErrorResult(rowsErr))

	err = repo.UpdateEmailStatus(context.Background(), 42, nil, nil, 1)
	assert.EqualError(t, err, "failed to get rows affected: rows affected failed")
	assert.ErrorIs(t, err, rowsErr)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGuardianInvitationRepository_DeleteExpired(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GuardianInvitation
	ctx := testpkg.Ctx(t)

	t.Run("deletes expired invitations", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "DeleteExpired")

		// Create an expired invitation - bypass validation by inserting directly
		token := uuid.Must(uuid.NewV4()).String()
		expiredInvitation := &auth.GuardianInvitation{
			GuardianProfileID: guardian.ID,
			Token:             token,
			ExpiresAt:         time.Now().Add(-1 * time.Hour), // Expired
			CreatedBy:         1,
		}
		expiredInvitation.SetTenantID(testpkg.Tenant(t))
		_, err := db.NewInsert().
			Model(expiredInvitation).
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
