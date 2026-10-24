package migrations

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// Migration 1.15.443 holds the Abholart of single days (#3945). Its
// constraints are the model: one value per child and day, only the four
// Abholarten, and a care exit may archive a day value.
func TestStudentDepartureExceptionsConstraints(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	student := testpkg.CreateTestStudent(t, db, "Mila", "Heimweg", "3a")

	insert := func(date, mode string) error {
		_, err := db.ExecContext(ctx, `
			INSERT INTO schedule.student_departure_exceptions (tenant_id, student_id, exception_date, departure_mode)
			VALUES (?, ?, ?, ?)`, tenantID, student.ID, date, mode)
		return err
	}

	require.NoError(t, insert("2026-09-09", "bus"))
	require.ErrorContains(t, insert("2026-09-09", "pickup"), "uq_student_departure_exceptions_day")
	require.ErrorContains(t, insert("2026-09-10", "taxi"), "chk_student_departure_exceptions_mode")

	_, err := db.ExecContext(ctx, `
		INSERT INTO users.student_care_exit_source_removals (tenant_id, student_id, kind, source_row_id, was_deleted, snapshot)
		VALUES (?, ?, 'departure_exception', ?, TRUE, '{}'::jsonb)`, tenantID, student.ID, student.ID)
	require.NoError(t, err)
}
