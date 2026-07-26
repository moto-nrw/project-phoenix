package users_test

import (
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// ============================================================================
// Test Helpers
// ============================================================================

// createTestStudentGuardian creates a student-guardian relationship in the database.
func createTestStudentGuardian(t *testing.T, db *bun.DB, studentID, guardianProfileID int64, relType string, isPrimary bool) *users.StudentGuardian {
	t.Helper()

	ctx := testpkg.TenantContext(1)
	sg := &users.StudentGuardian{
		StudentID:          studentID,
		GuardianProfileID:  guardianProfileID,
		RelationshipType:   relType,
		IsPrimary:          isPrimary,
		IsEmergencyContact: false,
		CanPickup:          true,
		Permissions:        map[string]interface{}{},
	}
	sg.SetTenantID(1)

	_, err := db.NewInsert().
		Model(sg).
		ModelTableExpr(`users.students_guardians`).
		Exec(ctx)
	require.NoError(t, err)

	return sg
}

// cleanupStudentGuardians removes student_guardian records by ID.
func cleanupStudentGuardians(t *testing.T, db *bun.DB, ids ...int64) {
	t.Helper()
	testpkg.CleanupTableRecords(t, db, "users.students_guardians", ids...)
}

// ============================================================================
// FindByStudentID Tests
// ============================================================================

func TestStudentGuardianRepository_FindByStudentID_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentGuardian
	ctx := testpkg.TenantContext(1)

	// Create dependencies
	student := testpkg.CreateTestStudent(t, db, "Guardian", "Student", "1a")
	guardian1 := testpkg.CreateTestGuardianProfile(t, db, "guardian1")
	guardian2 := testpkg.CreateTestGuardianProfile(t, db, "guardian2")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID, guardian1.ID, guardian2.ID)

	// Create relationships
	sg1 := createTestStudentGuardian(t, db, student.ID, guardian1.ID, "parent", true)
	sg2 := createTestStudentGuardian(t, db, student.ID, guardian2.ID, "parent", false)
	defer cleanupStudentGuardians(t, db, sg1.ID, sg2.ID)

	// ACT
	results, err := repo.FindByStudentID(ctx, student.ID)

	// ASSERT
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(results), 2)

	for _, sg := range results {
		assert.Equal(t, student.ID, sg.StudentID)
	}
}

func TestStudentGuardianRepository_FindByStudentID_Empty(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentGuardian
	ctx := testpkg.TenantContext(1)

	// Create student with no guardians
	student := testpkg.CreateTestStudent(t, db, "NoGuardian", "Student", "1b")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	// ACT
	results, err := repo.FindByStudentID(ctx, student.ID)

	// ASSERT
	require.NoError(t, err)
	assert.Empty(t, results)
}

// ============================================================================
// FindByGuardianProfileID Tests
// ============================================================================

func TestStudentGuardianRepository_FindByGuardianProfileID_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentGuardian
	ctx := testpkg.TenantContext(1)

	// Create dependencies
	student1 := testpkg.CreateTestStudent(t, db, "Multi1", "Student", "2a")
	student2 := testpkg.CreateTestStudent(t, db, "Multi2", "Student", "2b")
	guardian := testpkg.CreateTestGuardianProfile(t, db, "multi-guardian")
	defer testpkg.CleanupActivityFixtures(t, db, student1.ID, student2.ID, guardian.ID)

	// Create relationships - same guardian, different students
	sg1 := createTestStudentGuardian(t, db, student1.ID, guardian.ID, "parent", true)
	sg2 := createTestStudentGuardian(t, db, student2.ID, guardian.ID, "parent", true)
	defer cleanupStudentGuardians(t, db, sg1.ID, sg2.ID)

	// ACT
	results, err := repo.FindByGuardianProfileID(ctx, guardian.ID)

	// ASSERT
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(results), 2)

	for _, sg := range results {
		assert.Equal(t, guardian.ID, sg.GuardianProfileID)
	}
}

// ============================================================================
// FindPrimaryByStudentID Tests
// ============================================================================

// ============================================================================
// FindEmergencyContactsByStudentID Tests
// ============================================================================

// ============================================================================
// FindPickupAuthoritiesByStudentID Tests
// ============================================================================

// ============================================================================
// SetPrimary Tests
// ============================================================================

func TestStudentGuardianRepository_SetPrimary_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentGuardian
	ctx := testpkg.TenantContext(1)

	// Create dependencies
	student := testpkg.CreateTestStudent(t, db, "SetPrimary", "Student", "6a")
	guardian := testpkg.CreateTestGuardianProfile(t, db, "set-primary-guardian")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID, guardian.ID)

	// Create relationship that is NOT primary
	sg := createTestStudentGuardian(t, db, student.ID, guardian.ID, "parent", false)
	defer cleanupStudentGuardians(t, db, sg.ID)

	// ACT
	err := repo.SetPrimary(ctx, sg.ID, true)

	// ASSERT
	require.NoError(t, err)

	// Verify it was updated
	results, err := repo.FindByStudentID(ctx, student.ID)
	require.NoError(t, err)

	var found bool
	for _, r := range results {
		if r.ID == sg.ID {
			found = true
			assert.True(t, r.IsPrimary)
			break
		}
	}
	assert.True(t, found)
}

// ============================================================================
// SetEmergencyContact Tests
// ============================================================================

// ============================================================================
// SetCanPickup Tests
// ============================================================================

// ============================================================================
// UpdatePermissions Tests
// ============================================================================

// ============================================================================
// Create Tests
// ============================================================================

func TestStudentGuardianRepository_Create_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentGuardian
	ctx := testpkg.TenantContext(1)

	// Create dependencies
	student := testpkg.CreateTestStudent(t, db, "CreateSG", "Student", "10a")
	guardian := testpkg.CreateTestGuardianProfile(t, db, "create-sg-guardian")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID, guardian.ID)

	// ACT
	sg := &users.StudentGuardian{
		StudentID:          student.ID,
		GuardianProfileID:  guardian.ID,
		RelationshipType:   "parent",
		IsPrimary:          true,
		IsEmergencyContact: false,
		CanPickup:          true,
		Permissions:        map[string]interface{}{},
	}
	err := repo.Create(ctx, sg)

	// ASSERT
	require.NoError(t, err)
	assert.NotZero(t, sg.ID)
	defer cleanupStudentGuardians(t, db, sg.ID)

	// Verify it was created
	results, err := repo.FindByStudentID(ctx, student.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, results)
}

func TestStudentGuardianRepository_Create_NilReturnsError(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentGuardian
	ctx := testpkg.TenantContext(1)

	// ACT
	err := repo.Create(ctx, nil)

	// ASSERT
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

func TestStudentGuardianRepository_Create_ValidationError(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentGuardian
	ctx := testpkg.TenantContext(1)

	// ACT - Create with invalid data
	sg := &users.StudentGuardian{
		StudentID:         0, // Invalid
		GuardianProfileID: 0, // Invalid
		RelationshipType:  "",
	}
	err := repo.Create(ctx, sg)

	// ASSERT
	require.Error(t, err)
}

// ============================================================================
// FindByRelationshipType Tests
// ============================================================================

// ============================================================================
// List Tests
// ============================================================================

func TestStudentGuardianRepository_List_WithFilters(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentGuardian
	ctx := testpkg.TenantContext(1)

	// Create dependencies
	student := testpkg.CreateTestStudent(t, db, "ListTest", "Student", "13a")
	guardian := testpkg.CreateTestGuardianProfile(t, db, "list-guardian")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID, guardian.ID)

	// Create primary guardian
	sg := &users.StudentGuardian{
		StudentID:          student.ID,
		GuardianProfileID:  guardian.ID,
		RelationshipType:   "parent",
		IsPrimary:          true,
		IsEmergencyContact: true,
		CanPickup:          true,
		Permissions:        map[string]interface{}{},
	}
	sg.SetTenantID(1)
	_, err := db.NewInsert().Model(sg).ModelTableExpr(`users.students_guardians`).Exec(ctx)
	require.NoError(t, err)
	defer cleanupStudentGuardians(t, db, sg.ID)

	// ACT
	results, err := repo.List(ctx, map[string]interface{}{
		"is_primary": true,
	})

	// ASSERT
	require.NoError(t, err)
	for _, r := range results {
		assert.True(t, r.IsPrimary)
	}
}

// TestStudentGuardianRepository_LinkIfNotExists unit-tests the idempotent link
// used by the select-existing-guardian path (#1513) directly at the repo layer.
// It pins three contracts the service relies on: a fresh link inserts and
// reports true, a duplicate link is a silent no-op reporting false (never a
// unique-violation that would abort the surrounding tenant tx), and tenant_id is
// auto-populated from context when the caller leaves it unset.
func TestStudentGuardianRepository_LinkIfNotExists(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentGuardian
	ctx := testpkg.TenantContext(1)

	t.Run("inserts a new link and reports it as inserted", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Link", "Student", "1a")
		guardian := testpkg.CreateTestGuardianProfile(t, db, "linkfresh")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID, guardian.ID)

		rel := &users.StudentGuardian{
			StudentID:         student.ID,
			GuardianProfileID: guardian.ID,
			RelationshipType:  "parent",
			CanPickup:         true,
			EmergencyPriority: 1,
		}
		rel.SetTenantID(1)

		inserted, err := repo.LinkIfNotExists(ctx, rel)
		defer cleanupStudentGuardians(t, db, rel.ID)
		require.NoError(t, err)
		assert.True(t, inserted, "a brand-new link must report inserted=true")
		assert.Equal(t, authorize.GuardianRoleLegalGuardian, rel.GuardianRole)
		assert.True(t, authorize.StudentGuardianHasPermission(rel, authorize.GuardianPermissionPortalAccess))
	})

	t.Run("treats a duplicate link as a no-op reporting not-inserted", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Dup", "Student", "1b")
		guardian := testpkg.CreateTestGuardianProfile(t, db, "linkdup")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID, guardian.ID)

		first := &users.StudentGuardian{
			StudentID:         student.ID,
			GuardianProfileID: guardian.ID,
			RelationshipType:  "parent",
			EmergencyPriority: 1,
		}
		first.SetTenantID(1)
		inserted, err := repo.LinkIfNotExists(ctx, first)
		require.NoError(t, err)
		require.True(t, inserted)
		defer cleanupStudentGuardians(t, db, first.ID)

		// Same (tenant, student, guardian) again — must NOT raise a unique
		// violation and must report that nothing new was inserted.
		dup := &users.StudentGuardian{
			StudentID:         student.ID,
			GuardianProfileID: guardian.ID,
			RelationshipType:  "guardian",
			EmergencyPriority: 2,
		}
		dup.SetTenantID(1)
		inserted, err = repo.LinkIfNotExists(ctx, dup)
		require.NoError(t, err, "a duplicate link must not error")
		assert.False(t, inserted, "a duplicate link must report inserted=false")

		// Exactly one row survives for this (student, guardian) pair.
		count, err := repo.List(ctx, map[string]interface{}{"student_id": student.ID})
		require.NoError(t, err)
		var matched int
		for _, l := range count {
			if l.GuardianProfileID == guardian.ID {
				matched++
			}
		}
		assert.Equal(t, 1, matched, "the duplicate must not create a second row")
	})

	t.Run("auto-populates tenant_id from context when unset", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Auto", "Student", "1c")
		guardian := testpkg.CreateTestGuardianProfile(t, db, "linkauto")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID, guardian.ID)

		// TenantID deliberately left at zero — the repo must fill it from ctx.
		rel := &users.StudentGuardian{
			StudentID:         student.ID,
			GuardianProfileID: guardian.ID,
			RelationshipType:  "parent",
			EmergencyPriority: 1,
		}
		inserted, err := repo.LinkIfNotExists(ctx, rel)
		defer cleanupStudentGuardians(t, db, rel.ID)
		require.NoError(t, err)
		require.True(t, inserted)
		assert.Equal(t, int64(1), rel.GetTenantID(), "tenant_id must be auto-set from context")
		assert.Equal(t, authorize.GuardianRoleLegalGuardian, rel.GuardianRole)
		assert.True(t, authorize.StudentGuardianHasPermission(rel, authorize.GuardianPermissionPortalAccess))
	})

	t.Run("defaults pickup-style links without portal access", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Pickup", "Student", "1d")
		guardian := testpkg.CreateTestGuardianProfile(t, db, "linkpickup")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID, guardian.ID)

		rel := &users.StudentGuardian{
			StudentID:         student.ID,
			GuardianProfileID: guardian.ID,
			RelationshipType:  "other",
			CanPickup:         true,
			EmergencyPriority: 1,
		}
		inserted, err := repo.LinkIfNotExists(ctx, rel)
		defer cleanupStudentGuardians(t, db, rel.ID)
		require.NoError(t, err)
		require.True(t, inserted)
		assert.Equal(t, authorize.GuardianRolePickupOnly, rel.GuardianRole)
		assert.False(t, authorize.StudentGuardianHasPermission(rel, authorize.GuardianPermissionPortalAccess))
	})
}

// TestStudentGuardianRepository_ListLinkedChildrenForGuardians_EmptyInput pins
// the early-return contract the picker batch relies on: an empty id slice yields
// an empty result with no query (and no error), so SearchGuardiansForPicker can
// call it unconditionally.
func TestStudentGuardianRepository_ListLinkedChildrenForGuardians_EmptyInput(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentGuardian
	ctx := testpkg.TenantContext(1)

	rows, err := repo.ListLinkedChildrenForGuardians(ctx, []int64{})
	require.NoError(t, err)
	assert.Empty(t, rows, "an empty guardian-id slice must return no rows")
}

// TestStudentGuardianRepository_FindByStudentAndGuardianForUpdate covers the
// FOR UPDATE row lock the parent guardian write paths rely on (PR #1743 review,
// finding 3). UpdateGuardianContact and UpdateGuardianRelationship read the
// relationship row, decide authorization from its role/account, then write — a
// TOCTOU window a concurrent staff edit of the SAME students_guardians row could
// otherwise slip through (promote/demote/remove the link between the check and
// the write). FindByStudentAndGuardianForUpdate closes it: the FOR UPDATE lock
// makes a staff UPDATE/DELETE of that row block until the parent tx commits, so
// the authorization decision is made on a row that cannot change underneath it.
// Drop the For("UPDATE") clause and the blocking subtest fails.
func TestStudentGuardianRepository_FindByStudentAndGuardianForUpdate(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentGuardian
	ctx := testpkg.TenantContext(1)

	student := testpkg.CreateTestStudent(t, db, "Lock", "Relationship", "1a")
	guardian := testpkg.CreateTestGuardianProfile(t, db, "lock-relationship-guardian")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID, guardian.ID)
	sg := createTestStudentGuardian(t, db, student.ID, guardian.ID, "relative", false)
	defer cleanupStudentGuardians(t, db, sg.ID)

	t.Run("returns the relationship row", func(t *testing.T) {
		got, err := repo.FindByStudentAndGuardianForUpdate(ctx, student.ID, guardian.ID)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, sg.ID, got.ID)
	})

	t.Run("returns ErrStudentGuardianNotFound for an unlinked pair", func(t *testing.T) {
		_, err := repo.FindByStudentAndGuardianForUpdate(ctx, student.ID, int64(999_999_999))
		require.ErrorIs(t, err, users.ErrStudentGuardianNotFound)
	})

	t.Run("FOR UPDATE blocks a concurrent staff write of the same row", func(t *testing.T) {
		// Parent side: hold the FOR UPDATE lock from a real tx, uncommitted, so a
		// second tx can observe it.
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()
		holdCtx := modelBase.ContextWithTx(ctx, &tx)

		locked, err := repo.FindByStudentAndGuardianForUpdate(holdCtx, student.ID, guardian.ID)
		require.NoError(t, err)
		require.Equal(t, sg.ID, locked.ID)

		// Staff side (the path that does NOT take the guardian-profile lock): a
		// plain UPDATE of the relationship row on a separate connection. With a
		// short lock_timeout it must FAIL while the parent holds FOR UPDATE —
		// deterministic proof of serialization, no goroutine timing. A try-style
		// timeout (not an indefinite block) keeps the test from deadlocking if the
		// lock is ever lost.
		staffTx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = staffTx.Rollback() }()
		_, err = staffTx.ExecContext(ctx, "SET LOCAL lock_timeout = ?", "200ms")
		require.NoError(t, err)
		_, err = staffTx.NewUpdate().
			TableExpr("users.students_guardians").
			Set("guardian_role = ?", authorize.GuardianRoleSocialWorker).
			Where("id = ?", sg.ID).
			Exec(ctx)
		require.Error(t, err, "a concurrent staff UPDATE must block on the parent FOR UPDATE lock")
		assert.True(t, isLockTimeoutError(err), "expected lock_timeout, got: %v", err)
		_ = staffTx.Rollback()

		// Release the parent lock; the staff UPDATE must now succeed.
		require.NoError(t, tx.Rollback())
		_, err = db.NewUpdate().
			TableExpr("users.students_guardians").
			Set("guardian_role = ?", authorize.GuardianRoleSocialWorker).
			Where("id = ?", sg.ID).
			Exec(ctx)
		require.NoError(t, err, "staff UPDATE must succeed once the FOR UPDATE holder releases")
	})
}

// TestStudentGuardianRepository_AccountHasStudentPermission covers the #1663
// per-child re-enrollment authorization probe: parent-portal permissions are
// relationship-scoped, so holding parent_portal.enrollment.submit on one child
// must NOT grant it on another child of the same guardian, and an inactive /
// missing account_tenants mapping or a wrong tenant must report no permission.
func TestStudentGuardianRepository_AccountHasStudentPermission(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentGuardian
	ctx := testpkg.TenantContext(1)

	// Primary guardian (all parent-portal permissions) → chain.StudentID, with an
	// ACTIVE account_tenants mapping. This is the authorized child.
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	// A SECOND child of the SAME guardian, linked WITHOUT enrollment.submit (empty
	// permissions) — the exact P1 scenario: authority over child A must not extend
	// to child B.
	otherChild := testpkg.CreateTestStudent(t, db, "Mara", "Schneider", "2b")
	otherLink := createTestStudentGuardian(t, db, otherChild.ID, chain.GuardianProfileID, "parent", false)
	defer func() {
		cleanupStudentGuardians(t, db, otherLink.ID)
		testpkg.CleanupActivityFixtures(t, db, otherChild.ID)
	}()

	perm := authorize.GuardianPermissionEnrollmentSubmit

	t.Run("granted on the guardian's own child", func(t *testing.T) {
		granted, err := repo.AccountHasStudentPermission(ctx, chain.AccountID, chain.StudentID, chain.TenantID, perm)
		require.NoError(t, err)
		assert.True(t, granted)
	})

	t.Run("denied on a different child lacking the permission", func(t *testing.T) {
		granted, err := repo.AccountHasStudentPermission(ctx, chain.AccountID, otherChild.ID, chain.TenantID, perm)
		require.NoError(t, err)
		assert.False(t, granted, "submit permission on one child must not extend to another")
	})

	t.Run("denied for an unrelated account", func(t *testing.T) {
		granted, err := repo.AccountHasStudentPermission(ctx, chain.AccountID+9_000_000, chain.StudentID, chain.TenantID, perm)
		require.NoError(t, err)
		assert.False(t, granted)
	})

	t.Run("denied under a different tenant", func(t *testing.T) {
		granted, err := repo.AccountHasStudentPermission(ctx, chain.AccountID, chain.StudentID, chain.TenantID+1, perm)
		require.NoError(t, err)
		assert.False(t, granted, "tenant filter must isolate the check")
	})

	t.Run("rejects non-positive arguments", func(t *testing.T) {
		_, err := repo.AccountHasStudentPermission(ctx, 0, chain.StudentID, chain.TenantID, perm)
		require.Error(t, err)
		_, err = repo.AccountHasStudentPermission(ctx, chain.AccountID, chain.StudentID, chain.TenantID, "  ")
		require.Error(t, err)
	})
}

// TestStudentGuardianRepository_GuardianEmailHasStudentPermission covers the
// accountless sibling probe (#1663). It backs the late-invite enrollment path,
// whose recipient is identified by email and typically has no portal account, so
// the check must hold for a profile WITHOUT an account while keeping every other
// boundary the account variant enforces: relationship scope (permission on child
// A never covers child B), tenant isolation, and case-insensitive email matching.
func TestStudentGuardianRepository_GuardianEmailHasStudentPermission(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StudentGuardian
	ctx := testpkg.TenantContext(1)

	// Guardian WITH an account and an active mapping, primary role on its child.
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	// A second child of the same guardian, linked without enrollment.submit.
	otherChild := testpkg.CreateTestStudent(t, db, "Mara", "Schneider", "2b")
	otherLink := createTestStudentGuardian(t, db, otherChild.ID, chain.GuardianProfileID, "parent", false)
	defer func() {
		cleanupStudentGuardians(t, db, otherLink.ID)
		testpkg.CleanupActivityFixtures(t, db, otherChild.ID)
	}()

	// An ACCOUNTLESS guardian (the typical late-invite recipient) with the full
	// primary-guardian preset on a third child.
	accountlessChild := testpkg.CreateTestStudent(t, db, "Jonas", "Weber", "3a")
	accountlessProfile := testpkg.CreateTestGuardianProfile(t, db, "accountless.guardian")
	require.NotNil(t, accountlessProfile.Email)
	// The fixture uniquifies the address, so read back what was actually stored.
	accountlessEmail := *accountlessProfile.Email
	accountlessLink := createTestStudentGuardian(t, db, accountlessChild.ID, accountlessProfile.ID, "parent", true)
	_, err := db.NewUpdate().
		TableExpr("users.students_guardians").
		Set("permissions = ?", map[string]any{authorize.GuardianPermissionEnrollmentSubmit: true}).
		Where("id = ?", accountlessLink.ID).
		Exec(ctx)
	require.NoError(t, err)
	defer func() {
		cleanupStudentGuardians(t, db, accountlessLink.ID)
		testpkg.CleanupTableRecords(t, db, "users.guardian_profiles", accountlessProfile.ID)
		testpkg.CleanupActivityFixtures(t, db, accountlessChild.ID)
	}()

	perm := authorize.GuardianPermissionEnrollmentSubmit

	t.Run("granted for an accountless guardian of the child", func(t *testing.T) {
		granted, err := repo.GuardianEmailHasStudentPermission(ctx, accountlessEmail, accountlessChild.ID, 1, perm)
		require.NoError(t, err)
		assert.True(t, granted, "a guardian without a portal account must still be able to renew their own child")
	})

	t.Run("email matching is case-insensitive and trimmed", func(t *testing.T) {
		granted, err := repo.GuardianEmailHasStudentPermission(ctx, "  "+strings.ToUpper(accountlessEmail)+" ", accountlessChild.ID, 1, perm)
		require.NoError(t, err)
		assert.True(t, granted)
	})

	t.Run("denied on another family's child", func(t *testing.T) {
		granted, err := repo.GuardianEmailHasStudentPermission(ctx, accountlessEmail, chain.StudentID, 1, perm)
		require.NoError(t, err)
		assert.False(t, granted, "the reviewed P1: an invited guardian must not renew a child they have no relationship with")
	})

	t.Run("denied on the guardian's other child lacking the permission", func(t *testing.T) {
		granted, err := repo.GuardianEmailHasStudentPermission(ctx, chain.Email, otherChild.ID, 1, perm)
		require.NoError(t, err)
		assert.False(t, granted, "submit permission on one child must not extend to another")
	})

	t.Run("granted for an account-backed guardian on its own child", func(t *testing.T) {
		granted, err := repo.GuardianEmailHasStudentPermission(ctx, chain.Email, chain.StudentID, chain.TenantID, perm)
		require.NoError(t, err)
		assert.True(t, granted)
	})

	t.Run("denied for an unknown email", func(t *testing.T) {
		granted, err := repo.GuardianEmailHasStudentPermission(ctx, "stranger@example.test", chain.StudentID, chain.TenantID, perm)
		require.NoError(t, err)
		assert.False(t, granted)
	})

	t.Run("denied under a different tenant", func(t *testing.T) {
		granted, err := repo.GuardianEmailHasStudentPermission(ctx, accountlessEmail, accountlessChild.ID, chain.TenantID+1, perm)
		require.NoError(t, err)
		assert.False(t, granted, "tenant filter must isolate the check")
	})

	t.Run("rejects invalid arguments", func(t *testing.T) {
		_, err := repo.GuardianEmailHasStudentPermission(ctx, "  ", accountlessChild.ID, 1, perm)
		require.Error(t, err)
		_, err = repo.GuardianEmailHasStudentPermission(ctx, accountlessEmail, 0, 1, perm)
		require.Error(t, err)
		_, err = repo.GuardianEmailHasStudentPermission(ctx, accountlessEmail, accountlessChild.ID, 1, "  ")
		require.Error(t, err)
	})
}

// isLockTimeoutError reports whether PostgreSQL refused a lock request because
// the transaction-local lock_timeout elapsed (SQLSTATE 55P03). Used by the
// FOR UPDATE serialization test to detect a held lock deterministically instead
// of through goroutine timing.
func isLockTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "lock timeout") ||
		strings.Contains(msg, "lock_not_available") ||
		strings.Contains(msg, "55P03")
}
