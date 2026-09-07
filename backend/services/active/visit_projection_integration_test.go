package active_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type visitProjectionFixture struct {
	Student1, Student2 *users.Student
	GroupID, Room      int64
}

func newVisitProjectionFixture(t *testing.T, db *bun.DB) visitProjectionFixture {
	t.Helper()
	first := testpkg.CreateTestStudent(t, db, "Visit", "Student1", "1a")
	second := testpkg.CreateTestStudent(t, db, "Visit", "Student2", "1b")
	activity := testpkg.CreateTestActivityGroup(t, db, "VisitActivity")
	room := testpkg.CreateTestRoom(t, db, "VisitRoom")
	group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	return visitProjectionFixture{Student1: first, Student2: second, GroupID: group.ID, Room: room.ID}
}

func TestPresenceProjection_ActiveGroupStudentDisplay(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)
	data := newVisitProjectionFixture(t, db)

	educationGroup := testpkg.CreateTestEducationGroup(t, db, "Visit Display Group")
	_, err := db.NewUpdate().
		Table("users.students").
		Set("group_id = ?", educationGroup.ID).
		Where("id = ?", data.Student1.ID).
		Exec(ctx)
	require.NoError(t, err)
	defer func() {
		_, _ = db.NewUpdate().
			Table("users.students").
			Set("group_id = NULL").
			Where("id = ?", data.Student1.ID).
			Exec(ctx)
		_, _ = db.NewDelete().
			Table("education.groups").
			Where("id = ?", educationGroup.ID).
			Exec(ctx)
	}()

	activeVisit := testpkg.CreateTestVisit(t, db, data.Student1.ID, data.GroupID, time.Now().Add(-10*time.Minute), nil)
	exitTime := time.Now().Add(-5 * time.Minute)
	testpkg.CreateTestVisit(t, db, data.Student2.ID, data.GroupID, time.Now().Add(-20*time.Minute), &exitTime)

	results, err := repo.GetActiveGroupVisitsWithDisplay(ctx, data.GroupID)

	require.NoError(t, err)
	require.Len(t, results, 1)
	row := results[0]
	assert.Equal(t, activeVisit.ID, row.VisitID)
	assert.Equal(t, data.Student1.ID, row.StudentID)
	assert.Equal(t, data.GroupID, row.ActiveGroupID)
	assert.Equal(t, "Visit", row.FirstName)
	assert.Equal(t, "Student1", row.LastName)
	assert.Equal(t, "1a", row.SchoolClass)
	require.NotNil(t, row.GroupID)
	assert.Equal(t, educationGroup.ID, *row.GroupID)
	assert.Equal(t, educationGroup.Name, row.OGSGroupName)
	assert.Nil(t, row.ExitTime)
}

func TestPresenceProjection_CurrentVisitWithRoom(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := setupActiveService(t, db)
	presence := testSchoolPresence(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns visit with active group and room", func(t *testing.T) {
		data := newVisitProjectionFixture(t, db)
		now := time.Now()
		visit := studentpresence.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.GroupID,
			EntryTime:     now,
		}
		visit, err := presence.RecordVisit(ctx, visit)
		require.NoError(t, err)

		result, err := repo.GetStudentCurrentVisitWithRoom(ctx, data.Student1.ID)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, visit.ID, result.ID)
		assert.Nil(t, result.ExitTime)

		// ActiveGroup should be loaded
		require.NotNil(t, result.ActiveGroup, "ActiveGroup should be loaded")
		assert.Equal(t, data.GroupID, result.ActiveGroup.ID)

		// Room should be loaded on active group
		require.NotNil(t, result.ActiveGroup.Room, "Room should be loaded on ActiveGroup")
		assert.Equal(t, data.Room, result.ActiveGroup.Room.ID)
	})

	t.Run("returns visit when active group timeout_minutes is null", func(t *testing.T) {
		data := newVisitProjectionFixture(t, db)
		now := time.Now()
		visit := studentpresence.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.GroupID,
			EntryTime:     now,
		}
		_, err := presence.RecordVisit(ctx, visit)
		require.NoError(t, err)

		_, err = db.NewUpdate().
			Table("active.groups").
			Set("timeout_minutes = NULL").
			Where("id = ?", data.GroupID).
			Exec(ctx)
		require.NoError(t, err)

		result, err := repo.GetStudentCurrentVisitWithRoom(ctx, data.Student1.ID)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.NotNil(t, result.ActiveGroup)
		assert.Equal(t, 0, result.ActiveGroup.TimeoutMinutes)
		require.NotNil(t, result.ActiveGroup.Room)
		assert.Equal(t, data.Room, result.ActiveGroup.Room.ID)
	})

	t.Run("returns error for student with no active visit", func(t *testing.T) {
		data := newVisitProjectionFixture(t, db)
		_, err := repo.GetStudentCurrentVisitWithRoom(ctx, data.Student2.ID)
		require.Error(t, err)
	})

	t.Run("ignores exited visits", func(t *testing.T) {
		data := newVisitProjectionFixture(t, db)
		now := time.Now()
		exitTime := now.Add(-10 * time.Minute)
		visit := studentpresence.Visit{
			StudentID:     data.Student2.ID,
			ActiveGroupID: data.GroupID,
			EntryTime:     now.Add(-30 * time.Minute),
			ExitTime:      &exitTime,
		}
		_, err := presence.RecordVisit(ctx, visit)
		require.NoError(t, err)

		// Student2 should have no current visit (only exited one)
		_, err = repo.GetStudentCurrentVisitWithRoom(ctx, data.Student2.ID)
		require.Error(t, err)
	})
}
