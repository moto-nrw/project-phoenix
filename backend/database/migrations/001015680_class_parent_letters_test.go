package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClassParentLetterSenderConstraint pins what 1.15.680 lets a parent
// announcement hold for a class letter (#4800): sender name and class
// together, neither of them blank, or neither at all.
func TestClassParentLetterSenderConstraint(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	_, account := testpkg.CreateTestStaffWithAccount(t, db, "Lena", "Lehrerin")
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	insert := func(name, class any) error {
		_, err := db.ExecContext(ctx, `
			INSERT INTO users.parent_announcements
				(tenant_id, title, body, priority, requires_acknowledgement, send_email, active, created_by,
				 sender_name, sender_school_class)
			VALUES (?, 'Ausflug', 'Wir fahren in den Zoo.', 'info', FALSE, FALSE, TRUE, ?, ?, ?)`,
			tenantID, account.ID, name, class)
		return err
	}

	require.NoError(t, insert(nil, nil), "a letter of the OGS has no sender")
	require.NoError(t, insert("Lena Lehrerin", "3b"), "a class letter names its sender and class")
	assert.Error(t, insert("Lena Lehrerin", nil), "a sender needs the class")
	assert.Error(t, insert(nil, "3b"), "a class needs the sender")
	assert.Error(t, insert("  ", "3b"), "the sender is not blank")
	assert.Error(t, insert("Lena Lehrerin", " "), "the class is not blank")
}

// TestClassParentLetterGrant pins who 1.15.680 lets write a class letter:
// the system roles lehrkraft and schulverwaltung, not the Betreuer role and
// not a school's own role of the same name, also on a rerun.
func TestClassParentLetterGrant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	holds := func(t *testing.T, roleID int64) bool {
		t.Helper()
		var granted bool
		require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT 1 FROM auth.role_permissions rp
			JOIN auth.permissions p ON p.id = rp.permission_id
			WHERE rp.role_id = ? AND p.name = 'class_day:parent_letter_write')`, roleID).Scan(ctx, &granted))
		return granted
	}
	systemRole := func(t *testing.T, name string) int64 {
		t.Helper()
		var id int64
		require.NoError(t, db.NewRaw(`SELECT id FROM auth.roles WHERE name = ? AND tenant_id IS NULL`, name).Scan(ctx, &id))
		return id
	}

	assert.True(t, holds(t, systemRole(t, "lehrkraft")))
	assert.True(t, holds(t, systemRole(t, "schulverwaltung")))
	assert.False(t, holds(t, systemRole(t, "user")), "the Betreuer role does not write to parents")

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	own := testpkg.CreateTestRoleForTenant(t, db, "lehrkraft", tenantID)
	require.NoError(t, classParentLettersUp(ctx, db), "the migration is idempotent")
	assert.False(t, holds(t, own.ID), "a school's own role of the same name gets nothing")
	assert.True(t, holds(t, systemRole(t, "lehrkraft")))
}
