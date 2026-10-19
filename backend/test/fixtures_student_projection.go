package test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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

// StudentDirectory stands in for the student directory projection in suites
// that compose Care Plan or the retained People Directory repositories
// outside the root. Test support may not reach the
// projection (ADR 0058), so it embeds the same three-owner row source as the
// production projection does, like StudentGuardianLinks does for the guardian
// links. The care-exit reads are not mapped: a suite that reaches them fails
// loudly instead of seeing an empty directory.
type StudentDirectory struct{}

// studentDirectorySource joins the three owners of a student into the
// directory row, aliased "student", as the production projection does.
const studentDirectorySource = `(SELECT p.id, p.tenant_id, p.person_id, p.created_at,
	GREATEST(p.updated_at, m.updated_at, c.updated_at) AS updated_at,
	p.address_street, p.address_city, p.address_postal_code, p.extra_info,
	p.photo_path, p.photo_consent_given_at, p.photo_consent_given_by,
	p.agb_accepted_at, p.data_processing_accepted_at, p.email_contact_accepted_at,
	m.id AS membership_id, m.school_class, m.group_id, m.status, m.enrolled_from, m.enrolled_until,
	c.health_info, c.pickup_status, c.departure_days,
	c.allowed_departure_modes, c.departure_companion_note, c.pickup_days, c.bus_days,
	c.sick, c.sick_since, c.excused, c.excused_since
	FROM users.student_profiles AS p
	JOIN users.student_school_memberships AS m
		ON m.tenant_id = p.tenant_id AND m.student_profile_id = p.id AND m.deleted_at IS NULL
	JOIN users.student_care_profiles AS c
		ON c.tenant_id = m.tenant_id AND c.membership_id = m.id) AS "student"`

// StudentRows is the directory row source a Care Plan statement embeds.
func (StudentDirectory) StudentRows(db bun.IDB, tenantID int64) *bun.SelectQuery {
	query := db.NewSelect().TableExpr(studentDirectorySource)
	if tenantID <= 0 {
		return query.Err(errors.New("fixture projection: tenant is required"))
	}
	query = query.Where(`"student".tenant_id = ?`, tenantID)
	return query
}

// StudentModelRows is the directory row source with BUN's model column
// selection, for the retained People Directory repositories.
func (StudentDirectory) StudentModelRows(db bun.IDB, tenantID int64, model any) *bun.SelectQuery {
	query := db.NewSelect().Model(model).ModelTableExpr(studentDirectorySource)
	if tenantID <= 0 {
		return query.Err(errors.New("fixture projection: tenant is required"))
	}
	query = query.Where(`"student".tenant_id = ?`, tenantID)
	return query
}

// StudentDisplayRows is the display row source of every student.
func (StudentDirectory) StudentDisplayRows(db bun.IDB, tenantID int64) *bun.SelectQuery {
	query := db.NewSelect().TableExpr(studentDirectorySource).
		ColumnExpr(`"student".id, "student".tenant_id, "student".school_class, "person".first_name, "person".last_name`).
		Join(`LEFT JOIN users.persons AS "person" ON "person".tenant_id = "student".tenant_id AND "person".id = "student".person_id AND "person".deleted_at IS NULL`)
	if tenantID <= 0 {
		return query.Err(errors.New("fixture projection: tenant is required"))
	}
	query = query.Where(`"student".tenant_id = ?`, tenantID)
	return query
}

func (StudentDirectory) ListEndedCare(context.Context, bun.IDB, int64, string, string, []string, int, int,
	func(int64, string, string, string, calendar.Date)) (int, error) {
	return 0, errUnmappedStudentDirectoryRead
}

func (StudentDirectory) LockCareExitPeople(context.Context, bun.IDB, int64, []int64) error {
	return errUnmappedStudentDirectoryRead
}

func (StudentDirectory) ListCareStudents(context.Context, bun.IDB, int64, string, []int64, string, string,
	func(int64, string, string, string, *calendar.Date)) error {
	return errUnmappedStudentDirectoryRead
}

var errUnmappedStudentDirectoryRead = errors.New("the student directory's care-exit reads are not mapped in test support")

func (StudentDirectory) AllSchoolsStudentRows(db bun.IDB) *bun.SelectQuery {
	return db.NewSelect().TableExpr(studentDirectorySource)
}
func (StudentDirectory) AllSchoolsStudentModelRows(db bun.IDB, model any) *bun.SelectQuery {
	return db.NewSelect().Model(model).ModelTableExpr(studentDirectorySource)
}
