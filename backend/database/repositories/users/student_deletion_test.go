package users_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStudentDeletionRepository_RequiresTenantContext(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	repo := repositories.NewFactory(db).StudentDeletion

	_, err := repo.Preview(context.Background(), 1)
	assert.ErrorContains(t, err, "tenant context is required")

	err = repo.DeleteLegacyGuardianLinks(context.Background(), 1)
	assert.ErrorContains(t, err, "tenant context is required")

	_, err = repo.AnonymizePersonIfUnchanged(context.Background(), 1, time.Now())
	assert.ErrorContains(t, err, "tenant context is required")

	_, err = repo.DeleteTimetableAssignments(context.Background(), 1)
	assert.ErrorContains(t, err, "tenant context is required")

	err = repo.LockMessageThreads(context.Background(), 1)
	assert.ErrorContains(t, err, "tenant context is required")
}

func TestStudentDeletionRepository_LockMessageThreadsBlocksNewRead(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	ctx := testpkg.TenantContext(1)
	target := testpkg.CreateTestStudent(t, db, "DeletePreview", "Locked", "1a")
	guardian := testpkg.CreateTestAccount(t, db, "delete-preview-message-guardian@example.com")
	var threadID int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO users.parent_message_threads (tenant_id, student_id, guardian_account_id)
		VALUES (?, ?, ?)
		RETURNING id
	`, target.TenantID, target.ID, guardian.ID).Scan(ctx, &threadID))
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "users.students", target.ID)
		testpkg.CleanupTableRecords(t, db, "users.persons", target.PersonID)
		testpkg.CleanupAuthFixtures(t, db, guardian.ID)
	})

	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	txCtx := modelBase.ContextWithTx(ctx, &tx)
	repo := repositories.NewFactory(db).StudentDeletion
	require.NoError(t, repo.LockMessageThreads(txCtx, target.ID))

	concurrentTx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = concurrentTx.Rollback() })
	_, err = concurrentTx.ExecContext(ctx, `SET LOCAL lock_timeout = '100ms'`)
	require.NoError(t, err)
	_, err = concurrentTx.ExecContext(ctx, `
		INSERT INTO users.parent_message_reads (tenant_id, thread_id, account_id)
		VALUES (?, ?, ?)
	`, target.TenantID, threadID, guardian.ID)
	require.Error(t, err, "a read cursor must wait for the locked message thread")
}

func TestStudentDeletionRepository_DeletesOnlyTargetAssignments(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	ctx := testpkg.TenantContext(1)
	target := testpkg.CreateTestStudent(t, db, "DeletePreview", "Target", "1a")
	spared := testpkg.CreateTestStudent(t, db, "DeletePreview", "Spared", "1a")
	room := testpkg.CreateTestRoom(t, db, "delete-preview-room")
	instance := testpkg.CreateTestActivityInstance(t, db, timezone.TodayDate(), room.ID, testpkg.ActivityInstanceOpts{
		Title:         "Shared deletion preview instance",
		IsSpontaneous: true,
	})
	targetAssignment := testpkg.CreateTestInstanceStudent(t, db, instance.ID, target.ID, "")
	sparedAssignment := testpkg.CreateTestInstanceStudent(t, db, instance.ID, spared.ID, "")

	t.Cleanup(func() {
		testpkg.CleanupScheduleFixturesB11(t, db, nil, nil, nil, nil,
			[]int64{targetAssignment.ID, sparedAssignment.ID}, []int64{instance.ID})
		testpkg.CleanupTableRecords(t, db, "users.students", target.ID, spared.ID)
		testpkg.CleanupTableRecords(t, db, "users.persons", target.PersonID, spared.PersonID)
		testpkg.CleanupTableRecords(t, db, "facilities.rooms", room.ID)
	})

	repo := repositories.NewFactory(db).StudentDeletion
	preview, err := repo.Preview(ctx, target.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, preview.TimetableAssignments)

	deleted, err := repo.DeleteTimetableAssignments(ctx, target.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(preview.TimetableAssignments), deleted)

	var targetCount, sparedCount, instanceCount int
	require.NoError(t, db.NewSelect().TableExpr(`schedule.instance_students`).ColumnExpr("COUNT(*)").
		Where("id = ?", targetAssignment.ID).Scan(ctx, &targetCount))
	require.NoError(t, db.NewSelect().TableExpr(`schedule.instance_students`).ColumnExpr("COUNT(*)").
		Where("id = ?", sparedAssignment.ID).Scan(ctx, &sparedCount))
	require.NoError(t, db.NewSelect().TableExpr(`schedule.activity_instances`).ColumnExpr("COUNT(*)").
		Where("id = ?", instance.ID).Scan(ctx, &instanceCount))

	assert.Zero(t, targetCount)
	assert.Equal(t, 1, sparedCount)
	assert.Equal(t, 1, instanceCount)

	person, err := repositories.NewFactory(db).Person.FindByID(ctx, target.PersonID)
	require.NoError(t, err)
	_, err = db.NewUpdate().
		TableExpr(`users.persons AS "person"`).
		Set(`updated_at = updated_at + INTERVAL '1 second'`).
		Where(`"person".id = ?`, target.PersonID).
		Exec(ctx)
	require.NoError(t, err)
	anonymized, err := repo.AnonymizePersonIfUnchanged(ctx, target.PersonID, person.UpdatedAt)
	require.NoError(t, err)
	assert.False(t, anonymized, "a concurrent person edit must invalidate the confirmed preview")

	var firstName string
	require.NoError(t, db.NewSelect().TableExpr(`users.persons`).Column("first_name").
		Where("id = ?", target.PersonID).Scan(ctx, &firstName))
	assert.Equal(t, "DeletePreview", firstName)
}
