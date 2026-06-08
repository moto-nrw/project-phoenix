package schedule

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

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
	mustDate := func(y int, m time.Month, d int) time.Time {
		return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	}

	cases := []struct {
		name         string
		baseDate     time.Time
		weeksAhead   int
		expectedFrom time.Time
		expectedTo   time.Time
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
	d := func(y int, m time.Month, day int) time.Time {
		return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
	}
	p100 := int64(100)
	p200 := int64(200)
	validUntilApr20 := d(2026, time.April, 20)
	validUntilApr21 := d(2026, time.April, 21)

	target := d(2026, time.April, 20)

	cases := []struct {
		name   string
		e      *activities.StudentEnrollment
		period int64
		want   bool
	}{
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
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isEnrollmentValidOn(tc.e, target, tc.period)
			assert.Equal(t, tc.want, got)
		})
	}
}

// -----------------------------------------------------------------------------
// TestIsSupervisorValidOn — same shape as enrollments (ensures the two
// predicates stay symmetric by design).
// -----------------------------------------------------------------------------

func TestIsSupervisorValidOn(t *testing.T) {
	d := func(y int, m time.Month, day int) time.Time {
		return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
	}
	target := d(2026, time.April, 20)
	p100 := int64(100)
	p200 := int64(200)
	until := d(2026, time.April, 20) // exclusive

	assert.True(t, isSupervisorValidOn(&activities.SupervisorPlanned{ValidFrom: target}, target, p100))
	assert.False(t, isSupervisorValidOn(&activities.SupervisorPlanned{ValidFrom: d(2026, time.April, 21)}, target, p100))
	assert.False(t, isSupervisorValidOn(&activities.SupervisorPlanned{ValidFrom: d(2026, time.January, 1), ValidUntil: &until}, target, p100))
	assert.False(t, isSupervisorValidOn(&activities.SupervisorPlanned{ValidFrom: d(2026, time.January, 1), CalendarPeriodID: &p100}, target, p200))
	assert.True(t, isSupervisorValidOn(&activities.SupervisorPlanned{ValidFrom: d(2026, time.January, 1), CalendarPeriodID: &p100}, target, p100))
	assert.False(t, isSupervisorValidOn(nil, target, p100), "nil must be invalid")
}

// -----------------------------------------------------------------------------
// TestApplyException — cancellation skip + partial modify overrides.
// -----------------------------------------------------------------------------

func TestApplyException(t *testing.T) {
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
// TestMergeDecision — insert-only policy. Every non-empty existing status
// resolves to SkipExisting. decideMerge("") → Insert.
// -----------------------------------------------------------------------------

func TestMergeDecision(t *testing.T) {
	cases := []struct {
		status string
		want   mergeDecision
	}{
		{"", mergeDecisionInsert},
		{schedule.InstanceStatusPlanned, mergeDecisionSkipExisting},
		{schedule.InstanceStatusActive, mergeDecisionSkipExisting},
		{schedule.InstanceStatusCompleted, mergeDecisionSkipExisting},
		{schedule.InstanceStatusCancelled, mergeDecisionSkipExisting},
	}
	for _, tc := range cases {
		t.Run("status="+tc.status, func(t *testing.T) {
			assert.Equal(t, tc.want, decideMerge(tc.status))
		})
	}
}

// -----------------------------------------------------------------------------
// TestPeriodSelection — the WP-B8 period-selection rule.
// -----------------------------------------------------------------------------

func TestPeriodSelection(t *testing.T) {
	d := func(y int, m time.Month, day int) time.Time {
		return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
	}

	mkPeriod := func(id int64, start, end time.Time) *schedule.CalendarPeriod {
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
}

// -----------------------------------------------------------------------------
// TestCivilDate + TestIsoWeekday — two-line helpers, still worth pinning
// because they underpin the index keys and schedule.weekday comparison.
// -----------------------------------------------------------------------------

func TestCivilDate(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)

	// DST boundary (Mar 29 2026): 01:30 Berlin local exists pre-jump.
	local := time.Date(2026, time.March, 29, 1, 30, 0, 0, berlin)
	got := civilDate(local)
	assert.Equal(t, 2026, got.Year())
	assert.Equal(t, time.March, got.Month())
	assert.Equal(t, 29, got.Day())
	assert.Equal(t, time.UTC, got.Location())
	assert.Equal(t, 0, got.Hour())
	assert.Equal(t, 0, got.Minute())
}

func TestIsoWeekday(t *testing.T) {
	mon := time.Date(2026, time.April, 20, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, 1, isoWeekday(mon))
	assert.Equal(t, 2, isoWeekday(mon.AddDate(0, 0, 1)))
	assert.Equal(t, 3, isoWeekday(mon.AddDate(0, 0, 2)))
	assert.Equal(t, 4, isoWeekday(mon.AddDate(0, 0, 3)))
	assert.Equal(t, 5, isoWeekday(mon.AddDate(0, 0, 4)))
	assert.Equal(t, 6, isoWeekday(mon.AddDate(0, 0, 5)))
	assert.Equal(t, 7, isoWeekday(mon.AddDate(0, 0, 6)))
}

type materializationFakeGroupRepo struct {
	activities.GroupRepository
	templates []*activities.Group
}

func (r materializationFakeGroupRepo) FindAllTemplates(context.Context) ([]*activities.Group, error) {
	return r.templates, nil
}

type materializationFakeScheduleRepo struct {
	activities.ScheduleRepository
	schedules []*activities.Schedule
}

func (r materializationFakeScheduleRepo) FindByGroupID(context.Context, int64) ([]*activities.Schedule, error) {
	return r.schedules, nil
}

type materializationFakeEnrollmentRepo struct {
	activities.StudentEnrollmentRepository
}

func (r materializationFakeEnrollmentRepo) FindByGroupID(context.Context, int64) ([]*activities.StudentEnrollment, error) {
	return nil, nil
}

type materializationFakeSupervisorRepo struct {
	activities.SupervisorPlannedRepository
}

func (r materializationFakeSupervisorRepo) FindByGroupID(context.Context, int64) ([]*activities.SupervisorPlanned, error) {
	return nil, nil
}

type materializationFakePeriodRepo struct {
	schedule.CalendarPeriodRepository
	periods []*schedule.CalendarPeriod
}

func (r materializationFakePeriodRepo) FindActiveByTenantID(context.Context) ([]*schedule.CalendarPeriod, error) {
	return r.periods, nil
}

type materializationFakeInstanceRepo struct {
	schedule.ActivityInstanceRepository
	inserted bool
	err      error
}

func (r materializationFakeInstanceRepo) FindByTenantAndDateRange(context.Context, time.Time, time.Time) ([]*schedule.ActivityInstance, error) {
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
}

func (r materializationFakeExceptionRepo) FindByDateRange(context.Context, time.Time, time.Time) ([]*schedule.ActivityException, error) {
	return nil, nil
}

type materializationFakeTimeframeRepo struct {
	schedule.TimeframeRepository
	timeframes []*schedule.Timeframe
}

func (r materializationFakeTimeframeRepo) List(context.Context, *modelBase.QueryOptions) ([]*schedule.Timeframe, error) {
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

func (materializationAllowCalendarService) ShouldMaterialize(int, time.Time, *schedule.CalendarPeriod) bool {
	return true
}

func TestMaterializeForTenant_DuplicateInsertRaceDoesNotCopyChildren(t *testing.T) {
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

func newMaterializationBranchService(instanceRepo materializationFakeInstanceRepo) (MaterializationService, time.Time) {
	date := time.Date(2026, time.April, 20, 0, 0, 0, 0, time.UTC)
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
			Model:         modelBase.Model{ID: templateID},
		}}},
		materializationFakeScheduleRepo{schedules: []*activities.Schedule{{
			Model:           modelBase.Model{ID: 800},
			ActivityGroupID: templateID,
			Weekday:         activities.WeekdayMonday,
			TimeframeID:     &timeframeID,
		}}},
		materializationFakeEnrollmentRepo{},
		materializationFakeSupervisorRepo{},
		materializationFakePeriodRepo{periods: []*schedule.CalendarPeriod{{
			Name:            "Schuljahr",
			PeriodType:      schedule.PeriodTypeSchoolYear,
			StartDate:       date.AddDate(0, -1, 0),
			EndDate:         date.AddDate(0, 1, 0),
			WeekCycleLength: 1,
			IsActive:        true,
			Model:           modelBase.Model{ID: 400},
		}}},
		instanceRepo,
		materializationFakeStaffRepo{},
		materializationFakeStudentRepo{},
		materializationFakeExceptionRepo{},
		materializationFakeTimeframeRepo{timeframes: []*schedule.Timeframe{{
			StartTime: start,
			EndTime:   &end,
			Model:     modelBase.Model{ID: timeframeID},
		}}},
		materializationAllowCalendarService{},
		nil,
		slog.Default(),
	)

	return svc, date
}
