package test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The student-projection cutover fixtures (#3432, #3427). Every owner that
// reads children through the directory projection proves the same three
// things: it follows the live membership rather than an ID equality with the
// profile, it stays inside the tenant, and a missing half of the
// profile/membership/care chain hides the child. The People Directory and
// Care Plan suites share one copy so the two proofs cannot drift apart.

// AssertStudentProjectionTenants runs check once under the fixture's tenant
// (visible) and once under a freshly created foreign tenant.
func AssertStudentProjectionTenants(t *testing.T, db *bun.DB, check func(context.Context, bool)) {
	t.Helper()
	foreign := UniqueTestTenantID(t)
	EnsureTestTenant(t, db, foreign)
	for _, tenantID := range []int64{Tenant(t), foreign} {
		require.NoError(t, WithTenantTx(t, context.Background(), db, tenantID, func(ctx context.Context, _ bun.Tx) error {
			check(ctx, tenantID == Tenant(t))
			return nil
		}))
	}
}

// SeparateStudentMembership keeps a historical membership and its care row,
// but gives the live membership a sequence-generated ID distinct from the
// public student ID. This catches both an accidental ID equality join and a
// missing deleted_at predicate. It returns the live membership's ID.
func SeparateStudentMembership(t *testing.T, db *bun.DB, studentID int64) int64 {
	t.Helper()
	ctx := Ctx(t)
	var oldID, nextID int64
	require.NoError(t, db.NewRaw(`SELECT id FROM users.student_school_memberships
		WHERE tenant_id = ? AND student_profile_id = ? AND deleted_at IS NULL`, Tenant(t), studentID).Scan(ctx, &oldID))
	for nextID == 0 || nextID == studentID {
		require.NoError(t, db.NewRaw(`SELECT nextval('users.student_school_memberships_id_seq')`).Scan(ctx, &nextID))
	}
	_, err := db.ExecContext(ctx, `UPDATE users.student_school_memberships SET deleted_at = NOW() WHERE id = ?`, oldID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO users.student_school_memberships
		(id, tenant_id, student_profile_id, school_class, group_id, status, enrolled_from, enrolled_until)
		SELECT ?, tenant_id, student_profile_id, school_class, group_id, status, enrolled_from, enrolled_until
		FROM users.student_school_memberships WHERE id = ?`, nextID, oldID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO users.student_care_profiles (tenant_id, membership_id) VALUES (?, ?)`, Tenant(t), nextID)
	require.NoError(t, err)
	require.NotEqual(t, studentID, nextID)
	return nextID
}

// AssertStudentCompatibilityStorageAbsent proves the reads under test run
// without the retired student rollback storage.
func AssertStudentCompatibilityStorageAbsent(t *testing.T, db *bun.DB) {
	t.Helper()
	var absent bool
	require.NoError(t, db.NewRaw(`SELECT to_regclass('users.students') IS NULL
		AND to_regclass('users.students_legacy') IS NULL`).Scan(Ctx(t), &absent))
	require.True(t, absent, "current reads must operate without rollback storage")
}

// AssertMissingStudentProjectionStates removes the live membership, then the
// care row, then the membership itself, and asserts the child is hidden after
// each step.
func AssertMissingStudentProjectionStates(t *testing.T, db *bun.DB, membershipID int64, assertHidden func()) {
	t.Helper()
	ctx := Ctx(t)
	_, err := db.ExecContext(ctx, `UPDATE users.student_school_memberships SET deleted_at = NOW() WHERE id = ?`, membershipID)
	require.NoError(t, err)
	assertHidden()
	_, err = db.ExecContext(ctx, `UPDATE users.student_school_memberships SET deleted_at = NULL WHERE id = ?`, membershipID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `DELETE FROM users.student_care_profiles WHERE membership_id = ?`, membershipID)
	require.NoError(t, err)
	assertHidden()
	_, err = db.ExecContext(ctx, `DELETE FROM users.student_school_memberships WHERE id = ?`, membershipID)
	require.NoError(t, err)
	assertHidden()
}
