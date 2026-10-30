package compose

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The week of 2027-09-06 (Monday) to 2027-09-12 (Sunday) is the fixed week
// of these tests (#4769); every instant is a Berlin evening of that week.
var (
	weekProtocolMonday = calendar.NewDate(2027, 9, 6)
	weekProtocolFriday = weekProtocolMonday.AddDays(4)
)

func weekProtocolEvening(day calendar.Date) time.Time {
	return day.BerlinMidnight().Add(20*time.Hour + 30*time.Minute)
}

// weekProtocolSources fakes what the evening run reads outside the owner:
// the groups, the day log by day and the calendar.
type weekProtocolSources struct {
	groups    []timetable.WeekProtocolGroup
	openCare  bool
	days      map[calendar.Date][]timetable.WeekProtocolGroupDay
	holidays  map[string]bool
	closing   map[string]bool
	attendCnt int
}

func (s *weekProtocolSources) GroupDayAttendance(_ context.Context, day calendar.Date, groupIDs []int64) ([]timetable.WeekProtocolGroupDay, error) {
	s.attendCnt++
	wanted := map[int64]bool{}
	for _, id := range groupIDs {
		wanted[id] = true
	}
	result := []timetable.WeekProtocolGroupDay{}
	for _, groupDay := range s.days[day] {
		if wanted[groupDay.GroupID] {
			result = append(result, groupDay)
		}
	}
	return result, nil
}

func (s *weekProtocolSources) TenantHolidayDates(context.Context, string, string) (map[string]bool, error) {
	return s.holidays, nil
}

func (s *weekProtocolSources) ClosingDayDates(context.Context, string, string) (map[string]bool, error) {
	return s.closing, nil
}

func (s *weekProtocolSources) of(db *bun.DB) timetable.WeekProtocolSources {
	return timetable.WeekProtocolSources{
		Notices:    NewStaffNotices(StaffNoticeDependencies{DB: db}),
		Calendar:   s,
		Attendance: s,
		Groups: func(context.Context) ([]timetable.WeekProtocolGroup, error) {
			return s.groups, nil
		},
		SchoolMode: func(context.Context) (bool, error) { return s.openCare, nil },
	}
}

func missingChild(id int64, name, reason, label string) timetable.WeekProtocolMissingChild {
	return timetable.WeekProtocolMissingChild{StudentID: id, Name: name, Reason: reason, Label: label}
}

var weekProtocolLeadership = timetable.WeekProtocolReach{Leadership: true, Staff: true}

func runWeekProtocolEvening(t *testing.T, module timetable.Owner, ctx context.Context, now time.Time, sources timetable.WeekProtocolSources) timetable.WeekProtocolEveningResult {
	t.Helper()
	result, err := module.RunWeekProtocolEvening(ctx, now, sources)
	require.NoError(t, err)
	return result
}

func weekProtocolOfGroup(t *testing.T, module timetable.Owner, ctx context.Context, groupID int64) timetable.WeekProtocol {
	t.Helper()
	listed, err := module.ListWeekProtocols(ctx, weekProtocolLeadership, timetable.WeekProtocolFilter{GroupID: &groupID})
	require.NoError(t, err)
	require.Len(t, listed, 1)
	protocol, err := module.OpenWeekProtocol(ctx, weekProtocolLeadership, listed[0].ID)
	require.NoError(t, err)
	return protocol
}

func weekProtocolDay(protocol timetable.WeekProtocol, day calendar.Date) *timetable.WeekProtocolDay {
	for index := range protocol.Days {
		if protocol.Days[index].Date == day {
			return &protocol.Days[index]
		}
	}
	return nil
}

// Week boundaries: the evenings record their day into the protocol of their
// Monday-to-Sunday week, the week becomes a draft on Friday evening and is
// closed automatically once the following week is over.
func TestWeekProtocolFollowsTheWeek(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, ctx := buildModule(t, db), testpkg.Ctx(t)
	sources := &weekProtocolSources{groups: []timetable.WeekProtocolGroup{{ID: 9001, Name: "Bären"}}}
	sources.days = map[calendar.Date][]timetable.WeekProtocolGroupDay{
		weekProtocolMonday: {{GroupID: 9001, Present: 10, Sick: 1, Missing: []timetable.WeekProtocolMissingChild{missingChild(1, "Mia Kurz", "sick", "Krank")}}},
	}

	before := runWeekProtocolEvening(t, module, ctx, weekProtocolMonday.BerlinMidnight().Add(17*time.Hour), sources.of(db))
	assert.Zero(t, before.DaysCaptured, "before the evening hour nothing is recorded")
	assert.Equal(t, 1, runWeekProtocolEvening(t, module, ctx, weekProtocolEvening(weekProtocolMonday), sources.of(db)).DaysCaptured)
	protocol := weekProtocolOfGroup(t, module, ctx, 9001)
	assert.Equal(t, weekProtocolMonday, protocol.WeekStart)
	assert.Equal(t, weekProtocolMonday.AddDays(6), protocol.WeekEnd)
	assert.Equal(t, timetable.WeekProtocolStatusCollecting, protocol.Status)
	monday := weekProtocolDay(protocol, weekProtocolMonday)
	require.NotNil(t, monday)
	require.NotNil(t, monday.Facts)
	assert.Equal(t, 10, monday.Facts.Present)
	assert.Equal(t, []timetable.WeekProtocolMissingChild{missingChild(1, "Mia Kurz", "sick", "Krank")}, monday.Facts.Missing)

	_, err := module.SaveWeekProtocolNotes(ctx, weekProtocolLeadership, protocol.ID, timetable.WeekProtocolNotes{WeekNote: "zu früh"})
	require.ErrorIs(t, err, timetable.ErrWeekProtocolNotDraft, "a collecting protocol takes no notes")

	friday := runWeekProtocolEvening(t, module, ctx, weekProtocolEvening(weekProtocolFriday), sources.of(db))
	assert.Equal(t, 1, friday.Drafted)
	assert.Equal(t, timetable.WeekProtocolStatusDraft, weekProtocolOfGroup(t, module, ctx, 9001).Status)

	// The following Sunday keeps the draft; the Monday after it closes it.
	followingSunday := weekProtocolMonday.AddDays(13)
	assert.Zero(t, runWeekProtocolEvening(t, module, ctx, weekProtocolEvening(followingSunday), sources.of(db)).AutoClosed)
	closing := runWeekProtocolEvening(t, module, ctx, followingSunday.AddDays(1).BerlinMidnight().Add(time.Hour), sources.of(db))
	assert.Equal(t, 1, closing.AutoClosed)
	closed := weekProtocolOfGroup(t, module, ctx, 9001)
	assert.Equal(t, timetable.WeekProtocolStatusClosed, closed.Status)
	assert.True(t, closed.ClosedAutomatically)
}

// A repeated tick on the same evening records nothing twice, and a later
// change of the day log does not rewrite a recorded day.
func TestWeekProtocolRecordsADayOnce(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, ctx := buildModule(t, db), testpkg.Ctx(t)
	sources := &weekProtocolSources{groups: []timetable.WeekProtocolGroup{{ID: 9002, Name: "Füchse"}}}
	sources.days = map[calendar.Date][]timetable.WeekProtocolGroupDay{weekProtocolMonday: {{GroupID: 9002, Present: 4}}}

	assert.Equal(t, 1, runWeekProtocolEvening(t, module, ctx, weekProtocolEvening(weekProtocolMonday), sources.of(db)).DaysCaptured)
	sources.days[weekProtocolMonday] = []timetable.WeekProtocolGroupDay{{GroupID: 9002, Present: 99}}
	assert.Zero(t, runWeekProtocolEvening(t, module, ctx, weekProtocolEvening(weekProtocolMonday).Add(15*time.Minute), sources.of(db)).DaysCaptured)
	assert.Equal(t, 4, weekProtocolDay(weekProtocolOfGroup(t, module, ctx, 9002), weekProtocolMonday).Facts.Present)
}

// A group change during the week: each evening records the children in the
// group they belong to that day, so the earlier days keep the old group.
func TestWeekProtocolKeepsTheGroupOfEachDay(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, ctx := buildModule(t, db), testpkg.Ctx(t)
	sources := &weekProtocolSources{groups: []timetable.WeekProtocolGroup{{ID: 9003, Name: "Alt"}, {ID: 9004, Name: "Neu"}}}
	wednesday := weekProtocolMonday.AddDays(2)
	sources.days = map[calendar.Date][]timetable.WeekProtocolGroupDay{
		weekProtocolMonday: {
			{GroupID: 9003, StudentIDs: []int64{7}, Absent: 1, Missing: []timetable.WeekProtocolMissingChild{missingChild(7, "Ole Wechsel", "absent", "Fehlt")}},
			{GroupID: 9004},
		},
		wednesday: {
			{GroupID: 9003},
			{GroupID: 9004, StudentIDs: []int64{7}, Excused: 1, Missing: []timetable.WeekProtocolMissingChild{missingChild(7, "Ole Wechsel", "excused", "Entschuldigt")}},
		},
	}
	runWeekProtocolEvening(t, module, ctx, weekProtocolEvening(weekProtocolMonday), sources.of(db))
	runWeekProtocolEvening(t, module, ctx, weekProtocolEvening(wednesday), sources.of(db))

	old, moved := weekProtocolOfGroup(t, module, ctx, 9003), weekProtocolOfGroup(t, module, ctx, 9004)
	assert.Len(t, weekProtocolDay(old, weekProtocolMonday).Facts.Missing, 1, "Monday stays with the old group")
	assert.Empty(t, weekProtocolDay(old, wednesday).Facts.Missing)
	assert.Empty(t, weekProtocolDay(moved, weekProtocolMonday).Facts.Missing)
	assert.Equal(t, "Entschuldigt", weekProtocolDay(moved, wednesday).Facts.Missing[0].Label, "Wednesday is recorded in the new group")
}

// Holidays and closing days: nobody counts as missing; the day says what it
// was. A holiday wins over a closing day.
func TestWeekProtocolMarksHolidaysAndClosingDays(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, ctx := buildModule(t, db), testpkg.Ctx(t)
	tuesday, thursday := weekProtocolMonday.AddDays(1), weekProtocolMonday.AddDays(3)
	sources := &weekProtocolSources{
		groups:   []timetable.WeekProtocolGroup{{ID: 9005, Name: "Igel"}},
		holidays: map[string]bool{tuesday.String(): true},
		closing:  map[string]bool{tuesday.String(): true, thursday.String(): true},
	}
	absent := []timetable.WeekProtocolGroupDay{{GroupID: 9005, Absent: 12, Missing: []timetable.WeekProtocolMissingChild{missingChild(1, "A B", "absent", "Fehlt")}}}
	sources.days = map[calendar.Date][]timetable.WeekProtocolGroupDay{tuesday: absent, thursday: absent}
	runWeekProtocolEvening(t, module, ctx, weekProtocolEvening(tuesday), sources.of(db))
	runWeekProtocolEvening(t, module, ctx, weekProtocolEvening(thursday), sources.of(db))

	protocol := weekProtocolOfGroup(t, module, ctx, 9005)
	holiday, closing := weekProtocolDay(protocol, tuesday).Facts, weekProtocolDay(protocol, thursday).Facts
	assert.Equal(t, timetable.WeekProtocolDayHoliday, holiday.Kind)
	assert.Equal(t, timetable.WeekProtocolDayClosing, closing.Kind)
	for _, facts := range []*timetable.WeekProtocolDayFacts{holiday, closing} {
		assert.Zero(t, facts.Absent)
		assert.Empty(t, facts.Missing)
	}
}

// A school without fixed groups keeps one protocol for the whole school with
// every block of the day, a cancelled one with its reason.
func TestWeekProtocolOfASchoolWithoutGroups(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, ctx := buildModule(t, db), testpkg.Ctx(t)
	fixture := newOwnedActivityInstanceFixture(t, db, "week-protocol")
	block := createOwnedActivityInstance(t, module, ctx, fixture, weekProtocolMonday.String(), "14:00:00", "Fußball")
	reason := "Regen"
	block.Status, block.CancelReason = timetable.InstanceStatusCancelled, &reason
	_, err := module.PatchActivityInstance(ctx, block.ID, timetable.ActivityInstanceInput{Status: block.Status, CancelReason: &reason}, []string{"status", "cancel_reason"})
	require.NoError(t, err)
	sources := &weekProtocolSources{openCare: true, groups: []timetable.WeekProtocolGroup{{ID: 9006, Name: "A"}, {ID: 9007, Name: "B"}}}
	sources.days = map[calendar.Date][]timetable.WeekProtocolGroupDay{weekProtocolMonday: {{GroupID: 9006, Present: 3}, {GroupID: 9007, Present: 4}}}
	runWeekProtocolEvening(t, module, ctx, weekProtocolEvening(weekProtocolMonday), sources.of(db))

	listed, err := module.ListWeekProtocols(ctx, timetable.WeekProtocolReach{Staff: true}, timetable.WeekProtocolFilter{})
	require.NoError(t, err)
	require.Len(t, listed, 1, "a member of staff sees the school's protocol")
	assert.Equal(t, timetable.WeekProtocolScopeSchool, listed[0].Scope)
	protocol, err := module.OpenWeekProtocol(ctx, weekProtocolLeadership, listed[0].ID)
	require.NoError(t, err)
	monday := weekProtocolDay(protocol, weekProtocolMonday).Facts
	assert.Equal(t, 7, monday.Present, "the groups add up")
	require.Len(t, monday.Blocks, 1)
	assert.Equal(t, timetable.WeekProtocolBlock{Title: "Fußball", StartTime: "14:00", EndTime: "15:00", Status: timetable.InstanceStatusCancelled, CancelReason: "Regen"}, monday.Blocks[0])
}

// Notes and closing: only a draft takes notes and is closed, the reach
// decides who sees the protocol, and another school sees none of it.
func TestWeekProtocolNotesReachAndTenantBoundary(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, ctx := buildModule(t, db), testpkg.Ctx(t)
	sources := &weekProtocolSources{groups: []timetable.WeekProtocolGroup{{ID: 9008, Name: "Eulen"}}}
	sources.days = map[calendar.Date][]timetable.WeekProtocolGroupDay{weekProtocolFriday: {{GroupID: 9008, Present: 5}}}
	runWeekProtocolEvening(t, module, ctx, weekProtocolEvening(weekProtocolFriday), sources.of(db))
	draft := weekProtocolOfGroup(t, module, ctx, 9008)
	require.Equal(t, timetable.WeekProtocolStatusDraft, draft.Status)

	stranger := timetable.WeekProtocolReach{Staff: true, GroupIDs: []int64{1}}
	_, err := module.OpenWeekProtocol(ctx, stranger, draft.ID)
	require.ErrorIs(t, err, timetable.ErrWeekProtocolNotFound, "staff of another group do not see it")
	member := timetable.WeekProtocolReach{Staff: true, GroupIDs: []int64{9008}}
	_, err = module.SaveWeekProtocolNotes(ctx, member, draft.ID, timetable.WeekProtocolNotes{Days: map[calendar.Date]string{weekProtocolMonday.AddDays(7): "x"}})
	require.ErrorIs(t, err, timetable.ErrWeekProtocolInvalid, "a day of another week is refused")

	saved, err := module.SaveWeekProtocolNotes(ctx, member, draft.ID, timetable.WeekProtocolNotes{
		WeekNote: "  Ruhige Woche  ", Days: map[calendar.Date]string{weekProtocolFriday: "Ausflug", weekProtocolMonday: "Elternabend"},
	})
	require.NoError(t, err)
	assert.Equal(t, "Ruhige Woche", saved.WeekNote)
	assert.Equal(t, "Elternabend", weekProtocolDay(saved, weekProtocolMonday).Note)
	assert.Nil(t, weekProtocolDay(saved, weekProtocolMonday).Facts, "a noted day without its evening stays unrecorded")

	foreignTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenantID)
	foreignCtx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), foreignTenantID)
	_, err = module.OpenWeekProtocol(foreignCtx, weekProtocolLeadership, draft.ID)
	require.ErrorIs(t, err, timetable.ErrWeekProtocolNotFound, "another school sees nothing")
	foreignList, err := module.ListWeekProtocols(foreignCtx, weekProtocolLeadership, timetable.WeekProtocolFilter{})
	require.NoError(t, err)
	assert.Empty(t, foreignList)

	closed, err := module.CloseWeekProtocol(ctx, member, draft.ID, timetable.WeekProtocolActor{AccountID: 1, Name: "Lea Leitung"})
	require.NoError(t, err)
	assert.Equal(t, timetable.WeekProtocolStatusClosed, closed.Status)
	assert.Equal(t, "Lea Leitung", closed.ClosedByName)
	_, err = module.CloseWeekProtocol(ctx, member, draft.ID, timetable.WeekProtocolActor{AccountID: 1, Name: "Lea Leitung"})
	require.ErrorIs(t, err, timetable.ErrWeekProtocolNotDraft)
	saturday := weekProtocolFriday.AddDays(1)
	sources.days[saturday] = []timetable.WeekProtocolGroupDay{{GroupID: 9008, Present: 2}}
	assert.Zero(t, runWeekProtocolEvening(t, module, ctx, weekProtocolEvening(saturday), sources.of(db)).DaysCaptured, "a closed protocol records no further day")
}

// Retention: a protocol goes with its days once its Sunday lies before the
// attendance window. The week of 2020-01-06 lies years back, so a window of
// one day deletes it and a window far beyond it keeps it.
func TestWeekProtocolRetentionCountsFromTheSunday(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, ctx := buildModule(t, db), testpkg.Ctx(t)
	sources := &weekProtocolSources{groups: []timetable.WeekProtocolGroup{{ID: 9009, Name: "Wale"}}}
	runWeekProtocolEvening(t, module, ctx, weekProtocolEvening(calendar.NewDate(2020, 1, 6)), sources.of(db))
	_, err := module.DeleteExpiredWeekProtocols(ctx, 0)
	require.ErrorIs(t, err, timetable.ErrWeekProtocolRetentionWindow)

	deleted, err := module.DeleteExpiredWeekProtocols(ctx, 100000)
	require.NoError(t, err)
	assert.Zero(t, deleted, "a window that still reaches the Sunday keeps the protocol")
	deleted, err = module.DeleteExpiredWeekProtocols(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, 1, deleted)
	listed, err := module.ListWeekProtocols(ctx, weekProtocolLeadership, timetable.WeekProtocolFilter{GroupID: &sources.groups[0].ID})
	require.NoError(t, err)
	assert.Empty(t, listed)
}
