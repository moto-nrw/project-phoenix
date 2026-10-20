package test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// insertStudentFixture creates all three owned rows in one statement.
func insertStudentFixture(ctx context.Context, db *bun.DB, student *Student) error {
	return db.NewRaw(`WITH profile AS (
		INSERT INTO users.student_profiles (tenant_id, person_id) VALUES (?, ?) RETURNING *
	), membership AS (
		INSERT INTO users.student_school_memberships (tenant_id, student_profile_id, school_class)
		SELECT tenant_id, id, ? FROM profile RETURNING *
	), care AS (
		INSERT INTO users.student_care_profiles (tenant_id, membership_id)
		SELECT tenant_id, id FROM membership RETURNING *
	)
	SELECT p.id, p.tenant_id, p.person_id, p.created_at, p.updated_at,
		m.school_class, m.status, c.sick, c.excused
	FROM profile p JOIN membership m ON m.student_profile_id=p.id AND m.tenant_id=p.tenant_id
	JOIN care c ON c.membership_id=m.id AND c.tenant_id=m.tenant_id`,
		student.TenantID, student.PersonID, student.SchoolClass).Scan(ctx, student)
}

func updateStudentMembershipFixture(ctx context.Context, db *bun.DB, studentID int64) (*bun.UpdateQuery, error) {
	return db.NewUpdate().TableExpr("users.student_school_memberships").
		Where("student_profile_id = ?", studentID).Where("deleted_at IS NULL"), nil
}

// ClearTestStudentCompanionNote drops the free-text "mit wem" note straight in
// SQL, leaving a child whose companion is answered only by the structured
// link — the state a note-carrying child reaches once it is linked and the
// note is removed.
func ClearTestStudentCompanionNote(tb testing.TB, db *bun.DB, studentID int64) {
	tb.Helper()
	_, err := db.NewUpdate().
		TableExpr("users.student_care_profiles").
		Set("departure_companion_note = NULL").
		Where("membership_id = (SELECT id FROM users.student_school_memberships WHERE student_profile_id = ? AND deleted_at IS NULL)", studentID).
		Exec(context.Background())
	require.NoError(tb, err)
}
