package users_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Birthday service against real repositories (#1542). The HTTP tests in
// api/birthdays pin the route contract; this file pins the rules the service
// itself owns: which days a view speaks for, who may appear, and what the
// staff list contains.

// birthdaySettings answers the two display settings and fails loudly on any
// other key, so a future setting cannot silently read as false here.
func birthdaySettings(enabled, includeStaff bool) *configtest.Mock {
	return &configtest.Mock{
		ResolveBoolFn: func(_ context.Context, key string) (bool, error) {
			switch key {
			case configModels.KeyBirthdayDisplayEnabled:
				return enabled, nil
			case configModels.KeyBirthdayDisplayIncludeStaff:
				return includeStaff, nil
			default:
				return false, errors.New("unexpected setting: " + key)
			}
		},
	}
}

func newBirthdayService(db *bun.DB, settings *configtest.Mock, now func() time.Time) usersService.BirthdayService {
	repos := repositories.NewFactory(db)
	return usersService.NewBirthdayService(usersService.BirthdayServiceDependencies{
		StudentRepo:     repos.Student,
		StaffRepo:       repos.Staff,
		PersonRepo:      repos.Person,
		SettingsService: settings,
		Now:             now,
	})
}

// setBirthday stamps a birth date on a fixture person. The fixtures create
// people without one, which is the realistic default: a school that has not
// maintained every date must still get a working list.
func setBirthday(t *testing.T, db *bun.DB, personID int64, date timezone.Date) {
	t.Helper()
	ctx, cancel := context.WithTimeout(testpkg.TenantContext(1), 5*time.Second)
	defer cancel()

	_, err := db.NewUpdate().
		Table("users.persons").
		Set("birthday = ?", date).
		Where("id = ?", personID).
		Exec(ctx)
	require.NoError(t, err, "stamp birthday on test person")
}

// fullAccess stands in for an admin caller: every child is visible. The
// group-scoped variants below carry the interesting cases.
type fullAccess struct{}

func (fullAccess) HasFullAccessByGroupID(*int64) bool { return true }

// groupScoped mirrors a caregiver: only children of the supervised groups.
type groupScoped struct{ groupIDs map[int64]bool }

func (g groupScoped) HasFullAccessByGroupID(groupID *int64) bool {
	if groupID == nil {
		return false
	}
	return g.groupIDs[*groupID]
}

func celebrationNames(overview *usersService.BirthdayOverview) []string {
	names := make([]string, 0, len(overview.Celebrations))
	for _, celebration := range overview.Celebrations {
		names = append(names, celebration.Name)
	}
	return names
}

// A Monday view speaks for the weekend before it: nobody opens the app on
// Saturday, so without this rule a child born on a Saturday is never
// celebrated.
func TestBirthdayOverviewMondayCarriesTheWeekend(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() {
		require.NoError(t, db.Close())
	}()

	monday := time.Date(2026, time.August, 3, 9, 0, 0, 0, timezone.Berlin)
	saturday := monday.AddDate(0, 0, -2)
	friday := monday.AddDate(0, 0, -3)

	onMonday := testpkg.CreateTestStudent(t, db, "Lina", "Montagskind", "1a")
	onSaturday := testpkg.CreateTestStudent(t, db, "Mika", "Samstagskind", "1a")
	onFriday := testpkg.CreateTestStudent(t, db, "Nils", "Freitagskind", "1a")
	noDate := testpkg.CreateTestStudent(t, db, "Ohne", "Datum", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, onMonday.ID, onSaturday.ID, onFriday.ID, noDate.ID)

	setBirthday(t, db, onMonday.PersonID, timezone.NewDate(2018, monday.Month(), monday.Day()))
	setBirthday(t, db, onSaturday.PersonID, timezone.NewDate(2019, saturday.Month(), saturday.Day()))
	setBirthday(t, db, onFriday.PersonID, timezone.NewDate(2019, friday.Month(), friday.Day()))

	service := newBirthdayService(db, birthdaySettings(true, false), func() time.Time { return monday })

	overview, err := service.Overview(testpkg.TenantContext(1), fullAccess{})
	require.NoError(t, err)

	names := celebrationNames(overview)
	assert.Contains(t, names, "Lina Montagskind")
	assert.Contains(t, names, "Mika Samstagskind")
	assert.NotContains(t, names, "Nils Freitagskind", "Friday is not part of a Monday view")
	assert.NotContains(t, names, "Ohne Datum", "a child without a stored date is never invented into the list")

	for _, celebration := range overview.Celebrations {
		switch celebration.Name {
		case "Lina Montagskind":
			assert.True(t, celebration.IsToday)
			assert.Equal(t, 8, celebration.Age)
		case "Mika Samstagskind":
			assert.False(t, celebration.IsToday, "the weekend entry must not read as today")
			assert.Equal(t, timezone.DateFromTime(saturday), celebration.Date)
			assert.Equal(t, 7, celebration.Age)
		}
	}
}

// An ordinary weekday speaks only for itself.
func TestBirthdayOverviewWeekdayIgnoresOtherDays(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() {
		require.NoError(t, db.Close())
	}()

	wednesday := time.Date(2026, time.August, 5, 9, 0, 0, 0, timezone.Berlin)
	yesterday := wednesday.AddDate(0, 0, -1)

	today := testpkg.CreateTestStudent(t, db, "Emma", "Heutekind", "2b")
	other := testpkg.CreateTestStudent(t, db, "Paul", "Gesternkind", "2b")
	defer testpkg.CleanupActivityFixtures(t, db, today.ID, other.ID)

	setBirthday(t, db, today.PersonID, timezone.NewDate(2017, wednesday.Month(), wednesday.Day()))
	setBirthday(t, db, other.PersonID, timezone.NewDate(2017, yesterday.Month(), yesterday.Day()))

	service := newBirthdayService(db, birthdaySettings(true, false), func() time.Time { return wednesday })

	overview, err := service.Overview(testpkg.TenantContext(1), fullAccess{})
	require.NoError(t, err)

	names := celebrationNames(overview)
	assert.Contains(t, names, "Emma Heutekind")
	assert.NotContains(t, names, "Paul Gesternkind")
	assert.Equal(t, timezone.DateFromTime(wednesday), overview.Today)
}

// A leap-day child must not disappear for three years out of four.
func TestBirthdayOverviewLeapDayFallsOnFirstOfMarch(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() {
		require.NoError(t, db.Close())
	}()

	student := testpkg.CreateTestStudent(t, db, "Jonas", "Schalttagskind", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)
	setBirthday(t, db, student.PersonID, timezone.NewDate(2020, time.February, 29))

	commonYear := time.Date(2027, time.March, 1, 9, 0, 0, 0, timezone.Berlin)
	service := newBirthdayService(db, birthdaySettings(true, false), func() time.Time { return commonYear })

	overview, err := service.Overview(testpkg.TenantContext(1), fullAccess{})
	require.NoError(t, err)

	assert.Contains(t, celebrationNames(overview), "Jonas Schalttagskind")

	// In a leap year the day belongs to 29 February and not to 1 March.
	leapYear := time.Date(2028, time.March, 1, 9, 0, 0, 0, timezone.Berlin)
	leapService := newBirthdayService(db, birthdaySettings(true, false), func() time.Time { return leapYear })

	leapOverview, err := leapService.Overview(testpkg.TenantContext(1), fullAccess{})
	require.NoError(t, err)

	assert.NotContains(t, celebrationNames(leapOverview), "Jonas Schalttagskind")
}

// Datenschutz: staff appear only when the school opted in, and never when the
// person opted out.
func TestBirthdayOverviewStaffVisibility(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() {
		require.NoError(t, db.Close())
	}()

	today := time.Date(2026, time.August, 5, 9, 0, 0, 0, timezone.Berlin)

	visible := testpkg.CreateTestStaff(t, db, "Anna", "Sichtbar")
	optedOut, account := testpkg.CreateTestStaffWithAccount(t, db, "Bea", "Abgemeldet")
	defer testpkg.CleanupStaffFixtures(t, db, visible.ID, optedOut.ID)
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	birthday := timezone.NewDate(1985, today.Month(), today.Day())
	setBirthday(t, db, visible.PersonID, birthday)
	setBirthday(t, db, optedOut.PersonID, birthday)

	ctx := testpkg.TenantContext(1)

	t.Run("hidden while the school has not opted in", func(t *testing.T) {
		service := newBirthdayService(db, birthdaySettings(true, false), func() time.Time { return today })

		overview, err := service.Overview(ctx, fullAccess{})
		require.NoError(t, err)

		assert.False(t, overview.IncludeStaff)
		assert.NotContains(t, celebrationNames(overview), "Anna Sichtbar")
	})

	t.Run("shown once the school opted in, except for the personal opt-out", func(t *testing.T) {
		service := newBirthdayService(db, birthdaySettings(true, true), func() time.Time { return today })
		require.NoError(t, service.SetOptOut(ctx, account.ID, true))

		overview, err := service.Overview(ctx, fullAccess{})
		require.NoError(t, err)

		assert.True(t, overview.IncludeStaff)
		names := celebrationNames(overview)
		assert.Contains(t, names, "Anna Sichtbar")
		assert.NotContains(t, names, "Bea Abgemeldet", "a personal opt-out outranks the school setting")

		for _, celebration := range overview.Celebrations {
			if celebration.Kind == userModels.BirthdayKindStaff {
				assert.Zero(t, celebration.Age, "a colleague's age is never disclosed")
			}
		}
	})
}

// The school switch is a real switch: off means nothing is queried at all.
func TestBirthdayOverviewDisabled(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() {
		require.NoError(t, db.Close())
	}()

	today := time.Date(2026, time.August, 5, 9, 0, 0, 0, timezone.Berlin)
	student := testpkg.CreateTestStudent(t, db, "Nicht", "Sichtbar", "3c")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)
	setBirthday(t, db, student.PersonID, timezone.NewDate(2016, today.Month(), today.Day()))

	service := newBirthdayService(db, birthdaySettings(false, true), func() time.Time { return today })

	overview, err := service.Overview(testpkg.TenantContext(1), fullAccess{})
	require.NoError(t, err)

	assert.False(t, overview.Enabled)
	assert.False(t, overview.IncludeStaff, "the staff setting is not even read once the display is off")
	assert.Empty(t, overview.Celebrations)
}

// A settings backend that errors must surface, not silently render an empty
// card that looks like "nobody has a birthday today".
func TestBirthdayOverviewSettingsErrors(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() {
		require.NoError(t, db.Close())
	}()

	ctx := testpkg.TenantContext(1)
	now := func() time.Time { return time.Date(2026, time.August, 5, 9, 0, 0, 0, timezone.Berlin) }

	t.Run("display setting fails", func(t *testing.T) {
		settings := &configtest.Mock{
			ResolveBoolFn: func(context.Context, string) (bool, error) {
				return false, errors.New("boom")
			},
		}
		_, err := newBirthdayService(db, settings, now).Overview(ctx, fullAccess{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "resolve birthday display setting")
	})

	t.Run("staff setting fails", func(t *testing.T) {
		settings := &configtest.Mock{
			ResolveBoolFn: func(_ context.Context, key string) (bool, error) {
				if key == configModels.KeyBirthdayDisplayEnabled {
					return true, nil
				}
				return false, errors.New("boom")
			},
		}
		_, err := newBirthdayService(db, settings, now).Overview(ctx, fullAccess{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "resolve staff birthday setting")
	})
}

// The staff list is the administrative view: sorted as a calendar, filtered by
// birth month, and it deliberately keeps people who opted out of the dashboard
// — the opt-out governs the shared screen, not the list an admin may pull.
func TestListStaffBirthdays(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() {
		require.NoError(t, db.Close())
	}()

	march := testpkg.CreateTestStaff(t, db, "Clara", "Maerz")
	augustLate := testpkg.CreateTestStaff(t, db, "Dora", "Spaetaugust")
	augustEarly, account := testpkg.CreateTestStaffWithAccount(t, db, "Erik", "Fruehaugust")
	without := testpkg.CreateTestStaff(t, db, "Frank", "Ohnedatum")
	defer testpkg.CleanupStaffFixtures(t, db, march.ID, augustLate.ID, augustEarly.ID, without.ID)
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	setBirthday(t, db, march.PersonID, timezone.NewDate(1979, time.March, 14))
	setBirthday(t, db, augustLate.PersonID, timezone.NewDate(1992, time.August, 21))
	setBirthday(t, db, augustEarly.PersonID, timezone.NewDate(1996, time.August, 9))

	ctx := testpkg.TenantContext(1)
	service := newBirthdayService(db, birthdaySettings(true, true), nil)
	require.NoError(t, service.SetOptOut(ctx, account.ID, true))

	t.Run("whole year, calendar order, opt-outs included", func(t *testing.T) {
		entries, err := service.ListStaffBirthdays(ctx, nil)
		require.NoError(t, err)

		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.FullName())
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
			names = append(names, entry.FullName())
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
	db := testpkg.SetupTestDB(t)
	defer func() {
		require.NoError(t, db.Close())
	}()

	staff, account := testpkg.CreateTestStaffWithAccount(t, db, "Greta", "Selbst")
	defer testpkg.CleanupStaffFixtures(t, db, staff.ID)
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	ctx := testpkg.TenantContext(1)
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
	db := testpkg.SetupTestDB(t)
	defer func() {
		require.NoError(t, db.Close())
	}()

	ctx := testpkg.TenantContext(1)
	service := newBirthdayService(db, birthdaySettings(true, true), nil)

	t.Run("account without a person", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "birthday-no-person@example.com")
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		_, err := service.GetOptOut(ctx, account.ID)
		require.ErrorIs(t, err, usersService.ErrStaffNotFound)

		require.ErrorIs(t, service.SetOptOut(ctx, account.ID, true), usersService.ErrStaffNotFound)
	})

	t.Run("person without a staff record", func(t *testing.T) {
		person, account := testpkg.CreateTestPersonWithAccount(t, db, "Hanna", "Nurperson")
		defer testpkg.CleanupPerson(t, db, person.ID)
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		_, err := service.GetOptOut(ctx, account.ID)
		require.ErrorIs(t, err, usersService.ErrStaffNotFound)
	})
}

// Datenschutz-Blocker aus dem Review: a birthday row is student data. A caller
// with users:read but only their own groups must not receive the whole school
// through this card, and an unclassified caller must receive nothing at all.
func TestBirthdayOverviewAppliesStudentDataScope(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() {
		require.NoError(t, db.Close())
	}()

	today := time.Date(2026, time.August, 5, 9, 0, 0, 0, timezone.Berlin)
	birthday := timezone.NewDate(2018, today.Month(), today.Day())

	myGroup := testpkg.CreateTestEducationGroup(t, db, "Meine Gruppe 1542")
	otherGroup := testpkg.CreateTestEducationGroup(t, db, "Fremde Gruppe 1542")

	// Defers run last-in-first-out: the children go before the groups they
	// reference, so the FK never blocks the cleanup.
	defer testpkg.CleanupActivityFixtures(t, db, myGroup.ID, otherGroup.ID)

	mine := testpkg.CreateTestStudent(t, db, "Mein", "Gruppenkind", "1a")
	foreign := testpkg.CreateTestStudent(t, db, "Fremdes", "Gruppenkind", "1a")
	groupless := testpkg.CreateTestStudent(t, db, "Ohne", "Gruppenkind", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, mine.ID, foreign.ID, groupless.ID)

	for personID := range map[int64]struct{}{
		mine.PersonID: {}, foreign.PersonID: {}, groupless.PersonID: {},
	} {
		setBirthday(t, db, personID, birthday)
	}
	assignStudentGroup(t, db, mine.ID, myGroup.ID)
	assignStudentGroup(t, db, foreign.ID, otherGroup.ID)

	ctx := testpkg.TenantContext(1)
	service := newBirthdayService(db, birthdaySettings(true, false), func() time.Time { return today })

	t.Run("group-scoped caller sees only their own children", func(t *testing.T) {
		overview, err := service.Overview(ctx, groupScoped{groupIDs: map[int64]bool{myGroup.ID: true}})
		require.NoError(t, err)

		names := celebrationNames(overview)
		assert.Contains(t, names, "Mein Gruppenkind")
		assert.NotContains(t, names, "Fremdes Gruppenkind", "a child outside the caller's groups must not appear")
		assert.NotContains(t, names, "Ohne Gruppenkind", "a group-less child is admin/all-staff territory")
	})

	t.Run("unclassified caller sees no child at all", func(t *testing.T) {
		overview, err := service.Overview(ctx, nil)
		require.NoError(t, err)

		assert.Empty(t, celebrationNames(overview), "the display fails closed rather than publishing the school")
	})

	t.Run("full access still sees everyone", func(t *testing.T) {
		overview, err := service.Overview(ctx, fullAccess{})
		require.NoError(t, err)

		names := celebrationNames(overview)
		assert.Contains(t, names, "Mein Gruppenkind")
		assert.Contains(t, names, "Fremdes Gruppenkind")
		assert.Contains(t, names, "Ohne Gruppenkind")
	})
}

// assignStudentGroup puts a fixture child into an education group.
func assignStudentGroup(t *testing.T, db *bun.DB, studentID, groupID int64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(testpkg.TenantContext(1), 5*time.Second)
	defer cancel()

	_, err := db.NewUpdate().
		Table("users.students").
		Set("group_id = ?", groupID).
		Where("id = ?", studentID).
		Exec(ctx)
	require.NoError(t, err, "assign group to test student")
}

// The repositories refuse an empty day set instead of building a WHERE clause
// with no values (which would degenerate into "every person of the school").
func TestBirthdayRepositoriesRejectAnEmptyDaySet(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() {
		require.NoError(t, db.Close())
	}()

	repos := repositories.NewFactory(db)
	ctx := testpkg.TenantContext(1)

	students, err := repos.Student.FindBirthdaysOn(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, students)

	staff, err := repos.Staff.FindBirthdaysOn(ctx, []userModels.MonthDay{})
	require.NoError(t, err)
	assert.Empty(t, staff)
}
