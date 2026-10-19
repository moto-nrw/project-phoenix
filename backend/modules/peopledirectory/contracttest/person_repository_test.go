package contracttest_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/api"

	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// cleanupPersonRecords removes specific person records
func cleanupPersonRecords(t *testing.T, db *bun.DB, ids ...int64) {
	ctx := testpkg.Ctx(t)
	for _, id := range ids {
		_, err := db.NewDelete().
			Model((*peopleValues.DirectoryPersonSnapshot)(nil)).
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).Person
	ctx := testpkg.Ctx(t)

	var createdIDs []int64
	defer func() { cleanupPersonRecords(t, db, createdIDs...) }()

	t.Run("create valid person", func(t *testing.T) {
		person := &peopleValues.DirectoryPersonSnapshot{
			FirstName: "Test",
			LastName:  "Person",
		}

		err := testpkg.TenantCommand1(t, ctx, db, repo.Create, person)
		require.NoError(t, err)
		createdIDs = append(createdIDs, person.ID)

		assert.NotZero(t, person.ID)
		assert.False(t, person.CreatedAt.IsZero())
		assert.False(t, person.UpdatedAt.IsZero())
	})

	t.Run("create person with birthday", func(t *testing.T) {
		birthday := calendar.NewDate(2010, 5, 15)
		person := &peopleValues.DirectoryPersonSnapshot{
			FirstName: "Birthday",
			LastName:  "Test",
			Birthday:  &birthday,
		}

		err := testpkg.TenantCommand1(t, ctx, db, repo.Create, person)
		require.NoError(t, err)
		createdIDs = append(createdIDs, person.ID)

		assert.NotZero(t, person.ID)
		require.NotNil(t, person.Birthday)
		assert.Equal(t, birthday.Year(), person.Birthday.Year())
	})

	t.Run("create with nil person should fail", func(t *testing.T) {
		err := testpkg.TenantCommand1(t, ctx, db, repo.Create, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("create with empty first name should fail validation", func(t *testing.T) {
		person := &peopleValues.DirectoryPersonSnapshot{
			FirstName: "",
			LastName:  "Test",
		}

		err := testpkg.TenantCommand1(t, ctx, db, repo.Create, person)
		assert.Error(t, err)
	})

	t.Run("create with empty last name should fail validation", func(t *testing.T) {
		person := &peopleValues.DirectoryPersonSnapshot{
			FirstName: "Test",
			LastName:  "",
		}

		err := testpkg.TenantCommand1(t, ctx, db, repo.Create, person)
		assert.Error(t, err)
	})
}

func TestPersonRepository_FindByID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).Person
	ctx := testpkg.Ctx(t)

	// Create test person
	person := testpkg.CreateTestPerson(t, db, "FindByID", "Test")

	t.Run("find existing person", func(t *testing.T) {
		found, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByID, any(person.ID))
		require.NoError(t, err)
		require.NotNil(t, found)

		assert.Equal(t, person.ID, found.ID)
		assert.Equal(t, "FindByID", found.FirstName)
		assert.Equal(t, "Test", found.LastName)
	})

	t.Run("find non-existent person", func(t *testing.T) {
		found, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByID, 999999999)
		// Base repository returns error with "no rows" for non-existent records
		if err != nil {
			assert.Contains(t, err.Error(), "no rows")
		} else {
			assert.Nil(t, found)
		}
	})
}

func TestPersonRepository_Update(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).Person
	ctx := testpkg.Ctx(t)

	// Create test person
	personFixture := testpkg.CreateTestPerson(t, db, "Update", "Original")
	person, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByID, any(personFixture.ID))
	require.NoError(t, err)

	t.Run("update person name", func(t *testing.T) {
		person.FirstName = "Updated"
		person.LastName = "Name"

		err := testpkg.TenantCommand1(t, ctx, db, repo.Update, person)
		require.NoError(t, err)

		// Verify update
		found, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByID, any(person.ID))
		require.NoError(t, err)
		assert.Equal(t, "Updated", found.FirstName)
		assert.Equal(t, "Name", found.LastName)
	})

	t.Run("update with nil person should fail", func(t *testing.T) {
		err := testpkg.TenantCommand1(t, ctx, db, repo.Update, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestPersonRepository_Delete(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).Person
	ctx := testpkg.Ctx(t)

	// Create test person
	person := testpkg.CreateTestPerson(t, db, "Delete", "Test")
	// No defer cleanup - we're testing deletion

	t.Run("delete existing person", func(t *testing.T) {
		err := testpkg.TenantCommand1(t, ctx, db, repo.Delete, any(person.ID))
		require.NoError(t, err)

		// Verify deletion
		found, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByID, any(person.ID))
		if err == nil {
			assert.Nil(t, found)
		}
	})
}

// ============================================================================
// Query Tests
// ============================================================================

func TestPersonRepository_List(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).Person
	ctx := testpkg.Ctx(t)

	// Create test persons with unique names for filtering
	uniquePrefix := fmt.Sprintf("%d", time.Now().UnixNano())
	person1 := testpkg.CreateTestPerson(t, db, "ListAlpha"+uniquePrefix, "Test")
	testpkg.CreateTestPerson(t, db, "ListBeta"+uniquePrefix, "Test")

	t.Run("list with no filters", func(t *testing.T) {
		persons, err := testpkg.TenantQuery1(t, ctx, db, repo.List, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, persons)
	})

	t.Run("list with first name filter", func(t *testing.T) {
		// Filter by the exact unique first name
		filters := map[string]interface{}{
			"first_name": person1.FirstName,
		}
		persons, err := testpkg.TenantQuery1(t, ctx, db, repo.List, filters)
		require.NoError(t, err)
		require.Len(t, persons, 1)
		assert.Equal(t, person1.ID, persons[0].ID)
	})

	t.Run("list with name like filter", func(t *testing.T) {
		// Filter using the unique prefix
		filters := map[string]interface{}{
			"first_name_like": "ListAlpha" + uniquePrefix,
		}
		persons, err := testpkg.TenantQuery1(t, ctx, db, repo.List, filters)
		require.NoError(t, err)
		assert.Len(t, persons, 1)
	})
}

func TestPersonRepository_FindByIDs(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).Person
	ctx := testpkg.Ctx(t)

	// Create test persons
	person1 := testpkg.CreateTestPerson(t, db, "FindByIDs1", "Test")
	person2 := testpkg.CreateTestPerson(t, db, "FindByIDs2", "Test")
	person3 := testpkg.CreateTestPerson(t, db, "FindByIDs3", "Test")

	t.Run("find multiple persons by IDs", func(t *testing.T) {
		ids := []int64{person1.ID, person2.ID, person3.ID}
		persons, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByIDs, ids)
		require.NoError(t, err)

		assert.Len(t, persons, 3)
		assert.NotNil(t, persons[person1.ID])
		assert.NotNil(t, persons[person2.ID])
		assert.NotNil(t, persons[person3.ID])
	})

	t.Run("find with partial IDs", func(t *testing.T) {
		ids := []int64{person1.ID, 999999999}
		persons, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByIDs, ids)
		require.NoError(t, err)

		assert.Len(t, persons, 1)
		assert.NotNil(t, persons[person1.ID])
	})

	t.Run("find with empty IDs returns empty map", func(t *testing.T) {
		persons, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByIDs, []int64{})
		require.NoError(t, err)
		assert.Empty(t, persons)
	})
}

// ============================================================================
// Account Linking Tests
// ============================================================================

func TestPersonRepository_LinkToAccount(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).Person
	ctx := testpkg.Ctx(t)

	// Create test person and account
	person := testpkg.CreateTestPerson(t, db, "LinkAccount", "Test")
	account := testpkg.CreateTestAccount(t, db, "linkaccount")

	t.Run("link person to account", func(t *testing.T) {
		err := testpkg.TenantCommand2(t, ctx, db, repo.LinkToAccount, person.ID, account.ID)
		require.NoError(t, err)

		// Verify link via FindByAccountID
		found, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByAccountID, account.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, person.ID, found.ID)
	})

	t.Run("link non-existent person should fail", func(t *testing.T) {
		err := testpkg.TenantCommand2(t, ctx, db, repo.LinkToAccount, 999999999, account.ID)
		assert.Error(t, err)
	})
}

func TestPersonRepository_UnlinkFromAccount(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).Person
	ctx := testpkg.Ctx(t)

	// Create person with account
	person, account := testpkg.CreateTestPersonWithAccount(t, db, "Unlink", "Account")

	t.Run("unlink person from account", func(t *testing.T) {
		err := testpkg.TenantCommand1(t, ctx, db, repo.UnlinkFromAccount, person.ID)
		require.NoError(t, err)

		// Verify unlink
		found, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByAccountID, account.ID)
		require.NoError(t, err)
		assert.Nil(t, found)
	})

	t.Run("unlink non-existent person should fail", func(t *testing.T) {
		err := testpkg.TenantCommand1(t, ctx, db, repo.UnlinkFromAccount, 999999999)
		assert.Error(t, err)
	})
}

func TestPersonRepository_FindByAccountID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).Person
	ctx := testpkg.Ctx(t)

	// Create person with account
	person, account := testpkg.CreateTestPersonWithAccount(t, db, "FindByAccount", "Test")

	t.Run("find person by account ID", func(t *testing.T) {
		found, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByAccountID, account.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, person.ID, found.ID)
	})

	t.Run("find by non-existent account ID returns nil", func(t *testing.T) {
		found, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByAccountID, 999999999)
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

// ============================================================================
// RFID Card Linking Tests
// ============================================================================

func TestPersonRepository_LinkToRFIDCard(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).Person
	ctx := testpkg.Ctx(t)

	// Create test person and RFID card
	person := testpkg.CreateTestPerson(t, db, "LinkRFID", "Test")
	rfidCard := testpkg.CreateTestRFIDCard(t, db, "LINK12345678")

	t.Run("link person to RFID card", func(t *testing.T) {
		err := testpkg.TenantCommand2(t, ctx, db, repo.LinkToRFIDCard, person.ID, rfidCard.ID)
		require.NoError(t, err)

		// Verify link via FindByTagID
		found, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByTagID, rfidCard.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, person.ID, found.ID)
	})

	t.Run("link non-existent person should fail", func(t *testing.T) {
		err := testpkg.TenantCommand2(t, ctx, db, repo.LinkToRFIDCard, 999999999, rfidCard.ID)
		assert.Error(t, err)
	})
}

func TestPersonRepository_UnlinkFromRFIDCard(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).Person
	ctx := testpkg.Ctx(t)

	// Create test person with RFID card linked
	person := testpkg.CreateTestPerson(t, db, "UnlinkRFID", "Test")
	rfidCard := testpkg.CreateTestRFIDCard(t, db, "UNLINK1234567")

	// Link first
	err := testpkg.TenantCommand2(t, ctx, db, repo.LinkToRFIDCard, person.ID, rfidCard.ID)
	require.NoError(t, err)

	t.Run("unlink person from RFID card", func(t *testing.T) {
		err := testpkg.TenantCommand1(t, ctx, db, repo.UnlinkFromRFIDCard, person.ID)
		require.NoError(t, err)

		// Verify unlink
		found, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByTagID, rfidCard.ID)
		require.NoError(t, err)
		assert.Nil(t, found)
	})

	t.Run("unlink non-existent person should fail", func(t *testing.T) {
		err := testpkg.TenantCommand1(t, ctx, db, repo.UnlinkFromRFIDCard, 999999999)
		assert.Error(t, err)
	})
}

func TestPersonRepository_FindByTagID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).Person
	ctx := testpkg.Ctx(t)

	// Create test person with RFID card linked
	person := testpkg.CreateTestPerson(t, db, "FindByTag", "Test")
	rfidCard := testpkg.CreateTestRFIDCard(t, db, "FINDTAG12345")

	// Link card to person
	err := testpkg.TenantCommand2(t, ctx, db, repo.LinkToRFIDCard, person.ID, rfidCard.ID)
	require.NoError(t, err)

	t.Run("find person by tag ID", func(t *testing.T) {
		found, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByTagID, rfidCard.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, person.ID, found.ID)
	})

	t.Run("find by tag ID with different case", func(t *testing.T) {
		// Tag IDs are normalized to uppercase
		_, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByTagID, "findtag12345")
		// May or may not match depending on normalization
		require.NoError(t, err)
		// Result depends on card ID format
	})

	t.Run("find by non-existent tag ID returns nil", func(t *testing.T) {
		found, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByTagID, "NONEXISTENT")
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

// ============================================================================
// Nested Relationship Tests
// ============================================================================

func TestPersonRepository_FindWithAccount(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).Person
	ctx := testpkg.Ctx(t)

	t.Run("find person with account", func(t *testing.T) {
		// Create person with account
		person, account := testpkg.CreateTestPersonWithAccount(t, db, "WithAccount", "Test")

		found, err := testpkg.TenantQuery1(t, ctx, db, repo.FindWithAccount, person.ID)
		require.NoError(t, err)
		require.NotNil(t, found)

		assert.Equal(t, person.ID, found.ID)
		require.NotNil(t, found.Account, "Account should be loaded")
		assert.Equal(t, account.ID, found.Account.ID)
		assert.Contains(t, found.Account.Email, "WithAccount")

		// The directory now attaches metadata, not an auth ORM row. Keep the
		// wire shape, including optional-field omission and timestamps.
		previousJSON, err := json.Marshal(account)
		require.NoError(t, err)
		metadataJSON, err := json.Marshal(found.Account)
		require.NoError(t, err)
		assert.JSONEq(t, string(previousJSON), string(metadataJSON))
	})

	t.Run("account metadata preserves optional fields without credentials", func(t *testing.T) {
		person, account := testpkg.CreateTestPersonWithAccount(t, db, "AccountMetadata", "Test")
		username := fmt.Sprintf("profile-%d", account.ID)
		lastLogin := time.Now().UTC().Truncate(time.Second)
		account.Username = &username
		account.Avatar = "/uploads/avatars/profile.png"
		account.LastLogin = &lastLogin
		account.Active = false
		account.IsPasswordOTP = true
		err := db.NewUpdate().Model(account).ModelTableExpr(`auth.accounts AS "account"`).
			Column("username", "avatar", "last_login", "active", "is_password_otp").
			WherePK().Returning("last_login, updated_at").Scan(ctx)
		require.NoError(t, err)

		found, err := testpkg.TenantQuery1(t, ctx, db, repo.FindWithAccount, person.ID)
		require.NoError(t, err)
		require.NotNil(t, found.Account)
		previousJSON, err := json.Marshal(account)
		require.NoError(t, err)
		metadataJSON, err := json.Marshal(found.Account)
		require.NoError(t, err)
		assert.JSONEq(t, string(previousJSON), string(metadataJSON))
	})

	t.Run("find person without account", func(t *testing.T) {
		// Create person without account
		person := testpkg.CreateTestPerson(t, db, "NoAccount", "Test")

		found, err := testpkg.TenantQuery1(t, ctx, db, repo.FindWithAccount, person.ID)
		require.NoError(t, err)
		require.NotNil(t, found)

		assert.Equal(t, person.ID, found.ID)
		assert.Nil(t, found.Account, "Account should be nil for person without account")
	})
}

// ============================================================================
// Filter Tests
// ============================================================================

func TestPersonRepository_ListWithNullableFilters(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).Person
	ctx := testpkg.Ctx(t)

	// Create test persons - one with account, one without
	personWithAccount, _ := testpkg.CreateTestPersonWithAccount(t, db, "HasAccount", "Test")
	personWithoutAccount := testpkg.CreateTestPerson(t, db, "NoAccount", "Test")

	t.Run("filter persons with account", func(t *testing.T) {
		filters := map[string]interface{}{
			"has_account": true,
		}
		persons, err := testpkg.TenantQuery1(t, ctx, db, repo.List, filters)
		require.NoError(t, err)

		// Should include personWithAccount
		var found bool
		for _, p := range persons {
			if p.ID == personWithAccount.ID {
				found = true
				break
			}
		}
		assert.True(t, found, "Should find person with account")
	})

	t.Run("filter persons without account", func(t *testing.T) {
		filters := map[string]interface{}{
			"has_account": false,
		}
		persons, err := testpkg.TenantQuery1(t, ctx, db, repo.List, filters)
		require.NoError(t, err)

		// Should include personWithoutAccount
		var found bool
		for _, p := range persons {
			if p.ID == personWithoutAccount.ID {
				found = true
				break
			}
		}
		assert.True(t, found, "Should find person without account")
	})

	t.Run("filter persons with RFID card", func(t *testing.T) {
		// Create person with RFID card
		personWithCard := testpkg.CreateTestPerson(t, db, "WithTag", "Test")
		rfidCard := testpkg.CreateTestRFIDCard(t, db, "WITHTAG12345")

		err := testpkg.TenantCommand2(t, ctx, db, repo.LinkToRFIDCard, personWithCard.ID, rfidCard.ID)
		require.NoError(t, err)

		filters := map[string]interface{}{
			"has_tag": true,
		}
		persons, err := testpkg.TenantQuery1(t, ctx, db, repo.List, filters)
		require.NoError(t, err)

		// Should include personWithCard
		var found bool
		for _, p := range persons {
			if p.ID == personWithCard.ID {
				found = true
				break
			}
		}
		assert.True(t, found, "Should find person with RFID card")
	})
}

// ============================================================================
// Edge Cases
// ============================================================================

func TestPersonRepository_EdgeCases(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).Person
	ctx := testpkg.Ctx(t)

	t.Run("create person with unicode names", func(t *testing.T) {
		person := &peopleValues.DirectoryPersonSnapshot{
			FirstName: "Müller",
			LastName:  "Über",
		}

		err := testpkg.TenantCommand1(t, ctx, db, repo.Create, person)
		require.NoError(t, err)

		found, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByID, any(person.ID))
		require.NoError(t, err)
		assert.Equal(t, "Müller", found.FirstName)
		assert.Equal(t, "Über", found.LastName)
	})

	t.Run("create person with long names", func(t *testing.T) {
		longName := "VeryLongFirstNameThatExceedsNormalLengthButShouldStillWork"
		person := &peopleValues.DirectoryPersonSnapshot{
			FirstName: longName,
			LastName:  "Test",
		}

		err := testpkg.TenantCommand1(t, ctx, db, repo.Create, person)
		require.NoError(t, err)

		found, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByID, any(person.ID))
		require.NoError(t, err)
		assert.Equal(t, longName, found.FirstName)
	})
}

// Identity & Access owns auth.accounts: without the owner lookup the
// repository fails closed instead of joining the table itself (#2720).
func TestPersonRepository_FindWithAccountRequiresAccountLookup(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	person, _ := testpkg.CreateTestPersonWithAccount(t, db, "NoLookup", "Test")

	_, err := peopleCompose.NewRetainedPersonRepository(db).FindWithAccount(testpkg.Ctx(t), person.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "account lookup is required")
}
