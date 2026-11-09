package owners

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The Worker's status-flag run over Care Plan and People Directory as the
// root binds them (#6291). Invariants:
//   - The status of a child on day X is the same in every view: the day log,
//     the group view and the class day read the standing rows of X
//     (FindSignedOffByStudentIDsAndDate), the dashboard counts them together
//     with the live flag.
//   - The archive loses neither status nor source.
//   - A late run archives every flag onto its own status day.

type statusFlagSuite struct {
	db   *bun.DB
	days *StudentStatusDayRepository
}

func newStatusFlagSuite(t *testing.T) statusFlagSuite {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	carePlan, err := NewUnobservedCarePlan(testpkg.NewDatabase(t, db), MustNewUnobservedTimetable(testpkg.NewDatabase(t, db)), MustNewPeopleDirectory(testpkg.NewDatabase(t, db)))
	require.NoError(t, err)
	return statusFlagSuite{db: db, days: NewStudentStatusDayRepository(carePlan)}
}

func (s statusFlagSuite) inTenant(t *testing.T, fn func(context.Context) error) {
	t.Helper()
	require.NoError(t, testpkg.WithinTenantContext(t, context.Background(), s.db, testpkg.Tenant(t), fn))
}

// flag sets the child's live flag as a report of since does.
func (s statusFlagSuite) flag(t *testing.T, studentID int64, column string, since time.Time) {
	t.Helper()
	_, err := s.db.ExecContext(context.Background(), `UPDATE users.student_care_profiles SET `+column+` = true, `+column+`_since = ?
		WHERE membership_id IN (SELECT id FROM users.student_school_memberships WHERE student_profile_id = ?)`, since, studentID)
	require.NoError(t, err)
}

func (s statusFlagSuite) flagged(t *testing.T, studentID int64, column string) bool {
	t.Helper()
	var set bool
	require.NoError(t, s.db.NewRaw(`SELECT COALESCE(c.`+column+`, false) FROM users.student_care_profiles AS c
		JOIN users.student_school_memberships AS m ON m.id = c.membership_id WHERE m.student_profile_id = ?`, studentID).Scan(context.Background(), &set))
	return set
}

// report writes the status day a report of the source leaves.
func (s statusFlagSuite) report(t *testing.T, studentID int64, date calendar.Date, status, source string, reportedAt time.Time) {
	t.Helper()
	_, err := s.db.ExecContext(context.Background(), `INSERT INTO active.student_status_days (tenant_id, student_id, date, status, reported_at, source)
		VALUES (?, ?, ?, ?, ?, ?)`, testpkg.Tenant(t), studentID, date, status, reportedAt, source)
	require.NoError(t, err)
}

// standing returns the status and source of every row that stands for the
// day, keyed by child, as the day log, the group view and the class day read
// them.
func (s statusFlagSuite) standing(t *testing.T, date calendar.Date, studentIDs ...int64) map[int64][2]string {
	t.Helper()
	out := map[int64][2]string{}
	s.inTenant(t, func(ctx context.Context) error {
		rows, err := s.days.FindSignedOffByStudentIDsAndDate(ctx, studentIDs, date)
		for _, row := range rows {
			out[row.StudentID] = [2]string{row.Status, row.Source}
		}
		return err
	})
	return out
}

func (s statusFlagSuite) dashboardSick(t *testing.T, date calendar.Date) int {
	t.Helper()
	var sick int
	s.inTenant(t, func(ctx context.Context) error {
		counts, err := s.days.CountEffectiveDashboardAbsences(ctx, date)
		if counts != nil {
			sick = counts.Sick
		}
		return err
	})
	return sick
}

func berlinAt(date calendar.Date, hour, minute int) time.Time {
	day := date.BerlinMidnight()
	return time.Date(day.Year(), day.Month(), day.Day(), hour, minute, 0, 0, calendar.Berlin)
}

// The 18:00 run closes the day of a child the school reported sick: the day
// keeps the school as its source and stays visible to every view that reads
// it afterwards, a flag-only day gets an end-of-day row, and a report after
// 18:00 waits for the midnight run, which dates it on its own day.
func TestEndOfDayArchiveKeepsStatusAndSourceOfTheDay(t *testing.T) {
	t.Parallel()
	s := newStatusFlagSuite(t)
	day := calendar.NewDate(2026, time.November, 10)
	reported := testpkg.CreateTestStudent(t, s.db, "Sina", "Schule", "3a")
	flagOnly := testpkg.CreateTestStudent(t, s.db, "Fritz", "Flag", "3a")
	evening := testpkg.CreateTestStudent(t, s.db, "Emil", "Abend", "3a")
	s.report(t, reported.ID, day, "sick", "school", berlinAt(day, 8, 0))
	s.flag(t, reported.ID, "sick", berlinAt(day, 8, 0))
	s.flag(t, flagOnly.ID, "sick", berlinAt(day, 9, 0))
	s.flag(t, evening.ID, "sick", berlinAt(day, 18, 30))

	endOfDay := berlinAt(day, 18, 0)
	s.inTenant(t, func(ctx context.Context) error {
		_, err := s.days.ArchiveEndOfDayStatusFlag(ctx, "sick", "sick_since", endOfDay, day, endOfDay)
		return err
	})

	standing := s.standing(t, day, reported.ID, flagOnly.ID, evening.ID)
	assert.Equal(t, [2]string{"sick", "school"}, standing[reported.ID], "the archive keeps the school's report")
	assert.Equal(t, [2]string{"sick", "end_of_day"}, standing[flagOnly.ID])
	assert.NotContains(t, standing, evening.ID, "a report after the end of the day is not archived yet")
	assert.False(t, s.flagged(t, reported.ID, "sick"))
	assert.False(t, s.flagged(t, flagOnly.ID, "sick"))
	assert.True(t, s.flagged(t, evening.ID, "sick"))
	assert.Equal(t, 3, s.dashboardSick(t, day), "the dashboard counts the archived days and the live flag")

	midnight := day.AddDays(1).BerlinMidnight()
	s.inTenant(t, func(ctx context.Context) error {
		_, err := s.days.ArchiveEndOfDayStatusFlag(ctx, "sick", "sick_since", midnight, day, midnight)
		return err
	})
	assert.Equal(t, [2]string{"sick", "end_of_day"}, s.standing(t, day, evening.ID)[evening.ID], "the evening report belongs to its own day")
	assert.Empty(t, s.standing(t, day.AddDays(1), reported.ID, flagOnly.ID, evening.ID), "no status moves into the next day")
	assert.False(t, s.flagged(t, evening.ID, "sick"))
	assert.Zero(t, s.dashboardSick(t, day.AddDays(1)))
}

// A run that comes late, on the next day and after the daylight saving
// change, still dates a flag on its own Berlin day: 00:30 on 25 October 2026
// is 22:30 UTC the day before, and the run's own day is 26 October.
func TestEndOfDayArchiveDatesTheFlagOnItsBerlinDayAcrossDST(t *testing.T) {
	t.Parallel()
	s := newStatusFlagSuite(t)
	day := calendar.NewDate(2026, time.October, 25)
	child := testpkg.CreateTestStudent(t, s.db, "Zora", "Zeit", "4b")
	since := berlinAt(day, 0, 30)
	require.Equal(t, 24, since.UTC().Day())
	s.flag(t, child.ID, "excused", since)

	nextMorning := day.AddDays(1).BerlinMidnight().Add(8 * time.Hour)
	s.inTenant(t, func(ctx context.Context) error {
		_, err := s.days.ArchiveEndOfDayStatusFlag(ctx, "excused", "excused_since", day.AddDays(1).BerlinMidnight(), day.AddDays(1), nextMorning)
		return err
	})

	assert.Equal(t, [2]string{"excused", "end_of_day"}, s.standing(t, day, child.ID)[child.ID])
	assert.Empty(t, s.standing(t, day.AddDays(-1), child.ID), "not the UTC day")
	assert.Empty(t, s.standing(t, day.AddDays(1), child.ID), "not the day of the run")
	assert.False(t, s.flagged(t, child.ID, "excused"))
}

// With manual or next_checkin the flag outlasts its day. The midnight run
// lets it stand for the new day with the source of the report, so the views
// that read rows show what the group view and the dashboard show from the
// flag; a flag of the new day and a carried day are not written twice.
func TestStatusFlagCarryStandsForTheNewDay(t *testing.T) {
	t.Parallel()
	s := newStatusFlagSuite(t)
	day := calendar.NewDate(2026, time.November, 10)
	carried := testpkg.CreateTestStudent(t, s.db, "Paula", "Eltern", "2c")
	today := testpkg.CreateTestStudent(t, s.db, "Tim", "Heute", "2c")
	yesterday := day.AddDays(-1)
	s.report(t, carried.ID, yesterday, "sick", "parent", berlinAt(yesterday, 7, 0))
	s.flag(t, carried.ID, "sick", berlinAt(yesterday, 7, 0))
	s.report(t, today.ID, day, "sick", "manual", berlinAt(day, 7, 0))
	s.flag(t, today.ID, "sick", berlinAt(day, 7, 0))
	require.Empty(t, s.standing(t, day, carried.ID), "before the run the flag is the only trace of the new day")

	for range 2 {
		s.inTenant(t, func(ctx context.Context) error {
			_, err := s.days.CarryStatusFlag(ctx, "sick", "sick_since", day, berlinAt(day, 7, 30))
			return err
		})
	}

	standing := s.standing(t, day, carried.ID, today.ID)
	assert.Equal(t, [2]string{"sick", "parent"}, standing[carried.ID])
	assert.Equal(t, [2]string{"sick", "manual"}, standing[today.ID], "the day's own report stays as it is")
	assert.True(t, s.flagged(t, carried.ID, "sick"), "the carry keeps the flag")
	assert.Equal(t, [2]string{"sick", "parent"}, s.standing(t, yesterday, carried.ID)[carried.ID])
	assert.Equal(t, 2, s.dashboardSick(t, day))
	var rows int
	require.NoError(t, s.db.NewRaw(`SELECT COUNT(*) FROM active.student_status_days WHERE student_id = ? AND date = ?`, carried.ID, day).Scan(context.Background(), &rows))
	assert.Equal(t, 1, rows, "a second run writes no second day")
}
