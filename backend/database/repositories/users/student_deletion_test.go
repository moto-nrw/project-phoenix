package users_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStudentDeletionRepository_DeletesOnlyTargetAssignments(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

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
