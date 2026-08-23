package users_test

import (
	"fmt"
	"testing"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A child whose care has ended must disappear from the same staff-facing reads
// as a graduate (#2487). The shared newStudentWithGroupQuery is the one that
// matters most here: it backs the kiosk roster (GET /api/iot/students, which
// PyrePortal lists on the tablet) and the calendar recipient picker. Filtering
// only the scan while the list keeps showing the child is the worst of both —
// staff sees a name that cannot be checked in.
//
// The boundary is the enrollment interval, not the lifecycle status: the
// status only follows with the next scheduler tick, up to an hour later.
func TestStudentRepository_EndedCareExcludedFromRosterReads(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repos := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	group := testpkg.CreateTestEducationGroup(t, db, fmt.Sprintf("CareEndGroup-%s", suffix))
	teacher := testpkg.CreateTestTeacher(t, db, "CareEnd", "Teacher")
	gt := testpkg.CreateTestGroupTeacher(t, db, group.ID, teacher.ID)
	defer func() {
		_, _ = db.NewDelete().
			TableExpr("education.group_teacher").
			Where("id = ?", gt.ID).
			Exec(ctx)
	}()

	class := fmt.Sprintf("1ce-%s", suffix)
	staying := testpkg.CreateTestStudent(t, db, "CareStaying", "Kid", class)
	lastDayToday := testpkg.CreateTestStudent(t, db, "CareLastDay", "Kid", class)
	departed := testpkg.CreateTestStudent(t, db, "CareDeparted", "Kid", class)

	assignGroup(t, db, staying.ID, group.ID)
	assignGroup(t, db, lastDayToday.ID, group.ID)
	assignGroup(t, db, departed.ID, group.ID)

	today := timezone.TodayDate()
	yesterday := today.AddDays(-1)
	// The interval's upper bound is INCLUSIVE: a child whose last care day is
	// today is still there today and leaves tomorrow.
	setLifecycle(t, db, lastDayToday.ID, users.StudentStatusActive, nil, &today)
	setLifecycle(t, db, departed.ID, users.StudentStatusActive, nil, &yesterday)

	t.Run("FindAllWithGroups keeps the last care day and drops the day after", func(t *testing.T) {
		infos, err := repos.Student.FindAllWithGroups(ctx)
		require.NoError(t, err)
		ids := make([]int64, 0, len(infos))
		for _, info := range infos {
			ids = append(ids, info.ID)
		}
		assert.Contains(t, ids, staying.ID)
		assert.Contains(t, ids, lastDayToday.ID,
			"a child still counts as in care ON their last care day")
		assert.NotContains(t, ids, departed.ID,
			"the kiosk roster must not list a child whose care ended yesterday")
	})

	t.Run("FindByTeacherIDWithGroups excludes the departed child", func(t *testing.T) {
		infos, err := repos.Student.FindByTeacherIDWithGroups(ctx, teacher.ID)
		require.NoError(t, err)
		ids := make([]int64, 0, len(infos))
		for _, info := range infos {
			ids = append(ids, info.ID)
		}
		assert.Contains(t, ids, staying.ID)
		assert.NotContains(t, ids, departed.ID)
	})
}
