package contracttest_test

import (
	"context"
	"testing"
	"time"

	peopleBirthday "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The birthday queries of the student and staff repositories (#1542, #2487).
// The display rules on top of them, including the ended-care rule, live with
// the People Directory birthday capability (birthdays_behavior_test.go).

// The repositories refuse an empty day set instead of building a WHERE clause
// with no values. A student and a staff member with a birthday today exist, so
// an empty result proves the refusal and not an empty school.
func TestBirthdayRepositoriesRejectAnEmptyDaySet(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repos := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	ctx := testpkg.Ctx(t)

	today := calendar.TodayDate()
	student := testpkg.CreateTestStudent(t, db, "Geburtstag", "Kind", "1a")
	member := testpkg.CreateTestStaff(t, db, "Geburtstag", "Personal")
	for _, personID := range []int64{student.PersonID, member.PersonID} {
		setBirthday(t, db, personID, calendar.NewDate(1990, today.Month(), today.Day()))
	}
	todaySet := []peopleBirthday.BirthdayMonthDay{{Month: today.Month(), Day: today.Day()}}

	students, err := testpkg.TenantQuery1(t, ctx, db, repos.Student.FindBirthdaysOn, todaySet)
	require.NoError(t, err)
	require.Len(t, students, 1, "the seeded student has a birthday today")
	staff, err := testpkg.TenantQuery1(t, ctx, db, repos.Staff.FindBirthdaysOn, todaySet)
	require.NoError(t, err)
	require.Len(t, staff, 1, "the seeded staff member has a birthday today")

	students, err = testpkg.TenantQuery1(t, ctx, db, repos.Student.FindBirthdaysOn, nil)
	require.NoError(t, err)
	assert.Empty(t, students)

	staff, err = testpkg.TenantQuery1(t, ctx, db, repos.Staff.FindBirthdaysOn, []peopleBirthday.BirthdayMonthDay{})
	require.NoError(t, err)
	assert.Empty(t, staff)
}

// setBirthday stamps a birth date on a fixture person; fixtures create people
// without one.
func setBirthday(t *testing.T, db *bun.DB, personID int64, date calendar.Date) {
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
