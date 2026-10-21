package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
	schedule "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Materialization (WP-B8) behind timetable.MaterializationCapability.
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
//     an explicit admin action of the instance lifecycle, not here.
//
//   - Single period per (template, date). If the template pins a period via
//     calendar_period_id that period governs (dates outside its range are
//     skipped). Otherwise the lowest-ID active period containing the date is
//     used; if no active period contains the date the candidate is skipped.
//     Overlapping active periods log a warning but produce a single instance.
//
//   - DST-safe date iteration. Calendar dates are treated as civil values,
//     never wall-clock timestamps. The A/B week decision is the School
//     Calendar's single engine (schoolcalendar.WeekPatternApplies).
//
//   - Transactional tenant recurrence gate. The service reuses the scheduler/
//     HTTP tenant transaction or opens an RLS-aware tenant transaction when
//     called directly, then holds the tenant recurrence gate through every
//     bounds read and instance insert. This serializes against split/end/PUT
//     without cross-tenant contention.
//
//   - Duplicate template-slot races are absorbed via INSERT ... ON CONFLICT DO
//     NOTHING and counted as raced, not fatal.

const materializeForTenantOp = "materialize for tenant"

// materializationService is the concrete implementation.
type materializationService struct {
	groupRepo      timetable.ActivityGroupQuery
	scheduleRepo   timetable.ScheduleQuery
	enrollmentRepo timetable.StudentEnrollmentQuery
	supervisorRepo timetable.PlannedSupervisorQuery
	directories    ports.RepositoryDirectories
	periodRepo     ports.CalendarPeriodRecords
	instanceRepo   ports.ActivityInstanceRecords
	staffRepo      ports.InstanceStaffRecords
	studentRepo    ports.InstanceStudentRecords
	careBounds     ports.CareBoundReader
	nonWorkingDays timetable.NonWorkingDayCalendar // nil skips no holidays or closing days (#3594)
	exceptionRepo  timetable.ActivityExceptionQuery
	timeframeRepo  timetable.TimeframeQuery
	lock           timetable.RecurrenceWriteLock
	transactions   ports.TenantTransactions
	staffing       ports.StaffingAnnouncer
	logger         *slog.Logger
}

// NewMaterialization composes the Timetable owner's recurrence engine.
func NewMaterialization(deps ports.MaterializationDependencies) (timetable.MaterializationCapability, error) {
	if deps.GroupRepo == nil || deps.ScheduleRepo == nil || deps.EnrollmentRepo == nil || deps.SupervisorRepo == nil ||
		deps.PeriodRepo == nil || deps.InstanceRepo == nil || deps.StaffRepo == nil || deps.StudentRepo == nil ||
		deps.ExceptionRepo == nil || deps.TimeframeRepo == nil {
		return nil, errors.New("timetable materialization: required repository is nil")
	}
	if deps.Transactions != nil && deps.RecurrenceLock == nil {
		return nil, errors.New("timetable materialization: recurrence lock is required with a database")
	}
	return &materializationService{
		groupRepo:      deps.GroupRepo,
		scheduleRepo:   deps.ScheduleRepo,
		enrollmentRepo: deps.EnrollmentRepo,
		supervisorRepo: deps.SupervisorRepo,
		directories:    deps.Directories,
		periodRepo:     deps.PeriodRepo,
		instanceRepo:   deps.InstanceRepo,
		staffRepo:      deps.StaffRepo,
		studentRepo:    deps.StudentRepo,
		careBounds:     deps.CareBounds,
		nonWorkingDays: deps.NonWorkingDays,
		exceptionRepo:  deps.ExceptionRepo,
		timeframeRepo:  deps.TimeframeRepo,
		lock:           deps.RecurrenceLock,
		transactions:   deps.Transactions,
		staffing:       deps.Staffing,
		logger:         deps.Logger,
	}, nil
}

func (s *materializationService) getLogger() *slog.Logger {
	return OrDefaultLogger(s.logger)
}

// ResolveWindow computes the next full Monday–Sunday span covering weeksAhead
// weeks. When baseDate is itself a Monday the window starts the following
// Monday — by design, the scheduler never materialises the current partial
// week (planning is always for the next block).
func (s *materializationService) ResolveWindow(baseDate calendar.Date, weeksAhead int) (from, to calendar.Date) {
	return resolveWindow(baseDate, weeksAhead)
}

// MaterializeForTenant implements the core of WP-B8. See the interface comment
// for contract. Tenant context is mandatory; an ambient transaction is reused,
// otherwise production wiring opens an RLS-aware tenant transaction.
func (s *materializationService) MaterializeForTenant(
	ctx context.Context,
	from, to calendar.Date,
	source timetable.MaterializationSource,
) (*timetable.MaterializationResult, error) {
	start := time.Now()
	tenantID := tenant.FromContext(ctx)

	if err := validateMaterializationWindow(from, to); err != nil {
		return nil, err
	}
	if s.transactions != nil {
		if tenantID <= 0 {
			return nil, &ports.ScheduleError{Op: materializeForTenantOp, Err: errors.New("no tenant in context")}
		}
		if _, ok := tenant.TransactionFromContext(ctx); !ok {
			return s.materializeForTenantInTransaction(ctx, tenantID, from, to, source)
		}
		// The recurrence gate, then the grade-transition gate, in that order
		// (see schoolstructure.TransitionsLockKey — recurrence first,
		// transitions second, everywhere). expectedStudentIDsOn decides
		// whether to insert a roster row from the student status this pass
		// read; a grade transition committing its graduation and its
		// roster-archive pass in between would leave a departed child on an
		// upcoming roster with nothing left to remove them (#405 review).
		if err := s.lock.LockRecurrenceWritesThenGradeTransitions(ctx); err != nil {
			return nil, &ports.ScheduleError{Op: "materialize for tenant: lock recurrence", Err: err}
		}
	}

	return s.materializeForTenantLocked(ctx, tenantID, from, to, source, start)
}

func validateMaterializationWindow(from, to calendar.Date) error {
	if to.Before(from) {
		return &ports.ScheduleError{Op: materializeForTenantOp, Err: errors.New("to_date must not be before from_date")}
	}
	if days := from.DaysUntil(to) + 1; days > timetable.MaxMaterializationWindowDays {
		return &ports.ScheduleError{Op: materializeForTenantOp, Err: fmt.Errorf("window exceeds %d days", timetable.MaxMaterializationWindowDays)}
	}
	return nil
}

// materializeForTenantInTransaction deliberately re-enters the public method.
// That keeps transaction detection and recurrence locking on one path while
// preserving the partial result returned when a later insert fails.
func (s *materializationService) materializeForTenantInTransaction(
	ctx context.Context,
	tenantID int64,
	from, to calendar.Date,
	source timetable.MaterializationSource,
) (*timetable.MaterializationResult, error) {
	var result *timetable.MaterializationResult
	err := inTenant(ctx, s.transactions, tenantID, func(txCtx context.Context) error {
		var err error
		result, err = s.MaterializeForTenant(txCtx, from, to, source)
		return err
	})
	return result, err
}

// materializationWorld is everything a run loads once up front. The window is
// bounded (≤ 56 days), a tenant has O(dozens) of templates, O(hundreds) of
// enrollments — a single fetch per collection avoids the N+1 trap in the
// candidate loop.
type materializationWorld struct {
	from, to      calendar.Date
	periods       []*schedule.CalendarPeriod
	days          timetable.NonWorkingDays
	existingIdx   map[existingKey]struct{}
	exceptionIdx  map[exceptionKey]*schedule.ActivityException
	timeframeByID map[int64]*schedule.Timeframe
	// weekendFollowsFriday materializes Saturday and Sunday from Friday's
	// schedules (#3921).
	weekendFollowsFriday bool
}

func (s *materializationService) materializeForTenantLocked(
	ctx context.Context,
	tenantID int64,
	from, to calendar.Date,
	source timetable.MaterializationSource,
	start time.Time,
) (*timetable.MaterializationResult, error) {
	result := &timetable.MaterializationResult{From: from, To: to}

	s.getLogger().Info("materialization starting",
		slog.Int64("tenant_id", tenantID),
		slog.String("source", string(source)),
		slog.String("from", from.String()),
		slog.String("to", to.String()),
	)

	periods, templates, done, err := s.loadMaterializationPreconditions(ctx, result)
	if err != nil {
		return nil, err
	}
	if done {
		s.finishLog(tenantID, source, result, start)
		return result, nil
	}
	world, err := s.loadMaterializationWorld(ctx, from, to, periods)
	if err != nil {
		return nil, err
	}

	// Iterate templates. Per template: load schedules, enrollments, supervisors
	// once; loop over the date window; produce candidates.
	for _, tmpl := range templates {
		if err := s.materializeTemplate(ctx, tmpl, world, result); err != nil {
			return result, err
		}
	}

	// Materialization is not one of the instance CRUD paths, so nothing else
	// tells the staffing caches (planner, "Heute geplant" card) that new
	// assignments exist. Skip pure no-op re-runs — the scheduler sweeps the
	// same window nightly.
	if result.InstancesCreated > 0 || result.InstanceStaffCreated > 0 {
		announceStaffingChanged(ctx, s.staffing, s.getLogger(), "materialization")
	}

	s.finishLog(tenantID, source, result, start)
	return result, nil
}

// loadMaterializationPreconditions loads the active periods and the
// templates. Without either the run is a graceful no-op (done) that carries a
// typed warning, so the UI can prompt the admin instead of showing a
// misleading "0 angelegt" success toast.
func (s *materializationService) loadMaterializationPreconditions(
	ctx context.Context,
	result *timetable.MaterializationResult,
) ([]*schedule.CalendarPeriod, []*activityContract.Group, bool, error) {
	periods, err := s.periodRepo.FindActiveByTenantID(ctx)
	if err != nil {
		return nil, nil, false, &ports.ScheduleError{Op: "materialize for tenant: load periods", Err: err}
	}
	if len(periods) == 0 {
		// No active period means A/B resolution has no anchor and unbounded
		// templates have nothing to scope against.
		result.Warnings = append(result.Warnings, timetable.MaterializationWarning{
			Code:    timetable.MaterializationWarningCodeNoActivePeriod,
			Message: "Keine aktive Kalenderperiode hinterlegt — der Plan kann nicht materialisiert werden.",
		})
		return nil, nil, true, nil
	}
	isTemplate := true
	templates, err := listLegacyGroups(ctx, s.groupRepo, timetable.GroupFilter{IsTemplate: &isTemplate, ActiveOnly: true, OrderByName: true}, "find all templates")
	if err != nil {
		return nil, nil, false, &ports.ScheduleError{Op: "materialize for tenant: load templates", Err: err}
	}
	if len(templates) == 0 {
		// Periods exist but no recurring activities yet. Distinct warning so
		// the UI can guide the admin to "+ Wiederkehrende Aktivität" rather
		// than the period editor.
		result.Warnings = append(result.Warnings, timetable.MaterializationWarning{
			Code:    timetable.MaterializationWarningCodeNoTemplates,
			Message: "Keine wiederkehrenden Aktivitäten hinterlegt — lege eine Vorlage an.",
		})
		return nil, nil, true, nil
	}
	return periods, templates, false, nil
}

// loadMaterializationWorld pre-fetches the window's existing instances (an
// (activity_group_id, date, start_time) set for O(1) lookup), its holidays
// and closing days, its exceptions and every timeframe.
func (s *materializationService) loadMaterializationWorld(
	ctx context.Context,
	from, to calendar.Date,
	periods []*schedule.CalendarPeriod,
) (*materializationWorld, error) {
	days, err := timetable.LoadNonWorkingDays(ctx, s.nonWorkingDays, from.String(), to.String())
	if err != nil {
		return nil, &ports.ScheduleError{Op: "materialize for tenant: load non-working days", Err: err}
	}
	existing, err := s.instanceRepo.FindByTenantAndDateRange(ctx, calendar.Date(from), calendar.Date(to))
	if err != nil {
		return nil, &ports.ScheduleError{Op: "materialize for tenant: load existing instances", Err: err}
	}
	exceptions, err := exceptionsInRange(ctx, s.exceptionRepo, timetable.ActivityExceptionFilter{}, calendar.Date(from).String(), calendar.Date(to).String(), "find by date range")
	if err != nil {
		return nil, &ports.ScheduleError{Op: "materialize for tenant: load exceptions", Err: err}
	}
	timeframeByID, err := s.loadTimeframes(ctx, "materialize for tenant: load timeframes")
	if err != nil {
		return nil, err
	}
	follows, err := WeekendFollowsFridayIn(ctx, from, to)
	if err != nil {
		return nil, &ports.ScheduleError{Op: "materialize for tenant", Err: err}
	}
	return &materializationWorld{
		from:          from,
		to:            to,
		periods:       periods,
		days:          days,
		existingIdx:   buildExistingIndex(existing),
		exceptionIdx:  buildExceptionIndex(exceptions),
		timeframeByID: timeframeByID,

		weekendFollowsFriday: follows,
	}, nil
}

// loadTimeframes loads the tenant's timeframes in one query, keyed by id.
func (s *materializationService) loadTimeframes(ctx context.Context, op string) (map[int64]*schedule.Timeframe, error) {
	listed, err := s.timeframeRepo.ListTimeframes(ctx, timetable.TimeframeFilter{})
	if err != nil {
		return nil, &ports.ScheduleError{Op: op, Err: WrapDatabaseError("list", err)}
	}
	timeframes, err := legacyTimeframes(listed)
	if err != nil {
		return nil, &ports.ScheduleError{Op: op, Err: WrapDatabaseError("list", err)}
	}
	timeframeByID := make(map[int64]*schedule.Timeframe, len(timeframes))
	for _, tf := range timeframes {
		timeframeByID[tf.ID] = tf
	}
	return timeframeByID, nil
}

func (s *materializationService) finishLog(tenantID int64, source timetable.MaterializationSource, r *timetable.MaterializationResult, start time.Time) {
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
		slog.Int("skipped_holidays", r.CandidatesSkippedHoliday),
		slog.Int("skipped_closing_days", r.CandidatesSkippedClosingDay),
		slog.Int("raced", r.CandidatesRaced),
		slog.Int("instance_students_created", r.InstanceStudentsCreated),
		slog.Int("instance_staff_created", r.InstanceStaffCreated),
		slog.Int64("duration_ms", r.DurationMS),
	)
}

// templateRoster is what one template materializes from: its schedules and
// the people its occurrences receive.
type templateRoster struct {
	schedules        []*activityContract.Schedule
	enrollments      []*activityContract.StudentEnrollment
	targetStudentIDs []int64
	supervisors      []*activityContract.SupervisorPlanned
	careBounds       map[int64]calendar.Date
}

// loadTemplatePeople loads the people a template's occurrences receive:
// enrollments, dynamic target students, supervisors and the children's care
// bounds. op prefixes the error operations of the caller.
func (s *materializationService) loadTemplatePeople(ctx context.Context, templateID int64, op string) (templateRoster, error) {
	enrollments, err := groupStudentEnrollments(ctx, s.enrollmentRepo, s.directories, templateID)
	if err != nil {
		return templateRoster{}, &ports.ScheduleError{Op: op + ": load enrollments", Err: err}
	}
	targetStudentIDs, err := listTargetStudentIDs(ctx, s.groupRepo, templateID)
	if err != nil {
		return templateRoster{}, &ports.ScheduleError{Op: op + ": load target students", Err: err}
	}
	supervisors, err := groupPlannedSupervisors(ctx, s.supervisorRepo, s.directories, templateID)
	if err != nil {
		return templateRoster{}, &ports.ScheduleError{Op: op + ": load supervisors", Err: err}
	}
	careBounds, err := s.loadCareBounds(ctx, targetStudentIDs, enrollments)
	if err != nil {
		return templateRoster{}, &ports.ScheduleError{Op: op + ": load care bounds", Err: err}
	}
	return templateRoster{
		enrollments:      enrollments,
		targetStudentIDs: targetStudentIDs,
		supervisors:      supervisors,
		careBounds:       careBounds,
	}, nil
}

// loadCareBounds resolves the last care day of every child a template could
// place on an instance. One query per template, reused for every date of the
// materialization window.
func (s *materializationService) loadCareBounds(
	ctx context.Context,
	targetStudentIDs []int64,
	enrollments []*activityContract.StudentEnrollment,
) (map[int64]calendar.Date, error) {
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
func careEndedOnDate(bounds map[int64]calendar.Date, studentID int64, date calendar.Date) bool {
	bound, ok := bounds[studentID]
	return ok && date.After(bound)
}

// expectedStudentIDsOn projects the deduplicated roster the template would
// materialize on one date. Both materialization and lost-edit detection use
// this function so manual enrollments and dynamic targets cannot drift apart.
func expectedStudentIDsOn(
	enrollments []*activityContract.StudentEnrollment,
	targetStudentIDs []int64,
	careBounds map[int64]calendar.Date,
	date calendar.Date,
	periodID int64,
	planWeekday int,
) []int64 {
	seen := make(map[int64]struct{}, len(enrollments)+len(targetStudentIDs))
	for _, enrollment := range enrollments {
		if !isEnrollmentValidOn(enrollment, date, periodID, planWeekday) ||
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

// materializeTemplate runs the inner loop for a single template. Templates
// created before the Mo–Fr planning rule may still contain weekend schedules:
// those records stay intact for administration, but never produce new
// invisible weekend instances.
func (s *materializationService) materializeTemplate(
	ctx context.Context,
	tmpl *activityContract.Group,
	world *materializationWorld,
	result *timetable.MaterializationResult,
) error {
	schedules, err := groupSchedules(ctx, s.scheduleRepo, tmpl.ID)
	if err != nil {
		return &ports.ScheduleError{Op: "materialize template: load schedules", Err: err}
	}
	if len(schedules) == 0 {
		return nil // nothing to materialize for this template
	}
	roster, err := s.loadTemplatePeople(ctx, tmpl.ID, "materialize template")
	if err != nil {
		return err
	}
	roster.schedules = schedules

	for date := world.from; !date.After(world.to); date = date.AddDays(1) {
		if isWeekend(date) && !world.weekendFollowsFriday {
			continue
		}
		isoWd := planWeekday(date, world.weekendFollowsFriday)
		for _, sch := range schedules {
			if sch.Weekday != isoWd {
				continue
			}
			if err := s.materializeCandidate(ctx, tmpl, sch, date, roster, world, result); err != nil {
				return err
			}
		}
	}
	return nil
}

// materializeCandidate creates the occurrence of one (template, schedule,
// date) candidate unless a rule skips it or the slot already exists, then
// copies its roster.
func (s *materializationService) materializeCandidate(
	ctx context.Context,
	tmpl *activityContract.Group,
	sch *activityContract.Schedule,
	date calendar.Date,
	roster templateRoster,
	world *materializationWorld,
	result *timetable.MaterializationResult,
) error {
	exc := world.exceptionIdx[exceptionKey{tmpl.ID, date}]
	effective, period, skip := candidateSlot(tmpl, sch, date, world.periods, world.days, exc, world.timeframeByID, s.getLogger())
	if skip != candidateKept {
		countSkippedCandidate(result, skip)
		return nil
	}
	key := existingKey{
		ActivityGroupID: tmpl.ID,
		Date:            date,
		StartTime:       FormatTimeOfDay(effective.StartTime),
	}
	if _, exists := world.existingIdx[key]; exists {
		result.CandidatesSkippedExisting++
		return nil
	}

	instance := materializedInstance(tmpl, date, period, effective)
	inserted, err := s.instanceRepo.CreateTemplateBackedIfAbsent(ctx, instance)
	if err != nil {
		return createInstanceError(ctx, tmpl, sch, date, period, effective, err)
	}
	// Mark the slot in the index either way, so a second schedule row on the
	// same (date, start_time) doesn't race again.
	world.existingIdx[key] = struct{}{}
	if !inserted {
		result.CandidatesRaced++
		s.getLogger().Warn("materialization race: instance already present",
			slog.Int64("template_id", tmpl.ID),
			slog.String("date", date.String()),
			slog.String("start_time", effective.StartTime.Format("15:04:05")),
		)
		return nil
	}
	result.InstancesCreated++

	if err := s.copyExpectedStudents(
		ctx,
		instance.ID,
		roster.enrollments,
		roster.targetStudentIDs,
		roster.careBounds,
		date,
		period.ID,
		planWeekday(date, world.weekendFollowsFriday),
		result,
		"materialize template: copy expected student",
	); err != nil {
		return err
	}
	return s.copySupervisors(ctx, instance.ID, roster.supervisors, date, period.ID, planWeekday(date, world.weekendFollowsFriday), result)
}

// createInstanceError names the candidate whose insert failed, so a
// constraint violation in the scheduler log points at one slot.
func createInstanceError(
	ctx context.Context,
	tmpl *activityContract.Group,
	sch *activityContract.Schedule,
	date calendar.Date,
	period *schedule.CalendarPeriod,
	effective materialParams,
	err error,
) error {
	return &ports.ScheduleError{
		Op: "materialize template: create instance",
		Err: fmt.Errorf(
			"tenant_id=%d template_id=%d schedule_id=%d date=%s period_id=%d room_id=%d start_time=%s end_time=%s: %w",
			tenant.FromContext(ctx),
			tmpl.ID,
			sch.ID,
			date.String(),
			period.ID,
			effective.RoomID,
			FormatTimeOfDay(effective.StartTime),
			FormatTimeOfDay(effective.EndTime),
			err,
		),
	}
}

// materializedInstance is the planned occurrence a candidate produces.
// RequiredStaff stays NULL on materialized rows: the template's Personalbedarf
// override is inherited at read time (#1839). Copying it here would make a
// template-level value indistinguishable from a deliberate per-occurrence pin,
// so a later series edit of the override could never propagate past
// ReplanWeek's deviation snapshot. A non-NULL instance value is therefore
// always a single-occurrence pin.
func materializedInstance(tmpl *activityContract.Group, date calendar.Date, period *schedule.CalendarPeriod, effective materialParams) *schedule.ActivityInstance {
	periodID := period.ID
	templateID := tmpl.ID
	return &schedule.ActivityInstance{
		Date:             calendar.Date(date),
		ActivityGroupID:  &templateID,
		CalendarPeriodID: &periodID,
		Title:            tmpl.Name,
		StartTime:        effective.StartTime,
		EndTime:          effective.EndTime,
		RoomID:           effective.RoomID,
		ListKind:         tmpl.ListKind,
		Status:           schedule.InstanceStatusPlanned,
		IsSpontaneous:    false,
		TemplateType:     tmpl.Type,
	}
}

func (s *materializationService) copyExpectedStudents(
	ctx context.Context,
	instanceID int64,
	enrollments []*activityContract.StudentEnrollment,
	targetStudentIDs []int64,
	careBounds map[int64]calendar.Date,
	date calendar.Date,
	periodID int64,
	planWeekday int,
	result *timetable.MaterializationResult,
	errorOp string,
) error {
	studentIDs := expectedStudentIDsOn(enrollments, targetStudentIDs, careBounds, date, periodID, planWeekday)
	for _, studentID := range studentIDs {
		row := &schedule.InstanceStudent{
			InstanceID: instanceID,
			StudentID:  studentID,
			Status:     schedule.AttendanceStatusExpected,
		}
		if err := s.studentRepo.Create(ctx, row); err != nil {
			return &ports.ScheduleError{Op: errorOp, Err: err}
		}
		result.InstanceStudentsCreated++
	}
	if len(studentIDs) == 0 {
		return nil
	}
	if _, err := s.studentRepo.ApplyActiveStatusDaysForInstance(ctx, instanceID, calendar.Date(date)); err != nil {
		return &ports.ScheduleError{Op: "materialize template: apply student status days", Err: err}
	}
	if _, err := s.studentRepo.ApplyActivePartialAbsencesForInstance(ctx, instanceID, calendar.Date(date)); err != nil {
		return &ports.ScheduleError{Op: "materialize template: apply student partial absences", Err: err}
	}
	return nil
}

func (s *materializationService) copySupervisors(
	ctx context.Context,
	instanceID int64,
	supervisors []*activityContract.SupervisorPlanned,
	date calendar.Date,
	periodID int64,
	planWeekday int,
	result *timetable.MaterializationResult,
) error {
	primaryStaffID, hasPrimary := effectivePrimarySupervisor(supervisors, date, periodID, planWeekday)

	// `unique_instance_staff (instance_id, staff_id)` rejects the same staff
	// on the same instance twice. Same staff on *different* instances at the
	// same time is a separate concept — surfaced by the conflict_warnings
	// system, not blocked here. Dedupe the input so a duplicate supervisor
	// row in `supervisors_planned` does not crash the whole materialization.
	seen := make(map[int64]struct{}, len(supervisors))
	for _, sup := range supervisors {
		if !isSupervisorValidOn(sup, date, periodID, planWeekday) {
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
			return &ports.ScheduleError{Op: "materialize template: copy supervisor", Err: err}
		}
		result.InstanceStaffCreated++
	}
	return nil
}
