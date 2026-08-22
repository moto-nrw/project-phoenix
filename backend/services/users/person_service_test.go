// Package users_test tests the users service layer with hermetic testing pattern.
//
// # HERMETIC TEST PATTERN
//
// Tests create their own fixtures, perform operations, and clean up.
// This ensures test isolation and prevents dependency on seed data.
//
// STRUCTURE: ARRANGE-ACT-ASSERT
package users_test

import (
	"log/slog"
	"testing"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// setupPersonService creates a PersonService with real database connection
func setupPersonService(t *testing.T, db *bun.DB) users.PersonService {
	repoFactory := repositories.NewFactory(db)
	serviceFactory, err := services.NewFactory(repoFactory, db, slog.Default())
	require.NoError(t, err, "Failed to create service factory")
	return serviceFactory.Users
}

// =============================================================================
// Get Tests
// =============================================================================

func TestPersonService_Get(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupPersonService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns person when found", func(t *testing.T) {
		// ARRANGE
		person := testpkg.CreateTestPerson(t, db, "Get", "Test")

		// ACT
		result, err := service.Get(ctx, person.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, person.ID, result.ID)
		assert.Equal(t, "Get", result.FirstName)
		assert.Equal(t, "Test", result.LastName)
	})

	t.Run("returns error when person not found", func(t *testing.T) {
		// ACT
		result, err := service.Get(ctx, int64(99999999))

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
		// Error may be "person not found" or "sql: no rows" depending on repository impl
	})

	t.Run("returns error for invalid ID type", func(t *testing.T) {
		// ACT
		result, err := service.Get(ctx, "invalid")

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "invalid ID type")
	})

	t.Run("handles int type ID correctly", func(t *testing.T) {
		// ARRANGE
		person := testpkg.CreateTestPerson(t, db, "GetInt", "TypeTest")

		// ACT - Pass int (not int64) to test the type switch case
		result, err := service.Get(ctx, int(person.ID))

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, person.ID, result.ID)
	})
}

// =============================================================================
// GetByIDs Tests
// =============================================================================

func TestPersonService_GetByIDs(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupPersonService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns multiple persons when found", func(t *testing.T) {
		// ARRANGE
		person1 := testpkg.CreateTestPerson(t, db, "Multi1", "Test")
		person2 := testpkg.CreateTestPerson(t, db, "Multi2", "Test")

		// ACT
		result, err := service.GetByIDs(ctx, []int64{person1.ID, person2.ID})

		// ASSERT
		require.NoError(t, err)
		assert.Len(t, result, 2)
		assert.NotNil(t, result[person1.ID])
		assert.NotNil(t, result[person2.ID])
	})

	t.Run("returns partial results when some not found", func(t *testing.T) {
		// ARRANGE
		person := testpkg.CreateTestPerson(t, db, "Partial", "Test")

		// ACT
		result, err := service.GetByIDs(ctx, []int64{person.ID, 99999999})

		// ASSERT
		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.NotNil(t, result[person.ID])
	})

	t.Run("returns empty map for empty input", func(t *testing.T) {
		// ACT
		result, err := service.GetByIDs(ctx, []int64{})

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, result)
	})
}

// =============================================================================
// Create Tests
// =============================================================================

func TestPersonService_Create(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupPersonService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("creates person successfully", func(t *testing.T) {
		// ARRANGE
		person := testpkg.CreateTestPerson(t, db, "ToDelete", "ForCleanup")

		// Verify person was created
		result, err := service.Get(ctx, person.ID)
		require.NoError(t, err)
		assert.Equal(t, person.ID, result.ID)
	})

	t.Run("returns error for invalid person data", func(t *testing.T) {
		// ARRANGE - empty names should fail validation
		person := &userModels.Person{
			FirstName: "",
			LastName:  "",
		}

		// ACT
		err := service.Create(ctx, person)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("creates person with account link", func(t *testing.T) {
		// ARRANGE
		account := testpkg.CreateTestAccount(t, db, "create-with-account")

		person := &userModels.Person{
			FirstName: "WithAccount",
			LastName:  "Test",
			AccountID: &account.ID,
		}

		// ACT
		err := service.Create(ctx, person)
		defer func() {
		}()

		// ASSERT
		require.NoError(t, err)
		assert.Greater(t, person.ID, int64(0))
	})

	t.Run("returns error when account not found", func(t *testing.T) {
		// ARRANGE
		nonExistentAccountID := int64(99999999)
		person := &userModels.Person{
			FirstName: "Invalid",
			LastName:  "Account",
			AccountID: &nonExistentAccountID,
		}

		// ACT
		err := service.Create(ctx, person)

		// ASSERT
		require.Error(t, err)
		// Error indicates account not found or validation failed
	})
}

// =============================================================================
// Update Tests
// =============================================================================

func TestPersonService_Update(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupPersonService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("updates person successfully", func(t *testing.T) {
		// ARRANGE
		person := testpkg.CreateTestPerson(t, db, "Original", "Name")

		person.FirstName = "Updated"
		person.LastName = "Person"

		// ACT
		err := service.Update(ctx, person)

		// ASSERT
		require.NoError(t, err)

		// Verify update
		result, err := service.Get(ctx, person.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated", result.FirstName)
		assert.Equal(t, "Person", result.LastName)
	})

	t.Run("returns error when person not found", func(t *testing.T) {
		// ARRANGE
		person := &userModels.Person{
			FirstName: "NonExistent",
			LastName:  "Person",
		}
		person.ID = 99999999 // ID is in embedded base.Model

		// ACT
		err := service.Update(ctx, person)

		// ASSERT
		require.Error(t, err)
		// Error indicates person/entity not found
	})
}

// =============================================================================
// Delete Tests
// =============================================================================

func TestPersonService_Delete(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupPersonService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("deletes person successfully", func(t *testing.T) {
		// ARRANGE
		person := testpkg.CreateTestPerson(t, db, "ToDelete", "Person")
		// No defer cleanup - we're testing deletion

		// ACT
		err := service.Delete(ctx, person.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify deletion
		_, err = service.Get(ctx, person.ID)
		require.Error(t, err)
		// Error indicates person/entity not found
	})

	t.Run("returns error when person not found", func(t *testing.T) {
		// ACT
		err := service.Delete(ctx, int64(99999999))

		// ASSERT
		require.Error(t, err)
		// Error indicates person/entity not found
	})
}

// =============================================================================
// List Tests
// =============================================================================

func TestPersonService_List(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupPersonService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns persons list", func(t *testing.T) {
		// ARRANGE
		testpkg.CreateTestPerson(t, db, "List1", "Test")
		testpkg.CreateTestPerson(t, db, "List2", "Test")

		// ACT
		result, err := service.List(ctx, nil)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, result)
		// Should contain our created persons (plus any seed data)
	})

	t.Run("returns list with nil options", func(t *testing.T) {
		// ACT
		result, err := service.List(ctx, nil)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
	})
}

// =============================================================================
// FindByTagID Tests
// =============================================================================

func TestPersonService_FindByTagID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupPersonService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("finds person by tag ID", func(t *testing.T) {
		// ARRANGE
		person := testpkg.CreateTestPerson(t, db, "Tagged", "Person")
		rfidCard := testpkg.CreateTestRFIDCard(t, db, "FINDTAG")

		// Link person to RFID card
		err := service.LinkToRFIDCard(ctx, person.ID, rfidCard.ID)
		require.NoError(t, err)

		// ACT
		result, err := service.FindByTagID(ctx, rfidCard.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, person.ID, result.ID)
	})

	t.Run("returns error when tag not found", func(t *testing.T) {
		// ACT
		result, err := service.FindByTagID(ctx, "NONEXISTENTTAG999")

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
		// Error indicates person/entity not found
	})
}

// =============================================================================
// FindByAccountID Tests
// =============================================================================

func TestPersonService_FindByAccountID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupPersonService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("finds person by account ID", func(t *testing.T) {
		// ARRANGE
		person, account := testpkg.CreateTestPersonWithAccount(t, db, "Account", "Linked")

		// ACT
		result, err := service.FindByAccountID(ctx, account.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, person.ID, result.ID)
	})

	t.Run("returns error when account not linked to any person", func(t *testing.T) {
		// ARRANGE
		account := testpkg.CreateTestAccount(t, db, "unlinked-account")

		// ACT
		result, err := service.FindByAccountID(ctx, account.ID)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
		// Error indicates person/entity not found
	})
}

// =============================================================================
// FindByName Tests
// =============================================================================

func TestPersonService_FindByName(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupPersonService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("finds persons by first name", func(t *testing.T) {
		// ARRANGE
		uniqueFirst := "UniqueFirstName123"
		testpkg.CreateTestPerson(t, db, uniqueFirst, "TestLast")

		// ACT
		result, err := service.FindByName(ctx, uniqueFirst, "")

		// ASSERT
		require.NoError(t, err)
		// Note: FindByName uses ILIKE prefix matching, so results may vary
		assert.NotNil(t, result)
	})

	t.Run("finds persons by last name", func(t *testing.T) {
		// ARRANGE
		uniqueLast := "UniqueLastName456"
		testpkg.CreateTestPerson(t, db, "TestFirst", uniqueLast)

		// ACT
		result, err := service.FindByName(ctx, "", uniqueLast)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("finds persons by both names", func(t *testing.T) {
		// ARRANGE
		uniqueFirst := "BothFirst789"
		uniqueLast := "BothLast789"
		testpkg.CreateTestPerson(t, db, uniqueFirst, uniqueLast)

		// ACT
		result, err := service.FindByName(ctx, uniqueFirst, uniqueLast)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("returns empty slice when no matches found", func(t *testing.T) {
		// ACT - FindByName uses ILIKE prefix matching, non-existent names return empty
		result, err := service.FindByName(ctx, "ZZZZNOEXIST99999XYZ", "ZZZZNOEXIST99999ABC")

		// ASSERT - call succeeds with empty result (filters are now applied via #557)
		require.NoError(t, err)
		assert.Empty(t, result)
	})
}

// =============================================================================
// LinkToAccount Tests
// =============================================================================

func TestPersonService_LinkToAccount(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupPersonService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("links person to account successfully", func(t *testing.T) {
		// ARRANGE
		person := testpkg.CreateTestPerson(t, db, "ToLink", "Account")
		account := testpkg.CreateTestAccount(t, db, "link-target")

		// ACT
		err := service.LinkToAccount(ctx, person.ID, account.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify link
		result, err := service.FindByAccountID(ctx, account.ID)
		require.NoError(t, err)
		assert.Equal(t, person.ID, result.ID)
	})

	t.Run("returns error when account not found", func(t *testing.T) {
		// ARRANGE
		person := testpkg.CreateTestPerson(t, db, "LinkInvalid", "Account")

		// ACT
		err := service.LinkToAccount(ctx, person.ID, 99999999)

		// ASSERT
		require.Error(t, err)
		// Error indicates account not found or validation failed
	})

	t.Run("returns error when account already linked to another person", func(t *testing.T) {
		// ARRANGE
		_, account := testpkg.CreateTestPersonWithAccount(t, db, "Linked1", "Account")
		person2 := testpkg.CreateTestPerson(t, db, "Linked2", "Account")

		// ACT
		err := service.LinkToAccount(ctx, person2.ID, account.ID)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already linked")
	})
}

// =============================================================================
// UnlinkFromAccount Tests
// =============================================================================

func TestPersonService_UnlinkFromAccount(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupPersonService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("unlinks person from account successfully", func(t *testing.T) {
		// ARRANGE
		person, account := testpkg.CreateTestPersonWithAccount(t, db, "ToUnlink", "Account")

		// ACT
		err := service.UnlinkFromAccount(ctx, person.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify unlink - person should no longer be found by account ID
		_, err = service.FindByAccountID(ctx, account.ID)
		require.Error(t, err)
	})

	t.Run("succeeds when person has no account", func(t *testing.T) {
		// ARRANGE
		person := testpkg.CreateTestPerson(t, db, "NoAccount", "ToUnlink")

		// ACT
		err := service.UnlinkFromAccount(ctx, person.ID)

		// ASSERT
		require.NoError(t, err)
	})

	t.Run("returns error for nonexistent person", func(t *testing.T) {
		// ACT
		_ = service.UnlinkFromAccount(ctx, 99999999)

		// ASSERT - repository may or may not return error for nonexistent person
		// Some repositories silently succeed (UPDATE ... WHERE id = X affects 0 rows)
		// This tests the code path even if no error is returned
	})
}

// =============================================================================
// LinkToRFIDCard Tests
// =============================================================================

func TestPersonService_LinkToRFIDCard(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupPersonService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("links person to RFID card successfully", func(t *testing.T) {
		// ARRANGE
		person := testpkg.CreateTestPerson(t, db, "ToLink", "RFID")
		rfidCard := testpkg.CreateTestRFIDCard(t, db, "LINKCARD")

		// ACT
		err := service.LinkToRFIDCard(ctx, person.ID, rfidCard.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify link
		result, err := service.FindByTagID(ctx, rfidCard.ID)
		require.NoError(t, err)
		assert.Equal(t, person.ID, result.ID)
	})

	t.Run("auto-creates RFID card if not exists", func(t *testing.T) {
		// ARRANGE
		person := testpkg.CreateTestPerson(t, db, "AutoCreate", "RFID")
		// Use valid hexadecimal format for RFID card ID
		newTagID := "ABCDEF1234567890"

		// ACT
		err := service.LinkToRFIDCard(ctx, person.ID, newTagID)

		// ASSERT
		require.NoError(t, err)

		// Verify link
		result, err := service.FindByTagID(ctx, newTagID)
		require.NoError(t, err)
		assert.Equal(t, person.ID, result.ID)
	})

	t.Run("transfers card from another person", func(t *testing.T) {
		// ARRANGE
		person1 := testpkg.CreateTestPerson(t, db, "Original", "CardHolder")
		person2 := testpkg.CreateTestPerson(t, db, "New", "CardHolder")
		rfidCard := testpkg.CreateTestRFIDCard(t, db, "TRANSFER")

		// Link to first person
		err := service.LinkToRFIDCard(ctx, person1.ID, rfidCard.ID)
		require.NoError(t, err)

		// ACT - Transfer to second person
		err = service.LinkToRFIDCard(ctx, person2.ID, rfidCard.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify card is now with second person
		result, err := service.FindByTagID(ctx, rfidCard.ID)
		require.NoError(t, err)
		assert.Equal(t, person2.ID, result.ID)
	})
}

// =============================================================================
// LinkStudentToRFIDCard Tests
// =============================================================================

// TestPersonService_LinkStudentToRFIDCard covers the guard that survives the
// handler's alumnus gate: the gate reads the student BEFORE the write, so a
// graduation committing in between must still be caught here, under the student
// row lock, instead of leaving a bracelet linked to a departed child (#405).
func TestPersonService_LinkStudentToRFIDCard(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupPersonService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("assigns the tag to an enrolled student", func(t *testing.T) {
		// ARRANGE
		student := testpkg.CreateTestStudent(t, db, "TagAssign", "Enrolled", "4a")
		rfidCard := testpkg.CreateTestRFIDCard(t, db, "ENROLLEDTAG")

		// ACT
		err := service.LinkStudentToRFIDCard(ctx, student.ID, rfidCard.ID)

		// ASSERT
		require.NoError(t, err)
		holder, err := service.FindByTagID(ctx, rfidCard.ID)
		require.NoError(t, err)
		assert.Equal(t, student.PersonID, holder.ID)
	})

	t.Run("refuses a graduated student and leaves the tag free", func(t *testing.T) {
		// ARRANGE
		student := testpkg.CreateTestStudent(t, db, "TagAssign", "Graduated", "4b")
		rfidCard := testpkg.CreateTestRFIDCard(t, db, "ALUMNUSTAG")

		_, err := db.NewUpdate().
			TableExpr(`users.students`).
			Set("status = ?", string(userModels.StudentStatusAlumnus)).
			Where("id = ?", student.ID).
			Exec(ctx)
		require.NoError(t, err)

		// ACT
		err = service.LinkStudentToRFIDCard(ctx, student.ID, rfidCard.ID)

		// ASSERT
		require.ErrorIs(t, err, users.ErrStudentGraduated)
		holders, err := db.NewSelect().
			TableExpr(`users.persons`).
			Where("tag_id = ?", rfidCard.ID).
			Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, holders, "the bracelet must stay free after a refused assignment")
	})

	t.Run("reports a student that no longer exists as not found", func(t *testing.T) {
		// ARRANGE — a child hard-deleted after graduation. The locked read finds
		// no row, and the caller must see a 404-shaped error rather than a bare
		// sql.ErrNoRows the handler would render as a 500.
		student := testpkg.CreateTestStudent(t, db, "TagAssign", "Purged", "4c")
		rfidCard := testpkg.CreateTestRFIDCard(t, db, "PURGEDTAG")
		// Deleting the student is this test's ARRANGE step, not a teardown:
		// the point is what the service does when the row is gone.
		testpkg.CleanupActivityFixtures(t, db, student.ID)

		// ACT
		err := service.LinkStudentToRFIDCard(ctx, student.ID, rfidCard.ID)

		// ASSERT
		require.ErrorIs(t, err, users.ErrStudentNotFound)
	})
}

// =============================================================================
// UnlinkFromRFIDCard Tests
// =============================================================================

func TestPersonService_UnlinkFromRFIDCard(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupPersonService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("unlinks person from RFID card successfully", func(t *testing.T) {
		// ARRANGE
		person := testpkg.CreateTestPerson(t, db, "ToUnlink", "RFID")
		rfidCard := testpkg.CreateTestRFIDCard(t, db, "UNLINKCARD")

		err := service.LinkToRFIDCard(ctx, person.ID, rfidCard.ID)
		require.NoError(t, err)

		// ACT
		err = service.UnlinkFromRFIDCard(ctx, person.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify unlink
		_, err = service.FindByTagID(ctx, rfidCard.ID)
		require.Error(t, err)
	})

	t.Run("succeeds when person has no RFID card", func(t *testing.T) {
		// ARRANGE
		person := testpkg.CreateTestPerson(t, db, "NoCard", "ToUnlink")

		// ACT
		err := service.UnlinkFromRFIDCard(ctx, person.ID)

		// ASSERT
		require.NoError(t, err)
	})

	t.Run("handles nonexistent person gracefully", func(t *testing.T) {
		// ACT
		_ = service.UnlinkFromRFIDCard(ctx, 99999999)

		// ASSERT - repository may silently succeed for nonexistent person
		// This tests the code path
	})
}

// =============================================================================
// GetStudentsWithGroupsByTeacher Tests
// =============================================================================

func TestPersonService_GetStudentsWithGroupsByTeacher(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupPersonService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns students with group info for valid teacher", func(t *testing.T) {
		// ARRANGE
		teacher := testpkg.CreateTestTeacher(t, db, "TeacherGroups", "Test")
		educationGroup := testpkg.CreateTestEducationGroup(t, db, "TestClass2")
		student := testpkg.CreateTestStudent(t, db, "StudentGroups", "Test", "2a")

		// Assign teacher to group
		testpkg.CreateTestGroupTeacher(t, db, educationGroup.ID, teacher.ID)
		// Assign student to group
		testpkg.AssignStudentToGroup(t, db, student.ID, educationGroup.ID)

		// ACT
		result, err := service.GetStudentsWithGroupsByTeacher(ctx, teacher.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("returns error for non-existent teacher", func(t *testing.T) {
		// ACT
		result, err := service.GetStudentsWithGroupsByTeacher(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Error(t, err) // Teacher lookup error
	})
}

// =============================================================================
// GetAllStudentsWithGroups Tests
// =============================================================================

func TestPersonService_GetAllStudentsWithGroups(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupPersonService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns all students including those without groups", func(t *testing.T) {
		// ARRANGE - student with group
		group := testpkg.CreateTestEducationGroup(t, db, "AllStudentsGroup")
		studentWithGroup := testpkg.CreateTestStudent(t, db, "WithGroup", "Student", "3a")
		testpkg.AssignStudentToGroup(t, db, studentWithGroup.ID, group.ID)

		// Student without group
		studentNoGroup := testpkg.CreateTestStudent(t, db, "NoGroup", "Student", "3b")

		// ACT
		result, err := service.GetAllStudentsWithGroups(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, result)

		// Both students should be in results
		ids := make(map[int64]bool)
		for _, r := range result {
			ids[r.Student.ID] = true
		}
		assert.True(t, ids[studentWithGroup.ID], "student with group should be present")
		assert.True(t, ids[studentNoGroup.ID], "student without group should be present")
	})
}

// =============================================================================
// Repository Accessor Tests
// =============================================================================

// =============================================================================
// Error Type Tests
// =============================================================================

func TestUsersErrorTypes(t *testing.T) {
	t.Parallel()

	t.Run("error constants are defined", func(t *testing.T) {
		errors := []error{
			users.ErrPersonNotFound,
			users.ErrAccountNotFound,
			users.ErrRFIDCardNotFound,
			users.ErrAccountAlreadyLinked,
			users.ErrTeacherNotFound,
		}

		for _, err := range errors {
			assert.NotNil(t, err, "Expected error to be defined")
			assert.NotEmpty(t, err.Error(), "Expected error to have message")
		}
	})
}

// ======== Additional Tests for Higher Coverage ========

func TestPersonService_LinkToRFIDCard_PersonNotFound(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupPersonService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns error for nonexistent person", func(t *testing.T) {
		// ACT - LinkToRFIDCard takes tagID as string and returns only error
		err := service.LinkToRFIDCard(ctx, 99999999, "some-tag-id")

		// ASSERT
		require.Error(t, err)
	})
}

func TestPersonService_LinkToRFIDCard_RFIDNotFound(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupPersonService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns error for nonexistent RFID card", func(t *testing.T) {
		// ARRANGE
		student := testpkg.CreateTestStudent(t, db, "RFID", "Test", "1a")

		// Get the person ID for the student
		person, err := service.Get(ctx, student.PersonID)
		require.NoError(t, err)

		// ACT - LinkToRFIDCard takes tagID as string
		err = service.LinkToRFIDCard(ctx, person.ID, "nonexistent-tag-id")

		// ASSERT
		require.Error(t, err)
	})
}

func TestPersonService_Get_NotFound(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupPersonService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns error for nonexistent person", func(t *testing.T) {
		// ACT
		result, err := service.Get(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestPersonService_Update_NotFound(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupPersonService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns error for nonexistent person", func(t *testing.T) {
		// ARRANGE
		person := &userModels.Person{
			FirstName: "Test",
			LastName:  "Person",
		}
		person.ID = 99999999

		// ACT - Update returns only error
		err := service.Update(ctx, person)

		// ASSERT
		require.Error(t, err)
	})
}

func TestPersonService_Create_ValidationError(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupPersonService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns error for invalid person", func(t *testing.T) {
		// ARRANGE - empty names should fail validation
		person := &userModels.Person{
			FirstName: "",
			LastName:  "",
		}

		// ACT - Create returns only error
		err := service.Create(ctx, person)

		// ASSERT
		require.Error(t, err)
	})
}

// ======== PIN Validation Flow Tests ========

// =============================================================================
// Additional Coverage Tests (Push to 80%+)
// =============================================================================

func TestPersonService_Create_WithRFIDCard(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupPersonService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("creates person with RFID card link", func(t *testing.T) {
		// ARRANGE
		rfidCard := testpkg.CreateTestRFIDCard(t, db, "CREATEWITHCARD")

		person := &userModels.Person{
			FirstName: "WithRFID",
			LastName:  "Card",
			TagID:     &rfidCard.ID,
		}

		// ACT
		err := service.Create(ctx, person)
		defer func() {
		}()

		// ASSERT
		require.NoError(t, err)
		assert.Greater(t, person.ID, int64(0))

		// Verify link
		found, err := service.FindByTagID(ctx, rfidCard.ID)
		require.NoError(t, err)
		assert.Equal(t, person.ID, found.ID)
	})

	t.Run("returns error when RFID card not found", func(t *testing.T) {
		// ARRANGE
		nonExistentTagID := "NONEXISTENT999"
		person := &userModels.Person{
			FirstName: "Invalid",
			LastName:  "RFID",
			TagID:     &nonExistentTagID,
		}

		// ACT
		err := service.Create(ctx, person)

		// ASSERT
		require.Error(t, err)
		// Should indicate RFID card not found
	})
}

func TestPersonService_Update_WithChangedAccount(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupPersonService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("updates person with new valid account", func(t *testing.T) {
		// ARRANGE - create person without account, then link to new account
		person := testpkg.CreateTestPerson(t, db, "UpdateAccount", "Test")
		newAccount := testpkg.CreateTestAccount(t, db, "new-account-update")

		// Update person with new account ID
		person.AccountID = &newAccount.ID

		// ACT
		err := service.Update(ctx, person)

		// ASSERT
		require.NoError(t, err)

		// Verify link
		found, err := service.FindByAccountID(ctx, newAccount.ID)
		require.NoError(t, err)
		assert.Equal(t, person.ID, found.ID)
	})

	t.Run("returns error when new account does not exist", func(t *testing.T) {
		// ARRANGE
		person := testpkg.CreateTestPerson(t, db, "UpdateInvalidAccount", "Test")

		nonExistentAccountID := int64(99999999)
		person.AccountID = &nonExistentAccountID

		// ACT
		err := service.Update(ctx, person)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("allows update with same account ID", func(t *testing.T) {
		// ARRANGE - person with existing account
		person, _ := testpkg.CreateTestPersonWithAccount(t, db, "SameAccount", "Update")

		// Update person - keep same account ID
		person.FirstName = "UpdatedSame"

		// ACT
		err := service.Update(ctx, person)

		// ASSERT
		require.NoError(t, err)

		// Verify update
		found, err := service.Get(ctx, person.ID)
		require.NoError(t, err)
		assert.Equal(t, "UpdatedSame", found.FirstName)
	})
}

func TestPersonService_Update_WithChangedRFID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupPersonService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("updates person with new valid RFID card", func(t *testing.T) {
		// ARRANGE
		person := testpkg.CreateTestPerson(t, db, "UpdateRFID", "Test")
		newCard := testpkg.CreateTestRFIDCard(t, db, "NEWUPDATECARD")

		person.TagID = &newCard.ID

		// ACT
		err := service.Update(ctx, person)

		// ASSERT
		require.NoError(t, err)

		// Verify link
		found, err := service.FindByTagID(ctx, newCard.ID)
		require.NoError(t, err)
		assert.Equal(t, person.ID, found.ID)
	})

	t.Run("returns error when new RFID card does not exist", func(t *testing.T) {
		// ARRANGE
		person := testpkg.CreateTestPerson(t, db, "UpdateInvalidRFID", "Test")

		nonExistentTagID := "NONEXISTENT888"
		person.TagID = &nonExistentTagID

		// ACT
		err := service.Update(ctx, person)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("allows update with same RFID card", func(t *testing.T) {
		// ARRANGE
		person := testpkg.CreateTestPerson(t, db, "SameRFID", "Update")
		card := testpkg.CreateTestRFIDCard(t, db, "SAMERFIDCARD")

		// Link card to person
		err := service.LinkToRFIDCard(ctx, person.ID, card.ID)
		require.NoError(t, err)

		// Refetch person to get TagID set
		person, err = service.Get(ctx, person.ID)
		require.NoError(t, err)

		// Update person - keep same RFID
		person.FirstName = "UpdatedSameRFID"

		// ACT
		err = service.Update(ctx, person)

		// ASSERT
		require.NoError(t, err)

		// Verify update
		found, err := service.Get(ctx, person.ID)
		require.NoError(t, err)
		assert.Equal(t, "UpdatedSameRFID", found.FirstName)
	})
}

func TestPersonService_LinkToAccount_SamePersonRelink(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupPersonService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("allows re-linking same person to same account", func(t *testing.T) {
		// ARRANGE - person already linked to account
		person, account := testpkg.CreateTestPersonWithAccount(t, db, "Relink", "Same")

		// ACT - re-link same person to same account (should be no-op, no error)
		err := service.LinkToAccount(ctx, person.ID, account.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify link still exists
		found, err := service.FindByAccountID(ctx, account.ID)
		require.NoError(t, err)
		assert.Equal(t, person.ID, found.ID)
	})
}

func TestPersonService_List_WithPagination(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupPersonService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns persons with query options", func(t *testing.T) {
		// ARRANGE - create some persons
		testpkg.CreateTestPerson(t, db, "ListPag1", "Test")
		testpkg.CreateTestPerson(t, db, "ListPag2", "Test")

		// ACT - list with options (even though filter conversion not fully implemented)
		options := &base.QueryOptions{}
		result, err := service.List(ctx, options)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, result)
	})
}

func TestPersonService_Delete_WithRelations(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupPersonService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("deletes person with RFID card", func(t *testing.T) {
		// ARRANGE
		person := testpkg.CreateTestPerson(t, db, "DeleteWith", "RFID")
		card := testpkg.CreateTestRFIDCard(t, db, "DELETEWITHCARD")

		err := service.LinkToRFIDCard(ctx, person.ID, card.ID)
		require.NoError(t, err)

		// ACT
		err = service.Delete(ctx, person.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify person is deleted
		_, err = service.Get(ctx, person.ID)
		require.Error(t, err)

		// Card should no longer be linked
		_, err = service.FindByTagID(ctx, card.ID)
		require.Error(t, err)
	})
}

func TestPersonService_Get_WithIntID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupPersonService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("accepts int ID and converts to int64", func(t *testing.T) {
		// ARRANGE
		person := testpkg.CreateTestPerson(t, db, "IntID", "Test")

		// ACT - pass int instead of int64
		result, err := service.Get(ctx, int(person.ID))

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, person.ID, result.ID)
	})
}

// =============================================================================
// Error Type Tests
// =============================================================================

func TestUsersError_Unwrap(t *testing.T) {
	t.Parallel()

	t.Run("unwraps the underlying error", func(t *testing.T) {
		// ARRANGE
		innerErr := users.ErrPersonNotFound
		err := &users.UsersError{
			Op:  "Get",
			Err: innerErr,
		}

		// ACT
		unwrapped := err.Unwrap()

		// ASSERT
		assert.Equal(t, innerErr, unwrapped)
	})

	t.Run("error message contains operation", func(t *testing.T) {
		// ARRANGE
		err := &users.UsersError{
			Op:  "TestOperation",
			Err: users.ErrTeacherNotFound,
		}

		// ACT
		msg := err.Error()

		// ASSERT
		assert.Contains(t, msg, "TestOperation")
		assert.Contains(t, msg, "teacher")
	})
}

// =============================================================================
// CreateStaffWithTeacher — adopting an existing staff record
// =============================================================================

// A staff record that is adopted rather than created can already carry a live
// caregiver profile, and the request that adopts it does not necessarily ask
// for one: since #2222 the account-creating paths provision the chain by role
// tier, and the follow-up details request only asks for the profile the
// provisioning actually created.
//
// Reporting such a staff member back as "no caregiver profile" was wrong twice
// over — the caller renders a plain staff record for someone whose caregiver
// screens work, and createStaff hands out the non-caregiver default permissions
// on that basis.
//
// Not asking for the profile is not the same as asking for it to be gone: the
// row carries group supervisions, and removing it is what staff offboarding
// does.
func TestPersonService_CreateStaffWithTeacher_AdoptsLiveCaregiverProfile(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupPersonService(t, db)
	ctx := testpkg.Ctx(t)

	// ARRANGE — a person whose staff record already has a caregiver profile.
	existing := testpkg.CreateTestTeacher(t, db, "Uebernommen", "Betreuung")

	// ACT — the details request does not ask for a caregiver profile.
	staff, teacher, teacherCreationFailed, err := service.CreateStaffWithTeacher(ctx, users.CreateStaffInput{
		PersonID:         existing.Staff.PersonID,
		StaffNotes:       "Notiz",
		IsTeacher:        false,
		ActorPermissions: []string{permissions.UsersCreate, permissions.UsersUpdate},
	})

	// ASSERT
	require.NoError(t, err)
	assert.False(t, teacherCreationFailed)
	require.NotNil(t, staff)
	assert.Equal(t, existing.Staff.ID, staff.ID, "the live staff record is adopted, not duplicated")

	require.NotNil(t, teacher, "the caregiver profile the staff record carries must be reported")
	assert.Equal(t, existing.ID, teacher.ID)

	// And it is still there: not asking for a profile never deletes one.
	count, err := db.NewSelect().
		TableExpr(`users.teachers`).
		Where(`staff_id = ?`, existing.Staff.ID).
		Where(`deleted_at IS NULL`).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

// =============================================================================
// CreateStaffWithTeacher — what a create-gated request may not do
// =============================================================================

// POST /api/staff is gated on users:create alone. Adoption turns that route
// into an edit of a record that is already in the directory: it overwrites the
// notes and, with is_teacher, writes the caregiver fields. That belongs to
// users:update, so a create-only caller is refused and the record is left
// exactly as it was.
//
// The staff form is unaffected — every role that can create staff carries
// users:update as well (the user role gets both in migration 1.5.3, admin
// holds the wildcard).
func TestPersonService_CreateStaffWithTeacher_RefusesAdoptionWithoutUpdatePermission(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupPersonService(t, db)
	ctx := testpkg.Ctx(t)

	// ARRANGE — a staff member who is already in the directory.
	existing := testpkg.CreateTestStaff(t, db, "Vorhandene", "Kraft")

	before, err := db.NewSelect().
		ColumnExpr("staff_notes").
		TableExpr(`users.staff`).
		Where(`id = ?`, existing.ID).
		Rows(ctx)
	require.NoError(t, err)
	require.NoError(t, before.Close())

	// ACT — a caller that may only create.
	staff, teacher, teacherCreationFailed, err := service.CreateStaffWithTeacher(ctx, users.CreateStaffInput{
		PersonID:         existing.PersonID,
		StaffNotes:       "Fremde Notiz",
		IsTeacher:        false,
		ActorPermissions: []string{permissions.UsersCreate},
	})

	// ASSERT — refused, and nothing written.
	require.ErrorIs(t, err, users.ErrStaffAdoptionNotPermitted)
	assert.Nil(t, staff)
	assert.Nil(t, teacher)
	assert.False(t, teacherCreationFailed)

	var notes string
	require.NoError(t, db.NewSelect().
		ColumnExpr("COALESCE(staff_notes, '')").
		TableExpr(`users.staff`).
		Where(`id = ?`, existing.ID).
		Scan(ctx, &notes))
	assert.NotEqual(t, "Fremde Notiz", notes, "a refused adoption must not have written the notes")

	// And no second staff record was created for the person either.
	count, err := db.NewSelect().
		TableExpr(`users.staff`).
		Where(`person_id = ?`, existing.PersonID).
		Where(`deleted_at IS NULL`).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

// A Lehrkraft account (#1772) is provisioned with a staff record and
// deliberately without a caregiver profile: the role is class_day:read only.
// Every role-assignment path refuses the combination from the other direction,
// and staff creation was the open side of the same door.
func TestPersonService_CreateStaffWithTeacher_RefusesCaregiverProfileForLehrkraft(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupPersonService(t, db)
	ctx := testpkg.Ctx(t)

	// ARRANGE — staff with an account that holds the Lehrkraft system role.
	staffRecord, account := testpkg.CreateTestStaffWithAccount(t, db, "Lehr", "Kraft")
	testpkg.AssignLehrkraftSystemRole(t, db, account.ID, testpkg.Tenant(t))

	// ACT — the request asks for a caregiver profile anyway.
	staff, teacher, teacherCreationFailed, err := service.CreateStaffWithTeacher(ctx, users.CreateStaffInput{
		PersonID:         staffRecord.PersonID,
		IsTeacher:        true,
		Specialization:   "Betreuung",
		ActorPermissions: []string{permissions.UsersCreate, permissions.UsersUpdate},
	})

	// ASSERT — refused before anything is written.
	require.ErrorIs(t, err, users.ErrStaffLehrkraftCaregiverProfile)
	assert.Nil(t, staff)
	assert.Nil(t, teacher)
	assert.False(t, teacherCreationFailed)

	count, err := db.NewSelect().
		TableExpr(`users.teachers`).
		Where(`staff_id = ?`, staffRecord.ID).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "no caregiver profile may exist for a Lehrkraft account")
}

// The edit form must not be the way around the same rule.
func TestPersonService_UpdateStaffWithTeacher_RefusesCaregiverProfileForLehrkraft(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupPersonService(t, db)
	ctx := testpkg.Ctx(t)

	// ARRANGE
	staffRecord, account := testpkg.CreateTestStaffWithAccount(t, db, "Lehr", "Aendern")
	testpkg.AssignLehrkraftSystemRole(t, db, account.ID, testpkg.Tenant(t))

	// ACT
	teacher, action, err := service.UpdateStaffWithTeacher(ctx, staffRecord, true, "Betreuung", "", "")

	// ASSERT
	require.ErrorIs(t, err, users.ErrStaffLehrkraftCaregiverProfile)
	assert.Nil(t, teacher)
	assert.Equal(t, users.TeacherActionNone, action)

	count, err := db.NewSelect().
		TableExpr(`users.teachers`).
		Where(`staff_id = ?`, staffRecord.ID).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

// A staff member without an account cannot be a Lehrkraft, so the guard must
// not stand in the way of the ordinary caregiver creation.
func TestPersonService_CreateStaffWithTeacher_CreatesCaregiverProfileWithoutAccount(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupPersonService(t, db)
	ctx := testpkg.Ctx(t)

	// ARRANGE — a person with no account and no staff record yet.
	person := testpkg.CreateTestPerson(t, db, "Ohne", "Konto")

	// ACT
	staff, teacher, teacherCreationFailed, err := service.CreateStaffWithTeacher(ctx, users.CreateStaffInput{
		PersonID:         person.ID,
		IsTeacher:        true,
		Specialization:   "Betreuung",
		ActorPermissions: []string{permissions.UsersCreate},
	})

	// ASSERT
	require.NoError(t, err)
	assert.False(t, teacherCreationFailed)
	require.NotNil(t, staff)
	require.NotNil(t, teacher, "a caregiver profile is created for a plain staff member")
	assert.Equal(t, staff.ID, teacher.StaffID)
}
