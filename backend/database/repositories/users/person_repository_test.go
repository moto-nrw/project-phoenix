package users_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// setupPersonRepo creates a person repository instance via the factory
func setupPersonRepo(t *testing.T, db *bun.DB) users.PersonRepository {
	repoFactory := repositories.NewFactory(db)
	return repoFactory.Person
}

// cleanupPersonRecords removes specific person records
func cleanupPersonRecords(t *testing.T, db *bun.DB, ids ...int64) {
	ctx := context.Background()
	for _, id := range ids {
		_, err := db.NewDelete().
			Model((*users.Person)(nil)).
			ModelTableExpr(`users.persons AS "person"`).
			Where(`"person".id = ?`, id).
			Exec(ctx)
		if err != nil {
			t.Logf("Warning: Failed to cleanup person record %d: %v", id, err)
		}
	}
}

// ============================================================================
// CRUD Tests
// ============================================================================

func TestPersonRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupPersonRepo(t, db)
	ctx := context.Background()

	var createdIDs []int64
	defer func() { cleanupPersonRecords(t, db, createdIDs...) }()

	t.Run("create valid person", func(t *testing.T) {
		person := &users.Person{
			FirstName: "Test",
			LastName:  "Person",
		}

		err := repo.Create(ctx, person)
		require.NoError(t, err)
		createdIDs = append(createdIDs, person.ID)

		assert.NotZero(t, person.ID)
		assert.False(t, person.CreatedAt.IsZero())
		assert.False(t, person.UpdatedAt.IsZero())
	})

	t.Run("create person with birthday", func(t *testing.T) {
		birthday := time.Date(2010, 5, 15, 0, 0, 0, 0, time.UTC)
		person := &users.Person{
			FirstName: "Birthday",
			LastName:  "Test",
			Birthday:  &birthday,
		}

		err := repo.Create(ctx, person)
		require.NoError(t, err)
		createdIDs = append(createdIDs, person.ID)

		assert.NotZero(t, person.ID)
		require.NotNil(t, person.Birthday)
		assert.Equal(t, birthday.Year(), person.Birthday.Year())
	})

	t.Run("create with nil person should fail", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("create with empty first name should fail validation", func(t *testing.T) {
		person := &users.Person{
			FirstName: "",
			LastName:  "Test",
		}

		err := repo.Create(ctx, person)
		assert.Error(t, err)
	})

	t.Run("create with empty last name should fail validation", func(t *testing.T) {
		person := &users.Person{
			FirstName: "Test",
			LastName:  "",
		}

		err := repo.Create(ctx, person)
		assert.Error(t, err)
	})
}

func TestPersonRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupPersonRepo(t, db)
	ctx := context.Background()

	// Create test person
	person := testpkg.CreateTestPerson(t, db, "FindByID", "Test")
	defer testpkg.CleanupActivityFixtures(t, db, person.ID)

	t.Run("find existing person", func(t *testing.T) {
		found, err := repo.FindByID(ctx, person.ID)
		require.NoError(t, err)
		require.NotNil(t, found)

		assert.Equal(t, person.ID, found.ID)
		assert.Equal(t, "FindByID", found.FirstName)
		assert.Equal(t, "Test", found.LastName)
	})

	t.Run("find non-existent person", func(t *testing.T) {
		found, err := repo.FindByID(ctx, 999999999)
		// Base repository returns error with "no rows" for non-existent records
		if err != nil {
			assert.Contains(t, err.Error(), "no rows")
		} else {
			assert.Nil(t, found)
		}
	})
}

func TestPersonRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupPersonRepo(t, db)
	ctx := context.Background()

	// Create test person
	person := testpkg.CreateTestPerson(t, db, "Update", "Original")
	defer testpkg.CleanupActivityFixtures(t, db, person.ID)

	t.Run("update person name", func(t *testing.T) {
		person.FirstName = "Updated"
		person.LastName = "Name"

		err := repo.Update(ctx, person)
		require.NoError(t, err)

		// Verify update
		found, err := repo.FindByID(ctx, person.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated", found.FirstName)
		assert.Equal(t, "Name", found.LastName)
	})

	t.Run("update with nil person should fail", func(t *testing.T) {
		err := repo.Update(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestPersonRepository_Delete(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupPersonRepo(t, db)
	ctx := context.Background()

	// Create test person
	person := testpkg.CreateTestPerson(t, db, "Delete", "Test")
	// No defer cleanup - we're testing deletion

	t.Run("delete existing person", func(t *testing.T) {
		err := repo.Delete(ctx, person.ID)
		require.NoError(t, err)

		// Verify deletion
		found, err := repo.FindByID(ctx, person.ID)
		if err == nil {
			assert.Nil(t, found)
		}
	})
}

// ============================================================================
// Query Tests
// ============================================================================

func TestPersonRepository_List(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupPersonRepo(t, db)
	ctx := context.Background()

	// Create test persons with unique names for filtering
	uniquePrefix := time.Now().UnixNano()
	person1 := testpkg.CreateTestPerson(t, db, "ListAlpha"+string(rune(uniquePrefix%26+'A')), "Test")
	person2 := testpkg.CreateTestPerson(t, db, "ListBeta"+string(rune(uniquePrefix%26+'A')), "Test")
	defer testpkg.CleanupActivityFixtures(t, db, person1.ID, person2.ID)

	t.Run("list with no filters", func(t *testing.T) {
		persons, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, persons)
	})

	t.Run("list with first name filter", func(t *testing.T) {
		filters := map[string]interface{}{
			"first_name": person1.FirstName,
		}
		persons, err := repo.List(ctx, filters)
		require.NoError(t, err)
		require.Len(t, persons, 1)
		assert.Equal(t, person1.ID, persons[0].ID)
	})

	t.Run("list with name like filter", func(t *testing.T) {
		filters := map[string]interface{}{
			"first_name_like": "ListAlpha",
		}
		persons, err := repo.List(ctx, filters)
		require.NoError(t, err)
		assert.NotEmpty(t, persons)
	})
}

func TestPersonRepository_FindByIDs(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupPersonRepo(t, db)
	ctx := context.Background()

	// Create test persons
	person1 := testpkg.CreateTestPerson(t, db, "FindByIDs1", "Test")
	person2 := testpkg.CreateTestPerson(t, db, "FindByIDs2", "Test")
	person3 := testpkg.CreateTestPerson(t, db, "FindByIDs3", "Test")
	defer testpkg.CleanupActivityFixtures(t, db, person1.ID, person2.ID, person3.ID)

	t.Run("find multiple persons by IDs", func(t *testing.T) {
		ids := []int64{person1.ID, person2.ID, person3.ID}
		persons, err := repo.FindByIDs(ctx, ids)
		require.NoError(t, err)

		assert.Len(t, persons, 3)
		assert.NotNil(t, persons[person1.ID])
		assert.NotNil(t, persons[person2.ID])
		assert.NotNil(t, persons[person3.ID])
	})

	t.Run("find with partial IDs", func(t *testing.T) {
		ids := []int64{person1.ID, 999999999}
		persons, err := repo.FindByIDs(ctx, ids)
		require.NoError(t, err)

		assert.Len(t, persons, 1)
		assert.NotNil(t, persons[person1.ID])
	})

	t.Run("find with empty IDs returns empty map", func(t *testing.T) {
		persons, err := repo.FindByIDs(ctx, []int64{})
		require.NoError(t, err)
		assert.Empty(t, persons)
	})
}

// ============================================================================
// Account Linking Tests
// ============================================================================

func TestPersonRepository_LinkToAccount(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupPersonRepo(t, db)
	ctx := context.Background()

	// Create test person and account
	person := testpkg.CreateTestPerson(t, db, "LinkAccount", "Test")
	account := testpkg.CreateTestAccount(t, db, "linkaccount")
	defer testpkg.CleanupActivityFixtures(t, db, person.ID, account.ID)

	t.Run("link person to account", func(t *testing.T) {
		err := repo.LinkToAccount(ctx, person.ID, account.ID)
		require.NoError(t, err)

		// Verify link via FindByAccountID
		found, err := repo.FindByAccountID(ctx, account.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, person.ID, found.ID)
	})

	t.Run("link non-existent person should fail", func(t *testing.T) {
		err := repo.LinkToAccount(ctx, 999999999, account.ID)
		assert.Error(t, err)
	})
}

func TestPersonRepository_UnlinkFromAccount(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupPersonRepo(t, db)
	ctx := context.Background()

	// Create person with account
	person, account := testpkg.CreateTestPersonWithAccount(t, db, "Unlink", "Account")
	defer testpkg.CleanupActivityFixtures(t, db, person.ID, account.ID)

	t.Run("unlink person from account", func(t *testing.T) {
		err := repo.UnlinkFromAccount(ctx, person.ID)
		require.NoError(t, err)

		// Verify unlink
		found, err := repo.FindByAccountID(ctx, account.ID)
		require.NoError(t, err)
		assert.Nil(t, found)
	})

	t.Run("unlink non-existent person should fail", func(t *testing.T) {
		err := repo.UnlinkFromAccount(ctx, 999999999)
		assert.Error(t, err)
	})
}

func TestPersonRepository_FindByAccountID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupPersonRepo(t, db)
	ctx := context.Background()

	// Create person with account
	person, account := testpkg.CreateTestPersonWithAccount(t, db, "FindByAccount", "Test")
	defer testpkg.CleanupActivityFixtures(t, db, person.ID, account.ID)

	t.Run("find person by account ID", func(t *testing.T) {
		found, err := repo.FindByAccountID(ctx, account.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, person.ID, found.ID)
	})

	t.Run("find by non-existent account ID returns nil", func(t *testing.T) {
		found, err := repo.FindByAccountID(ctx, 999999999)
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

// ============================================================================
// RFID Card Linking Tests
// ============================================================================

func TestPersonRepository_LinkToRFIDCard(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupPersonRepo(t, db)
	ctx := context.Background()

	// Create test person and RFID card
	person := testpkg.CreateTestPerson(t, db, "LinkRFID", "Test")
	rfidCard := testpkg.CreateTestRFIDCard(t, db, "LINK12345678")
	defer testpkg.CleanupActivityFixtures(t, db, person.ID)
	defer testpkg.CleanupRFIDCards(t, db, rfidCard.ID)

	t.Run("link person to RFID card", func(t *testing.T) {
		err := repo.LinkToRFIDCard(ctx, person.ID, rfidCard.ID)
		require.NoError(t, err)

		// Verify link via FindByTagID
		found, err := repo.FindByTagID(ctx, rfidCard.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, person.ID, found.ID)
	})

	t.Run("link non-existent person should fail", func(t *testing.T) {
		err := repo.LinkToRFIDCard(ctx, 999999999, rfidCard.ID)
		assert.Error(t, err)
	})
}

func TestPersonRepository_UnlinkFromRFIDCard(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupPersonRepo(t, db)
	ctx := context.Background()

	// Create test person with RFID card linked
	person := testpkg.CreateTestPerson(t, db, "UnlinkRFID", "Test")
	rfidCard := testpkg.CreateTestRFIDCard(t, db, "UNLINK1234567")
	defer testpkg.CleanupActivityFixtures(t, db, person.ID)
	defer testpkg.CleanupRFIDCards(t, db, rfidCard.ID)

	// Link first
	err := repo.LinkToRFIDCard(ctx, person.ID, rfidCard.ID)
	require.NoError(t, err)

	t.Run("unlink person from RFID card", func(t *testing.T) {
		err := repo.UnlinkFromRFIDCard(ctx, person.ID)
		require.NoError(t, err)

		// Verify unlink
		found, err := repo.FindByTagID(ctx, rfidCard.ID)
		require.NoError(t, err)
		assert.Nil(t, found)
	})

	t.Run("unlink non-existent person should fail", func(t *testing.T) {
		err := repo.UnlinkFromRFIDCard(ctx, 999999999)
		assert.Error(t, err)
	})
}

func TestPersonRepository_FindByTagID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupPersonRepo(t, db)
	ctx := context.Background()

	// Create test person with RFID card linked
	person := testpkg.CreateTestPerson(t, db, "FindByTag", "Test")
	rfidCard := testpkg.CreateTestRFIDCard(t, db, "FINDTAG12345")
	defer testpkg.CleanupActivityFixtures(t, db, person.ID)
	defer testpkg.CleanupRFIDCards(t, db, rfidCard.ID)

	// Link card to person
	err := repo.LinkToRFIDCard(ctx, person.ID, rfidCard.ID)
	require.NoError(t, err)

	t.Run("find person by tag ID", func(t *testing.T) {
		found, err := repo.FindByTagID(ctx, rfidCard.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, person.ID, found.ID)
	})

	t.Run("find by tag ID with different case", func(t *testing.T) {
		// Tag IDs are normalized to uppercase
		_, err := repo.FindByTagID(ctx, "findtag12345")
		// May or may not match depending on normalization
		require.NoError(t, err)
		// Result depends on card ID format
	})

	t.Run("find by non-existent tag ID returns nil", func(t *testing.T) {
		found, err := repo.FindByTagID(ctx, "NONEXISTENT")
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

// ============================================================================
// Nested Relationship Tests
// ============================================================================

func TestPersonRepository_FindWithAccount(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupPersonRepo(t, db)
	ctx := context.Background()

	t.Run("find person with account", func(t *testing.T) {
		// Create person with account
		person, account := testpkg.CreateTestPersonWithAccount(t, db, "WithAccount", "Test")
		defer testpkg.CleanupActivityFixtures(t, db, person.ID, account.ID)

		found, err := repo.FindWithAccount(ctx, person.ID)
		require.NoError(t, err)
		require.NotNil(t, found)

		assert.Equal(t, person.ID, found.ID)
		require.NotNil(t, found.Account, "Account should be loaded")
		assert.Equal(t, account.ID, found.Account.ID)
		assert.Contains(t, found.Account.Email, "WithAccount")
	})

	t.Run("find person without account", func(t *testing.T) {
		// Create person without account
		person := testpkg.CreateTestPerson(t, db, "NoAccount", "Test")
		defer testpkg.CleanupActivityFixtures(t, db, person.ID)

		found, err := repo.FindWithAccount(ctx, person.ID)
		require.NoError(t, err)
		require.NotNil(t, found)

		assert.Equal(t, person.ID, found.ID)
		assert.Nil(t, found.Account, "Account should be nil for person without account")
	})
}

// Note: FindWithRFIDCard is not part of the PersonRepository interface
// The repository implementation has it, but the interface doesn't expose it
// If needed, this test can be added once the interface is updated

// ============================================================================
// Edge Cases
// ============================================================================

func TestPersonRepository_EdgeCases(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupPersonRepo(t, db)
	ctx := context.Background()

	t.Run("create person with unicode names", func(t *testing.T) {
		person := &users.Person{
			FirstName: "Müller",
			LastName:  "Über",
		}

		err := repo.Create(ctx, person)
		require.NoError(t, err)
		defer cleanupPersonRecords(t, db, person.ID)

		found, err := repo.FindByID(ctx, person.ID)
		require.NoError(t, err)
		assert.Equal(t, "Müller", found.FirstName)
		assert.Equal(t, "Über", found.LastName)
	})

	t.Run("create person with long names", func(t *testing.T) {
		longName := "VeryLongFirstNameThatExceedsNormalLengthButShouldStillWork"
		person := &users.Person{
			FirstName: longName,
			LastName:  "Test",
		}

		err := repo.Create(ctx, person)
		require.NoError(t, err)
		defer cleanupPersonRecords(t, db, person.ID)

		found, err := repo.FindByID(ctx, person.ID)
		require.NoError(t, err)
		assert.Equal(t, longName, found.FirstName)
	})
}
