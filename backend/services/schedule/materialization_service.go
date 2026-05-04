// Package schedule — materialization service (WP-B8).
//
// Converts template groups (activities.groups WHERE is_template = true) plus
// their schedules, enrollments, supervisors and per-date exceptions into
// concrete schedule.activity_instances + schedule.instance_staff +
// schedule.instance_students rows for a given date window.
//
// Design invariants enforced here:
//
//   - Insert-only. Existing rows in any status (planned / active / completed /
//     cancelled) are never mutated by this service. A re-run over the same
//     window creates zero new rows. Template propagation ("Re-plan week") is
//     an explicit admin action that belongs to WP-B9, not here.
//
//   - Single period per (template, date). If the template pins a period via
//     calendar_period_id that period governs (dates outside its range are
//     skipped). Otherwise the lowest-ID active period containing the date is
//     used; if no active period contains the date the candidate is skipped.
//     Overlapping active periods log a warning but produce a single instance.
//
//   - DST-safe date iteration. Calendar dates are treated as civil values,
//     never wall-clock timestamps. See calendar_period_service.ShouldMaterialize
//     for the anchor math.
//
//   - Transaction boundary is the caller's. The scheduler wraps each tenant
//     in its own WithTenantTx; the HTTP handler uses the tenant middleware's
//     tx. We do not open nested transactions — a hard DB error bubbles up and
//     aborts the tenant's run; the scheduler will retry on the next matching
//     weekday.
//
//   - UniqueViolation on Create is absorbed (race with a concurrent run) and
//     counted as raced, not fatal.
package schedule

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

// MaxMaterializationWindowDays caps how many civil days a single call may
// cover. 56 days (8 weeks) matches the manual-endpoint validation and keeps
// one tenant's run well under a minute even with many templates.
const MaxMaterializationWindowDays = 56

// MaterializationSource identifies who triggered the run. Included in structured
// logs so the Grafana dashboards can split scheduler cadence from manual spikes.
type MaterializationSource string

const (
	MaterializationSourceScheduler MaterializationSource = "scheduler"
	MaterializationSourceManual    MaterializationSource = "manual"
)

// MaterializationResult summarises one MaterializeForTenant call.
//
// All counts are mutually exclusive: a candidate is classified into exactly
// one bucket per (template, schedule, target_date) tuple. InstancesCreated is
// the only bucket that produced a database row; every "Skipped*" bucket is a
// reason why a candidate was passed over and is safe to surface in the UI.
type MaterializationResult struct {
	From                        time.Time
	To                          time.Time
	InstancesCreated            int
	CandidatesSkippedExisting   int // merge-strategy protection: row already present
	CandidatesSkippedException  int // activity_exceptions row with type='cancelled'
	CandidatesSkippedABWeek     int // week_pattern did not match period cycle
	CandidatesSkippedNoPeriod   int // template pinned to period not covering date, or no active period matches
	CandidatesSkippedIncomplete int // template missing planned room or schedule missing timeframe/end_time
	CandidatesRaced             int // UNIQUE violation absorbed (concurrent run won the insert)
	InstanceStudentsCreated     int
	InstanceStaffCreated        int
	DurationMS                  int64
}

// MaterializationService drives conversion of templates into activity_instances.
type MaterializationService interface {
	// MaterializeForTenant creates missing schedule.activity_instances (plus
	// their instance_staff / instance_students rows) for the current tenant
	// for every civil date in [from, to]. The caller is expected to have
	// established tenant context and a transaction.
	//
	// source is a logging tag only — it has no behavioural effect. Use
	// MaterializationSourceScheduler or MaterializationSourceManual.
	MaterializeForTenant(ctx context.Context, from, to time.Time, source MaterializationSource) (*MaterializationResult, error)

	// ResolveWindow returns the default (from, to) pair the scheduler uses
	// when no window is explicitly supplied. `from` is the next Monday
	// strictly after `baseDate` (a baseDate that is itself a Monday rolls
	// forward to the following Monday — we never plan the current partial
	// week); `to` is `from + weeksAhead*7 − 1` days.
	ResolveWindow(baseDate time.Time, weeksAhead int) (from, to time.Time)
}

// materializationService is the concrete implementation.
type materializationService struct {
	groupRepo       activities.GroupRepository
	scheduleRepo    activities.ScheduleRepository
	enrollmentRepo  activities.StudentEnrollmentRepository
	supervisorRepo  activities.SupervisorPlannedRepository
	periodRepo      schedule.CalendarPeriodRepository
	instanceRepo    schedule.ActivityInstanceRepository
	staffRepo       schedule.InstanceStaffRepository
	studentRepo     schedule.InstanceStudentRepository
	exceptionRepo   schedule.ActivityExceptionRepository
	timeframeRepo   schedule.TimeframeRepository
	calendarService CalendarPeriodService
	db              *bun.DB
	logger          *slog.Logger
}

// NewMaterializationService constructs a MaterializationService with all
// repository dependencies. The CalendarPeriodService is injected (rather than
// reconstructed) so the shared A/B week algorithm stays the single source of
// truth for the DST-safe math.
func NewMaterializationService(
	groupRepo activities.GroupRepository,
	scheduleRepo activities.ScheduleRepository,
	enrollmentRepo activities.StudentEnrollmentRepository,
	supervisorRepo activities.SupervisorPlannedRepository,
	periodRepo schedule.CalendarPeriodRepository,
	instanceRepo schedule.ActivityInstanceRepository,
	staffRepo schedule.InstanceStaffRepository,
	studentRepo schedule.InstanceStudentRepository,
	exceptionRepo schedule.ActivityExceptionRepository,
	timeframeRepo schedule.TimeframeRepository,
	calendarService CalendarPeriodService,
	db *bun.DB,
	logger *slog.Logger,
) MaterializationService {
	return &materializationService{
		groupRepo:       groupRepo,
		scheduleRepo:    scheduleRepo,
		enrollmentRepo:  enrollmentRepo,
		supervisorRepo:  supervisorRepo,
		periodRepo:      periodRepo,
		instanceRepo:    instanceRepo,
		staffRepo:       staffRepo,
		studentRepo:     studentRepo,
		exceptionRepo:   exceptionRepo,
		timeframeRepo:   timeframeRepo,
		calendarService: calendarService,
		db:              db,
		logger:          logger,
	}
}

func (s *materializationService) getLogger() *slog.Logger {
	if s.logger != nil {
		return s.logger
	}
	return slog.Default()
}

// ResolveWindow computes the next full Monday–Sunday span covering weeksAhead
// weeks. When baseDate is itself a Monday the window starts the following
// Monday — by design, the scheduler never materialises the current partial
// week (planning is always for the next block).
func (s *materializationService) ResolveWindow(baseDate time.Time, weeksAhead int) (from, to time.Time) {
	return resolveWindow(baseDate, weeksAhead)
}

// MaterializeForTenant implements the core of WP-B8. See the interface comment
// for contract. The caller supplies tenant context + transaction.
func (s *materializationService) MaterializeForTenant(
	ctx context.Context,
	from, to time.Time,
	source MaterializationSource,
) (*MaterializationResult, error) {
	start := time.Now()
	tenantID := tenant.FromContext(ctx)

	// Normalise the window to civil date (UTC midnight) — we always compare
	// and persist dates this way regardless of the caller's zone.
	from = civilDate(from)
	to = civilDate(to)

	if to.Before(from) {
		return nil, &ScheduleError{Op: "materialize for tenant", Err: errors.New("to_date must not be before from_date")}
	}
	if days := int(to.Sub(from)/(24*time.Hour)) + 1; days > MaxMaterializationWindowDays {
		return nil, &ScheduleError{Op: "materialize for tenant", Err: fmt.Errorf("window exceeds %d days", MaxMaterializationWindowDays)}
	}

	result := &MaterializationResult{From: from, To: to}

	s.getLogger().Info("materialization starting",
		slog.Int64("tenant_id", tenantID),
		slog.String("source", string(source)),
		slog.String("from", from.Format("2006-01-02")),
		slog.String("to", to.Format("2006-01-02")),
	)

	// Load the world once up front. The window is bounded (≤ 56 days), a
	// tenant has O(dozens) of templates, O(hundreds) of enrollments — a single
	// fetch per collection avoids the N+1 trap in the candidate loop.
	periods, err := s.periodRepo.FindActiveByTenantID(ctx)
	if err != nil {
		return nil, &ScheduleError{Op: "materialize for tenant: load periods", Err: err}
	}
	if len(periods) == 0 {
		// Graceful no-op: no active period means A/B resolution has no anchor
		// and unbounded templates have nothing to scope against.
		s.finishLog(tenantID, source, result, start)
		return result, nil
	}

	templates, err := s.groupRepo.FindAllTemplates(ctx)
	if err != nil {
		return nil, &ScheduleError{Op: "materialize for tenant: load templates", Err: err}
	}
	if len(templates) == 0 {
		s.finishLog(tenantID, source, result, start)
		return result, nil
	}

	// Pre-fetch existing instances for the whole window. Builds an
	// (activity_group_id, date, start_time) → bool set for O(1) lookup.
	existing, err := s.instanceRepo.FindByTenantAndDateRange(ctx, from, to)
	if err != nil {
		return nil, &ScheduleError{Op: "materialize for tenant: load existing instances", Err: err}
	}
	existingIdx := buildExistingIndex(existing)

	// Pre-fetch exceptions for the window.
	exceptions, err := s.exceptionRepo.FindByDateRange(ctx, from, to)
	if err != nil {
		return nil, &ScheduleError{Op: "materialize for tenant: load exceptions", Err: err}
	}
	exceptionIdx := buildExceptionIndex(exceptions)

	// Load timeframes in one query and cache by ID.
	timeframes, err := s.timeframeRepo.List(ctx, nil)
	if err != nil {
		return nil, &ScheduleError{Op: "materialize for tenant: load timeframes", Err: err}
	}
	timeframeByID := make(map[int64]*schedule.Timeframe, len(timeframes))
	for _, tf := range timeframes {
		timeframeByID[tf.ID] = tf
	}

	// Iterate templates. Per template: load schedules, enrollments, supervisors
	// once; loop over the date window; produce candidates.
	for _, tmpl := range templates {
		if err := s.materializeTemplate(ctx, tmpl, from, to, periods, existingIdx, exceptionIdx, timeframeByID, result); err != nil {
			return result, err
		}
	}

	s.finishLog(tenantID, source, result, start)
	return result, nil
}

func (s *materializationService) finishLog(tenantID int64, source MaterializationSource, r *MaterializationResult, start time.Time) {
	r.DurationMS = time.Since(start).Milliseconds()
	s.getLogger().Info("materialization completed",
		slog.Int64("tenant_id", tenantID),
		slog.String("source", string(source)),
		slog.String("from", r.From.Format("2006-01-02")),
		slog.String("to", r.To.Format("2006-01-02")),
		slog.Int("created", r.InstancesCreated),
		slog.Int("skipped_existing", r.CandidatesSkippedExisting),
		slog.Int("skipped_exception", r.CandidatesSkippedException),
		slog.Int("skipped_ab", r.CandidatesSkippedABWeek),
		slog.Int("skipped_no_period", r.CandidatesSkippedNoPeriod),
		slog.Int("skipped_incomplete", r.CandidatesSkippedIncomplete),
		slog.Int("raced", r.CandidatesRaced),
		slog.Int("instance_students_created", r.InstanceStudentsCreated),
		slog.Int("instance_staff_created", r.InstanceStaffCreated),
		slog.Int64("duration_ms", r.DurationMS),
	)
}

// materializeTemplate runs the inner loop for a single template. Extracted so
// the main entry point stays readable.
func (s *materializationService) materializeTemplate(
	ctx context.Context,
	tmpl *activities.Group,
	from, to time.Time,
	periods []*schedule.CalendarPeriod,
	existingIdx map[existingKey]struct{},
	exceptionIdx map[exceptionKey]*schedule.ActivityException,
	timeframeByID map[int64]*schedule.Timeframe,
	result *MaterializationResult,
) error {
	schedules, err := s.scheduleRepo.FindByGroupID(ctx, tmpl.ID)
	if err != nil {
		return &ScheduleError{Op: "materialize template: load schedules", Err: err}
	}
	if len(schedules) == 0 {
		return nil // nothing to materialize for this template
	}

	enrollments, err := s.enrollmentRepo.FindByGroupID(ctx, tmpl.ID)
	if err != nil {
		return &ScheduleError{Op: "materialize template: load enrollments", Err: err}
	}
	supervisors, err := s.supervisorRepo.FindByGroupID(ctx, tmpl.ID)
	if err != nil {
		return &ScheduleError{Op: "materialize template: load supervisors", Err: err}
	}

	for date := from; !date.After(to); date = date.AddDate(0, 0, 1) {
		isoWd := isoWeekday(date)
		for _, sch := range schedules {
			if sch.Weekday != isoWd {
				continue
			}

			period := selectPeriod(tmpl, sch, date, periods, s.getLogger())
			if period == nil {
				result.CandidatesSkippedNoPeriod++
				continue
			}

			if !s.calendarService.ShouldMaterialize(sch.WeekPattern, date, period) {
				result.CandidatesSkippedABWeek++
				continue
			}

			tfID := int64(0)
			if sch.TimeframeID != nil {
				tfID = *sch.TimeframeID
			}
			tf, ok := timeframeByID[tfID]
			if !ok || tf.EndTime == nil {
				result.CandidatesSkippedIncomplete++
				continue
			}

			// schedule.timeframes stores start/end as TIMESTAMPTZ, so
			// tf.StartTime carries a timezone (typically the server's local
			// zone). schedule.activity_instances.start_time is plain TIME,
			// which is wall-clock only. Passing the tz-aware value directly
			// causes bun to serialize via UTC — "08:00 CEST" would land as
			// "06:00" in the TIME column. Extract the wall-clock components
			// in the timeframe's OWN location and rebuild as UTC so both
			// sides of the round-trip agree.
			base := materialParams{
				StartTime: extractTimeOfDay(tf.StartTime),
				EndTime:   extractTimeOfDay(*tf.EndTime),
				RoomID:    0,
			}
			if tmpl.PlannedRoomID != nil {
				base.RoomID = *tmpl.PlannedRoomID
			}

			exc := exceptionIdx[exceptionKey{tmpl.ID, formatCivilDate(date)}]
			effective, skip := applyException(base, exc)
			if skip {
				result.CandidatesSkippedException++
				continue
			}
			if effective.RoomID <= 0 {
				// No primary room on the template and no override from an
				// exception — we can't satisfy the NOT NULL on room_id.
				result.CandidatesSkippedIncomplete++
				continue
			}

			key := existingKey{
				ActivityGroupID: tmpl.ID,
				Date:            formatCivilDate(date),
				StartTime:       formatTimeOfDay(effective.StartTime),
			}
			if _, exists := existingIdx[key]; exists {
				result.CandidatesSkippedExisting++
				continue
			}

			periodID := period.ID
			templateID := tmpl.ID
			instance := &schedule.ActivityInstance{
				Date:             date,
				ActivityGroupID:  &templateID,
				CalendarPeriodID: &periodID,
				Title:            tmpl.Name,
				StartTime:        effective.StartTime,
				EndTime:          effective.EndTime,
				RoomID:           effective.RoomID,
				Status:           schedule.InstanceStatusPlanned,
				IsSpontaneous:    false,
			}

			if err := s.instanceRepo.Create(ctx, instance); err != nil {
				if isUniqueViolation(err) {
					result.CandidatesRaced++
					s.getLogger().Warn("materialization race: instance already present",
						slog.Int64("template_id", tmpl.ID),
						slog.String("date", date.Format("2006-01-02")),
						slog.String("start_time", effective.StartTime.Format("15:04:05")),
					)
					// Mark it in the index so a second schedule row on the same
					// (date, start_time) doesn't race again.
					existingIdx[key] = struct{}{}
					continue
				}
				return &ScheduleError{Op: "materialize template: create instance", Err: err}
			}
			existingIdx[key] = struct{}{}
			result.InstancesCreated++

			if err := s.copyEnrollments(ctx, instance.ID, enrollments, date, period.ID, result); err != nil {
				return err
			}
			if err := s.copySupervisors(ctx, instance.ID, supervisors, date, period.ID, result); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *materializationService) copyEnrollments(
	ctx context.Context,
	instanceID int64,
	enrollments []*activities.StudentEnrollment,
	date time.Time,
	periodID int64,
	result *MaterializationResult,
) error {
	for _, e := range enrollments {
		if !isEnrollmentValidOn(e, date, periodID) {
			continue
		}
		row := &schedule.InstanceStudent{
			InstanceID: instanceID,
			StudentID:  e.StudentID,
			Status:     schedule.AttendanceStatusExpected,
		}
		if err := s.studentRepo.Create(ctx, row); err != nil {
			return &ScheduleError{Op: "materialize template: copy enrollment", Err: err}
		}
		result.InstanceStudentsCreated++
	}
	return nil
}

func (s *materializationService) copySupervisors(
	ctx context.Context,
	instanceID int64,
	supervisors []*activities.SupervisorPlanned,
	date time.Time,
	periodID int64,
	result *MaterializationResult,
) error {
	for _, sup := range supervisors {
		if !isSupervisorValidOn(sup, date, periodID) {
			continue
		}
		row := &schedule.InstanceStaff{
			InstanceID:   instanceID,
			StaffID:      sup.StaffID,
			IsPrimary:    sup.IsPrimary,
			IsSubstitute: false,
			IsAbsent:     false,
		}
		if err := s.staffRepo.Create(ctx, row); err != nil {
			return &ScheduleError{Op: "materialize template: copy supervisor", Err: err}
		}
		result.InstanceStaffCreated++
	}
	return nil
}

// --- pure helpers (unit-tested without a DB) ---

// materialParams groups the three fields a schedule/exception can vary:
// start/end times and room. Title comes from the template unchanged.
type materialParams struct {
	StartTime time.Time
	EndTime   time.Time
	RoomID    int64
}

// existingKey identifies a row in the (template, date, start_time) search
// space. We key by formatted civil-date + time-of-day strings rather than by
// Unix timestamp because bun reads PostgreSQL DATE/TIME columns back with
// arbitrary (driver-chosen) time zones on the Go side — string formatting
// via civilDate → "2006-01-02" and StartTime → "15:04:05" makes the key
// timezone-independent by construction.
type existingKey struct {
	ActivityGroupID int64
	Date            string // "2006-01-02"
	StartTime       string // "15:04:05"
}

type exceptionKey struct {
	ActivityGroupID int64
	Date            string
}

// civilDate strips the time component and returns UTC midnight for the civil
// date. Matches the normalization used in CalendarPeriodService.ShouldMaterialize
// so A/B math, index keys and DB-side DATE columns all line up.
func civilDate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// isoWeekday returns the ISO 8601 weekday number for t (1=Mon … 7=Sun),
// matching the storage convention of activities.schedules.weekday.
func isoWeekday(t time.Time) int {
	wd := t.Weekday()
	if wd == time.Sunday {
		return 7
	}
	return int(wd)
}

// resolveWindow picks the next-Monday / following-Sunday window the scheduler
// uses by default. If baseDate is a Monday we intentionally skip to the
// following Monday — planning always targets the next block, never the
// current partial one. weeksAhead is clamped to [1, 8].
func resolveWindow(baseDate time.Time, weeksAhead int) (from, to time.Time) {
	if weeksAhead < 1 {
		weeksAhead = 1
	}
	if weeksAhead > 8 {
		weeksAhead = 8
	}
	d := civilDate(baseDate)
	// Go's Weekday: Sunday=0, Monday=1, ..., Saturday=6.
	// Days until next Monday (strictly after d):
	//   Sunday → 1, Monday → 7, Tuesday → 6, ..., Saturday → 2.
	var delta int
	switch d.Weekday() {
	case time.Sunday:
		delta = 1
	case time.Monday:
		delta = 7
	default:
		delta = int(time.Saturday-d.Weekday()) + 2
	}
	from = d.AddDate(0, 0, delta)
	to = from.AddDate(0, 0, weeksAhead*7-1)
	return from, to
}

// isEnrollmentValidOn answers: does this enrollment row contribute a student
// to an instance dated `date`, given the instance was scoped to `periodID`?
//
// Rules (RFC §5.3 / E17 / team-iteration-4):
//   - valid_from ≤ date
//   - valid_until IS NULL OR valid_until > date  (end is exclusive; a row
//     whose valid_until equals the instance date is NO LONGER contributing)
//   - calendar_period_id IS NULL OR calendar_period_id == periodID
func isEnrollmentValidOn(e *activities.StudentEnrollment, date time.Time, periodID int64) bool {
	if e == nil {
		return false
	}
	d := civilDate(date)
	if !e.ValidFrom.IsZero() && civilDate(e.ValidFrom).After(d) {
		return false
	}
	if e.ValidUntil != nil && !civilDate(*e.ValidUntil).After(d) {
		return false
	}
	if e.CalendarPeriodID != nil && *e.CalendarPeriodID != periodID {
		return false
	}
	return true
}

// isSupervisorValidOn mirrors isEnrollmentValidOn for activities.supervisors.
func isSupervisorValidOn(sp *activities.SupervisorPlanned, date time.Time, periodID int64) bool {
	if sp == nil {
		return false
	}
	d := civilDate(date)
	if !sp.ValidFrom.IsZero() && civilDate(sp.ValidFrom).After(d) {
		return false
	}
	if sp.ValidUntil != nil && !civilDate(*sp.ValidUntil).After(d) {
		return false
	}
	if sp.CalendarPeriodID != nil && *sp.CalendarPeriodID != periodID {
		return false
	}
	return true
}

// applyException returns the effective (start, end, room) for a candidate and
// whether the candidate should be skipped outright (cancelled exception).
// A nil exception is a no-op.
//
// Start/end overrides are normalized through extractTimeOfDay so the values
// land cleanly in schedule.activity_instances' TIME columns. The exception
// table's columns are TIME-typed but bun reads them as time.Time with year 0
// (Postgres TIME has no date), which would be rejected by Postgres on the
// subsequent INSERT if passed through unchanged.
func applyException(base materialParams, exc *schedule.ActivityException) (materialParams, bool) {
	if exc == nil {
		return base, false
	}
	if exc.ExceptionType == schedule.ActivityExceptionCancelled {
		return base, true
	}
	out := base
	if exc.StartTime != nil {
		out.StartTime = extractTimeOfDay(*exc.StartTime)
	}
	if exc.EndTime != nil {
		out.EndTime = extractTimeOfDay(*exc.EndTime)
	}
	if exc.RoomID != nil {
		out.RoomID = *exc.RoomID
	}
	return out, false
}

// selectPeriod implements the period-selection rule from the WP-B8 plan §1:
//
//  1. If template pins calendar_period_id, use that period iff `date` is
//     within its [start_date, end_date] range.
//  2. Otherwise, among active periods containing `date`, pick the one with
//     the lowest ID (deterministic). Warn if more than one matches so
//     operators see overlap in logs without the service blowing up.
//  3. If no period matches, return nil (caller counts as SkippedNoPeriod).
//
// Active-only filtering is the caller's responsibility: `periods` must already
// contain only is_active = true rows.
//
// The schedule may also pin a period via activities.schedules.calendar_period_id;
// we prefer the schedule's pin over the template's when both are present
// because the schedule is the more specific scope.
func selectPeriod(
	tmpl *activities.Group,
	sch *activities.Schedule,
	date time.Time,
	periods []*schedule.CalendarPeriod,
	logger *slog.Logger,
) *schedule.CalendarPeriod {
	pinned := schedulePinnedPeriodID(tmpl, sch)
	if pinned != nil {
		for _, p := range periods {
			if p.ID == *pinned {
				if p.ContainsDate(date) {
					return p
				}
				return nil
			}
		}
		// Pinned to a period that isn't in the active set → treat as no match.
		return nil
	}

	// Collect active periods containing the date, sorted ascending by ID.
	var matches []*schedule.CalendarPeriod
	for _, p := range periods {
		if p.ContainsDate(date) {
			matches = append(matches, p)
		}
	}
	if len(matches) == 0 {
		return nil
	}
	if len(matches) > 1 {
		sort.Slice(matches, func(i, j int) bool { return matches[i].ID < matches[j].ID })
		ids := make([]int64, 0, len(matches))
		for _, m := range matches {
			ids = append(ids, m.ID)
		}
		if logger != nil {
			logger.Warn("overlapping active calendar periods for materialization",
				slog.Int64("template_id", tmpl.ID),
				slog.String("date", date.Format("2006-01-02")),
				slog.Any("candidate_period_ids", ids),
				slog.Int64("chosen_period_id", matches[0].ID),
			)
		}
	}
	return matches[0]
}

// schedulePinnedPeriodID prefers the schedule's calendar_period_id when set
// (it's the more specific scope), falling back to the template's if present.
// Returns nil when neither is set.
func schedulePinnedPeriodID(tmpl *activities.Group, sch *activities.Schedule) *int64 {
	if sch != nil && sch.CalendarPeriodID != nil {
		return sch.CalendarPeriodID
	}
	// activities.groups doesn't currently expose a calendar_period_id column
	// at the model level — only schedules and enrollments/supervisors do.
	// Keep the signature future-proof: if the model gains the field later,
	// selectPeriod already honours it without further changes.
	_ = tmpl
	return nil
}

// mergeDecision is the per-candidate branch used by the materialization loop.
// Exposed as a pure function so tests can cover each status without a DB.
type mergeDecision int

const (
	mergeDecisionInsert mergeDecision = iota
	mergeDecisionSkipExisting
)

// decideMerge chooses what to do given an existing row of the given status.
// Under the WP-B8 insert-only policy, every non-nil existing row is skipped —
// template propagation becomes the "Re-plan week" WP-B9 action.
func decideMerge(existingStatus string) mergeDecision {
	if existingStatus == "" {
		return mergeDecisionInsert
	}
	return mergeDecisionSkipExisting
}

// --- indexing helpers (not part of the public surface) ---

func buildExistingIndex(existing []*schedule.ActivityInstance) map[existingKey]struct{} {
	idx := make(map[existingKey]struct{}, len(existing))
	for _, inst := range existing {
		if inst.ActivityGroupID == nil {
			continue // spontaneous rows are not keyed by template
		}
		k := existingKey{
			ActivityGroupID: *inst.ActivityGroupID,
			Date:            formatCivilDate(inst.Date),
			StartTime:       formatTimeOfDay(inst.StartTime),
		}
		idx[k] = struct{}{}
	}
	return idx
}

func buildExceptionIndex(exceptions []*schedule.ActivityException) map[exceptionKey]*schedule.ActivityException {
	idx := make(map[exceptionKey]*schedule.ActivityException, len(exceptions))
	for _, e := range exceptions {
		idx[exceptionKey{e.ActivityGroupID, formatCivilDate(e.ExceptionDate)}] = e
	}
	return idx
}

// formatCivilDate formats t as a civil date string ("2006-01-02"), based on
// the time's own Year/Month/Day components. This is independent of the
// time.Location the bun driver returns DATE columns with.
func formatCivilDate(t time.Time) string {
	return fmt.Sprintf("%04d-%02d-%02d", t.Year(), int(t.Month()), t.Day())
}

// formatTimeOfDay formats the time-of-day component of t as "15:04:05", based
// on the time's own Hour/Minute/Second components. Also location-independent.
func formatTimeOfDay(t time.Time) string {
	return fmt.Sprintf("%02d:%02d:%02d", t.Hour(), t.Minute(), t.Second())
}

// extractTimeOfDay is a thin wrapper around timezone.WallClock preserved so
// the local call sites read unchanged. See timezone.WallClock for the
// full rationale on why TIMESTAMPTZ → TIME round-trips need this.
func extractTimeOfDay(t time.Time) time.Time {
	return timezone.WallClock(t)
}

// isUniqueViolation returns true when err (or a wrapped modelBase.DatabaseError)
// carries PostgreSQL error code 23505. Mirrors services/platform/db_helpers.go
// without cross-package-importing it (keeps the dependency graph acyclic).
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var dbErr *modelBase.DatabaseError
	if errors.As(err, &dbErr) {
		err = dbErr.Err
	}
	var pgErr pgdriver.Error
	if errors.As(err, &pgErr) {
		return pgErr.IntegrityViolation() && pgErr.Field('C') == "23505"
	}
	return false
}
