package users_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The birthday queries of the student and staff repositories (#1542, #2487).
// The display rules on top of them live with the People Directory birthday
// capability. These two tests stay here, next to the external tests whose
// imports of the retained repositories and models are already recorded for
// services/users; they move to the repository package with the last of them.

// setBirthday stamps a birth date on a fixture person. The fixtures create
// people without one, which is the realistic default: a school that has not
// maintained every date must still get a working list.
func setBirthday(t *testing.T, db *bun.DB, personID int64, date timezone.Date) {
	t.Helper()
	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 5*time.Second)
	defer cancel()

	_, err := db.NewUpdate().
		Table("users.persons").
		Set("birthday = ?", date).
		Where("id = ?", personID).
		Exec(ctx)
	require.NoError(t, err, "stamp birthday on test person")
}

// A child whose care has ended is off the birthday card from the day after
// their last care day (#2487): the card is a staff-facing list of the children
// the school currently cares for, and congratulating a child who left in
// August is exactly the kind of thing the office has to explain afterwards.
func TestStudentBirthdaysExcludeEndedCare(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	ctx := testpkg.Ctx(t)

	today := timezone.TodayDate()
	staying := testpkg.CreateTestStudent(t, db, "Geburtstag", "Bleibt", "1a")
	lastDay := testpkg.CreateTestStudent(t, db, "Geburtstag", "LetzterTag", "1a")
	departed := testpkg.CreateTestStudent(t, db, "Geburtstag", "Weg", "1a")
	for _, student := range []int64{staying.PersonID, lastDay.PersonID, departed.PersonID} {
		setBirthday(t, db, student, timezone.NewDate(2018, today.Month(), today.Day()))
	}
	setEnrolledUntil(t, db, lastDay.ID, today)
	setEnrolledUntil(t, db, departed.ID, today.AddDays(-1))

	entries, err := repos.Student.FindBirthdaysOn(ctx, []userModels.MonthDay{
		{Month: today.Month(), Day: today.Day()},
	})
	require.NoError(t, err)

	ids := make([]int64, 0, len(entries))
	for _, entry := range entries {
		ids = append(ids, entry.ID)
	}
	assert.Contains(t, ids, staying.ID)
	assert.Contains(t, ids, lastDay.ID, "the last care day still counts as care")
	assert.NotContains(t, ids, departed.ID)
}

// setEnrolledUntil stamps the enrollment interval's inclusive upper bound.
func setEnrolledUntil(t *testing.T, db *bun.DB, studentID int64, until timezone.Date) {
	t.Helper()
	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 5*time.Second)
	defer cancel()

	_, err := db.NewUpdate().
		Table("users.student_school_memberships").
		Set("enrolled_until = ?", until).
		Where("student_profile_id = ?", studentID).Where("deleted_at IS NULL").
		Exec(ctx)
	require.NoError(t, err, "stamp enrolled_until on test student")
}

// The repositories refuse an empty day set instead of building a WHERE clause
// with no values (which would degenerate into "every person of the school").
func TestBirthdayRepositoriesRejectAnEmptyDaySet(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	ctx := testpkg.Ctx(t)

	students, err := repos.Student.FindBirthdaysOn(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, students)

	staff, err := repos.Staff.FindBirthdaysOn(ctx, []userModels.MonthDay{})
	require.NoError(t, err)
	assert.Empty(t, staff)
}
