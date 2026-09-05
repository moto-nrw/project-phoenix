package users_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Setup Helpers
// ============================================================================

// ============================================================================
// CRUD Tests
// ============================================================================

func TestProfileRepository_Create(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).Profile
	ctx := testpkg.Ctx(t)

	t.Run("creates profile with valid data", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "profile-create")

		profile := &users.Profile{
			AccountID: account.ID,
			Avatar:    "https://example.com/avatar.png",
			Bio:       "Test bio",
			Settings:  `{"theme": "dark"}`,
		}

		err := repo.Create(ctx, profile)
		require.NoError(t, err)
		assert.NotZero(t, profile.ID)

	})

	t.Run("creates profile with minimal data", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "profile-minimal")

		// Settings must be valid JSON or empty object
		profile := &users.Profile{
			AccountID: account.ID,
			Settings:  `{}`,
		}

		err := repo.Create(ctx, profile)
		require.NoError(t, err)
		assert.NotZero(t, profile.ID)

	})

	t.Run("fails with nil profile", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("fails without account ID", func(t *testing.T) {
		profile := &users.Profile{
			Bio: "Test bio",
		}

		err := repo.Create(ctx, profile)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "account ID")
	})

	t.Run("fails with invalid settings JSON", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "profile-bad-json")

		profile := &users.Profile{
			AccountID: account.ID,
			Settings:  "not-valid-json",
		}

		err := repo.Create(ctx, profile)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid settings JSON")
	})
}

func TestProfileRepository_FindByID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).Profile
	ctx := testpkg.Ctx(t)

	t.Run("finds existing profile", func(t *testing.T) {
		profile := testpkg.CreateTestProfile(t, db, "findbyid")

		found, err := repo.FindByID(ctx, profile.ID)
		require.NoError(t, err)
		assert.Equal(t, profile.ID, found.ID)
		assert.Equal(t, profile.AccountID, found.AccountID)
	})

	t.Run("returns error for non-existent profile", func(t *testing.T) {
		_, err := repo.FindByID(ctx, int64(999999))
		require.Error(t, err)
	})
}

func TestProfileRepository_FindByAccountID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).Profile
	ctx := testpkg.Ctx(t)

	t.Run("finds profile by account ID", func(t *testing.T) {
		profile := testpkg.CreateTestProfile(t, db, "byaccount")

		found, err := repo.FindByAccountID(ctx, profile.AccountID)
		require.NoError(t, err)
		assert.Equal(t, profile.ID, found.ID)
		assert.Equal(t, profile.AccountID, found.AccountID)
	})

	t.Run("returns error for non-existent account ID", func(t *testing.T) {
		_, err := repo.FindByAccountID(ctx, int64(999999))
		require.Error(t, err)
	})
}

func TestProfileRepository_Update(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).Profile
	ctx := testpkg.Ctx(t)

	t.Run("updates profile", func(t *testing.T) {
		profile := testpkg.CreateTestProfile(t, db, "update")

		profile.Bio = "Updated bio"
		profile.Avatar = "https://example.com/new-avatar.png"

		err := repo.Update(ctx, profile)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, profile.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated bio", found.Bio)
		assert.Equal(t, "https://example.com/new-avatar.png", found.Avatar)
	})

	t.Run("fails with nil profile", func(t *testing.T) {
		err := repo.Update(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestProfileRepository_Delete(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).Profile
	ctx := testpkg.Ctx(t)

	t.Run("deletes existing profile", func(t *testing.T) {
		profile := testpkg.CreateTestProfile(t, db, "delete")

		err := repo.Delete(ctx, profile.ID)
		require.NoError(t, err)

		_, err = repo.FindByID(ctx, profile.ID)
		require.Error(t, err)
	})

	// NOTE: Base repository Delete returns nil for non-existent records
}

// ============================================================================
// Field Update Tests
// ============================================================================

func TestProfileRepository_UpdateAvatar(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).Profile
	ctx := testpkg.Ctx(t)

	t.Run("updates avatar", func(t *testing.T) {
		profile := testpkg.CreateTestProfile(t, db, "avatar")

		newAvatar := "https://example.com/updated-avatar.png"
		err := repo.UpdateAvatar(ctx, profile.ID, newAvatar)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, profile.ID)
		require.NoError(t, err)
		assert.Equal(t, newAvatar, found.Avatar)
	})
}

// ============================================================================
// List and Filter Tests
// ============================================================================

func TestProfileRepository_List(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).Profile
	ctx := testpkg.Ctx(t)

	t.Run("lists all profiles", func(t *testing.T) {
		testpkg.CreateTestProfile(t, db, "listall")

		found, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, found)
	})

	t.Run("lists with account_id filter", func(t *testing.T) {
		profile := testpkg.CreateTestProfile(t, db, "filteraccount")

		filters := map[string]interface{}{
			"account_id": profile.AccountID,
		}

		found, err := repo.List(ctx, filters)
		require.NoError(t, err)
		require.Len(t, found, 1)
		assert.Equal(t, profile.ID, found[0].ID)
	})

	t.Run("lists with has_avatar filter", func(t *testing.T) {
		testpkg.CreateTestProfile(t, db, "hasavatar")

		filters := map[string]interface{}{
			"has_avatar": true,
		}

		found, err := repo.List(ctx, filters)
		require.NoError(t, err)

		// All returned profiles should have an avatar
		for _, p := range found {
			assert.NotEmpty(t, p.Avatar)
		}
	})

	t.Run("lists with has_bio filter", func(t *testing.T) {
		testpkg.CreateTestProfile(t, db, "hasbio")

		filters := map[string]interface{}{
			"has_bio": true,
		}

		found, err := repo.List(ctx, filters)
		require.NoError(t, err)

		// All returned profiles should have a bio
		for _, p := range found {
			assert.NotEmpty(t, p.Bio)
		}
	})

	t.Run("lists with bio_like filter", func(t *testing.T) {
		profile := testpkg.CreateTestProfile(t, db, "biolike")

		filters := map[string]interface{}{
			"bio_like": "biolike",
		}

		found, err := repo.List(ctx, filters)
		require.NoError(t, err)

		var foundProfile bool
		for _, p := range found {
			if p.ID == profile.ID {
				foundProfile = true
				break
			}
		}
		assert.True(t, foundProfile)
	})
}
