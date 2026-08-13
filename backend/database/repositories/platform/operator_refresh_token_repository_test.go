package platform_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	repoplatform "github.com/moto-nrw/project-phoenix/database/repositories/platform"
	"github.com/moto-nrw/project-phoenix/models/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOperatorRefreshTokenRepository_CreateAndFindByTokenForUpdate(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoplatform.NewOperatorRefreshTokenRepository(db)
	ctx := context.Background()
	operatorID := testpkg.CreateTestOperatorWithEmail(t, db, fmt.Sprintf("refresh-create-%d@test.local", time.Now().UnixNano()), "Test Operator").ID

	tokenValue := uuid.Must(uuid.NewV4()).String()
	token := &platform.OperatorRefreshToken{
		OperatorID: operatorID,
		Token:      tokenValue,
		Expiry:     time.Now().Add(time.Hour),
		FamilyID:   uuid.Must(uuid.NewV4()).String(),
		Generation: 0,
	}
	require.NoError(t, repo.Create(ctx, token))
	require.NotZero(t, token.ID)

	found, err := repo.FindByTokenForUpdate(ctx, tokenValue)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, operatorID, found.OperatorID)
	assert.Equal(t, tokenValue, found.Token)
}

func TestOperatorRefreshTokenRepository_DeleteByOperatorID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoplatform.NewOperatorRefreshTokenRepository(db)
	ctx := context.Background()
	operatorID := testpkg.CreateTestOperatorWithEmail(t, db, fmt.Sprintf("refresh-delete-op-%d@test.local", time.Now().UnixNano()), "Test Operator").ID

	for i := 0; i < 2; i++ {
		require.NoError(t, repo.Create(ctx, &platform.OperatorRefreshToken{
			OperatorID: operatorID,
			Token:      uuid.Must(uuid.NewV4()).String(),
			Expiry:     time.Now().Add(time.Hour),
			FamilyID:   uuid.Must(uuid.NewV4()).String(),
			Generation: i,
		}))
	}

	deleted, err := repo.DeleteByOperatorIDReturning(ctx, operatorID)
	require.NoError(t, err)
	assert.Len(t, deleted, 2)

	remaining, err := db.NewSelect().
		TableExpr("platform.operator_refresh_tokens").
		Where("operator_id = ?", operatorID).
		Count(ctx)
	require.NoError(t, err)
	assert.Zero(t, remaining)
}

func TestOperatorRefreshTokenRepository_DeleteByFamilyIDAndLatest(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoplatform.NewOperatorRefreshTokenRepository(db)
	ctx := context.Background()
	operatorID := testpkg.CreateTestOperatorWithEmail(t, db, fmt.Sprintf("refresh-family-%d@test.local", time.Now().UnixNano()), "Test Operator").ID
	familyID := uuid.Must(uuid.NewV4()).String()

	for generation := 0; generation < 3; generation++ {
		require.NoError(t, repo.Create(ctx, &platform.OperatorRefreshToken{
			OperatorID: operatorID,
			Token:      uuid.Must(uuid.NewV4()).String(),
			Expiry:     time.Now().Add(time.Hour),
			FamilyID:   familyID,
			Generation: generation,
		}))
	}

	latest, err := repo.GetLatestTokenInFamily(ctx, familyID)
	require.NoError(t, err)
	require.NotNil(t, latest)
	assert.Equal(t, 2, latest.Generation)

	deleted, err := repo.DeleteByFamilyIDReturning(ctx, familyID)
	require.NoError(t, err)
	require.Len(t, deleted, 3)
	latest, err = repo.GetLatestTokenInFamily(ctx, familyID)
	require.NoError(t, err)
	assert.Nil(t, latest)
}

func TestOperatorRefreshTokenRepository_DeleteExpired(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoplatform.NewOperatorRefreshTokenRepository(db)
	ctx := context.Background()
	operatorID := testpkg.CreateTestOperatorWithEmail(t, db, fmt.Sprintf("refresh-expired-%d@test.local", time.Now().UnixNano()), "Test Operator").ID
	now := time.Now()

	require.NoError(t, repo.Create(ctx, &platform.OperatorRefreshToken{
		OperatorID: operatorID,
		Token:      uuid.Must(uuid.NewV4()).String(),
		Expiry:     now.Add(-time.Minute),
		FamilyID:   uuid.Must(uuid.NewV4()).String(),
	}))
	require.NoError(t, repo.Create(ctx, &platform.OperatorRefreshToken{
		OperatorID: operatorID,
		Token:      uuid.Must(uuid.NewV4()).String(),
		Expiry:     now.Add(time.Hour),
		FamilyID:   uuid.Must(uuid.NewV4()).String(),
	}))

	deleted, err := repo.DeleteExpired(ctx, now)
	require.NoError(t, err)
	assert.Equal(t, 1, deleted)
}

func TestOperatorRefreshTokenRepository_RotationHandoffLifecycle(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoplatform.NewOperatorRefreshTokenRepository(db)
	ctx := context.Background()
	operatorID := testpkg.CreateTestOperatorWithEmail(t, db, fmt.Sprintf("refresh-handoff-%d@test.local", time.Now().UnixNano()), "Test Operator").ID
	familyID := uuid.Must(uuid.NewV4()).String()
	predecessor := &platform.OperatorRefreshToken{
		OperatorID: operatorID,
		Token:      uuid.Must(uuid.NewV4()).String(),
		Expiry:     time.Now().Add(time.Hour),
		FamilyID:   familyID,
		Generation: 0,
	}
	successor := &platform.OperatorRefreshToken{
		OperatorID: operatorID,
		Token:      uuid.Must(uuid.NewV4()).String(),
		Expiry:     time.Now().Add(time.Hour),
		FamilyID:   familyID,
		Generation: 1,
	}
	require.NoError(t, repo.Create(ctx, predecessor))
	require.NoError(t, repo.Create(ctx, successor))

	rotatedAt := time.Now().Add(-10 * time.Minute)
	recoveryProofHash := make([]byte, 32)
	recoveryProofHash[0] = 1
	require.NoError(t, repo.MarkRotated(ctx, predecessor.ID, successor.Token, recoveryProofHash, rotatedAt))
	stored, err := repo.FindByTokenForUpdate(ctx, predecessor.Token)
	require.NoError(t, err)
	require.NotNil(t, stored)
	require.NotNil(t, stored.RotatedAt)
	require.NotNil(t, stored.ReplacementToken)
	assert.Equal(t, successor.Token, *stored.ReplacementToken)
	assert.Equal(t, recoveryProofHash, stored.RecoveryProofHash)

	// A handoff older than the recovery grace remains replay evidence until its
	// refresh JWT expires.
	require.NoError(t, repo.DeleteExpiredRotated(ctx, familyID, time.Now()))
	stored, err = repo.FindByTokenForUpdate(ctx, predecessor.Token)
	require.NoError(t, err)
	require.NotNil(t, stored)

	_, err = db.NewUpdate().
		Table("platform.operator_refresh_tokens").
		Set("expiry = ?", time.Now().Add(-time.Minute)).
		Where("id = ?", predecessor.ID).
		Exec(ctx)
	require.NoError(t, err)
	require.NoError(t, repo.DeleteExpiredRotated(ctx, familyID, time.Now()))
	stored, err = repo.FindByTokenForUpdate(ctx, predecessor.Token)
	require.NoError(t, err)
	assert.Nil(t, stored)
	stored, err = repo.FindByTokenForUpdate(ctx, successor.Token)
	require.NoError(t, err)
	require.NotNil(t, stored)
	assert.Equal(t, 1, stored.Generation)
}
