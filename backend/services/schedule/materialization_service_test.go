package schedule

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Hermetic-exempt: these are pure-logic tests — no DB, no fixtures. IDs that
// appear as literals here (e.g. `int64(1)` for a period) are test doubles
// rather than database row references. The hermetic linter only flags
// `int64(1)`-style literals, so we use `100` / `200` etc. to stay well clear
// of the low-integer pattern.

// -----------------------------------------------------------------------------
// TestResolveWindow — the next-Monday / following-Sunday selection rule.
// -----------------------------------------------------------------------------

func TestResolveWindow(t *testing.T) {
	t.Parallel()

	mustDate := func(y int, m time.Month, d int) timezone.Date {
		return timezone.NewDate(y, m, d)
	}

	cases := []struct {
		name         string
		baseDate     timezone.Date
		weeksAhead   int
		expectedFrom timezone.Date
		expectedTo   timezone.Date
	}{
		{
			name:         "Monday → skips to following Monday",
			baseDate:     mustDate(2026, time.April, 20), // Mon
			weeksAhead:   1,
			expectedFrom: mustDate(2026, time.April, 27), // next Mon
			expectedTo:   mustDate(2026, time.May, 3),    // Sun
		},
		{
			name:         "Wednesday → next Monday",
			baseDate:     mustDate(2026, time.April, 22), // Wed
			weeksAhead:   1,
			expectedFrom: mustDate(2026, time.April, 27),
			expectedTo:   mustDate(2026, time.May, 3),
		},
		{
			name:         "Saturday → next Monday",
			baseDate:     mustDate(2026, time.April, 25), // Sat
			weeksAhead:   1,
			expectedFrom: mustDate(2026, time.April, 27),
			expectedTo:   mustDate(2026, time.May, 3),
		},
		{
			name:         "Sunday → next Monday (+1 day)",
			baseDate:     mustDate(2026, time.April, 26), // Sun
			weeksAhead:   1,
			expectedFrom: mustDate(2026, time.April, 27),
			expectedTo:   mustDate(2026, time.May, 3),
		},
		{
			name:         "weeksAhead=4 produces a 28-day window",
			baseDate:     mustDate(2026, time.April, 22), // Wed
			weeksAhead:   4,
			expectedFrom: mustDate(2026, time.April, 27),
			expectedTo:   mustDate(2026, time.May, 24),
		},
		{
			name:         "weeksAhead=0 is clamped to 1",
			baseDate:     mustDate(2026, time.April, 22),
			weeksAhead:   0,
			expectedFrom: mustDate(2026, time.April, 27),
			expectedTo:   mustDate(2026, time.May, 3),
		},
		{
			name:         "weeksAhead=99 is clamped to 8",
			baseDate:     mustDate(2026, time.April, 22),
			weeksAhead:   99,
			expectedFrom: mustDate(2026, time.April, 27),
			expectedTo:   mustDate(2026, time.June, 21), // 27 Apr + 56 - 1 days
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			from, to := resolveWindow(tc.baseDate, tc.weeksAhead)
			assert.Equal(t, tc.expectedFrom, from, "from")
			assert.Equal(t, tc.expectedTo, to, "to")
		})
	}
}

// -----------------------------------------------------------------------------
// TestIsEnrollmentValidOn — validity predicate including calendar_period scope.
// -----------------------------------------------------------------------------

func TestIsEnrollmentValidOn(t *testing.T) {
	t.Parallel()

	d := func(y int, m time.Month, day int) activities.Date {
		return activities.Date(timezone.NewDate(y, m, day))
	}
	p100 := int64(100)
	p200 := int64(200)
	validUntilApr20 := d(2026, time.April, 20)
	validUntilApr21 := d(2026, time.April, 21)

	target := d(2026, time.April, 20)
	targetDay := timezone.NewDate(2026, 4, 20)

	cases := []struct {
		name   string
		e      *activities.StudentEnrollment
		period int64
		want   bool
	}{
		{
			name:   "nil enrollment → invalid",
			e:      nil,
			period: p100,
			want:   false,
		},
		{
			name: "unbounded, no period scope → valid",
			e: &activities.StudentEnrollment{
				ValidFrom: d(2026, time.January, 1),
			},
			period: p100,
			want:   true,
		},
		{
			name: "valid_from equals date (inclusive start) → valid",
			e: &activities.StudentEnrollment{
				ValidFrom: target,
			},
			period: p100,
			want:   true,
		},
		{
			name: "valid_from after date → invalid",
			e: &activities.StudentEnrollment{
				ValidFrom: d(2026, time.April, 21),
			},
			period: p100,
			want:   false,
		},
		{
			name: "valid_until equals date (exclusive end) → invalid",
			e: &activities.StudentEnrollment{
				ValidFrom:  d(2026, time.January, 1),
				ValidUntil: &validUntilApr20,
			},
			period: p100,
			want:   false,
		},
		{
			name: "valid_until one day after date → valid",
			e: &activities.StudentEnrollment{
				ValidFrom:  d(2026, time.January, 1),
				ValidUntil: &validUntilApr21,
			},
			period: p100,
			want:   true,
		},
		{
			name: "calendar_period_id nil matches any period",
			e: &activities.StudentEnrollment{
				ValidFrom: d(2026, time.January, 1),
			},
			period: p200,
			want:   true,
		},
		{
			name: "calendar_period_id mismatches selected period → invalid",
			e: &activities.StudentEnrollment{
				ValidFrom:        d(2026, time.January, 1),
				CalendarPeriodID: &p100,
			},
			period: p200,
			want:   false,
		},
		{
			name: "calendar_period_id matches selected period → valid",
			e: &activities.StudentEnrollment{
				ValidFrom:        d(2026, time.January, 1),
				CalendarPeriodID: &p100,
			},
			period: p100,
			want:   true,
		},
		{
			name: "selected weekdays empty matches any weekday",
			e: &activities.StudentEnrollment{
				ValidFrom:        d(2026, time.January, 1),
				SelectedWeekdays: nil,
			},
			period: p100,
			want:   true,
		},
		{
			name: "selected weekdays includes target weekday",
			e: &activities.StudentEnrollment{
				ValidFrom:        d(2026, time.January, 1),
				SelectedWeekdays: []int{1, 3},
			},
			period: p100,
			want:   true,
		},
		{
			name: "selected weekdays excludes target weekday",
			e: &activities.StudentEnrollment{
				ValidFrom:        d(2026, time.January, 1),
				SelectedWeekdays: []int{2, 3},
			},
			period: p100,
			want:   false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isEnrollmentValidOn(tc.e, targetDay, tc.period)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestEnrollmentStudentIsAlumnus_UnloadedStudent(t *testing.T) {
	t.Parallel()

	enrollment := &activities.StudentEnrollment{}
	assert.False(t, enrollmentStudentIsAlumnus(enrollment))
}

func TestExpectedStudentIDsOn_AppliesSharedRosterRules(t *testing.T) {
	t.Parallel()

	date := timezone.NewDate(2026, time.April, 20)
	periodID := int64(400)
	otherPeriodID := int64(401)
	endedBefore := date.AddDays(-1)
	enrollments := []*activities.StudentEnrollment{
		{StudentID: 501, ValidFrom: activities.Date(date.AddDays(-1)), CalendarPeriodID: &periodID},
		{StudentID: 502, ValidFrom: activities.Date(date.AddDays(1))},
		{StudentID: 503, ValidFrom: activities.Date(date.AddDays(-1)), CalendarPeriodID: &otherPeriodID},
		{StudentID: 504, ValidFrom: activities.Date(date.AddDays(-1)), SelectedWeekdays: []int{2}},
		{StudentID: 505, ValidFrom: activities.Date(date.AddDays(-1)), StudentAlumnus: true},
		{StudentID: 506, ValidFrom: activities.Date(date.AddDays(-1))},
	}
	targetStudentIDs := []int64{501, 507, 508, 507}
	careBounds := map[int64]timezone.Date{506: endedBefore, 507: date, 508: endedBefore}

	assert.Equal(t, []int64{501, 507}, expectedStudentIDsOn(
		enrollments, targetStudentIDs, careBounds, date, periodID,
	))
}

// -----------------------------------------------------------------------------
// TestIsSupervisorValidOn — same shape as enrollments (ensures the two
// predicates stay symmetric by design).
// -----------------------------------------------------------------------------

func TestIsSupervisorValidOn(t *testing.T) {
	t.Parallel()

	d := func(y int, m time.Month, day int) activities.Date {
		return activities.Date(timezone.NewDate(y, m, day))
	}
	target := d(2026, time.April, 20)
	targetDay := timezone.NewDate(2026, 4, 20)
	p100 := int64(100)
	p200 := int64(200)
	until := d(2026, time.April, 20) // exclusive

	assert.True(t, isSupervisorValidOn(&activities.SupervisorPlanned{ValidFrom: target}, targetDay, p100))
	assert.False(t, isSupervisorValidOn(&activities.SupervisorPlanned{ValidFrom: d(2026, time.April, 21)}, targetDay, p100))
	assert.False(t, isSupervisorValidOn(&activities.SupervisorPlanned{ValidFrom: d(2026, time.January, 1), ValidUntil: &until}, targetDay, p100))
	assert.False(t, isSupervisorValidOn(&activities.SupervisorPlanned{ValidFrom: d(2026, time.January, 1), CalendarPeriodID: &p100}, targetDay, p200))
	assert.True(t, isSupervisorValidOn(&activities.SupervisorPlanned{ValidFrom: d(2026, time.January, 1), CalendarPeriodID: &p100}, targetDay, p100))
	assert.False(t, isSupervisorValidOn(nil, targetDay, p100), "nil must be invalid")
}

func TestEffectivePrimarySupervisorPrefersTheMostSpecificScope(t *testing.T) {
	t.Parallel()

	monday := timezone.NewDate(2026, time.April, 20)
	tuesday := monday.AddDays(1)
	periodID := int64(100)
	mondayWeekday := activities.WeekdayMonday

	supervisors := []*activities.SupervisorPlanned{
		{Model: activities.Model{ID: 1}, StaffID: 10, IsPrimary: true},
		{
			Model:            activities.Model{ID: 2},
			StaffID:          20,
			IsPrimary:        true,
			CalendarPeriodID: &periodID,
			Weekday:          &mondayWeekday,
		},
	}

	mondayPrimary, ok := effectivePrimarySupervisor(supervisors, monday, periodID)
	require.True(t, ok)
	assert.Equal(t, int64(20), mondayPrimary, "the exact period and weekday override must win")

	tuesdayPrimary, ok := effectivePrimarySupervisor(supervisors, tuesday, periodID)
	require.True(t, ok)
	assert.Equal(t, int64(10), tuesdayPrimary, "the shared legacy primary remains the fallback")
}

// -----------------------------------------------------------------------------
// TestApplyException — cancellation skip + partial modify overrides.
// -----------------------------------------------------------------------------

func TestApplyException(t *testing.T) {
	t.Parallel()

	startBase := time.Date(1, 1, 1, 14, 0, 0, 0, time.UTC)
	endBase := time.Date(1, 1, 1, 15, 0, 0, 0, time.UTC)
	roomBase := int64(500)
	base := materialParams{StartTime: startBase, EndTime: endBase, RoomID: roomBase}

	t.Run("nil exception is no-op", func(t *testing.T) {
		got, skip := applyException(base, nil)
		assert.False(t, skip)
		assert.Equal(t, base, got)
	})

	t.Run("cancelled exception → skip", func(t *testing.T) {
		_, skip := applyException(base, &schedule.ActivityException{
			ExceptionType: schedule.ActivityExceptionCancelled,
		})
		assert.True(t, skip)
	})

	t.Run("modified exception overrides only the fields it specifies", func(t *testing.T) {
		// Use a year-0 input to prove the helper normalises to year 1 UTC,
		// which is what bun wants for Postgres TIME columns.
		newStart := time.Date(0, 1, 1, 13, 30, 0, 0, time.UTC)
		expectedNewStart := time.Date(1, 1, 1, 13, 30, 0, 0, time.UTC)
		newRoom := int64(600)
		got, skip := applyException(base, &schedule.ActivityException{
			ExceptionType: schedule.ActivityExceptionModified,
			StartTime:     &newStart,
			RoomID:        &newRoom,
		})
		assert.False(t, skip)
		assert.Equal(t, expectedNewStart, got.StartTime)
		assert.Equal(t, endBase, got.EndTime, "unspecified end_time preserved")
		assert.Equal(t, newRoom, got.RoomID)
	})

	t.Run("modified exception overriding only end_time", func(t *testing.T) {
		newEnd := time.Date(0, 1, 1, 15, 30, 0, 0, time.UTC)
		expectedNewEnd := time.Date(1, 1, 1, 15, 30, 0, 0, time.UTC)
		got, skip := applyException(base, &schedule.ActivityException{
			ExceptionType: schedule.ActivityExceptionModified,
			EndTime:       &newEnd,
		})
		assert.False(t, skip)
		assert.Equal(t, startBase, got.StartTime)
		assert.Equal(t, expectedNewEnd, got.EndTime)
		assert.Equal(t, roomBase, got.RoomID)
	})
}

// -----------------------------------------------------------------------------
// TestExtractTimeOfDay — SQL TIME normalisation.
// -----------------------------------------------------------------------------

func TestExtractTimeOfDay(t *testing.T) {
	t.Parallel()

	berlin, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)

	t.Run("CEST 08:00 stays 08:00 in UTC year 1", func(t *testing.T) {
		src := time.Date(2026, 4, 20, 8, 0, 0, 0, berlin) // CEST (UTC+2)
		got := extractTimeOfDay(src)
		assert.Equal(t, 1, got.Year())
		assert.Equal(t, time.January, got.Month())
		assert.Equal(t, 1, got.Day())
		assert.Equal(t, 8, got.Hour())
		assert.Equal(t, 0, got.Minute())
		assert.Equal(t, time.UTC, got.Location())
	})

	t.Run("UTC 15:00 stays 15:00", func(t *testing.T) {
		src := time.Date(2026, 10, 1, 15, 30, 45, 0, time.UTC)
		got := extractTimeOfDay(src)
		assert.Equal(t, 15, got.Hour())
		assert.Equal(t, 30, got.Minute())
		assert.Equal(t, 45, got.Second())
	})
}

// -----------------------------------------------------------------------------
// TestPeriodSelection — the WP-B8 period-selection rule.
// -----------------------------------------------------------------------------

func TestPeriodSelection(t *testing.T) {
	t.Parallel()

	d := func(y int, m time.Month, day int) timezone.Date {
		return timezone.NewDate(y, m, day)
	}

	mkPeriod := func(id int64, start, end timezone.Date) *schedule.CalendarPeriod {
		p := &schedule.CalendarPeriod{StartDate: start, EndDate: end, IsActive: true}
		p.ID = id
		return p
	}

	schoolYear := mkPeriod(400, d(2026, time.August, 1), d(2027, time.July, 31))
	holiday := mkPeriod(500, d(2026, time.October, 14), d(2026, time.October, 25))
	fallSemester := mkPeriod(300, d(2026, time.August, 1), d(2027, time.January, 31))

	target := d(2026, time.October, 20)
	logger := slog.Default()

	t.Run("template schedule pinned to period within range → uses it", func(t *testing.T) {
		pinnedID := holiday.ID
		sch := &activities.Schedule{CalendarPeriodID: &pinnedID}
		got := selectPeriod(&activities.Group{}, sch, target, []*schedule.CalendarPeriod{schoolYear, holiday}, logger)
		require.NotNil(t, got)
		assert.Equal(t, holiday.ID, got.ID)
	})

	t.Run("schedule pinned to period outside date range → nil (SkippedNoPeriod)", func(t *testing.T) {
		pinnedID := holiday.ID
		sch := &activities.Schedule{CalendarPeriodID: &pinnedID}
		outsideDate := d(2026, time.September, 1) // before holiday start
		got := selectPeriod(&activities.Group{}, sch, outsideDate, []*schedule.CalendarPeriod{schoolYear, holiday}, logger)
		assert.Nil(t, got)
	})

	t.Run("schedule pinned to unknown period → nil", func(t *testing.T) {
		unknown := int64(999999)
		sch := &activities.Schedule{CalendarPeriodID: &unknown}
		got := selectPeriod(&activities.Group{}, sch, target, []*schedule.CalendarPeriod{schoolYear, holiday}, logger)
		assert.Nil(t, got)
	})

	t.Run("unbound with single active period containing date", func(t *testing.T) {
		sch := &activities.Schedule{}
		got := selectPeriod(&activities.Group{}, sch, target, []*schedule.CalendarPeriod{schoolYear}, logger)
		require.NotNil(t, got)
		assert.Equal(t, schoolYear.ID, got.ID)
	})

	t.Run("unbound with multiple active periods → picks lowest-ID", func(t *testing.T) {
		sch := &activities.Schedule{}
		// On 2026-10-20, all three periods contain the date. Lowest ID is 300
		// (fallSemester).
		got := selectPeriod(&activities.Group{}, sch, target,
			[]*schedule.CalendarPeriod{schoolYear, holiday, fallSemester}, logger)
		require.NotNil(t, got)
		assert.Equal(t, fallSemester.ID, got.ID)
	})

	t.Run("unbound with no active period containing date → nil", func(t *testing.T) {
		sch := &activities.Schedule{}
		futureDate := d(2028, time.January, 1)
		got := selectPeriod(&activities.Group{}, sch, futureDate,
			[]*schedule.CalendarPeriod{schoolYear, holiday, fallSemester}, logger)
		assert.Nil(t, got)
	})

	t.Run("nil schedule has no pinned period", func(t *testing.T) {
		assert.Nil(t, schedulePinnedPeriodID(&activities.Group{}, nil))
	})

	t.Run("template pinned via Group.CalendarPeriodID, no schedule pin → uses template's period", func(t *testing.T) {
		pinnedID := holiday.ID
		tmpl := &activities.Group{CalendarPeriodID: &pinnedID}
		sch := &activities.Schedule{} // no schedule-level pin
		got := selectPeriod(tmpl, sch, target, []*schedule.CalendarPeriod{schoolYear, holiday}, logger)
		require.NotNil(t, got)
		assert.Equal(t, holiday.ID, got.ID)
	})

	t.Run("schedule pin wins over template pin when both set", func(t *testing.T) {
		schedulePinnedID := fallSemester.ID
		templatePinnedID := holiday.ID
		tmpl := &activities.Group{CalendarPeriodID: &templatePinnedID}
		sch := &activities.Schedule{CalendarPeriodID: &schedulePinnedID}
		got := selectPeriod(tmpl, sch, target, []*schedule.CalendarPeriod{schoolYear, holiday, fallSemester}, logger)
		require.NotNil(t, got)
		assert.Equal(t, fallSemester.ID, got.ID, "schedule's own pin must win over the template's")
	})
}

// -----------------------------------------------------------------------------
// TestIsoWeekday — two-line helper, still worth pinning because it underpins
// the schedule.weekday comparison. (The former civilDate helper is gone: the
// instant→calendar-day conversion now lives in timezone.DateFromTime.)
// -----------------------------------------------------------------------------

func TestIsoWeekday(t *testing.T) {
	t.Parallel()

	mon := timezone.NewDate(2026, 4, 20)
	assert.Equal(t, 1, isoWeekday(mon))
	assert.Equal(t, 2, isoWeekday(mon.AddDays(1)))
	assert.Equal(t, 3, isoWeekday(mon.AddDays(2)))
	assert.Equal(t, 4, isoWeekday(mon.AddDays(3)))
	assert.Equal(t, 5, isoWeekday(mon.AddDays(4)))
	assert.Equal(t, 6, isoWeekday(mon.AddDays(5)))
	assert.Equal(t, 7, isoWeekday(mon.AddDays(6)))
}

type materializationFakeGroupRepo struct {
	activities.GroupRepository
	templates []*activities.Group
	err       error
}

func (r materializationFakeGroupRepo) FindAllTemplates(context.Context) ([]*activities.Group, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.templates, nil
}

type materializationFakeScheduleRepo struct {
	activities.ScheduleRepository
	schedules []*activities.Schedule
	err       error
}

func (r materializationFakeScheduleRepo) FindByGroupID(context.Context, int64) ([]*activities.Schedule, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.schedules, nil
}

type materializationFakeEnrollmentRepo struct {
	activities.StudentEnrollmentRepository
	err error
}

func (r materializationFakeEnrollmentRepo) FindByGroupID(context.Context, int64) ([]*activities.StudentEnrollment, error) {
	if r.err != nil {
		return nil, r.err
	}
	return nil, nil
}

type materializationFakeSupervisorRepo struct {
	activities.SupervisorPlannedRepository
	err error
}

func (r materializationFakeSupervisorRepo) FindByGroupID(context.Context, int64) ([]*activities.SupervisorPlanned, error) {
	if r.err != nil {
		return nil, r.err
	}
	return nil, nil
}

type materializationFakePeriodRepo struct {
	schedule.CalendarPeriodRepository
	periods []*schedule.CalendarPeriod
	err     error
}

func (r materializationFakePeriodRepo) FindActiveByTenantID(context.Context) ([]*schedule.CalendarPeriod, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.periods, nil
}

type materializationFakeInstanceRepo struct {
	schedule.ActivityInstanceRepository
	inserted bool
	err      error
	findErr  error
}

func (r materializationFakeInstanceRepo) FindByTenantAndDateRange(context.Context, timezone.Date, timezone.Date) ([]*schedule.ActivityInstance, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	return nil, nil
}

func (r materializationFakeInstanceRepo) CreateTemplateBackedIfAbsent(context.Context, *schedule.ActivityInstance) (bool, error) {
	return r.inserted, r.err
}

type materializationFakeStaffRepo struct {
	schedule.InstanceStaffRepository
}

func (r materializationFakeStaffRepo) Create(context.Context, *schedule.InstanceStaff) error {
	panic("staff copy must not run when instance insert loses the race")
}

type materializationFakeStudentRepo struct {
	schedule.InstanceStudentRepository
}

func (r materializationFakeStudentRepo) Create(context.Context, *schedule.InstanceStudent) error {
	panic("student copy must not run when instance insert loses the race")
}

type materializationFakeExceptionRepo struct {
	schedule.ActivityExceptionRepository
	err error
}

func (r materializationFakeExceptionRepo) FindByDateRange(context.Context, timezone.Date, timezone.Date) ([]*schedule.ActivityException, error) {
	if r.err != nil {
		return nil, r.err
	}
	return nil, nil
}

type materializationFakeTimeframeRepo struct {
	schedule.TimeframeRepository
	timeframes []*schedule.Timeframe
	err        error
}

func (r materializationFakeTimeframeRepo) List(context.Context, *modelBase.QueryOptions) ([]*schedule.Timeframe, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.timeframes, nil
}

type materializationAllowCalendarService struct{}

func (materializationAllowCalendarService) GetAllPeriods(context.Context) ([]*schedule.CalendarPeriod, error) {
	panic("unused")
}

func (materializationAllowCalendarService) GetActivePeriods(context.Context) ([]*schedule.CalendarPeriod, error) {
	panic("unused")
}

func (materializationAllowCalendarService) GetPeriodByID(context.Context, int64) (*schedule.CalendarPeriod, error) {
	panic("unused")
}

func (materializationAllowCalendarService) CreatePeriod(context.Context, *schedule.CalendarPeriod) error {
	panic("unused")
}

func (materializationAllowCalendarService) UpdatePeriod(context.Context, *schedule.CalendarPeriod) error {
	panic("unused")
}

func (materializationAllowCalendarService) DeletePeriod(context.Context, int64) error {
	panic("unused")
}

func (materializationAllowCalendarService) EnsureDefaultSchoolYear(context.Context) ([]*schedule.CalendarPeriod, bool, error) {
	panic("unused")
}

func (materializationAllowCalendarService) FindActiveOverlaps(context.Context, *schedule.CalendarPeriod) ([]*schedule.CalendarPeriod, error) {
	panic("unused")
}

func (materializationAllowCalendarService) GetUsageCounts(context.Context) (map[int64]schedule.CalendarPeriodUsage, error) {
	panic("unused")
}

func (materializationAllowCalendarService) ShouldMaterialize(int, timezone.Date, *schedule.CalendarPeriod) bool {
	return true
}

func TestMaterializeForTenant_DuplicateInsertRaceDoesNotCopyChildren(t *testing.T) {
	t.Parallel()

	svc, date := newMaterializationBranchService(materializationFakeInstanceRepo{inserted: false})

	result, err := svc.MaterializeForTenant(
		tenant.WithTenantID(context.Background(), 300),
		date,
		date,
		MaterializationSourceManual,
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Zero(t, result.InstancesCreated)
	assert.Equal(t, 1, result.CandidatesRaced)
	assert.Zero(t, result.InstanceStudentsCreated)
	assert.Zero(t, result.InstanceStaffCreated)
}

func TestMaterializeForTenant_TemplateInsertErrorBubbles(t *testing.T) {
	t.Parallel()

	svc, date := newMaterializationBranchService(materializationFakeInstanceRepo{
		inserted: false,
		err:      errors.New("database unavailable"),
	})

	result, err := svc.MaterializeForTenant(
		tenant.WithTenantID(context.Background(), 300),
		date,
		date,
		MaterializationSourceManual,
	)

	require.Error(t, err)
	require.NotNil(t, result)
	assert.Contains(t, err.Error(), "materialize template: create instance")
	assert.Contains(t, err.Error(), "tenant_id=300")
	assert.Contains(t, err.Error(), "template_id=500")
	assert.Contains(t, err.Error(), "schedule_id=800")
	assert.Contains(t, err.Error(), "date=2026-04-20")
	assert.Contains(t, err.Error(), "period_id=400")
	assert.Contains(t, err.Error(), "room_id=700")
	assert.Contains(t, err.Error(), "start_time=14:00:00")
	assert.Contains(t, err.Error(), "end_time=15:00:00")
	assert.Zero(t, result.InstancesCreated)
	assert.Zero(t, result.CandidatesRaced)
}

func TestMaterializeForTenant_SkipsLegacyWeekendSchedules(t *testing.T) {
	t.Parallel()

	svc, saturday := newMaterializationBranchServiceForSchedule(
		materializationFakeInstanceRepo{inserted: true},
		timezone.NewDate(2026, time.April, 25),
		activities.WeekdaySaturday,
	)

	result, err := svc.MaterializeForTenant(
		tenant.WithTenantID(context.Background(), 300),
		saturday,
		saturday,
		MaterializationSourceManual,
	)

	require.NoError(t, err)
	assert.Zero(t, result.InstancesCreated)
}

func TestMaterializeForTenant_PreconditionWarnings(t *testing.T) {
	t.Parallel()

	date := timezone.NewDate(2026, 4, 20)

	t.Run("warns and no-ops without active periods", func(t *testing.T) {
		svc := NewMaterializationService(
			materializationFakeGroupRepo{templates: []*activities.Group{{Name: "Lernzeit"}}},
			materializationFakeScheduleRepo{},
			materializationFakeEnrollmentRepo{},
			materializationFakeSupervisorRepo{},
			materializationFakePeriodRepo{},
			materializationFakeInstanceRepo{inserted: true},
			materializationFakeStaffRepo{},
			materializationFakeStudentRepo{},
			materializationFakeExceptionRepo{},
			materializationFakeTimeframeRepo{},
			materializationAllowCalendarService{},
			nil,
			nil,
			slog.Default(),
		)

		result, err := svc.MaterializeForTenant(context.Background(), date, date, MaterializationSourceManual)

		require.NoError(t, err)
		require.Len(t, result.Warnings, 1)
		assert.Equal(t, MaterializationWarningCodeNoActivePeriod, result.Warnings[0].Code)
		assert.Zero(t, result.InstancesCreated)
	})

	t.Run("warns and no-ops without templates", func(t *testing.T) {
		svc := NewMaterializationService(
			materializationFakeGroupRepo{},
			materializationFakeScheduleRepo{},
			materializationFakeEnrollmentRepo{},
			materializationFakeSupervisorRepo{},
			materializationFakePeriodRepo{periods: []*schedule.CalendarPeriod{{
				StartDate: date.AddDays(-30),
				EndDate:   date.AddDays(30),
				IsActive:  true,
				Model:     schedule.Model{ID: 401},
			}}},
			materializationFakeInstanceRepo{inserted: true},
			materializationFakeStaffRepo{},
			materializationFakeStudentRepo{},
			materializationFakeExceptionRepo{},
			materializationFakeTimeframeRepo{},
			materializationAllowCalendarService{},
			nil,
			nil,
			slog.Default(),
		)

		result, err := svc.MaterializeForTenant(context.Background(), date, date, MaterializationSourceManual)

		require.NoError(t, err)
		require.Len(t, result.Warnings, 1)
		assert.Equal(t, MaterializationWarningCodeNoTemplates, result.Warnings[0].Code)
		assert.Zero(t, result.InstancesCreated)
	})
}

func TestMaterializeForTenant_ErrorBranches(t *testing.T) {
	t.Parallel()

	date := timezone.NewDate(2026, 4, 20)
	period := &schedule.CalendarPeriod{
		StartDate: date.AddDays(-30),
		EndDate:   date.AddDays(30),
		IsActive:  true,
		Model:     schedule.Model{ID: 405},
	}
	start := time.Date(2024, time.January, 1, 14, 0, 0, 0, time.UTC)
	end := time.Date(2024, time.January, 1, 15, 0, 0, 0, time.UTC)
	tfID := int64(406)
	roomID := int64(407)
	template := &activities.Group{Name: "Lernzeit", PlannedRoomID: &roomID, Model: activities.Model{ID: 408}}
	scheduleRow := &activities.Schedule{Weekday: 1, TimeframeID: &tfID}
	timeframe := &schedule.Timeframe{StartTime: start, EndTime: &end, Model: schedule.Model{ID: tfID}}

	makeSvc := func(
		groupRepo materializationFakeGroupRepo,
		scheduleRepo materializationFakeScheduleRepo,
		enrollmentRepo materializationFakeEnrollmentRepo,
		supervisorRepo materializationFakeSupervisorRepo,
		periodRepo materializationFakePeriodRepo,
		instanceRepo materializationFakeInstanceRepo,
		exceptionRepo materializationFakeExceptionRepo,
		timeframeRepo materializationFakeTimeframeRepo,
	) MaterializationService {
		return NewMaterializationService(
			groupRepo,
			scheduleRepo,
			enrollmentRepo,
			supervisorRepo,
			periodRepo,
			instanceRepo,
			materializationFakeStaffRepo{},
			materializationFakeStudentRepo{},
			exceptionRepo,
			timeframeRepo,
			materializationAllowCalendarService{},
			nil,
			nil,
			slog.Default(),
		)
	}

	baseGroup := materializationFakeGroupRepo{templates: []*activities.Group{template}}
	baseSchedule := materializationFakeScheduleRepo{schedules: []*activities.Schedule{scheduleRow}}
	basePeriod := materializationFakePeriodRepo{periods: []*schedule.CalendarPeriod{period}}
	baseInstance := materializationFakeInstanceRepo{inserted: true}
	baseTimeframe := materializationFakeTimeframeRepo{timeframes: []*schedule.Timeframe{timeframe}}

	cases := []struct {
		name           string
		groupRepo      materializationFakeGroupRepo
		scheduleRepo   materializationFakeScheduleRepo
		enrollmentRepo materializationFakeEnrollmentRepo
		supervisorRepo materializationFakeSupervisorRepo
		periodRepo     materializationFakePeriodRepo
		instanceRepo   materializationFakeInstanceRepo
		exceptionRepo  materializationFakeExceptionRepo
		timeframeRepo  materializationFakeTimeframeRepo
		want           string
	}{
		{
			name:          "period repository error",
			groupRepo:     baseGroup,
			scheduleRepo:  baseSchedule,
			periodRepo:    materializationFakePeriodRepo{err: errors.New("periods failed")},
			instanceRepo:  baseInstance,
			timeframeRepo: baseTimeframe,
			want:          "load periods",
		},
		{
			name:          "template repository error",
			groupRepo:     materializationFakeGroupRepo{err: errors.New("templates failed")},
			scheduleRepo:  baseSchedule,
			periodRepo:    basePeriod,
			instanceRepo:  baseInstance,
			timeframeRepo: baseTimeframe,
			want:          "load templates",
		},
		{
			name:          "existing instance repository error",
			groupRepo:     baseGroup,
			scheduleRepo:  baseSchedule,
			periodRepo:    basePeriod,
			instanceRepo:  materializationFakeInstanceRepo{findErr: errors.New("existing failed")},
			timeframeRepo: baseTimeframe,
			want:          "load existing instances",
		},
		{
			name:          "exception repository error",
			groupRepo:     baseGroup,
			scheduleRepo:  baseSchedule,
			periodRepo:    basePeriod,
			instanceRepo:  baseInstance,
			exceptionRepo: materializationFakeExceptionRepo{err: errors.New("exceptions failed")},
			timeframeRepo: baseTimeframe,
			want:          "load exceptions",
		},
		{
			name:          "timeframe repository error",
			groupRepo:     baseGroup,
			scheduleRepo:  baseSchedule,
			periodRepo:    basePeriod,
			instanceRepo:  baseInstance,
			timeframeRepo: materializationFakeTimeframeRepo{err: errors.New("timeframes failed")},
			want:          "load timeframes",
		},
		{
			name:          "schedule repository error",
			groupRepo:     baseGroup,
			scheduleRepo:  materializationFakeScheduleRepo{err: errors.New("schedules failed")},
			periodRepo:    basePeriod,
			instanceRepo:  baseInstance,
			timeframeRepo: baseTimeframe,
			want:          "load schedules",
		},
		{
			name:           "enrollment repository error",
			groupRepo:      baseGroup,
			scheduleRepo:   baseSchedule,
			enrollmentRepo: materializationFakeEnrollmentRepo{err: errors.New("enrollments failed")},
			periodRepo:     basePeriod,
			instanceRepo:   baseInstance,
			timeframeRepo:  baseTimeframe,
			want:           "load enrollments",
		},
		{
			name:           "supervisor repository error",
			groupRepo:      baseGroup,
			scheduleRepo:   baseSchedule,
			supervisorRepo: materializationFakeSupervisorRepo{err: errors.New("supervisors failed")},
			periodRepo:     basePeriod,
			instanceRepo:   baseInstance,
			timeframeRepo:  baseTimeframe,
			want:           "load supervisors",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := makeSvc(tc.groupRepo, tc.scheduleRepo, tc.enrollmentRepo, tc.supervisorRepo, tc.periodRepo, tc.instanceRepo, tc.exceptionRepo, tc.timeframeRepo)

			result, err := svc.MaterializeForTenant(context.Background(), date, date, MaterializationSourceManual)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
			if result != nil {
				assert.Zero(t, result.InstancesCreated)
			}
		})
	}
}

func TestMaterializationServiceMethodsAndCopyBranches(t *testing.T) {
	t.Parallel()

	date := timezone.NewDate(2026, 4, 20)
	validFrom := activities.Date(date)
	periodID := int64(400)

	t.Run("interface ResolveWindow delegates to pure resolver", func(t *testing.T) {
		svc, _ := newMaterializationBranchService(materializationFakeInstanceRepo{inserted: false})

		from, to := svc.ResolveWindow(timezone.NewDate(2026, 4, 22), 1)

		assert.Equal(t, timezone.NewDate(2026, 4, 27), from)
		assert.Equal(t, timezone.NewDate(2026, 5, 3), to)
	})

	t.Run("nil logger falls back to slog default", func(t *testing.T) {
		svc := &materializationService{}

		assert.NotNil(t, svc.getLogger())
	})

	t.Run("copy enrollments skips invalid and duplicate students", func(t *testing.T) {
		studentRepo := &materializationCountingStudentRepo{}
		svc := &materializationService{studentRepo: studentRepo, logger: slog.Default()}
		valid := &activities.StudentEnrollment{StudentID: 501, ValidFrom: validFrom}
		duplicate := &activities.StudentEnrollment{StudentID: 501, ValidFrom: validFrom}
		wrongWeekday := &activities.StudentEnrollment{StudentID: 502, ValidFrom: validFrom, SelectedWeekdays: []int{2}}
		result := &MaterializationResult{}

		err := svc.copyExpectedStudents(context.Background(), 601, []*activities.StudentEnrollment{valid, duplicate, wrongWeekday}, nil, nil, date, periodID, result, "materialize template: copy enrollment")

		require.NoError(t, err)
		assert.Equal(t, 1, result.InstanceStudentsCreated)
		require.Len(t, studentRepo.rows, 1)
		assert.Equal(t, int64(501), studentRepo.rows[0].StudentID)
	})

	t.Run("copy enrollments wraps create errors", func(t *testing.T) {
		studentRepo := &materializationCountingStudentRepo{err: errors.New("insert student failed")}
		svc := &materializationService{studentRepo: studentRepo, logger: slog.Default()}
		result := &MaterializationResult{}

		err := svc.copyExpectedStudents(context.Background(), 602, []*activities.StudentEnrollment{{StudentID: 503, ValidFrom: validFrom}}, nil, nil, date, periodID, result, "materialize template: copy enrollment")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "copy enrollment")
		assert.Zero(t, result.InstanceStudentsCreated)
	})

	t.Run("copy supervisors skips invalid and duplicate staff", func(t *testing.T) {
		staffRepo := &materializationCountingStaffRepo{}
		svc := &materializationService{staffRepo: staffRepo, logger: slog.Default()}
		valid := &activities.SupervisorPlanned{StaffID: 701, ValidFrom: validFrom, IsPrimary: true}
		duplicate := &activities.SupervisorPlanned{StaffID: 701, ValidFrom: validFrom}
		future := &activities.SupervisorPlanned{StaffID: 702, ValidFrom: validFrom.AddDays(1)}
		result := &MaterializationResult{}

		err := svc.copySupervisors(context.Background(), 603, []*activities.SupervisorPlanned{valid, duplicate, future}, date, periodID, result)

		require.NoError(t, err)
		assert.Equal(t, 1, result.InstanceStaffCreated)
		require.Len(t, staffRepo.rows, 1)
		assert.Equal(t, int64(701), staffRepo.rows[0].StaffID)
		assert.True(t, staffRepo.rows[0].IsPrimary)
	})

	t.Run("copy supervisors wraps create errors", func(t *testing.T) {
		staffRepo := &materializationCountingStaffRepo{err: errors.New("insert staff failed")}
		svc := &materializationService{staffRepo: staffRepo, logger: slog.Default()}
		result := &MaterializationResult{}

		err := svc.copySupervisors(context.Background(), 604, []*activities.SupervisorPlanned{{StaffID: 703, ValidFrom: validFrom}}, date, periodID, result)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "copy supervisor")
		assert.Zero(t, result.InstanceStaffCreated)
	})
}

type materializationCountingStudentRepo struct {
	schedule.InstanceStudentRepository
	rows []*schedule.InstanceStudent
	err  error
}

func (r *materializationCountingStudentRepo) Create(_ context.Context, row *schedule.InstanceStudent) error {
	if r.err != nil {
		return r.err
	}
	r.rows = append(r.rows, row)
	return nil
}

func (r *materializationCountingStudentRepo) ApplyActiveStatusDaysForInstance(context.Context, int64, timezone.Date) (int, error) {
	return 0, nil
}

func (r *materializationCountingStudentRepo) ApplyActivePartialAbsencesForInstance(context.Context, int64, timezone.Date) (int, error) {
	return 0, nil
}

type materializationCountingStaffRepo struct {
	schedule.InstanceStaffRepository
	rows []*schedule.InstanceStaff
	err  error
}

func (r *materializationCountingStaffRepo) Create(_ context.Context, row *schedule.InstanceStaff) error {
	if r.err != nil {
		return r.err
	}
	r.rows = append(r.rows, row)
	return nil
}

func newMaterializationBranchService(instanceRepo materializationFakeInstanceRepo) (MaterializationService, timezone.Date) {
	date := timezone.NewDate(2026, 4, 20)
	return newMaterializationBranchServiceForSchedule(instanceRepo, date, activities.WeekdayMonday)
}

func newMaterializationBranchServiceForSchedule(
	instanceRepo materializationFakeInstanceRepo,
	date timezone.Date,
	weekday int,
) (MaterializationService, timezone.Date) {
	start := time.Date(2024, time.January, 1, 14, 0, 0, 0, time.UTC)
	end := time.Date(2024, time.January, 1, 15, 0, 0, 0, time.UTC)
	roomID := int64(700)
	templateID := int64(500)
	timeframeID := int64(600)

	svc := NewMaterializationService(
		materializationFakeGroupRepo{templates: []*activities.Group{{
			Name:          "Lernzeit",
			PlannedRoomID: &roomID,
			IsTemplate:    true,
			Model:         activities.Model{ID: templateID},
		}}},
		materializationFakeScheduleRepo{schedules: []*activities.Schedule{{
			Model:           activities.Model{ID: 800},
			ActivityGroupID: templateID,
			Weekday:         weekday,
			TimeframeID:     &timeframeID,
		}}},
		materializationFakeEnrollmentRepo{},
		materializationFakeSupervisorRepo{},
		materializationFakePeriodRepo{periods: []*schedule.CalendarPeriod{{
			Name:            "Schuljahr",
			PeriodType:      schedule.PeriodTypeSchoolYear,
			StartDate:       date.AddDays(-30),
			EndDate:         date.AddDays(30),
			WeekCycleLength: 1,
			IsActive:        true,
			Model:           schedule.Model{ID: 400},
		}}},
		instanceRepo,
		materializationFakeStaffRepo{},
		materializationFakeStudentRepo{},
		materializationFakeExceptionRepo{},
		materializationFakeTimeframeRepo{timeframes: []*schedule.Timeframe{{
			StartTime: start,
			EndTime:   &end,
			Model:     schedule.Model{ID: timeframeID},
		}}},
		materializationAllowCalendarService{},
		nil,
		nil,
		slog.Default(),
	)

	return svc, date
}

// -----------------------------------------------------------------------------
// TestScheduleEndedOn — WP-B3: schedules capped by a template split stop
// producing instances ON or AFTER valid_until (exclusive end).
// -----------------------------------------------------------------------------

func TestScheduleEndedOn(t *testing.T) {
	t.Parallel()

	date := timezone.NewDate(2026, time.June, 15)
	until := activities.Date(timezone.NewDate(2026, time.June, 15))

	assert.False(t, scheduleEndedOn(nil, date), "nil schedule never matches")
	assert.False(t, scheduleEndedOn(&activities.Schedule{}, date), "nil valid_until = open-ended")
	assert.True(t, scheduleEndedOn(&activities.Schedule{ValidUntil: &until}, date),
		"valid_until is exclusive: the schedule is ended ON that date")
	assert.True(t, scheduleEndedOn(&activities.Schedule{ValidUntil: &until}, date.AddDays(1)),
		"dates after valid_until are ended")
	assert.False(t, scheduleEndedOn(&activities.Schedule{ValidUntil: &until}, date.AddDays(-1)),
		"dates before valid_until still materialize")
}

// -----------------------------------------------------------------------------
// TestScheduleNotStartedOn — #2135: schedules with a series start (valid_from,
// inclusive) produce no instances before that date.
// -----------------------------------------------------------------------------

func TestScheduleNotStartedOn(t *testing.T) {
	t.Parallel()

	date := timezone.NewDate(2026, time.August, 13)
	from := activities.Date(timezone.NewDate(2026, time.August, 13))

	assert.False(t, scheduleNotStartedOn(nil, date), "nil schedule never matches")
	assert.False(t, scheduleNotStartedOn(&activities.Schedule{}, date), "nil valid_from = open start")
	assert.False(t, scheduleNotStartedOn(&activities.Schedule{ValidFrom: &from}, date),
		"valid_from is inclusive: the schedule materializes ON that date")
	assert.False(t, scheduleNotStartedOn(&activities.Schedule{ValidFrom: &from}, date.AddDays(1)),
		"dates after valid_from materialize")
	assert.True(t, scheduleNotStartedOn(&activities.Schedule{ValidFrom: &from}, date.AddDays(-1)),
		"dates before valid_from must not materialize")
}
