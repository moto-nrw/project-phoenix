package platform_test

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The retained operator repository contracts are adapters over the Identity
// & Access owner (#2720). These tests pin the contract the operator services
// still consume: validation on the retained model, (nil, nil) for a missing
// row, and the identity and timestamps written back into the caller's value.

func TestOperatorRepositoryAdapter_Create(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).Operator
	ctx := testpkg.Ctx(t)

	t.Run("success", func(t *testing.T) {
		operator := &platformModels.Operator{Email: fmt.Sprintf("adapter-create-%d@example.com", time.Now().UnixNano()), DisplayName: "New Operator", PasswordHash: "hashed-password", Active: true}
		require.NoError(t, repo.Create(ctx, operator))
		testpkg.OwnTestOperator(t, db, operator.ID)
		assert.NotZero(t, operator.ID)
		assert.NotZero(t, operator.CreatedAt)
	})
	t.Run("nil operator", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
	t.Run("validation runs on the retained model", func(t *testing.T) {
		err := repo.Create(ctx, &platformModels.Operator{Email: "", DisplayName: "Test", PasswordHash: "hash"})
		require.EqualError(t, err, "email is required")
		err = repo.Create(ctx, &platformModels.Operator{Email: "invalid-email", DisplayName: "Test", PasswordHash: "hash"})
		require.EqualError(t, err, "invalid email format")
		err = repo.Create(ctx, &platformModels.Operator{Email: "test@example.com", DisplayName: "", PasswordHash: "hash"})
		require.EqualError(t, err, "display name is required")
	})
}

func TestOperatorRepositoryAdapter_ReadUpdateDelete(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).Operator
	ctx := testpkg.Ctx(t)
	operator := testpkg.CreateTestOperator(t, db)

	found, err := repo.FindByID(ctx, operator.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, operator.Email, found.Email)
	locked, err := repo.FindByIDForUpdate(ctx, operator.ID)
	require.NoError(t, err)
	require.NotNil(t, locked)
	byEmail, err := repo.FindByEmail(ctx, operator.Email)
	require.NoError(t, err)
	require.NotNil(t, byEmail)
	assert.Equal(t, operator.ID, byEmail.ID)

	missing, err := repo.FindByID(ctx, operator.ID+1_000_000)
	require.NoError(t, err)
	assert.Nil(t, missing, "a missing operator is (nil, nil)")
	missing, err = repo.FindByEmail(ctx, "nonexistent@example.com")
	require.NoError(t, err)
	assert.Nil(t, missing)

	found.DisplayName = "Updated Name"
	found.Active = false
	require.NoError(t, repo.Update(ctx, found))
	reloaded, err := repo.FindByID(ctx, operator.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", reloaded.DisplayName)
	assert.False(t, reloaded.Active)
	require.Error(t, repo.Update(ctx, nil))
	found.Email = "invalid-email"
	require.Error(t, repo.Update(ctx, found))

	assert.Nil(t, reloaded.LastLogin)
	require.NoError(t, repo.UpdateLastLogin(ctx, operator.ID))
	reloaded, err = repo.FindByID(ctx, operator.ID)
	require.NoError(t, err)
	require.NotNil(t, reloaded.LastLogin)

	listed, err := repo.List(ctx)
	require.NoError(t, err)
	var seen bool
	for _, entry := range listed {
		seen = seen || entry.ID == operator.ID
	}
	assert.True(t, seen)

	require.NoError(t, repo.Delete(ctx, operator.ID))
	gone, err := repo.FindByID(ctx, operator.ID)
	require.NoError(t, err)
	assert.Nil(t, gone)
}

func TestOperatorRefreshTokenRepositoryAdapter_Lifecycle(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).OperatorRefreshToken
	ctx := testpkg.Ctx(t)
	operator := testpkg.CreateTestOperator(t, db)
	familyID := uuid.Must(uuid.NewV4()).String()

	require.Error(t, repo.Create(ctx, nil))
	require.EqualError(t, repo.Create(ctx, &platformModels.OperatorRefreshToken{OperatorID: operator.ID}), "token value is required")

	predecessor := &platformModels.OperatorRefreshToken{OperatorID: operator.ID, Token: uuid.Must(uuid.NewV4()).String(), Expiry: time.Now().Add(time.Hour), FamilyID: familyID}
	require.NoError(t, repo.Create(ctx, predecessor))
	require.NotZero(t, predecessor.ID)
	successor := &platformModels.OperatorRefreshToken{OperatorID: operator.ID, Token: uuid.Must(uuid.NewV4()).String(), Expiry: time.Now().Add(time.Hour), FamilyID: familyID, Generation: 1}
	require.NoError(t, repo.Create(ctx, successor))

	found, err := repo.FindByTokenForUpdate(ctx, predecessor.Token)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, operator.ID, found.OperatorID)
	missing, err := repo.FindByTokenForUpdate(ctx, "no-such-token")
	require.NoError(t, err)
	assert.Nil(t, missing, "a missing session is (nil, nil)")

	latest, err := repo.GetLatestTokenInFamily(ctx, familyID)
	require.NoError(t, err)
	require.NotNil(t, latest)
	assert.Equal(t, 1, latest.Generation)

	proof := []byte{9}
	require.NoError(t, repo.MarkRotated(ctx, predecessor.ID, successor.Token, proof, time.Now().Add(-10*time.Minute)))
	err = repo.MarkRotated(ctx, predecessor.ID, successor.Token, proof, time.Now())
	require.Error(t, err, "a second hand-off must be reported")
	rotated, err := repo.FindByTokenForUpdate(ctx, predecessor.Token)
	require.NoError(t, err)
	require.NotNil(t, rotated.RotatedAt)
	assert.Equal(t, successor.Token, *rotated.ReplacementToken)
	assert.Equal(t, proof, rotated.RecoveryProofHash)

	require.NoError(t, repo.DeleteExpiredRotated(ctx, familyID, time.Now()))
	stillThere, err := repo.FindByTokenForUpdate(ctx, predecessor.Token)
	require.NoError(t, err)
	require.NotNil(t, stillThere, "replay evidence is kept until the refresh JWT expires")

	deleted, err := repo.DeleteByFamilyIDReturning(ctx, familyID)
	require.NoError(t, err)
	assert.Len(t, deleted, 2)
	latest, err = repo.GetLatestTokenInFamily(ctx, familyID)
	require.NoError(t, err)
	assert.Nil(t, latest)

	for generation := range 2 {
		require.NoError(t, repo.Create(ctx, &platformModels.OperatorRefreshToken{OperatorID: operator.ID, Token: uuid.Must(uuid.NewV4()).String(), Expiry: time.Now().Add(time.Hour), FamilyID: uuid.Must(uuid.NewV4()).String(), Generation: generation}))
	}
	expired := &platformModels.OperatorRefreshToken{OperatorID: operator.ID, Token: uuid.Must(uuid.NewV4()).String(), Expiry: time.Now().Add(-time.Minute), FamilyID: uuid.Must(uuid.NewV4()).String()}
	require.NoError(t, repo.Create(ctx, expired))
	swept, err := repo.DeleteExpired(ctx, time.Now())
	require.NoError(t, err)
	assert.GreaterOrEqual(t, swept, 1)
	require.NoError(t, repo.Delete(ctx, expired.ID))
	revoked, err := repo.DeleteByOperatorIDReturning(ctx, operator.ID)
	require.NoError(t, err)
	assert.Len(t, revoked, 2)
}

func TestOperatorAuditLogRepositoryAdapter(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).OperatorAuditLog
	ctx := testpkg.Ctx(t)
	operator := testpkg.CreateTestOperator(t, db)

	resourceID := int64(123)
	entry := &platformModels.OperatorAuditLog{
		OperatorID: operator.ID, Action: platformModels.ActionCreate, ResourceType: platformModels.ResourceAnnouncement,
		ResourceID: &resourceID, RequestIP: net.ParseIP("192.168.1.1"),
	}
	require.NoError(t, entry.SetChanges(map[string]any{"title": "New Announcement", "active": true}))
	require.NoError(t, repo.Create(ctx, entry))
	assert.NotZero(t, entry.ID)
	assert.NotZero(t, entry.CreatedAt)

	entries, err := repo.FindByOperatorID(ctx, operator.ID, 10)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, entry.ID, entries[0].ID)
	changes, err := entries[0].GetChanges()
	require.NoError(t, err)
	assert.Equal(t, "New Announcement", changes["title"])
	assert.Equal(t, "192.168.1.1", entries[0].RequestIP.String())

	ranged, err := repo.FindByDateRange(ctx, time.Now().Add(-time.Hour), time.Now().Add(time.Hour), 0)
	require.NoError(t, err)
	var seen bool
	for _, candidate := range ranged {
		seen = seen || candidate.ID == entry.ID
	}
	assert.True(t, seen)
}
