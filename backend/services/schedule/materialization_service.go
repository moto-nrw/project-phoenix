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
//   - Transactional tenant recurrence gate. The service reuses the scheduler/
//     HTTP tenant transaction or opens an RLS-aware tenant transaction when
//     called directly, then holds the tenant recurrence gate through every
//     bounds read and instance insert. This serializes against split/end/PUT
//     without cross-tenant contention.
//
//   - Duplicate template-slot races are absorbed via INSERT ... ON CONFLICT DO
//     NOTHING and counted as raced, not fatal.
package schedule

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// MaxMaterializationWindowDays caps how many civil days a single call may
// cover. 56 days (8 weeks) matches the manual-endpoint validation and keeps
// one tenant's run well under a minute even with many templates.
const (
	MaxMaterializationWindowDays = 56
	materializeForTenantOp       = "materialize for tenant"
)

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
//
// Warnings collects soft, non-error conditions the caller should surface in
// the UI — e.g. "tenant has no active period" or "tenant has no templates".
// They are produced when a precondition is unmet and the run effectively
// no-ops; without them the admin sees `created:0, skipped_*:0` and has no way
// to tell why.
type MaterializationResult struct {
	From                        timezone.Date
	To                          timezone.Date
	InstancesCreated            int
	CandidatesSkippedExisting   int // merge-strategy protection: row already present
	CandidatesSkippedException  int // activity_exceptions row with type='cancelled'
	CandidatesSkippedABWeek     int // week_pattern did not match period cycle
	CandidatesSkippedNoPeriod   int // template pinned to period not covering date, or no active period matches
	CandidatesSkippedIncomplete int // template missing planned room or schedule missing timeframe/end_time
	CandidatesSkippedEnded      int // schedule.valid_until reached (template split ended this recurrence)
	CandidatesSkippedNotStarted int // schedule.valid_from not yet reached (successor schedule from a template split)
	CandidatesRaced             int // UNIQUE violation absorbed (concurrent run won the insert)
	InstanceStudentsCreated     int
	InstanceStaffCreated        int
	Warnings                    []MaterializationWarning
	DurationMS                  int64
}

// MaterializationWarning is a typed, UI-ready hint about a precondition.
// Code is a stable machine-readable discriminant; Message is German for
// direct display in the admin toast.
type MaterializationWarning struct {
	Code    string
	Message string
}

// Warning codes — keep in sync with the frontend MaterializeWarning union
// in lib/timetable-types.ts.
const (
	MaterializationWarningCodeNoActivePeriod = "no_active_period"
	MaterializationWarningCodeNoTemplates    = "no_templates"
)

// MaterializationService drives conversion of templates into activity_instances.
type MaterializationService interface {
	// MaterializeForTenant creates missing schedule.activity_instances (plus
	// their instance_staff / instance_students rows) for the current tenant
	// for every civil date in [from, to]. Tenant context is required; an
	// ambient transaction is reused and direct calls open their own.
	//
	// source is a logging tag only — it has no behavioural effect. Use
	// MaterializationSourceScheduler or MaterializationSourceManual.
	MaterializeForTenant(ctx context.Context, from, to timezone.Date, source MaterializationSource) (*MaterializationResult, error)

	// ResolveWindow returns the default (from, to) pair the scheduler uses
	// when no window is explicitly supplied. `from` is the next Monday
	// strictly after `baseDate` (a baseDate that is itself a Monday rolls
	// forward to the following Monday — we never plan the current partial
	// week); `to` is `from + weeksAhead*7 − 1` days.
	ResolveWindow(baseDate timezone.Date, weeksAhead int) (from, to timezone.Date)

	// DetectEditedInWindow returns the planned, template-backed occurrences of
	// one template in [from, to] whose content diverges from what the current
	// template would materialize — the single-occurrence edits a series re-plan
	// (#1875) would silently discard. Read-only; runs under the caller's
	// ambient tenant (RLS) transaction. Deviation-only rows (absences,
	// substitutes, understaffed ack, required_staff pin) are NOT reported —
	// ReplanWeek preserves those. When includeDeletions is set, individually
	// deleted occurrences (cancelled exceptions) are ALSO reported — a
	// following-series split resurrects them under the successor template; a
	// same-template re-plan preserves them, so callers on that path pass false.
	DetectEditedInWindow(ctx context.Context, activityGroupID int64, from, to timezone.Date, includeDeletions bool) ([]EditedOccurrence, error)
}

// materializationService is the concrete implementation.
type materializationService struct {
	groupRepo      activities.GroupRepository
	scheduleRepo   activities.ScheduleRepository
	enrollmentRepo activities.StudentEnrollmentRepository
	supervisorRepo activities.SupervisorPlannedRepository
	periodRepo     schedule.CalendarPeriodRepository
	instanceRepo   schedule.ActivityInstanceRepository
	staffRepo      schedule.InstanceStaffRepository
	studentRepo    schedule.InstanceStudentRepository
	// careBounds answers "until which day is this child in care" for the
	// per-date roster filter (#2487). Optional: nil means no child has an end
	// of care, which is what a bare unit-test service should assume.
	careBounds      CareBoundReader
	exceptionRepo   schedule.ActivityExceptionRepository
	timeframeRepo   schedule.TimeframeRepository
	calendarService CalendarPeriodService
	db              *bun.DB
	broadcaster     realtime.Broadcaster
	logger          *slog.Logger
}

// NewMaterializationService constructs a MaterializationService with all
// repository dependencies. The CalendarPeriodService is injected (rather than
// reconstructed) so the shared A/B week algorithm stays the single source of
// truth for the DST-safe math. Production wiring must provide db so direct
// calls get an RLS-aware transaction and recurrence gate; nil is reserved for
// pure repository-double unit tests. broadcaster is optional (nil → no SSE);
// production wiring must provide it so runs that create instances invalidate
// the staffing caches (#1844).
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
	broadcaster realtime.Broadcaster,
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
		broadcaster:     broadcaster,
		logger:          logger,
	}
}

func (s *materializationService) getLogger() *slog.Logger {
	return cmp.Or(s.logger, slog.Default())
}

// ResolveWindow computes the next full Monday–Sunday span covering weeksAhead
// weeks. When baseDate is itself a Monday the window starts the following
// Monday — by design, the scheduler never materialises the current partial
// week (planning is always for the next block).
func (s *materializationService) ResolveWindow(baseDate timezone.Date, weeksAhead int) (from, to timezone.Date) {
	return resolveWindow(baseDate, weeksAhead)
}

// MaterializeForTenant implements the core of WP-B8. See the interface comment
// for contract. Tenant context is mandatory; an ambient transaction is reused,
// otherwise production wiring opens an RLS-aware tenant transaction.
func (s *materializationService) MaterializeForTenant(
	ctx context.Context,
	from, to timezone.Date,
	source MaterializationSource,
) (*MaterializationResult, error) {
	start := time.Now()
	tenantID := tenant.FromContext(ctx)

	if err := validateMaterializationWindow(from, to); err != nil {
		return nil, err
	}
	if s.db != nil {
		if tenantID <= 0 {
			return nil, &ScheduleError{Op: materializeForTenantOp, Err: errors.New("no tenant in context")}
		}
		if _, ok := tenant.TransactionFromContext(ctx); !ok {
			return s.materializeForTenantInTransaction(ctx, tenantID, from, to, source)
		}
		if err := lockTenantRecurrenceWrites(ctx, s.db); err != nil {
			return nil, &ScheduleError{Op: "materialize for tenant: lock recurrence", Err: err}
		}
		// Then the grade-transition gate, in that order (see
		// education.TenantTransitionsLockKey — recurrence first, transitions
		// second, everywhere). expectedStudentIDsOn decides whether to insert a
		// roster row from the student status this pass read; a grade transition
		// committing its graduation and its roster-archive pass in between would
		// leave a departed child on an upcoming roster with nothing left to
		// remove them (#405 review).
		if err := lockTenantGradeTransitions(ctx, s.db); err != nil {
			return nil, &ScheduleError{Op: "materialize for tenant: lock grade transitions", Err: err}
		}
	}

	return s.materializeForTenantLocked(ctx, tenantID, from, to, source, start)
}

func validateMaterializationWindow(from, to timezone.Date) error {
	if to.Before(from) {
		return &ScheduleError{Op: materializeForTenantOp, Err: errors.New("to_date must not be before from_date")}
	}
	if days := from.DaysUntil(to) + 1; days > MaxMaterializationWindowDays {
		return &ScheduleError{Op: materializeForTenantOp, Err: fmt.Errorf("window exceeds %d days", MaxMaterializationWindowDays)}
	}
	return nil
}

// materializeForTenantInTransaction deliberately re-enters the public method.
// That keeps transaction detection and recurrence locking on one path while
// preserving the partial result returned when a later insert fails.
func (s *materializationService) materializeForTenantInTransaction(
	ctx context.Context,
	tenantID int64,
	from, to timezone.Date,
	source MaterializationSource,
) (*MaterializationResult, error) {
	var result *MaterializationResult
	err := tenant.WithTenantTx(ctx, s.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		var err error
		result, err = s.MaterializeForTenant(txCtx, from, to, source)
		return err
	})
	return result, err
}

func (s *materializationService) materializeForTenantLocked(
	ctx context.Context,
	tenantID int64,
	from, to timezone.Date,
	source MaterializationSource,
	start time.Time,
) (*MaterializationResult, error) {
	result := &MaterializationResult{From: from, To: to}

	s.getLogger().Info("materialization starting",
		slog.Int64("tenant_id", tenantID),
		slog.String("source", string(source)),
		slog.String("from", from.String()),
		slog.String("to", to.String()),
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
		// and unbounded templates have nothing to scope against. Surface a
		// typed warning so the UI can prompt the admin instead of showing
		// a misleading "0 angelegt" success toast.
		result.Warnings = append(result.Warnings, MaterializationWarning{
			Code:    MaterializationWarningCodeNoActivePeriod,
			Message: "Keine aktive Kalenderperiode hinterlegt — der Plan kann nicht materialisiert werden.",
		})
		s.finishLog(tenantID, source, result, start)
		return result, nil
	}

	templates, err := s.groupRepo.FindAllTemplates(ctx)
	if err != nil {
		return nil, &ScheduleError{Op: "materialize for tenant: load templates", Err: err}
	}
	if len(templates) == 0 {
		// Periods exist but no recurring activities yet. Distinct warning so
		// the UI can guide the admin to "+ Wiederkehrende Aktivität" rather
		// than the period editor.
		result.Warnings = append(result.Warnings, MaterializationWarning{
			Code:    MaterializationWarningCodeNoTemplates,
			Message: "Keine wiederkehrenden Aktivitäten hinterlegt — lege eine Vorlage an.",
		})
		s.finishLog(tenantID, source, result, start)
		return result, nil
	}

	// Pre-fetch existing instances for the whole window. Builds an
	// (activity_group_id, date, start_time) → bool set for O(1) lookup.
	existing, err := s.instanceRepo.FindByTenantAndDateRange(ctx, schedule.Date(from), schedule.Date(to))
	if err != nil {
		return nil, &ScheduleError{Op: "materialize for tenant: load existing instances", Err: err}
	}
	existingIdx := buildExistingIndex(existing)

	// Pre-fetch exceptions for the window.
	exceptions, err := s.exceptionRepo.FindByDateRange(ctx, schedule.Date(from), schedule.Date(to))
	if err != nil {
		return nil, &ScheduleError{Op: "materialize for tenant: load exceptions", Err: err}
	}
	exceptionIdx := buildExceptionIndex(exceptions)

	// Load timeframes in one query and cache by ID.
	timeframes, err := s.timeframeRepo.ListAll(ctx)
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

	// Materialization is not one of the instance CRUD paths, so nothing else
	// tells the staffing caches (planner, "Heute geplant" card) that new
	// assignments exist. Skip pure no-op re-runs — the scheduler sweeps the
	// same window nightly.
	if result.InstancesCreated > 0 || result.InstanceStaffCreated > 0 {
		broadcastStaffingChanged(ctx, s.broadcaster, s.getLogger(), "materialization")
	}

	s.finishLog(tenantID, source, result, start)
	return result, nil
}

func (s *materializationService) finishLog(tenantID int64, source MaterializationSource, r *MaterializationResult, start time.Time) {
	r.DurationMS = time.Since(start).Milliseconds()
	s.getLogger().Info("materialization completed",
		slog.Int64("tenant_id", tenantID),
		slog.String("source", string(source)),
		slog.String("from", r.From.String()),
		slog.String("to", r.To.String()),
		slog.Int("created", r.InstancesCreated),
		slog.Int("skipped_existing", r.CandidatesSkippedExisting),
		slog.Int("skipped_exception", r.CandidatesSkippedException),
		slog.Int("skipped_ab", r.CandidatesSkippedABWeek),
		slog.Int("skipped_no_period", r.CandidatesSkippedNoPeriod),
		slog.Int("skipped_incomplete", r.CandidatesSkippedIncomplete),
		slog.Int("skipped_ended", r.CandidatesSkippedEnded),
		slog.Int("skipped_not_started", r.CandidatesSkippedNotStarted),
		slog.Int("raced", r.CandidatesRaced),
		slog.Int("instance_students_created", r.InstanceStudentsCreated),
		slog.Int("instance_staff_created", r.InstanceStaffCreated),
		slog.Int64("duration_ms", r.DurationMS),
	)
}

// CareBoundReader projects the last care day of a set of children. Narrow on
// purpose — the materializer needs one DATE column, not student rows (#2487).
type CareBoundReader interface {
	FindCareBoundsByIDs(ctx context.Context, ids []int64) (map[int64]timezone.Date, error)
}

// SetCareBoundReader wires the per-date care filter. Without it the
// materializer behaves exactly as before, which keeps bare unit-test services
// compiling and correct.
func (s *materializationService) SetCareBoundReader(reader CareBoundReader) {
	s.careBounds = reader
}

// WireMaterializationCareBounds attaches the care-bound reader to a
// materialization service that supports it.
func WireMaterializationCareBounds(svc any, reader CareBoundReader) {
	if setter, ok := svc.(interface{ SetCareBoundReader(CareBoundReader) }); ok {
		setter.SetCareBoundReader(reader)
	}
}

// loadCareBounds resolves the last care day of every child a template could
// place on an instance. One query per template, reused for every date of the
// materialization window.
func (s *materializationService) loadCareBounds(
	ctx context.Context,
	targetStudentIDs []int64,
	enrollments []*activities.StudentEnrollment,
) (map[int64]timezone.Date, error) {
	if s.careBounds == nil {
		return nil, nil
	}
	ids := make([]int64, 0, len(targetStudentIDs)+len(enrollments))
	seen := make(map[int64]struct{}, len(targetStudentIDs)+len(enrollments))
	appendID := func(id int64) {
		if id <= 0 {
			return
		}
		if _, dup := seen[id]; dup {
			return
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	for _, id := range targetStudentIDs {
		appendID(id)
	}
	for _, enrollment := range enrollments {
		if enrollment != nil {
			appendID(enrollment.StudentID)
		}
	}
	return s.careBounds.FindCareBoundsByIDs(ctx, ids)
}

// careEndedOnDate reports whether the child's care had already ended on the
// given day. The interval's upper bound is inclusive, so the last care day
// itself still gets a roster row.
func careEndedOnDate(bounds map[int64]timezone.Date, studentID int64, date timezone.Date) bool {
	bound, ok := bounds[studentID]
	return ok && date.After(bound)
}

// expectedStudentIDsOn projects the deduplicated roster the template would
// materialize on one date. Both materialization and lost-edit detection use
// this function so manual enrollments and dynamic targets cannot drift apart.
func expectedStudentIDsOn(
	enrollments []*activities.StudentEnrollment,
	targetStudentIDs []int64,
	careBounds map[int64]timezone.Date,
	date timezone.Date,
	periodID int64,
) []int64 {
	seen := make(map[int64]struct{}, len(enrollments)+len(targetStudentIDs))
	for _, enrollment := range enrollments {
		if !isEnrollmentValidOn(enrollment, date, periodID) ||
			enrollmentStudentIsAlumnus(enrollment) ||
			careEndedOnDate(careBounds, enrollment.StudentID, date) {
			continue
		}
		seen[enrollment.StudentID] = struct{}{}
	}
	for _, studentID := range targetStudentIDs {
		if studentID <= 0 || careEndedOnDate(careBounds, studentID, date) {
			continue
		}
		seen[studentID] = struct{}{}
	}

	ids := make([]int64, 0, len(seen))
	for studentID := range seen {
		ids = append(ids, studentID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// materializeTemplate runs the inner loop for a single template. Extracted so
// the main entry point stays readable.
func (s *materializationService) materializeTemplate(
	ctx context.Context,
	tmpl *activities.Group,
	from, to timezone.Date,
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
	targetStudentIDs := make([]int64, 0)
	if targetRepo, ok := s.groupRepo.(activities.GroupTargetRepository); ok {
		targetStudentIDs, err = targetRepo.FindTargetStudentIDs(ctx, tmpl.ID)
		if err != nil {
			return &ScheduleError{Op: "materialize template: load target students", Err: err}
		}
	}
	supervisors, err := s.supervisorRepo.FindByGroupID(ctx, tmpl.ID)
	if err != nil {
		return &ScheduleError{Op: "materialize template: load supervisors", Err: err}
	}
	careBounds, err := s.loadCareBounds(ctx, targetStudentIDs, enrollments)
	if err != nil {
		return &ScheduleError{Op: "materialize template: load care bounds", Err: err}
	}

	for date := from; !date.After(to); date = date.AddDays(1) {
		// Templates created before the Mo–Fr planning rule may still contain
		// weekend schedules. Keep those records intact for administration, but
		// never create new invisible weekend instances from them.
		if date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
			continue
		}
		isoWd := isoWeekday(date)
		for _, sch := range schedules {
			if sch.Weekday != isoWd {
				continue
			}

			if scheduleEndedOn(sch, date) {
				result.CandidatesSkippedEnded++
				continue
			}

			if scheduleNotStartedOn(sch, date) {
				result.CandidatesSkippedNotStarted++
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

			// Both schedule.timeframes and schedule.activity_instances store
			// clock values as SQL TIME. Normalise through WallClock anyway so
			// driver-specific date anchors never affect comparisons or writes.
			base := materialParams{
				StartTime: extractTimeOfDay(tf.StartTime),
				EndTime:   extractTimeOfDay(*tf.EndTime),
				RoomID:    0,
			}
			if tmpl.PlannedRoomID != nil {
				base.RoomID = *tmpl.PlannedRoomID
			}

			exc := exceptionIdx[exceptionKey{tmpl.ID, date}]
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
				Date:            date,
				StartTime:       formatTimeOfDay(effective.StartTime),
			}
			if _, exists := existingIdx[key]; exists {
				result.CandidatesSkippedExisting++
				continue
			}

			periodID := period.ID
			templateID := tmpl.ID
			// RequiredStaff stays NULL on materialized rows: the template's
			// Personalbedarf override is inherited at read time (#1839). Copying
			// it here would make a template-level value indistinguishable from a
			// deliberate per-occurrence pin, so a later series edit of the
			// override could never propagate past ReplanWeek's deviation
			// snapshot. A non-NULL instance value is therefore always a
			// single-occurrence pin.
			instance := &schedule.ActivityInstance{
				Date:             schedule.Date(date),
				ActivityGroupID:  &templateID,
				CalendarPeriodID: &periodID,
				Title:            tmpl.Name,
				StartTime:        effective.StartTime,
				EndTime:          effective.EndTime,
				RoomID:           effective.RoomID,
				ListKind:         tmpl.ListKind,
				Status:           schedule.InstanceStatusPlanned,
				IsSpontaneous:    false,
			}

			inserted, err := s.instanceRepo.CreateTemplateBackedIfAbsent(ctx, instance)
			if err != nil {
				return &ScheduleError{
					Op: "materialize template: create instance",
					Err: fmt.Errorf(
						"tenant_id=%d template_id=%d schedule_id=%d date=%s period_id=%d room_id=%d start_time=%s end_time=%s: %w",
						tenant.FromContext(ctx),
						tmpl.ID,
						sch.ID,
						date.String(),
						period.ID,
						effective.RoomID,
						formatTimeOfDay(effective.StartTime),
						formatTimeOfDay(effective.EndTime),
						err,
					),
				}
			}
			if !inserted {
				result.CandidatesRaced++
				s.getLogger().Warn("materialization race: instance already present",
					slog.Int64("template_id", tmpl.ID),
					slog.String("date", date.String()),
					slog.String("start_time", effective.StartTime.Format("15:04:05")),
				)
				// Mark it in the index so a second schedule row on the same
				// (date, start_time) doesn't race again.
				existingIdx[key] = struct{}{}
				continue
			}
			existingIdx[key] = struct{}{}
			result.InstancesCreated++

			if err := s.copyExpectedStudents(
				ctx,
				instance.ID,
				enrollments,
				targetStudentIDs,
				careBounds,
				date,
				period.ID,
				result,
				"materialize template: copy expected student",
			); err != nil {
				return err
			}
			if err := s.copySupervisors(ctx, instance.ID, supervisors, date, period.ID, result); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *materializationService) copyExpectedStudents(
	ctx context.Context,
	instanceID int64,
	enrollments []*activities.StudentEnrollment,
	targetStudentIDs []int64,
	careBounds map[int64]timezone.Date,
	date timezone.Date,
	periodID int64,
	result *MaterializationResult,
	errorOp string,
) error {
	studentIDs := expectedStudentIDsOn(enrollments, targetStudentIDs, careBounds, date, periodID)
	for _, studentID := range studentIDs {
		row := &schedule.InstanceStudent{
			InstanceID: instanceID,
			StudentID:  studentID,
			Status:     schedule.AttendanceStatusExpected,
		}
		if err := s.studentRepo.Create(ctx, row); err != nil {
			return &ScheduleError{Op: errorOp, Err: err}
		}
		result.InstanceStudentsCreated++
	}
	if len(studentIDs) == 0 {
		return nil
	}
	if _, err := s.studentRepo.ApplyActiveStatusDaysForInstance(ctx, instanceID, schedule.Date(date)); err != nil {
		return &ScheduleError{Op: "materialize template: apply student status days", Err: err}
	}
	if _, err := s.studentRepo.ApplyActivePartialAbsencesForInstance(ctx, instanceID, schedule.Date(date)); err != nil {
		return &ScheduleError{Op: "materialize template: apply student partial absences", Err: err}
	}
	return nil
}

func (s *materializationService) copySupervisors(
	ctx context.Context,
	instanceID int64,
	supervisors []*activities.SupervisorPlanned,
	date timezone.Date,
	periodID int64,
	result *MaterializationResult,
) error {
	primaryStaffID, hasPrimary := effectivePrimarySupervisor(supervisors, date, periodID)

	// `unique_instance_staff (instance_id, staff_id)` rejects the same staff
	// on the same instance twice. Same staff on *different* instances at the
	// same time is a separate concept — surfaced by the conflict_warnings
	// system, not blocked here. Dedupe the input so a duplicate supervisor
	// row in `supervisors_planned` does not crash the whole materialization.
	seen := make(map[int64]struct{}, len(supervisors))
	for _, sup := range supervisors {
		if !isSupervisorValidOn(sup, date, periodID) {
			continue
		}
		if _, dup := seen[sup.StaffID]; dup {
			s.getLogger().Warn("supervisor listed twice on template — skipping duplicate",
				slog.Int64("instance_id", instanceID),
				slog.Int64("staff_id", sup.StaffID),
				slog.String("date", date.String()),
			)
			continue
		}
		seen[sup.StaffID] = struct{}{}
		row := &schedule.InstanceStaff{
			InstanceID:   instanceID,
			StaffID:      sup.StaffID,
			IsPrimary:    hasPrimary && sup.StaffID == primaryStaffID,
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
// space. The date is a timezone.Date (comparable, no instant); the start time
// stays a formatted "15:04:05" string because bun reads PostgreSQL TIME
// columns back with arbitrary (driver-chosen) time zones on the Go side —
// string formatting makes the key timezone-independent by construction.
type existingKey struct {
	ActivityGroupID int64
	Date            timezone.Date
	StartTime       string // "15:04:05"
}

type exceptionKey struct {
	ActivityGroupID int64
	Date            timezone.Date
}

// isoWeekday returns the ISO 8601 weekday number for d (1=Mon … 7=Sun),
// matching the storage convention of activities.schedules.weekday.
func isoWeekday(d timezone.Date) int {
	wd := d.Weekday()
	if wd == time.Sunday {
		return 7
	}
	return int(wd)
}

// resolveWindow picks the next-Monday / following-Sunday window the scheduler
// uses by default. If baseDate is a Monday we intentionally skip to the
// following Monday — planning always targets the next block, never the
// current partial one. weeksAhead is clamped to [1, 8].
func resolveWindow(baseDate timezone.Date, weeksAhead int) (from, to timezone.Date) {
	if weeksAhead < 1 {
		weeksAhead = 1
	}
	if weeksAhead > 8 {
		weeksAhead = 8
	}
	// Go's Weekday: Sunday=0, Monday=1, ..., Saturday=6.
	// Days until next Monday (strictly after baseDate):
	//   Sunday → 1, Monday → 7, Tuesday → 6, ..., Saturday → 2.
	var delta int
	switch baseDate.Weekday() {
	case time.Sunday:
		delta = 1
	case time.Monday:
		delta = 7
	default:
		delta = int(time.Saturday-baseDate.Weekday()) + 2
	}
	from = baseDate.AddDays(delta)
	to = from.AddDays(weeksAhead*7 - 1)
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
//   - weekday IS NULL OR weekday == date's ISO weekday (#2129)
//   - selected_weekdays IS NULL/empty OR contains date's ISO weekday
//
// enrollmentStudentIsAlumnus reports whether the People Directory projection
// marks the enrollment's student as graduated. Graduated students keep their
// enrollment rows for transition reverts but must drop off every current and
// future planning surface (#405).
func enrollmentStudentIsAlumnus(e *activities.StudentEnrollment) bool {
	return e != nil && e.StudentAlumnus
}

func isEnrollmentValidOn(e *activities.StudentEnrollment, date timezone.Date, periodID int64) bool {
	if e == nil {
		return false
	}
	if !e.ValidFrom.IsZero() && e.ValidFrom.After(date) {
		return false
	}
	if e.ValidUntil != nil && !e.ValidUntil.After(date) {
		return false
	}
	if e.CalendarPeriodID != nil && *e.CalendarPeriodID != periodID {
		return false
	}
	if !rosterWeekdayApplies(e.Weekday, date) {
		return false
	}
	if len(e.SelectedWeekdays) > 0 {
		weekday := isoWeekday(date)
		for _, selected := range e.SelectedWeekdays {
			if selected == weekday {
				return true
			}
		}
		return false
	}
	return true
}

// scheduleEndedOn answers: has this schedule's recurrence ended by `date`?
// valid_until is EXCLUSIVE — the schedule no longer produces instances ON or
// AFTER that date (same convention as enrollment valid_until). A nil
// valid_until means open-ended.
func scheduleEndedOn(sch *activities.Schedule, date timezone.Date) bool {
	return sch != nil && sch.ValidUntil != nil && !sch.ValidUntil.After(date)
}

// scheduleNotStartedOn answers: has this schedule's recurrence not yet begun
// on `date`? valid_from is INCLUSIVE — the schedule produces instances ON and
// AFTER that date, never before. A nil valid_from means an open start.
// Symmetric guard to scheduleEndedOn: the template split sets valid_from on
// successor schedules so materializing a window that begins before the
// effective date does not emit phantom successor instances next to the old
// template's rows.
func scheduleNotStartedOn(sch *activities.Schedule, date timezone.Date) bool {
	return sch != nil && sch.ValidFrom != nil && sch.ValidFrom.After(date)
}

// isSupervisorValidOn mirrors isEnrollmentValidOn for activities.supervisors.
func isSupervisorValidOn(sp *activities.SupervisorPlanned, date timezone.Date, periodID int64) bool {
	if sp == nil {
		return false
	}
	if !sp.ValidFrom.IsZero() && sp.ValidFrom.After(date) {
		return false
	}
	if sp.ValidUntil != nil && !sp.ValidUntil.After(date) {
		return false
	}
	if sp.CalendarPeriodID != nil && *sp.CalendarPeriodID != periodID {
		return false
	}
	return rosterWeekdayApplies(sp.Weekday, date)
}

// effectivePrimarySupervisor resolves overlapping legacy and scoped primary
// rows for one concrete occurrence. A NULL period or weekday is a fallback,
// while an exact period/weekday is more specific and therefore wins. Period
// specificity precedes weekday specificity because materialization first
// selects the occurrence's calendar period and then its weekday.
//
// The trigger can only enforce uniqueness inside an exact scope: clearing an
// unscoped legacy row when a Monday override is inserted would also remove the
// legacy primary from Tuesday. Resolve that overlap here instead, where both
// the occurrence period and date are known.
func effectivePrimarySupervisor(
	supervisors []*activities.SupervisorPlanned,
	date timezone.Date,
	periodID int64,
) (int64, bool) {
	selectedRank := -1
	var selectedStaffID, selectedRowID int64
	for _, supervisor := range supervisors {
		if !isSupervisorValidOn(supervisor, date, periodID) || !supervisor.IsPrimary {
			continue
		}
		rank := 0
		if supervisor.CalendarPeriodID != nil {
			rank += 2
		}
		if supervisor.Weekday != nil {
			rank++
		}
		if rank > selectedRank || (rank == selectedRank && supervisor.ID > selectedRowID) {
			selectedRank = rank
			selectedStaffID = supervisor.StaffID
			selectedRowID = supervisor.ID
		}
	}
	return selectedStaffID, selectedRank >= 0
}

// rosterWeekdayApplies answers whether a weekday-scoped roster row (#2129)
// contributes on `date`. A nil scope is the series-wide default and applies on
// every weekday the template runs; a set scope applies only on that ISO
// weekday. This is the single rule behind per-weekday staff and child lists —
// the template writer expands "shared default + deviations" into concrete
// per-weekday rows, so nothing here needs to know about that distinction.
func rosterWeekdayApplies(weekday *int, date timezone.Date) bool {
	return weekday == nil || *weekday == isoWeekday(date)
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
	date timezone.Date,
	periods []*schedule.CalendarPeriod,
	logger *slog.Logger,
) *schedule.CalendarPeriod {
	pinned := schedulePinnedPeriodID(tmpl, sch)
	if pinned != nil {
		for _, p := range periods {
			if p.ID == *pinned {
				if p.ContainsDay(schedule.Date(date)) {
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
		if p.ContainsDay(schedule.Date(date)) {
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
				slog.String("date", date.String()),
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
	// The schedule row's own pin (checked above) is the more specific,
	// materialization-time-authoritative value. The template's pin
	// (activities.Group.CalendarPeriodID, issue #1838) is the fallback for
	// templates whose schedule rows carry no explicit pin.
	if tmpl != nil && tmpl.CalendarPeriodID != nil {
		return tmpl.CalendarPeriodID
	}
	return nil
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
			Date:            timezone.Date(inst.Date),
			StartTime:       formatTimeOfDay(inst.StartTime),
		}
		idx[k] = struct{}{}
	}
	return idx
}

func buildExceptionIndex(exceptions []*schedule.ActivityException) map[exceptionKey]*schedule.ActivityException {
	idx := make(map[exceptionKey]*schedule.ActivityException, len(exceptions))
	for _, e := range exceptions {
		idx[exceptionKey{e.ActivityGroupID, timezone.Date(e.ExceptionDate)}] = e
	}
	return idx
}

// formatTimeOfDay formats the time-of-day component of t as "15:04:05", based
// on the time's own Hour/Minute/Second components. Also location-independent.
func formatTimeOfDay(t time.Time) string {
	return fmt.Sprintf("%02d:%02d:%02d", t.Hour(), t.Minute(), t.Second())
}

// extractTimeOfDay is a thin wrapper around timezone.NormalizeWallClock preserved so
// the local call sites read unchanged. See timezone.NormalizeWallClock for the
// full rationale on why TIMESTAMPTZ → TIME round-trips need this.
func extractTimeOfDay(t time.Time) time.Time {
	return timezone.NormalizeWallClock(t)
}
