// Class-list entry service tests (#2382): hermetic pattern, real DB fixtures.
package users_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func setupClassListEntryService(t *testing.T, db *bun.DB) (usersService.ClassListEntryService, *repositories.Factory) {
	t.Helper()
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	svc := usersService.NewClassListEntryService(repos.ClassListEntry, repos.Student, repos.ClassListEntryChange)
	return svc, repos
}

func TestClassListEntryService_CreateUpdateDelete(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	svc, repos := setupClassListEntryService(t, db)
	ctx := testpkg.Ctx(t)

	actor := testpkg.CreateTestAccount(t, db, "cle-actor@test.local")

	t.Run("create records audit and trims input", func(t *testing.T) {
		entry, err := svc.Create(ctx, usersService.ClassListEntryInput{
			FirstName:   " CleVorname ",
			LastName:    " CleNachname ",
			SchoolClass: " 7z ",
		}, actor.ID)
		require.NoError(t, err)

		assert.Equal(t, "CleVorname", entry.FirstName)
		assert.Equal(t, "7z", entry.SchoolClass)
		require.NotNil(t, entry.CreatedBy)
		assert.Equal(t, actor.ID, *entry.CreatedBy)

		trail, err := repos.ClassListEntryChange.ListByEntryID(ctx, entry.ID)
		require.NoError(t, err)
		require.Len(t, trail, 1)
		assert.Equal(t, auditModels.ClassListEntryActionCreated, trail[0].Action)
		assert.Equal(t, "CleVorname CleNachname (7z)", trail[0].NewValue)
	})

	t.Run("duplicate entry is rejected case-insensitively", func(t *testing.T) {
		testpkg.CreateTestClassListEntry(t, db, "CleDup", "Kind", "7z")

		_, err := svc.Create(ctx, usersService.ClassListEntryInput{
			FirstName:   "cledup",
			LastName:    "KIND",
			SchoolClass: " 7Z ",
		}, actor.ID)
		require.ErrorIs(t, err, usersService.ErrClassListEntryDuplicate)
	})

	t.Run("existing student with same name and class is rejected", func(t *testing.T) {
		testpkg.CreateTestStudent(t, db, "CleStudent", "Kind", "7z")

		_, err := svc.Create(ctx, usersService.ClassListEntryInput{
			FirstName:   "CleStudent",
			LastName:    "Kind",
			SchoolClass: "7z",
		}, actor.ID)
		require.ErrorIs(t, err, usersService.ErrClassListEntryStudentExists)
	})

	t.Run("update moves the entry to another class and records audit", func(t *testing.T) {
		entry := testpkg.CreateTestClassListEntry(t, db, "CleMove", "Kind", "7z")

		updated, err := svc.Update(ctx, entry.ID, usersService.ClassListEntryInput{
			FirstName:   "CleMove",
			LastName:    "Kind",
			SchoolClass: "8z",
		}, actor.ID)
		require.NoError(t, err)
		assert.Equal(t, "8z", updated.SchoolClass)

		trail, err := repos.ClassListEntryChange.ListByEntryID(ctx, entry.ID)
		require.NoError(t, err)
		require.Len(t, trail, 1)
		assert.Equal(t, auditModels.ClassListEntryActionUpdated, trail[0].Action)
		assert.Equal(t, "CleMove Kind (7z)", trail[0].OldValue)
		assert.Equal(t, "CleMove Kind (8z)", trail[0].NewValue)
	})

	t.Run("delete removes the entry and records audit", func(t *testing.T) {
		entry := testpkg.CreateTestClassListEntry(t, db, "CleDelete", "Kind", "7z")

		require.NoError(t, svc.Delete(ctx, entry.ID, actor.ID))

		entries, err := svc.ListAll(ctx)
		require.NoError(t, err)
		for _, remaining := range entries {
			assert.NotEqual(t, entry.ID, remaining.ID)
		}

		trail, err := repos.ClassListEntryChange.ListByEntryID(ctx, entry.ID)
		require.NoError(t, err)
		require.Len(t, trail, 1)
		assert.Equal(t, auditModels.ClassListEntryActionDeleted, trail[0].Action)
	})

	t.Run("missing entry yields not found", func(t *testing.T) {
		err := svc.Delete(ctx, int64(987654321), actor.ID)
		require.ErrorIs(t, err, usersService.ErrClassListEntryNotFound)
	})
}

func TestClassListEntryService_AssignResolvesDuplicate(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	svc, repos := setupClassListEntryService(t, db)
	ctx := testpkg.Ctx(t)

	actor := testpkg.CreateTestAccount(t, db, "cle-assign-actor@test.local")

	t.Run("assign deletes the entry and records the matched student", func(t *testing.T) {
		entry := testpkg.CreateTestClassListEntry(t, db, "CleAssign", "Kind", "7y")
		student := testpkg.CreateTestStudent(t, db, "CleAssign", "Kind", "7y")

		require.NoError(t, svc.Assign(ctx, entry.ID, student.ID, actor.ID))

		entries, err := svc.ListAll(ctx)
		require.NoError(t, err)
		for _, remaining := range entries {
			assert.NotEqual(t, entry.ID, remaining.ID)
		}

		trail, err := repos.ClassListEntryChange.ListByEntryID(ctx, entry.ID)
		require.NoError(t, err)
		require.Len(t, trail, 1)
		assert.Equal(t, auditModels.ClassListEntryActionAssigned, trail[0].Action)
		require.NotNil(t, trail[0].MatchedStudentID)
		assert.Equal(t, student.ID, *trail[0].MatchedStudentID)
	})

	t.Run("assign to a missing student is refused", func(t *testing.T) {
		entry := testpkg.CreateTestClassListEntry(t, db, "CleAssignMiss", "Kind", "7y")

		err := svc.Assign(ctx, entry.ID, int64(987654321), actor.ID)
		require.ErrorIs(t, err, usersService.ErrClassListEntryStudentNotFound)
	})

	t.Run("assign to a student with a different name or class is refused", func(t *testing.T) {
		entry := testpkg.CreateTestClassListEntry(t, db, "CleAssignFremd", "Kind", "7y")
		otherName := testpkg.CreateTestStudent(t, db, "CleAnders", "Kind", "7y")
		otherClass := testpkg.CreateTestStudent(t, db, "CleAssignFremd", "Kind", "8y")

		require.ErrorIs(t, svc.Assign(ctx, entry.ID, otherName.ID, actor.ID),
			usersService.ErrClassListEntryAssignMismatch,
			"a student with another name must not swallow the entry")
		require.ErrorIs(t, svc.Assign(ctx, entry.ID, otherClass.ID, actor.ID),
			usersService.ErrClassListEntryAssignMismatch,
			"a student in another class must not swallow the entry")

		// The entry survives both refused attempts.
		entries, err := svc.ListAll(ctx)
		require.NoError(t, err)
		var found bool
		for _, remaining := range entries {
			if remaining.ID == entry.ID {
				found = true
			}
		}
		assert.True(t, found, "the entry must still exist after refused assigns")
	})

	t.Run("list reports the matching student as a hint", func(t *testing.T) {
		entry := testpkg.CreateTestClassListEntry(t, db, "CleMatch", "Kind", "7y")
		// The student arrives AFTER the entry (enrollment approval, import) —
		// exactly the Dubletten case the hint exists for.
		student := testpkg.CreateTestStudent(t, db, "CleMatch", "Kind", "7y")

		listed, err := svc.List(ctx)
		require.NoError(t, err)
		var found bool
		for _, item := range listed {
			if item.Entry.ID == entry.ID {
				found = true
				assert.Contains(t, item.MatchingStudentIDs, student.ID)
			}
		}
		require.True(t, found, "entry must be listed")
	})
}

func TestClassListEntryService_ListAllSortsClassThenName(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	svc, _ := setupClassListEntryService(t, db)
	ctx := testpkg.Ctx(t)

	// Deliberately created out of order: 10x sorts AFTER 9x (numeric class
	// collation), names sort by German collation within the class.
	third := testpkg.CreateTestClassListEntry(t, db, "CleSortA", "Kind", "10x")
	second := testpkg.CreateTestClassListEntry(t, db, "CleSortB", "Zorn", "9x")
	first := testpkg.CreateTestClassListEntry(t, db, "CleSortC", "Aalders", "9x")

	entries, err := svc.ListAll(ctx)
	require.NoError(t, err)

	positions := map[int64]int{}
	for i, entry := range entries {
		positions[entry.ID] = i
	}
	require.Contains(t, positions, first.ID)
	assert.Less(t, positions[first.ID], positions[second.ID],
		"within a class the names sort by German collation")
	assert.Less(t, positions[second.ID], positions[third.ID],
		"class 9x sorts before 10x")
}

func TestClassListEntryService_UpdateGuards(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	svc, repos := setupClassListEntryService(t, db)
	ctx := testpkg.Ctx(t)

	actor := testpkg.CreateTestAccount(t, db, "cle-update-actor@test.local")

	t.Run("unchanged update writes nothing and records no audit", func(t *testing.T) {
		entry := testpkg.CreateTestClassListEntry(t, db, "CleNoop", "Kind", "6v")

		updated, err := svc.Update(ctx, entry.ID, usersService.ClassListEntryInput{
			FirstName:   " CleNoop ",
			LastName:    "Kind",
			SchoolClass: "6v",
		}, actor.ID)
		require.NoError(t, err)
		assert.Equal(t, entry.ID, updated.ID)

		trail, err := repos.ClassListEntryChange.ListByEntryID(ctx, entry.ID)
		require.NoError(t, err)
		assert.Empty(t, trail, "a no-op update must not spam the audit trail")
	})

	t.Run("the unique index reports as the documented duplicate error", func(t *testing.T) {
		// The advisory duplicate check cannot catch a concurrent create; the
		// DB index is the backstop. Pin that a raw insert collision carries
		// the index name the service maps to ErrClassListEntryDuplicate.
		testpkg.CreateTestClassListEntry(t, db, "CleRace", "Kind", "6v")

		clone := &userModels.ClassListEntry{
			FirstName:   "clerace",
			LastName:    "KIND",
			SchoolClass: "6V",
		}
		err := repos.ClassListEntry.Create(testpkg.Ctx(t), clone)
		require.Error(t, err)
		assert.True(t, modelBase.IsUniqueViolationOn(err, userModels.ClassListEntryUniqueIndexName),
			"the collision must be recognizable via the pinned index name")
	})

	t.Run("update into an existing entry's identity is refused", func(t *testing.T) {
		entry := testpkg.CreateTestClassListEntry(t, db, "CleMoveDup", "Kind", "6v")
		testpkg.CreateTestClassListEntry(t, db, "CleBlock", "Kind", "6v")

		_, err := svc.Update(ctx, entry.ID, usersService.ClassListEntryInput{
			FirstName:   "cleblock",
			LastName:    "KIND",
			SchoolClass: "6v",
		}, actor.ID)
		require.ErrorIs(t, err, usersService.ErrClassListEntryDuplicate)
	})

	t.Run("update to whitespace-only fields fails validation", func(t *testing.T) {
		entry := testpkg.CreateTestClassListEntry(t, db, "CleLeer", "Kind", "6v")

		_, err := svc.Update(ctx, entry.ID, usersService.ClassListEntryInput{
			FirstName:   "  ",
			LastName:    "Kind",
			SchoolClass: "6v",
		}, actor.ID)
		require.Error(t, err)
	})

	t.Run("create with whitespace-only fields fails validation", func(t *testing.T) {
		_, err := svc.Create(ctx, usersService.ClassListEntryInput{
			FirstName:   "CleLeer2",
			LastName:    " ",
			SchoolClass: "6v",
		}, actor.ID)
		require.Error(t, err)
	})
}

func TestClassListEntryService_MissingTargets(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	svc, _ := setupClassListEntryService(t, db)
	ctx := testpkg.Ctx(t)

	actor := testpkg.CreateTestAccount(t, db, "cle-missing-actor@test.local")

	missingID := int64(987654321)

	_, err := svc.Update(ctx, missingID, usersService.ClassListEntryInput{
		FirstName: "Cle", LastName: "Weg", SchoolClass: "5u",
	}, actor.ID)
	require.ErrorIs(t, err, usersService.ErrClassListEntryNotFound)

	require.ErrorIs(t, svc.Delete(ctx, missingID, actor.ID), usersService.ErrClassListEntryNotFound)
	require.ErrorIs(t, svc.Assign(ctx, missingID, actor.ID, actor.ID), usersService.ErrClassListEntryNotFound)

	t.Run("assign to an alumnus is refused", func(t *testing.T) {
		entry := testpkg.CreateTestClassListEntry(t, db, "CleAlt", "Kind", "5u")
		student := testpkg.CreateTestStudent(t, db, "CleAlt", "Kind", "5u")

		_, err := db.NewUpdate().TableExpr("users.students").
			Set("status = ?", "alumnus").
			Where("id = ?", student.ID).
			Exec(ctx)
		require.NoError(t, err)

		require.ErrorIs(t, svc.Assign(ctx, entry.ID, student.ID, actor.ID),
			usersService.ErrClassListEntryStudentNotFound,
			"an alumnus must not be an assign target")
	})
}

// Errors from the storage layer must propagate, not vanish: a canceled
// context fails the first repository call of every service path.
func TestClassListEntryService_StorageErrorsPropagate(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	svc, repos := setupClassListEntryService(t, db)

	entry := testpkg.CreateTestClassListEntry(t, db, "CleErr", "Kind", "4t")

	canceled, cancel := context.WithCancel(testpkg.Ctx(t))
	cancel()

	_, err := svc.ListAll(canceled)
	require.Error(t, err)
	_, err = svc.List(canceled)
	require.Error(t, err)
	_, err = svc.Create(canceled, usersService.ClassListEntryInput{
		FirstName: "CleErr2", LastName: "Kind", SchoolClass: "4t",
	}, 0)
	require.Error(t, err)
	_, err = svc.Update(canceled, entry.ID, usersService.ClassListEntryInput{
		FirstName: "CleErr", LastName: "Kind", SchoolClass: "4u",
	}, 0)
	require.Error(t, err)
	require.Error(t, svc.Delete(canceled, entry.ID, 0))
	require.Error(t, svc.Assign(canceled, entry.ID, entry.ID, 0))

	_, err = repos.ClassListEntry.FindBySchoolClass(canceled, "4t")
	require.Error(t, err)
	_, err = repos.ClassListEntry.FindByNameAndClass(canceled, "CleErr", "Kind", "4t")
	require.Error(t, err)
	_, err = repos.ClassListEntryChange.ListByEntryID(canceled, entry.ID)
	require.Error(t, err)
	require.Error(t, repos.ClassListEntryChange.Create(canceled, &auditModels.ClassListEntryChange{
		EntryID: entry.ID, Action: auditModels.ClassListEntryActionCreated, ChangedBy: 1,
	}))
	require.Error(t, repos.ClassListEntryChange.Create(testpkg.Ctx(t), &auditModels.ClassListEntryChange{
		EntryID: entry.ID, Action: "renamed", ChangedBy: 1,
	}), "an invalid audit row must be rejected by validation")
}

func TestClassListEntryService_TenantIsolation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	svc, _ := setupClassListEntryService(t, db)

	foreignTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenant)
	foreign := testpkg.CreateTestClassListEntryForTenant(t, db, foreignTenant, "CleForeign", "Kind", "9x")

	t.Run("foreign tenant entries are invisible", func(t *testing.T) {
		entries, err := svc.ListAll(testpkg.Ctx(t))
		require.NoError(t, err)
		for _, entry := range entries {
			assert.NotEqual(t, foreign.ID, entry.ID)
		}
	})

	t.Run("foreign tenant entries cannot be deleted", func(t *testing.T) {
		actor := testpkg.CreateTestAccount(t, db, "cle-iso-actor@test.local")

		err := svc.Delete(testpkg.Ctx(t), foreign.ID, actor.ID)
		require.ErrorIs(t, err, usersService.ErrClassListEntryNotFound)
	})
}
