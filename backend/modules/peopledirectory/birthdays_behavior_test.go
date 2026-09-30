package peopledirectory_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Birthday service against real repositories (#1542). The HTTP tests in
// modules/peopledirectory/inbound/birthdays pin the route contract; this file pins the rules the service
// itself owns: which week a view speaks for, who may appear, and what the
// staff list contains.

// The registry keys of the two display settings. The mock below fails on any
// other key, so a renamed key fails these tests instead of reading as false.
const (
	keyDisplayEnabled = "operations.birthday_display_enabled"
	keyIncludeStaff   = "operations.birthday_display_include_staff"
)

// birthdaySettings answers the two display settings and fails loudly on any
// other key, so a future setting cannot silently read as false here.
func birthdaySettings(enabled, includeStaff bool) *configtest.Mock {
	return &configtest.Mock{
		ResolveBoolFn: func(_ context.Context, key string) (bool, error) {
			switch key {
			case keyDisplayEnabled:
				return enabled, nil
			case keyIncludeStaff:
				return includeStaff, nil
			default:
				return false, errors.New("unexpected setting: " + key)
			}
		},
	}
}

func newBirthdayService(db *bun.DB, settings *configtest.Mock, now func() time.Time) peopledirectory.Birthdays {
	return testutil.NewBirthdayCapability(db, settings, now)
}

// setBirthday stamps a birth date on a fixture person. The fixtures create
// people without one, which is the realistic default: a school that has not
// maintained every date must still get a working list.
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

// fullAccess stands in for an admin or verified staff caller: every child is
// visible (#2329 — the visibility decision is all-or-nothing).
type fullAccess struct{}

func (fullAccess) HasFullAccess() bool { return true }

// noAccess mirrors a caller without a staff record (guest, guardian): no child
// is visible.
type noAccess struct{}

func (noAccess) HasFullAccess() bool { return false }

func celebrationNames(overview peopledirectory.BirthdayOverview) []string {
	names := make([]string, 0, len(overview.Celebrations))
	for _, celebration := range overview.Celebrations {
		names = append(names, celebration.Name)
	}
	return names
}

// A view speaks for one Monday-to-Sunday week (#3777): the days before today
// matter as much as the ones after it, because the OGS often celebrates after
// the actual birthday. The days around the week stay out.
func TestBirthdayOverviewShowsTheWholeWeek(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	wednesday := time.Date(2026, time.August, 5, 9, 0, 0, 0, calendar.Berlin)
	monday := calendar.NewDate(2026, time.August, 3)
	sunday := calendar.NewDate(2026, time.August, 9)

	onToday := testpkg.CreateTestStudent(t, db, "Emma", "Heutekind", "2b")
	onMonday := testpkg.CreateTestStudent(t, db, "Lina", "Montagskind", "1a")
	onSunday := testpkg.CreateTestStudent(t, db, "Mika", "Sonntagskind", "1a")
	lastSunday := testpkg.CreateTestStudent(t, db, "Nils", "Vorwochenkind", "1a")
	nextMonday := testpkg.CreateTestStudent(t, db, "Olga", "Folgewochenkind", "1a")
	testpkg.CreateTestStudent(t, db, "Ohne", "Datum", "1a")

	setBirthday(t, db, onToday.PersonID, calendar.NewDate(2017, time.August, 5))
	setBirthday(t, db, onMonday.PersonID, calendar.NewDate(2018, time.August, 3))
	setBirthday(t, db, onSunday.PersonID, calendar.NewDate(2019, time.August, 9))
	setBirthday(t, db, lastSunday.PersonID, calendar.NewDate(2019, time.August, 2))
	setBirthday(t, db, nextMonday.PersonID, calendar.NewDate(2019, time.August, 10))

	service := newBirthdayService(db, birthdaySettings(true, false), func() time.Time { return wednesday })

	overview, err := service.Overview(testpkg.Ctx(t), fullAccess{}, nil)
	require.NoError(t, err)

	assert.Equal(t, calendar.DateFromTime(wednesday), overview.Today)
	assert.Equal(t, monday, overview.WeekStart)
	assert.Equal(t, sunday, overview.WeekEnd)

	names := celebrationNames(overview)
	assert.Contains(t, names, "Emma Heutekind")
	assert.Contains(t, names, "Lina Montagskind", "earlier days of the week stay visible")
	assert.Contains(t, names, "Mika Sonntagskind", "the coming weekend belongs to the week")
	assert.NotContains(t, names, "Nils Vorwochenkind", "the Sunday before belongs to the previous week")
	assert.NotContains(t, names, "Olga Folgewochenkind")
	assert.NotContains(t, names, "Ohne Datum", "a child without a stored date is never invented into the list")

	for _, celebration := range overview.Celebrations {
		switch celebration.Name {
		case "Emma Heutekind":
			assert.True(t, celebration.IsToday)
			assert.Equal(t, 9, celebration.Age)
		case "Lina Montagskind":
			assert.False(t, celebration.IsToday, "an earlier day must not read as today")
			assert.Equal(t, monday, celebration.Date)
			assert.Equal(t, 8, celebration.Age)
		}
	}
}

// Stepping back shows who had a birthday last week; stepping further than
// the reach is refused rather than silently clamped.
func TestBirthdayOverviewOtherWeeks(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	wednesday := time.Date(2026, time.August, 5, 9, 0, 0, 0, calendar.Berlin)
	lastWeek := testpkg.CreateTestStudent(t, db, "Paul", "Letztewoche", "2b")
	thisWeek := testpkg.CreateTestStudent(t, db, "Rita", "Diesewoche", "2b")
	setBirthday(t, db, lastWeek.PersonID, calendar.NewDate(2017, time.July, 30))
	setBirthday(t, db, thisWeek.PersonID, calendar.NewDate(2017, time.August, 4))

	service := newBirthdayService(db, birthdaySettings(true, false), func() time.Time { return wednesday })
	ctx := testpkg.Ctx(t)

	t.Run("previous week", func(t *testing.T) {
		thursday := calendar.NewDate(2026, time.July, 30)
		overview, err := service.Overview(ctx, fullAccess{}, &thursday)
		require.NoError(t, err)

		assert.Equal(t, calendar.NewDate(2026, time.July, 27), overview.WeekStart, "any day selects its week")
		names := celebrationNames(overview)
		assert.Contains(t, names, "Paul Letztewoche")
		assert.NotContains(t, names, "Rita Diesewoche")
		for _, celebration := range overview.Celebrations {
			assert.False(t, celebration.IsToday, "no day of another week is today")
		}
	})

	t.Run("bounds travel with every answer", func(t *testing.T) {
		overview, err := service.Overview(ctx, fullAccess{}, nil)
		require.NoError(t, err)

		assert.Equal(t, calendar.NewDate(2026, time.July, 6), overview.EarliestWeekStart)
		assert.Equal(t, calendar.NewDate(2026, time.August, 31), overview.LatestWeekStart)
	})

	t.Run("beyond the reach", func(t *testing.T) {
		tooFar := calendar.NewDate(2026, time.June, 29)
		_, err := service.Overview(ctx, fullAccess{}, &tooFar)
		require.ErrorIs(t, err, peopledirectory.ErrBirthdayWeekOutOfRange)
	})
}

// A leap-day child must not disappear for three years out of four.
func TestBirthdayOverviewLeapDayFallsOnFirstOfMarch(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	student := testpkg.CreateTestStudent(t, db, "Jonas", "Schalttagskind", "1a")
	setBirthday(t, db, student.PersonID, calendar.NewDate(2020, time.February, 29))

	commonYear := time.Date(2027, time.March, 1, 9, 0, 0, 0, calendar.Berlin)
	service := newBirthdayService(db, birthdaySettings(true, false), func() time.Time { return commonYear })

	overview, err := service.Overview(testpkg.Ctx(t), fullAccess{}, nil)
	require.NoError(t, err)

	assert.Equal(t, []calendar.Date{calendar.NewDate(2027, time.March, 1)}, shownOn(overview, "Jonas Schalttagskind"))

	// In a leap year the day belongs to 29 February and not to 1 March. The
	// week of 1 March 2028 (a Wednesday) contains both days, so the entry
	// must appear exactly once, on 29 February.
	leapYear := time.Date(2028, time.March, 1, 9, 0, 0, 0, calendar.Berlin)
	leapService := newBirthdayService(db, birthdaySettings(true, false), func() time.Time { return leapYear })

	leapOverview, err := leapService.Overview(testpkg.Ctx(t), fullAccess{}, nil)
	require.NoError(t, err)

	assert.Equal(t, []calendar.Date{calendar.NewDate(2028, time.February, 29)}, shownOn(leapOverview, "Jonas Schalttagskind"))
}

// shownOn lists the days a person's birthday appears on in one overview.
func shownOn(overview peopledirectory.BirthdayOverview, name string) []calendar.Date {
	var dates []calendar.Date
	for _, celebration := range overview.Celebrations {
		if celebration.Name == name {
			dates = append(dates, celebration.Date)
		}
	}
	return dates
}

// Datenschutz: staff appear only when the school opted in, and never when the
// person opted out.
func TestBirthdayOverviewStaffVisibility(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	today := time.Date(2026, time.August, 5, 9, 0, 0, 0, calendar.Berlin)

	visible := testpkg.CreateTestStaff(t, db, "Anna", "Sichtbar")
	optedOut, account := testpkg.CreateTestStaffWithAccount(t, db, "Bea", "Abgemeldet")

	birthday := calendar.NewDate(1985, today.Month(), today.Day())
	setBirthday(t, db, visible.PersonID, birthday)
	setBirthday(t, db, optedOut.PersonID, birthday)

	ctx := testpkg.Ctx(t)

	t.Run("hidden while the school has not opted in", func(t *testing.T) {
		service := newBirthdayService(db, birthdaySettings(true, false), func() time.Time { return today })

		overview, err := service.Overview(ctx, fullAccess{}, nil)
		require.NoError(t, err)

		assert.False(t, overview.IncludeStaff)
		assert.NotContains(t, celebrationNames(overview), "Anna Sichtbar")
	})

	t.Run("shown once the school opted in, except for the personal opt-out", func(t *testing.T) {
		service := newBirthdayService(db, birthdaySettings(true, true), func() time.Time { return today })
		require.NoError(t, service.SetOptOut(ctx, account.ID, true))

		overview, err := service.Overview(ctx, fullAccess{}, nil)
		require.NoError(t, err)

		assert.True(t, overview.IncludeStaff)
		names := celebrationNames(overview)
		assert.Contains(t, names, "Anna Sichtbar")
		assert.NotContains(t, names, "Bea Abgemeldet", "a personal opt-out outranks the school setting")

		for _, celebration := range overview.Celebrations {
			if celebration.Kind == peopledirectory.BirthdayKindStaff {
				assert.Zero(t, celebration.Age, "a colleague's age is never disclosed")
			}
		}
	})
}

// The school switch is a real switch: off means nothing is queried at all.
func TestBirthdayOverviewDisabled(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	today := time.Date(2026, time.August, 5, 9, 0, 0, 0, calendar.Berlin)
	student := testpkg.CreateTestStudent(t, db, "Nicht", "Sichtbar", "3c")
	setBirthday(t, db, student.PersonID, calendar.NewDate(2016, today.Month(), today.Day()))

	service := newBirthdayService(db, birthdaySettings(false, true), func() time.Time { return today })

	overview, err := service.Overview(testpkg.Ctx(t), fullAccess{}, nil)
	require.NoError(t, err)

	assert.False(t, overview.Enabled)
	assert.False(t, overview.IncludeStaff, "the staff setting is not even read once the display is off")
	assert.Empty(t, overview.Celebrations)
}

// A settings backend that errors must surface, not silently render an empty
// card that looks like "nobody has a birthday today".
func TestBirthdayOverviewSettingsErrors(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	now := func() time.Time { return time.Date(2026, time.August, 5, 9, 0, 0, 0, calendar.Berlin) }

	t.Run("display setting fails", func(t *testing.T) {
		settings := &configtest.Mock{
			ResolveBoolFn: func(context.Context, string) (bool, error) {
				return false, errors.New("boom")
			},
		}
		_, err := newBirthdayService(db, settings, now).Overview(ctx, fullAccess{}, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "resolve birthday display setting")
	})

	t.Run("staff setting fails", func(t *testing.T) {
		settings := &configtest.Mock{
			ResolveBoolFn: func(_ context.Context, key string) (bool, error) {
				if key == keyDisplayEnabled {
					return true, nil
				}
				return false, errors.New("boom")
			},
		}
		_, err := newBirthdayService(db, settings, now).Overview(ctx, fullAccess{}, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "resolve staff birthday setting")
	})
}

// The staff list is the administrative view: sorted as a calendar, filtered by
// birth month, and it deliberately keeps people who opted out of the dashboard
// — the opt-out governs the shared screen, not the list an admin may pull.
func TestListStaffBirthdays(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	march := testpkg.CreateTestStaff(t, db, "Clara", "Maerz")
	augustLate := testpkg.CreateTestStaff(t, db, "Dora", "Spaetaugust")
	augustEarly, account := testpkg.CreateTestStaffWithAccount(t, db, "Erik", "Fruehaugust")
	testpkg.CreateTestStaff(t, db, "Frank", "Ohnedatum")

	setBirthday(t, db, march.PersonID, calendar.NewDate(1979, time.March, 14))
	setBirthday(t, db, augustLate.PersonID, calendar.NewDate(1992, time.August, 21))
	setBirthday(t, db, augustEarly.PersonID, calendar.NewDate(1996, time.August, 9))

	ctx := testpkg.Ctx(t)
	service := newBirthdayService(db, birthdaySettings(true, true), nil)
	require.NoError(t, service.SetOptOut(ctx, account.ID, true))

	t.Run("whole year, calendar order, opt-outs included", func(t *testing.T) {
		entries, err := service.ListStaffBirthdays(ctx, nil)
		require.NoError(t, err)

		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name)
		}

		assert.Contains(t, names, "Clara Maerz")
		assert.Contains(t, names, "Erik Fruehaugust", "the administrative list keeps a dashboard opt-out")
		assert.NotContains(t, names, "Frank Ohnedatum", "no birth date, no row")
		assert.Less(t,
			indexOf(names, "Clara Maerz"), indexOf(names, "Erik Fruehaugust"),
			"March sorts before August")
		assert.Less(t,
			indexOf(names, "Erik Fruehaugust"), indexOf(names, "Dora Spaetaugust"),
			"the 9th sorts before the 21st within the same month")
	})

	t.Run("month filter keeps only the selected months", func(t *testing.T) {
		entries, err := service.ListStaffBirthdays(ctx, map[time.Month]bool{time.March: true})
		require.NoError(t, err)

		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name)
		}

		assert.Contains(t, names, "Clara Maerz")
		assert.NotContains(t, names, "Erik Fruehaugust")
		assert.NotContains(t, names, "Dora Spaetaugust")
	})
}

func indexOf(values []string, want string) int {
	for i, value := range values {
		if value == want {
			return i
		}
	}
	return -1
}

// The opt-out is self-service and idempotent: setting the value it already has
// must not write, and must not fail either.
func TestBirthdayOptOutRoundTrip(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	_, account := testpkg.CreateTestStaffWithAccount(t, db, "Greta", "Selbst")

	ctx := testpkg.Ctx(t)
	service := newBirthdayService(db, birthdaySettings(true, true), nil)

	optOut, err := service.GetOptOut(ctx, account.ID)
	require.NoError(t, err)
	assert.False(t, optOut, "a new staff member is visible by default")

	require.NoError(t, service.SetOptOut(ctx, account.ID, true))
	require.NoError(t, service.SetOptOut(ctx, account.ID, true), "setting the same value again is a no-op")

	optOut, err = service.GetOptOut(ctx, account.ID)
	require.NoError(t, err)
	assert.True(t, optOut)

	require.NoError(t, service.SetOptOut(ctx, account.ID, false))

	optOut, err = service.GetOptOut(ctx, account.ID)
	require.NoError(t, err)
	assert.False(t, optOut)
}

// An account that is not staff of this tenant has nothing to opt out of. That
// is a clean not-found, not a 500 and not a silent success.
func TestBirthdayOptOutWithoutStaffRecord(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	service := newBirthdayService(db, birthdaySettings(true, true), nil)

	t.Run("account without a person", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "birthday-no-person@example.com")

		_, err := service.GetOptOut(ctx, account.ID)
		require.ErrorIs(t, err, peopledirectory.ErrStaffNotFound)

		require.ErrorIs(t, service.SetOptOut(ctx, account.ID, true), peopledirectory.ErrStaffNotFound)
	})

	t.Run("person without a staff record", func(t *testing.T) {
		_, account := testpkg.CreateTestPersonWithAccount(t, db, "Hanna", "Nurperson")

		_, err := service.GetOptOut(ctx, account.ID)
		require.ErrorIs(t, err, peopledirectory.ErrStaffNotFound)
	})
}

// Datenschutz-Blocker aus dem Review: a birthday row is student data. Since
// #2329 the decision is all-or-nothing — admins and verified staff see every
// child of the tenant, an unclassified caller (guest, guardian, or none at all)
// receives nothing.
func TestBirthdayOverviewAppliesStudentDataScope(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	today := time.Date(2026, time.August, 5, 9, 0, 0, 0, calendar.Berlin)
	birthday := calendar.NewDate(2018, today.Month(), today.Day())

	myGroup := testpkg.CreateTestEducationGroup(t, db, "Meine Gruppe 1542")
	otherGroup := testpkg.CreateTestEducationGroup(t, db, "Fremde Gruppe 1542")

	// Defers run last-in-first-out: the children go before the groups they
	// reference, so the FK never blocks the cleanup.

	mine := testpkg.CreateTestStudent(t, db, "Mein", "Gruppenkind", "1a")
	foreign := testpkg.CreateTestStudent(t, db, "Fremdes", "Gruppenkind", "1a")
	groupless := testpkg.CreateTestStudent(t, db, "Ohne", "Gruppenkind", "1a")

	for personID := range map[int64]struct{}{
		mine.PersonID: {}, foreign.PersonID: {}, groupless.PersonID: {},
	} {
		setBirthday(t, db, personID, birthday)
	}
	testpkg.AssignStudentGroup(t, db, mine.ID, myGroup.ID)
	testpkg.AssignStudentGroup(t, db, foreign.ID, otherGroup.ID)

	ctx := testpkg.Ctx(t)
	service := newBirthdayService(db, birthdaySettings(true, false), func() time.Time { return today })

	t.Run("caller without full access sees no child at all", func(t *testing.T) {
		overview, err := service.Overview(ctx, noAccess{}, nil)
		require.NoError(t, err)

		assert.Empty(t, celebrationNames(overview), "the display fails closed rather than publishing the school")
	})

	t.Run("unclassified caller sees no child at all", func(t *testing.T) {
		overview, err := service.Overview(ctx, nil, nil)
		require.NoError(t, err)

		assert.Empty(t, celebrationNames(overview), "a nil visibility must never be read as full access")
	})

	t.Run("full access sees every child, group or not", func(t *testing.T) {
		overview, err := service.Overview(ctx, fullAccess{}, nil)
		require.NoError(t, err)

		names := celebrationNames(overview)
		assert.Contains(t, names, "Mein Gruppenkind")
		assert.Contains(t, names, "Fremdes Gruppenkind")
		assert.Contains(t, names, "Ohne Gruppenkind", "a group-less child is visible to staff like any other")
	})
}

// A child whose care has ended is off the birthday card from the day after
// their last care day (#2487): the card is a staff-facing list of the children
// the school currently cares for, and congratulating a child who left in
// August is exactly the kind of thing the office has to explain afterwards.
func TestStudentBirthdaysExcludeEndedCare(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)

	today := calendar.TodayDate()
	staying := testpkg.CreateTestStudent(t, db, "Geburtstag", "Bleibt", "1a")
	lastDay := testpkg.CreateTestStudent(t, db, "Geburtstag", "LetzterTag", "1a")
	departed := testpkg.CreateTestStudent(t, db, "Geburtstag", "Weg", "1a")
	for _, student := range []int64{staying.PersonID, lastDay.PersonID, departed.PersonID} {
		setBirthday(t, db, student, calendar.NewDate(2018, today.Month(), today.Day()))
	}
	setEnrolledUntil(t, db, lastDay.ID, today)
	setEnrolledUntil(t, db, departed.ID, today.AddDays(-1))

	overview, err := newBirthdayService(db, birthdaySettings(true, false), nil).Overview(ctx, fullAccess{}, nil)
	require.NoError(t, err)

	names := celebrationNames(overview)
	assert.Contains(t, names, "Geburtstag Bleibt")
	assert.Contains(t, names, "Geburtstag LetzterTag", "the last care day still counts as care")
	assert.NotContains(t, names, "Geburtstag Weg")
}

// setEnrolledUntil stamps the enrollment interval's inclusive upper bound.
func setEnrolledUntil(t *testing.T, db *bun.DB, studentID int64, until calendar.Date) {
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
