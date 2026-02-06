package platform_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/platform"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOperatorRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := platform.NewOperatorRepository(db)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		operator := &platformModels.Operator{
			Email:        "newop@example.com",
			DisplayName:  "New Operator",
			PasswordHash: "hashed-password",
			Active:       true,
		}

		err := repo.Create(ctx, operator)
		require.NoError(t, err)
		assert.NotZero(t, operator.ID)
		assert.NotZero(t, operator.CreatedAt)

		defer cleanupTestOperator(t, db, operator.ID)
	})

	t.Run("NilOperator", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "operator cannot be nil")
	})

	t.Run("ValidationError_EmptyEmail", func(t *testing.T) {
		operator := &platformModels.Operator{
			Email:        "",
			DisplayName:  "Test",
			PasswordHash: "hash",
		}

		err := repo.Create(ctx, operator)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "email is required")
	})

	t.Run("ValidationError_InvalidEmail", func(t *testing.T) {
		operator := &platformModels.Operator{
			Email:        "invalid-email",
			DisplayName:  "Test",
			PasswordHash: "hash",
		}

		err := repo.Create(ctx, operator)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid email format")
	})

	t.Run("ValidationError_EmptyDisplayName", func(t *testing.T) {
		operator := &platformModels.Operator{
			Email:        "test@example.com",
			DisplayName:  "",
			PasswordHash: "hash",
		}

		err := repo.Create(ctx, operator)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "display name is required")
	})
}

func TestOperatorRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := platform.NewOperatorRepository(db)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		operator := createTestOperator(t, db, "find@example.com", "Find Test")
		defer cleanupTestOperator(t, db, operator.ID)

		found, err := repo.FindByID(ctx, operator.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, operator.ID, found.ID)
		assert.Equal(t, operator.Email, found.Email)
		assert.Equal(t, operator.DisplayName, found.DisplayName)
	})

	t.Run("NotFound", func(t *testing.T) {
		found, err := repo.FindByID(ctx, 999999)
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

func TestOperatorRepository_FindByEmail(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := platform.NewOperatorRepository(db)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		operator := createTestOperator(t, db, "email@example.com", "Email Test")
		defer cleanupTestOperator(t, db, operator.ID)

		found, err := repo.FindByEmail(ctx, "email@example.com")
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, operator.ID, found.ID)
		assert.Equal(t, "email@example.com", found.Email)
	})

	t.Run("NotFound", func(t *testing.T) {
		found, err := repo.FindByEmail(ctx, "nonexistent@example.com")
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

func TestOperatorRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := platform.NewOperatorRepository(db)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		operator := createTestOperator(t, db, "update@example.com", "Original Name")
		defer cleanupTestOperator(t, db, operator.ID)

		operator.DisplayName = "Updated Name"
		operator.Active = false

		err := repo.Update(ctx, operator)
		require.NoError(t, err)

		// Verify update
		found, err := repo.FindByID(ctx, operator.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated Name", found.DisplayName)
		assert.False(t, found.Active)
	})

	t.Run("NilOperator", func(t *testing.T) {
		err := repo.Update(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "operator cannot be nil")
	})

	t.Run("ValidationError", func(t *testing.T) {
		operator := createTestOperator(t, db, "validate@example.com", "Test")
		defer cleanupTestOperator(t, db, operator.ID)

		operator.Email = "invalid-email"
		err := repo.Update(ctx, operator)
		require.Error(t, err)
	})
}

func TestOperatorRepository_Delete(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := platform.NewOperatorRepository(db)
	ctx := context.Background()

	operator := createTestOperator(t, db, "delete@example.com", "To Delete")

	err := repo.Delete(ctx, operator.ID)
	require.NoError(t, err)

	// Verify deletion
	found, err := repo.FindByID(ctx, operator.ID)
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestOperatorRepository_List(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := platform.NewOperatorRepository(db)
	ctx := context.Background()

	// Create test operators
	op1 := createTestOperator(t, db, "list1@example.com", "Alpha Operator")
	defer cleanupTestOperator(t, db, op1.ID)

	op2 := createTestOperator(t, db, "list2@example.com", "Beta Operator")
	defer cleanupTestOperator(t, db, op2.ID)

	operators, err := repo.List(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, operators)

	// Find our operators
	found1 := false
	found2 := false
	for _, op := range operators {
		if op.ID == op1.ID {
			found1 = true
		}
		if op.ID == op2.ID {
			found2 = true
		}
	}
	assert.True(t, found1, "should find first operator")
	assert.True(t, found2, "should find second operator")

	// Verify ordering by display_name ASC
	if len(operators) >= 2 {
		for i := 0; i < len(operators)-1; i++ {
			assert.LessOrEqual(t, operators[i].DisplayName, operators[i+1].DisplayName,
				"operators should be ordered by display_name ASC")
		}
	}
}

func TestOperatorRepository_UpdateLastLogin(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := platform.NewOperatorRepository(db)
	ctx := context.Background()

	operator := createTestOperator(t, db, "login@example.com", "Login Test")
	defer cleanupTestOperator(t, db, operator.ID)

	// Initially no last login
	assert.Nil(t, operator.LastLogin)

	// Update last login
	beforeUpdate := time.Now().Add(-1 * time.Second)
	err := repo.UpdateLastLogin(ctx, operator.ID)
	require.NoError(t, err)
	afterUpdate := time.Now().Add(1 * time.Second)

	// Verify update
	found, err := repo.FindByID(ctx, operator.ID)
	require.NoError(t, err)
	require.NotNil(t, found.LastLogin)
	assert.True(t, found.LastLogin.After(beforeUpdate))
	assert.True(t, found.LastLogin.Before(afterUpdate))

	// Update again to test multiple updates
	firstLogin := *found.LastLogin
	time.Sleep(10 * time.Millisecond) // Small delay to ensure timestamp difference

	err = repo.UpdateLastLogin(ctx, operator.ID)
	require.NoError(t, err)

	found, err = repo.FindByID(ctx, operator.ID)
	require.NoError(t, err)
	require.NotNil(t, found.LastLogin)
	assert.True(t, found.LastLogin.After(firstLogin), "last login should be updated to newer timestamp")
}
