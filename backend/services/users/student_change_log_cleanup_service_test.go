package users_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	repoFactory "github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/audit"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Each test inserts audit.student_field_edits rows with controlled created_at
// via raw INSERT (the repo/Validate path would stamp created_at = NOW()).

func daysAgo(n int) time.Time {
	return time.Now().AddDate(0, 0, -n)
}

func insertFieldEdit(tb testing.TB, db *bun.DB, studentID int64, createdAt time.Time) {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := db.NewRaw(
		`INSERT INTO audit.student_field_edits
			(tenant_id, student_id, edited_by, edited_by_name, field_name, old_value, new_value, created_at)
		 VALUES (?, ?, 1, 'Test Editor', 'supervisor_notes', 'alt', 'neu', ?)`,
		testpkg.Tenant(tb), studentID, createdAt,
	).Exec(ctx)
	require.NoError(tb, err, "failed to insert student_field_edit")
}

func countFieldEdits(tb testing.TB, db *bun.DB, studentID int64) int {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	count, err := db.NewSelect().
		ModelTableExpr(`audit.student_field_edits`).
		Where(`student_id = ?`, studentID).
		Count(ctx)
	require.NoError(tb, err)
	return count
}

func countChangeLogDeletions(tb testing.TB, db *bun.DB, studentID int64) int {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	count, err := db.NewSelect().
		ModelTableExpr(`audit.data_deletions`).
		Where(`student_id = ?`, studentID).
		Where(`deletion_type = ?`, audit.DeletionTypeStudentChangeLogRetention).
		Count(ctx)
	require.NoError(tb, err)
	return count
}

func cleanupChangeLogRows(tb testing.TB, db *bun.DB, studentID int64) {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = db.NewRaw(`DELETE FROM audit.data_deletions WHERE student_id = ?`, studentID).Exec(ctx)
	_, _ = db.NewRaw(`DELETE FROM audit.student_field_edits WHERE student_id = ?`, studentID).Exec(ctx)
}

func changeLogSettings(retentionDays int) *configtest.Mock {
	return &configtest.Mock{
		ResolveIntFn: func(_ context.Context, key string) (int, error) {
			if key != configModel.KeyGDPRStudentChangeLogRetentionDays {
				return 0, fmt.Errorf("unexpected setting key %q", key)
			}
			return retentionDays, nil
		},
	}
}

// Deliberately NOT parallel: the code under test sweeps rows across tenants.
// These service-level tests call it with a plain tenant context instead of a
// tenant transaction, so RLS never narrows the query and the sweep also picks
// up the rows of every test running beside it.
func TestStudentChangeLogCleanup_DeletesOldEdits(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)

	student := testpkg.CreateTestStudent(t, db, "Cleanup", "Old", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)
	defer cleanupChangeLogRows(t, db, student.ID)

	// Two expired edits (200d, 400d) and one fresh (10d, inside the 90d window).
	insertFieldEdit(t, db, student.ID, daysAgo(200))
	insertFieldEdit(t, db, student.ID, daysAgo(400))
	insertFieldEdit(t, db, student.ID, daysAgo(10))

	repos := repoFactory.NewFactory(db)
	svc := usersSvc.NewStudentChangeLogCleanupService(
		repos.StudentFieldEdit,
		repos.DataDeletion,
		changeLogSettings(90),
		nil,
	)

	result, err := svc.CleanupExpiredChangeLog(ctx)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, 2, result.EditsDeleted, "two expired edits deleted")
	assert.Equal(t, 1, result.StudentsAffected)
	assert.Equal(t, 90, result.RetentionDays)

	assert.Equal(t, 1, countFieldEdits(t, db, student.ID), "fresh edit must remain")
	assert.Equal(t, 1, countChangeLogDeletions(t, db, student.ID), "one audit row for the affected student")
}

// Deliberately NOT parallel: the code under test sweeps rows across tenants.
// These service-level tests call it with a plain tenant context instead of a
// tenant transaction, so RLS never narrows the query and the sweep also picks
// up the rows of every test running beside it.
func TestStudentChangeLogCleanup_NoOpWhenNothingExpired(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)

	student := testpkg.CreateTestStudent(t, db, "Cleanup", "Fresh", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)
	defer cleanupChangeLogRows(t, db, student.ID)

	insertFieldEdit(t, db, student.ID, daysAgo(5))
	insertFieldEdit(t, db, student.ID, daysAgo(30))

	repos := repoFactory.NewFactory(db)
	svc := usersSvc.NewStudentChangeLogCleanupService(
		repos.StudentFieldEdit,
		repos.DataDeletion,
		changeLogSettings(90),
		nil,
	)

	result, err := svc.CleanupExpiredChangeLog(ctx)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, 0, result.EditsDeleted)
	assert.Equal(t, 0, result.StudentsAffected)

	assert.Equal(t, 2, countFieldEdits(t, db, student.ID), "no edits deleted")
	assert.Equal(t, 0, countChangeLogDeletions(t, db, student.ID), "no audit row when nothing deleted")
}
