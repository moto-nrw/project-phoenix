// Class-list entry service tests (#2382): hermetic pattern, real DB fixtures.
package users_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func setupClassListEntryService(t *testing.T, db *bun.DB) (usersService.ClassListEntryService, *repositories.Factory) {
	t.Helper()
	repos := repositories.NewFactory(db)
	svc := usersService.NewClassListEntryService(repos.ClassListEntry, repos.Student, repos.ClassListEntryChange)
	return svc, repos
}

func TestClassListEntryService_CreateUpdateDelete(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	svc, repos := setupClassListEntryService(t, db)
	ctx := testpkg.TenantContext(1)

	actor := testpkg.CreateTestAccount(t, db, "cle-actor@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, actor.ID)

	t.Run("create records audit and trims input", func(t *testing.T) {
		entry, err := svc.Create(ctx, usersService.ClassListEntryInput{
			FirstName:   " CleVorname ",
			LastName:    " CleNachname ",
			SchoolClass: " 7z ",
		}, actor.ID)
		require.NoError(t, err)
		defer testpkg.CleanupClassListEntryFixtures(t, db, entry.ID)

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
		entry := testpkg.CreateTestClassListEntry(t, db, "CleDup", "Kind", "7z")
		_ = entry

		_, err := svc.Create(ctx, usersService.ClassListEntryInput{
			FirstName:   "cledup",
			LastName:    "KIND",
			SchoolClass: " 7Z ",
		}, actor.ID)
		require.ErrorIs(t, err, usersService.ErrClassListEntryDuplicate)
	})

	t.Run("existing student with same name and class is rejected", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "CleStudent", "Kind", "7z")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	svc, repos := setupClassListEntryService(t, db)
	ctx := testpkg.TenantContext(1)

	actor := testpkg.CreateTestAccount(t, db, "cle-assign-actor@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, actor.ID)

	t.Run("assign deletes the entry and records the matched student", func(t *testing.T) {
		entry := testpkg.CreateTestClassListEntry(t, db, "CleAssign", "Kind", "7y")
		student := testpkg.CreateTestStudent(t, db, "CleAssign", "Kind", "7y")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

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

	t.Run("list reports the matching student as a hint", func(t *testing.T) {
		entry := testpkg.CreateTestClassListEntry(t, db, "CleMatch", "Kind", "7y")
		// The student arrives AFTER the entry (enrollment approval, import) —
		// exactly the Dubletten case the hint exists for.
		student := testpkg.CreateTestStudent(t, db, "CleMatch", "Kind", "7y")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

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

func TestClassListEntryService_TenantIsolation(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	svc, _ := setupClassListEntryService(t, db)

	testpkg.EnsureTestTenant(t, db, 42)
	foreign := testpkg.CreateTestClassListEntryForTenant(t, db, 42, "CleForeign", "Kind", "9x")

	t.Run("foreign tenant entries are invisible", func(t *testing.T) {
		entries, err := svc.ListAll(testpkg.TenantContext(1))
		require.NoError(t, err)
		for _, entry := range entries {
			assert.NotEqual(t, foreign.ID, entry.ID)
		}
	})

	t.Run("foreign tenant entries cannot be deleted", func(t *testing.T) {
		actor := testpkg.CreateTestAccount(t, db, "cle-iso-actor@test.local")
		defer testpkg.CleanupAuthFixtures(t, db, actor.ID)

		err := svc.Delete(testpkg.TenantContext(1), foreign.ID, actor.ID)
		require.ErrorIs(t, err, usersService.ErrClassListEntryNotFound)
	})
}
