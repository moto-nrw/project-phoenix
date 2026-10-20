package migrations

import (
	"context"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestMealAbsenceHistoryRecordsEveryAbsenceStatus(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	student := testpkg.CreateTestStudent(t, db, "Ali", "Abwesend", "3a")
	reportedAt := time.Date(2026, 9, 7, 7, 30, 0, 0, time.UTC)
	statusDay := func(date, status string) int64 {
		t.Helper()
		var id int64
		require.NoError(t, db.NewRaw(`
			INSERT INTO active.student_status_days (tenant_id, student_id, date, status, reported_at, source)
			VALUES (?, ?, ?, ?, ?, 'manual') RETURNING id
		`, student.TenantID, student.ID, date, status, reportedAt).Scan(ctx, &id))
		return id
	}
	history := func(date string) []string {
		t.Helper()
		var rows []string
		require.NoError(t, db.NewRaw(`
			SELECT status || CASE WHEN cleared_at IS NULL THEN ':open' ELSE ':cleared' END
			FROM schedule.meal_sickness_status_history
			WHERE tenant_id = ? AND student_id = ? AND date = ?
			ORDER BY id
		`, student.TenantID, student.ID, date).Scan(ctx, &rows))
		return rows
	}

	// Before: only sick days reach the history.
	require.NoError(t, mealAbsenceHistoryDown(ctx, db))
	statusDay("2026-09-08", "excused")
	statusDay("2026-09-09", "sick")
	assert.Empty(t, historyWithoutStatus(t, db, student.TenantID, student.ID, "2026-09-08"))
	assert.Equal(t, []string{"sick:open"}, historyWithoutStatus(t, db, student.TenantID, student.ID, "2026-09-09"), "the sick history keeps working without the column")

	// Up carries the existing excused day over and records new ones.
	require.NoError(t, mealAbsenceHistoryUp(ctx, db))
	assert.Equal(t, []string{"excused:open"}, history("2026-09-08"))
	require.NoError(t, mealAbsenceHistoryUp(ctx, db), "the migration can run twice")
	assert.Equal(t, []string{"excused:open"}, history("2026-09-08"), "a rerun adds nothing")

	tripID := statusDay("2026-09-10", "class_trip")
	_, err := db.NewRaw(`UPDATE active.student_status_days SET cleared_at = ? WHERE id = ?`, reportedAt.Add(time.Hour), tripID).Exec(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{"class_trip:open", "class_trip:cleared"}, history("2026-09-10"))

	// A second status on the same day keeps its own rows.
	sickID := statusDay("2026-09-08", "sick")
	deleteStatusDay(t, db, sickID)
	assert.Equal(t, []string{"excused:open", "sick:open", "sick:cleared"}, history("2026-09-08"))

	// Down drops what only Up can read and restores the sick-only trigger.
	require.NoError(t, mealAbsenceHistoryDown(ctx, db))
	assert.Equal(t, []string{"sick:open", "sick:cleared"}, historyWithoutStatus(t, db, student.TenantID, student.ID, "2026-09-08"))
	require.NoError(t, mealAbsenceHistoryUp(ctx, db))
}

func deleteStatusDay(t *testing.T, db *bun.DB, id int64) {
	t.Helper()
	_, err := db.NewRaw(`DELETE FROM active.student_status_days WHERE id = ?`, id).Exec(context.Background())
	require.NoError(t, err)
}

// historyWithoutStatus reads the history as 1.15.446 left it, before the
// status column existed.
func historyWithoutStatus(t *testing.T, db *bun.DB, tenantID, studentID int64, date string) []string {
	t.Helper()
	var rows []string
	require.NoError(t, db.NewRaw(`
		SELECT 'sick' || CASE WHEN cleared_at IS NULL THEN ':open' ELSE ':cleared' END
		FROM schedule.meal_sickness_status_history
		WHERE tenant_id = ? AND student_id = ? AND date = ?
		ORDER BY id
	`, tenantID, studentID, date).Scan(context.Background(), &rows))
	return rows
}
