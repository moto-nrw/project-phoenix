// Package schedule — template split service (WP-B3, "Dieser und alle folgenden").
//
// Splits a recurring template at an effective date: the old template's
// schedules and rosters are capped (valid_until = effective date, exclusive),
// a successor template is created with the updated fields, the old template's
// still-planned future instances inside the materialization horizon are
// deleted, and (optionally) the window is re-materialized so the successor's
// instances appear on the grid immediately.
//
// Invariants:
//
//   - The old template row itself is never mutated beyond its schedule/roster
//     valid_until caps — history (started/completed/cancelled/spontaneous
//     instances) stays attached to the old template untouched.
//   - Only status='planned' AND is_spontaneous=false instances of the OLD
//     template are deleted, and only from the effective date onward.
//   - The successor's schedules start at the effective date (valid_from)
//     and end open (valid_until = NULL); its roster rows start at the
//     effective date.
//
// Transaction boundary is the caller's (TenantTxMiddleware) — any failure
// rolls back the whole split.
package schedule

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/sliceutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Sentinel errors callers branch on via errors.Is.
var (
	// ErrSplitTemplateNotFound is returned when the template id does not
	// resolve to a non-archived template in the current tenant. → 404.
	ErrSplitTemplateNotFound = errors.New("template not found")

	// ErrSplitInvalidInput is returned for semantically invalid split input
	// (past effective date, bad weekdays, …). → 400.
	ErrSplitInvalidInput = errors.New("invalid template split input")
)

// TemplateSplitInput mirrors the update-template request plus the split
// controls. StartTime/EndTime are wall-clock values (parsed from HH:MM).
// StudentIDs/StaffIDs follow tri-state semantics: nil = carry over the
// previously-active roster of the old template; non-nil (including empty) =
// use exactly the provided ids. PrimaryStaffID applies to an explicitly
// provided StaffIDs roster; carried-over supervisors keep their own
// is_primary flag.
type TemplateSplitInput struct {
	TemplateID       int64
	EffectiveDate    timezone.Date
	Name             string
	Type             string // care | activity | external
	Weekdays         []int  // ISO 8601, Mo=1 … Su=7
	StartTime        time.Time
	EndTime          time.Time
	RoomID           int64
	CategoryID       int64
	MaxParticipants  *int
	WeekPattern      *int // 0=every week, 1=A, 2=B; nil = 0
	CalendarPeriodID *int64
	EducationGroupID *int64
	// Zielgruppe (target-group) fields, carried onto the successor Group
	// (see createSuccessorGroup). "gruppe" reuses EducationGroupID above.
	TargetGroupType   string
	TargetGradeLevel  *int16
	TargetSchoolClass *string
	StudentIDs        []int64
	StaffIDs          []int64
	PrimaryStaffID    *int64
	MaterializeFrom   *timezone.Date
	MaterializeTo     *timezone.Date
}

// TemplateSplitResult summarises one Split call.
type TemplateSplitResult struct {
	OldTemplateID    int64
	NewTemplateID    int64
	NewScheduleIDs   []int64
	DeletedInstances int
	Materialization  *MaterializationResult
}

// TemplateEndInput describes the destructive "delete this and following"
// recurrence action. EffectiveDate is inclusive for planned instance deletes
// and exclusive for schedule/roster valid_until caps.
type TemplateEndInput struct {
	TemplateID    int64
	EffectiveDate timezone.Date
}

// TemplateEndResult summarises one EndFromDate call.
type TemplateEndResult struct {
	TemplateID        int64
	EffectiveDate     timezone.Date
	DeletedInstances  int
	CappedSchedules   int64
	CappedEnrollments int64
	CappedSupervisors int64
}

// TemplateSplitDependencies aggregates wiring. All fields are required;
// Logger may be nil (falls back to slog.Default).
type TemplateSplitDependencies struct {
	GroupRepo       activitiesModel.GroupRepository
	ScheduleRepo    activitiesModel.ScheduleRepository
	EnrollmentRepo  activitiesModel.StudentEnrollmentRepository
	SupervisorRepo  activitiesModel.SupervisorPlannedRepository
	InstanceRepo    scheduleModel.ActivityInstanceRepository
	TimeframeRepo   scheduleModel.TimeframeRepository
	Materialization MaterializationService
	Logger          *slog.Logger
}

// TemplateSplitService performs recurring-template scope operations.
type TemplateSplitService struct {
	deps TemplateSplitDependencies
}

// NewTemplateSplitService constructs a TemplateSplitService. Panics if a
// required dependency is nil — the split has no sensible degraded mode, so
// the factory must wire it completely at startup.
func NewTemplateSplitService(deps TemplateSplitDependencies) *TemplateSplitService {
	if deps.GroupRepo == nil || deps.ScheduleRepo == nil || deps.EnrollmentRepo == nil ||
		deps.SupervisorRepo == nil || deps.InstanceRepo == nil || deps.TimeframeRepo == nil ||
		deps.Materialization == nil {
		panic("schedule.NewTemplateSplitService: required dependency is nil")
	}
	return &TemplateSplitService{deps: deps}
}

func (s *TemplateSplitService) getLogger() *slog.Logger {
	return cmp.Or(s.deps.Logger, slog.Default())
}

// Split caps the old template at in.EffectiveDate (exclusive), creates a
// successor template from the updated fields, deletes the old template's
// planned future instances and optionally re-materializes the window.
// The caller is expected to have established tenant context and a
// transaction.
func (s *TemplateSplitService) Split(ctx context.Context, in TemplateSplitInput) (*TemplateSplitResult, error) {
	if err := validateSplitInput(in); err != nil {
		return nil, err
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return nil, &ScheduleError{Op: "split template", Err: errors.New("no tenant in context")}
	}

	old, err := s.loadTemplate(ctx, in.TemplateID)
	if err != nil {
		return nil, err
	}

	// Read the previously-active roster BEFORE capping — after the cap the
	// "active" predicate (valid_until IS NULL) matches nothing.
	activeEnrollments, activeSupervisors, err := s.loadActiveRoster(ctx, old.ID)
	if err != nil {
		return nil, err
	}

	// Cap the old template: schedules stop producing instances ON or AFTER
	// the effective date; roster rows end the same way (exclusive end).
	if _, err := s.deps.ScheduleRepo.CapValidUntil(ctx, old.ID, in.EffectiveDate); err != nil {
		return nil, &ScheduleError{Op: "split template: cap schedules", Err: err}
	}
	if _, err := s.deps.EnrollmentRepo.CapActiveByGroup(ctx, old.ID, in.EffectiveDate); err != nil {
		return nil, &ScheduleError{Op: "split template: cap enrollments", Err: err}
	}
	if _, err := s.deps.SupervisorRepo.CapActiveByGroup(ctx, old.ID, in.EffectiveDate); err != nil {
		return nil, &ScheduleError{Op: "split template: cap supervisors", Err: err}
	}

	timeframeID, err := FindOrCreateTimeframe(ctx, s.deps.TimeframeRepo, in.StartTime, in.EndTime, in.Name)
	if err != nil {
		return nil, &ScheduleError{Op: "split template: resolve timeframe", Err: err}
	}

	newGroup, err := s.createSuccessorGroup(ctx, old, in, tenantID)
	if err != nil {
		return nil, err
	}
	scheduleIDs, err := s.createSuccessorSchedules(ctx, newGroup.ID, timeframeID, in, tenantID)
	if err != nil {
		return nil, err
	}
	if err := s.createStudentRoster(ctx, newGroup.ID, in, activeEnrollments, tenantID); err != nil {
		return nil, err
	}
	if err := s.createStaffRoster(ctx, newGroup.ID, in, activeSupervisors, tenantID); err != nil {
		return nil, err
	}

	// Remove ALL of the old template's still-planned future instances from
	// the effective date onward — deliberately open-ended (nil `to`), NOT
	// tied to the materialization window: the split must guarantee that no
	// planned non-spontaneous old-template instance survives on/after the
	// effective date, however far materialization once reached.
	// Started/completed/cancelled and spontaneous rows survive (same
	// protection rule as ReplanWeek).
	oldID := old.ID
	deleted, err := s.deps.InstanceRepo.DeletePlannedNonSpontaneousInWindow(ctx, in.EffectiveDate, nil, &oldID)
	if err != nil {
		return nil, &ScheduleError{Op: "split template: delete planned instances", Err: err}
	}

	mat, err := s.materializeWindow(ctx, in)
	if err != nil {
		return nil, err
	}

	s.getLogger().Info("template split completed",
		slog.Int64("tenant_id", tenantID),
		slog.Int64("old_template_id", old.ID),
		slog.Int64("new_template_id", newGroup.ID),
		slog.String("effective_date", in.EffectiveDate.String()),
		slog.Int("schedule_count", len(scheduleIDs)),
		slog.Int64("deleted_instances", deleted),
	)

	return &TemplateSplitResult{
		OldTemplateID:    old.ID,
		NewTemplateID:    newGroup.ID,
		NewScheduleIDs:   scheduleIDs,
		DeletedInstances: int(deleted),
		Materialization:  mat,
	}, nil
}

// EndFromDate caps a template from the effective date onward without creating
// a successor ("Dieser und alle folgenden löschen") — intentionally the
// destructive half of Split. Planned non-spontaneous future instances are
// deleted; active/completed/cancelled/spontaneous rows remain as history.
func (s *TemplateSplitService) EndFromDate(ctx context.Context, in TemplateEndInput) (*TemplateEndResult, error) {
	if err := validateTemplateEndInput(in); err != nil {
		return nil, err
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return nil, &ScheduleError{Op: "end template", Err: errors.New("no tenant in context")}
	}

	old, err := s.loadTemplate(ctx, in.TemplateID)
	if err != nil {
		return nil, err
	}

	cappedSchedules, err := s.deps.ScheduleRepo.CapValidUntil(ctx, old.ID, in.EffectiveDate)
	if err != nil {
		return nil, &ScheduleError{Op: "end template: cap schedules", Err: err}
	}
	cappedEnrollments, err := s.deps.EnrollmentRepo.CapActiveByGroup(ctx, old.ID, in.EffectiveDate)
	if err != nil {
		return nil, &ScheduleError{Op: "end template: cap enrollments", Err: err}
	}
	cappedSupervisors, err := s.deps.SupervisorRepo.CapActiveByGroup(ctx, old.ID, in.EffectiveDate)
	if err != nil {
		return nil, &ScheduleError{Op: "end template: cap supervisors", Err: err}
	}

	templateID := old.ID
	deleted, err := s.deps.InstanceRepo.DeletePlannedNonSpontaneousInWindow(ctx, in.EffectiveDate, nil, &templateID)
	if err != nil {
		return nil, &ScheduleError{Op: "end template: delete planned instances", Err: err}
	}

	s.getLogger().Info("template ended from date",
		slog.Int64("tenant_id", tenantID),
		slog.Int64("template_id", old.ID),
		slog.String("effective_date", in.EffectiveDate.String()),
		slog.Int64("deleted_instances", deleted),
		slog.Int64("capped_schedules", cappedSchedules),
		slog.Int64("capped_enrollments", cappedEnrollments),
		slog.Int64("capped_supervisors", cappedSupervisors),
	)

	return &TemplateEndResult{
		TemplateID:        old.ID,
		EffectiveDate:     in.EffectiveDate,
		DeletedInstances:  int(deleted),
		CappedSchedules:   cappedSchedules,
		CappedEnrollments: cappedEnrollments,
		CappedSupervisors: cappedSupervisors,
	}, nil
}

// validateSplitInput checks the semantic rules the handler's Bind cannot:
// dates, weekday range, week pattern, time order, activity type.
func validateSplitInput(in TemplateSplitInput) error {
	if in.TemplateID <= 0 {
		return fmt.Errorf("%w: template id is required", ErrSplitInvalidInput)
	}
	if in.Name == "" {
		return fmt.Errorf("%w: name is required", ErrSplitInvalidInput)
	}
	switch in.Type {
	case activitiesModel.GroupTypeCare, activitiesModel.GroupTypeActivity, activitiesModel.GroupTypeExternal:
	default:
		return fmt.Errorf("%w: invalid type %q (must be care, activity, or external)", ErrSplitInvalidInput, in.Type)
	}
	if in.RoomID <= 0 {
		return fmt.Errorf("%w: room_id is required", ErrSplitInvalidInput)
	}
	if in.CategoryID <= 0 {
		return fmt.Errorf("%w: category_id is required", ErrSplitInvalidInput)
	}
	if len(in.Weekdays) == 0 {
		return fmt.Errorf("%w: at least one weekday is required", ErrSplitInvalidInput)
	}
	for _, w := range in.Weekdays {
		if !activitiesModel.IsValidWeekday(w) {
			return fmt.Errorf("%w: invalid weekday %d (must be 1=Mon … 7=Sun)", ErrSplitInvalidInput, w)
		}
	}
	if !in.EndTime.After(in.StartTime) {
		return fmt.Errorf("%w: end_time must be after start_time", ErrSplitInvalidInput)
	}
	if wp := in.WeekPattern; wp != nil && (*wp < 0 || *wp > 2) {
		return fmt.Errorf("%w: week_pattern must be 0 (every), 1 (A), or 2 (B)", ErrSplitInvalidInput)
	}
	if in.EffectiveDate.IsZero() {
		return fmt.Errorf("%w: effective_date is required", ErrSplitInvalidInput)
	}
	if in.EffectiveDate.Before(timezone.TodayDate()) {
		return fmt.Errorf("%w: effective_date must not be in the past", ErrSplitInvalidInput)
	}
	// Reuses Group.ValidateTargetGroup() rather than re-implementing the
	// type-conditional invariant here (Rule 10) — the handler's Bind() also
	// runs this check, but the service defends itself independently since
	// TemplateSplitInput is a public entry point other callers could reach.
	if err := (&activitiesModel.Group{
		TargetGroupType:   in.TargetGroupType,
		TargetGradeLevel:  in.TargetGradeLevel,
		TargetSchoolClass: in.TargetSchoolClass,
		EducationGroupID:  in.EducationGroupID,
	}).ValidateTargetGroup(); err != nil {
		return fmt.Errorf("%w: %s", ErrSplitInvalidInput, err.Error())
	}
	return validateSplitMaterializationWindow(in)
}

func validateTemplateEndInput(in TemplateEndInput) error {
	if in.TemplateID <= 0 {
		return fmt.Errorf("%w: template id is required", ErrSplitInvalidInput)
	}
	if in.EffectiveDate.IsZero() {
		return fmt.Errorf("%w: effective_date is required", ErrSplitInvalidInput)
	}
	if in.EffectiveDate.Before(timezone.TodayDate()) {
		return fmt.Errorf("%w: effective_date must not be in the past", ErrSplitInvalidInput)
	}
	return nil
}

// validateSplitMaterializationWindow rejects self-inconsistent materialization
// windows up front so they surface as 400 instead of bubbling out of the
// materialization service as 500s:
//
//   - materialize_to before effective_date (window entirely before the split)
//   - materialize_from after materialize_to (inverted window)
//   - clamped span over MaxMaterializationWindowDays (would trip the
//     materializer's own "window exceeds" guard mid-split)
//
// The span check uses the same clamp materializeWindow applies (from never
// before the effective date), so a wide materialize_from that gets clamped
// into range still passes.
func validateSplitMaterializationWindow(in TemplateSplitInput) error {
	if in.MaterializeTo == nil {
		return nil
	}
	if in.MaterializeTo.Before(in.EffectiveDate) {
		return fmt.Errorf("%w: materialize_to must not be before effective_date", ErrSplitInvalidInput)
	}
	if in.MaterializeFrom == nil {
		return nil
	}
	if in.MaterializeFrom.After(*in.MaterializeTo) {
		return fmt.Errorf("%w: materialize_from must not be after materialize_to", ErrSplitInvalidInput)
	}
	clampedFrom := *in.MaterializeFrom
	if clampedFrom.Before(in.EffectiveDate) {
		clampedFrom = in.EffectiveDate
	}
	if clampedFrom.DaysUntil(*in.MaterializeTo)+1 > MaxMaterializationWindowDays {
		return fmt.Errorf("%w: materialization window exceeds %d days", ErrSplitInvalidInput, MaxMaterializationWindowDays)
	}
	return nil
}

// loadTemplate resolves the old template and enforces the split preconditions:
// it must exist in the current tenant, be a template, and not be archived.
func (s *TemplateSplitService) loadTemplate(ctx context.Context, id int64) (*activitiesModel.Group, error) {
	group, err := s.deps.GroupRepo.FindByID(ctx, id)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return nil, ErrSplitTemplateNotFound
		}
		return nil, &ScheduleError{Op: "split template: load template", Err: err}
	}
	if group == nil || !group.IsTemplate || group.ArchivedAt != nil {
		return nil, ErrSplitTemplateNotFound
	}
	return group, nil
}

// loadActiveRoster returns the old template's still-active enrollment and
// supervisor rows (valid_until IS NULL). Since migration 1.15.52 the partial
// unique indexes are period-scoped — (tenant, person, group,
// COALESCE(calendar_period_id, 0)) WHERE valid_until IS NULL — so a person
// can have SEVERAL active rows on the same group, one per calendar period.
// Carry-over must therefore dedupe per person before stamping the successor's
// calendar_period_id, or the insert violates the index (see
// createStudentRoster / createStaffRoster).
func (s *TemplateSplitService) loadActiveRoster(ctx context.Context, groupID int64) ([]*activitiesModel.StudentEnrollment, []*activitiesModel.SupervisorPlanned, error) {
	enrollments, err := s.deps.EnrollmentRepo.FindByGroupID(ctx, groupID)
	if err != nil {
		return nil, nil, &ScheduleError{Op: "split template: load enrollments", Err: err}
	}
	supervisors, err := s.deps.SupervisorRepo.FindByGroupID(ctx, groupID)
	if err != nil {
		return nil, nil, &ScheduleError{Op: "split template: load supervisors", Err: err}
	}

	activeEnrollments := make([]*activitiesModel.StudentEnrollment, 0, len(enrollments))
	for _, e := range enrollments {
		if e != nil && e.ValidUntil == nil {
			activeEnrollments = append(activeEnrollments, e)
		}
	}
	activeSupervisors := make([]*activitiesModel.SupervisorPlanned, 0, len(supervisors))
	for _, sp := range supervisors {
		if sp != nil && sp.ValidUntil == nil {
			activeSupervisors = append(activeSupervisors, sp)
		}
	}
	return activeEnrollments, activeSupervisors, nil
}

// createSuccessorGroup copies the old template and applies the updated fields.
// MaxParticipants falls back to the old template's value when not provided.
func (s *TemplateSplitService) createSuccessorGroup(ctx context.Context, old *activitiesModel.Group, in TemplateSplitInput, tenantID int64) (*activitiesModel.Group, error) {
	maxParticipants := old.MaxParticipants
	if in.MaxParticipants != nil && *in.MaxParticipants > 0 {
		maxParticipants = *in.MaxParticipants
	}
	roomID := in.RoomID
	group := &activitiesModel.Group{
		Name:              in.Name,
		MaxParticipants:   maxParticipants,
		IsOpen:            old.IsOpen,
		CategoryID:        in.CategoryID,
		PlannedRoomID:     &roomID,
		CreatedBy:         old.CreatedBy,
		Type:              in.Type,
		EducationGroupID:  in.EducationGroupID,
		IsTemplate:        true,
		CalendarPeriodID:  in.CalendarPeriodID,
		TargetGroupType:   in.TargetGroupType,
		TargetGradeLevel:  in.TargetGradeLevel,
		TargetSchoolClass: in.TargetSchoolClass,
	}
	group.SetTenantID(tenantID)
	if err := s.deps.GroupRepo.Create(ctx, group); err != nil {
		return nil, &ScheduleError{Op: "split template: create successor template", Err: err}
	}
	return group, nil
}

// createSuccessorSchedules creates one schedule row per weekday, starting at
// the effective date (valid_from inclusive) with an open end (valid_until
// NULL).
func (s *TemplateSplitService) createSuccessorSchedules(ctx context.Context, groupID, timeframeID int64, in TemplateSplitInput, tenantID int64) ([]int64, error) {
	weekPattern := 0
	if in.WeekPattern != nil {
		weekPattern = *in.WeekPattern
	}
	scheduleIDs := make([]int64, 0, len(in.Weekdays))
	for _, weekday := range in.Weekdays {
		tfID := timeframeID
		validFrom := in.EffectiveDate
		sched := &activitiesModel.Schedule{
			Weekday:          weekday,
			TimeframeID:      &tfID,
			ActivityGroupID:  groupID,
			WeekPattern:      weekPattern,
			CalendarPeriodID: in.CalendarPeriodID,
			ValidFrom:        &validFrom, // never materialize before the split point
			ValidUntil:       nil,        // successor ends open-ended
		}
		sched.SetTenantID(tenantID)
		if err := s.deps.ScheduleRepo.Create(ctx, sched); err != nil {
			return nil, &ScheduleError{Op: "split template: create successor schedule", Err: err}
		}
		scheduleIDs = append(scheduleIDs, sched.ID)
	}
	return scheduleIDs, nil
}

// createStudentRoster writes the successor's enrollments. Explicit StudentIDs
// win; nil carries over the previously-active roster (selected_weekdays
// preserved). All new rows start at the effective date.
//
// The carry path dedupes per student: loadActiveRoster can return several
// active rows per student (one per calendar period since migration 1.15.52),
// and stamping them all with in.CalendarPeriodID would collide on
// idx_student_enrollments_active. The row matching in.CalendarPeriodID is
// preferred; selected_weekdays are unioned across the student's rows (an
// empty list means "all weekdays" and wins the union).
func (s *TemplateSplitService) createStudentRoster(ctx context.Context, groupID int64, in TemplateSplitInput, carried []*activitiesModel.StudentEnrollment, tenantID int64) error {
	rows := make([]*activitiesModel.StudentEnrollment, 0, len(carried))
	if in.StudentIDs != nil {
		for _, studentID := range sliceutil.UniquePositive(in.StudentIDs) {
			rows = append(rows, &activitiesModel.StudentEnrollment{StudentID: studentID})
		}
	} else {
		byStudent := make(map[int64][]*activitiesModel.StudentEnrollment, len(carried))
		order := make([]int64, 0, len(carried))
		for _, e := range carried {
			if _, seen := byStudent[e.StudentID]; !seen {
				order = append(order, e.StudentID)
			}
			byStudent[e.StudentID] = append(byStudent[e.StudentID], e)
		}
		for _, studentID := range order {
			group := byStudent[studentID]
			preferred := preferCarriedRow(group, in.CalendarPeriodID, func(e *activitiesModel.StudentEnrollment) *int64 { return e.CalendarPeriodID })
			rows = append(rows, &activitiesModel.StudentEnrollment{
				StudentID:        studentID,
				SelectedWeekdays: unionSelectedWeekdays(group, preferred),
			})
		}
	}
	for _, row := range rows {
		row.ActivityGroupID = groupID
		row.ValidFrom = in.EffectiveDate
		row.CalendarPeriodID = in.CalendarPeriodID
		row.SetTenantID(tenantID)
		if err := s.deps.EnrollmentRepo.Create(ctx, row); err != nil {
			return &ScheduleError{Op: "split template: create enrollment", Err: err}
		}
	}
	return nil
}

// createStaffRoster writes the successor's supervisors. Explicit StaffIDs win
// (primary derived from PrimaryStaffID); nil carries over the previously-
// active roster with each row's is_primary flag preserved.
//
// Like createStudentRoster, the carry path dedupes per staff member: several
// active rows per staff (one per calendar period, migration 1.15.52) collapse
// to one successor row, preferring the row matching in.CalendarPeriodID. The
// preferred row's is_primary flag is kept.
func (s *TemplateSplitService) createStaffRoster(ctx context.Context, groupID int64, in TemplateSplitInput, carried []*activitiesModel.SupervisorPlanned, tenantID int64) error {
	rows := make([]*activitiesModel.SupervisorPlanned, 0, len(carried))
	if in.StaffIDs != nil {
		for _, staffID := range sliceutil.UniquePositive(in.StaffIDs) {
			rows = append(rows, &activitiesModel.SupervisorPlanned{
				StaffID:   staffID,
				IsPrimary: in.PrimaryStaffID != nil && *in.PrimaryStaffID == staffID,
			})
		}
	} else {
		byStaff := make(map[int64][]*activitiesModel.SupervisorPlanned, len(carried))
		order := make([]int64, 0, len(carried))
		for _, sp := range carried {
			if _, seen := byStaff[sp.StaffID]; !seen {
				order = append(order, sp.StaffID)
			}
			byStaff[sp.StaffID] = append(byStaff[sp.StaffID], sp)
		}
		for _, staffID := range order {
			preferred := preferCarriedRow(byStaff[staffID], in.CalendarPeriodID, func(sp *activitiesModel.SupervisorPlanned) *int64 { return sp.CalendarPeriodID })
			rows = append(rows, &activitiesModel.SupervisorPlanned{
				StaffID:   staffID,
				IsPrimary: preferred.IsPrimary,
			})
		}
	}
	for _, row := range rows {
		row.GroupID = groupID
		row.ValidFrom = in.EffectiveDate
		row.CalendarPeriodID = in.CalendarPeriodID
		row.SetTenantID(tenantID)
		if err := s.deps.SupervisorRepo.Create(ctx, row); err != nil {
			return &ScheduleError{Op: "split template: create supervisor", Err: err}
		}
	}
	return nil
}

// preferCarriedRow picks the carried roster row whose calendar_period_id
// matches the successor's target period (both nil counts as a match); when
// none matches it falls back to the first row. Generic over the two roster
// row types so the per-person selection rule lives in one place.
func preferCarriedRow[T any](rows []T, targetPeriodID *int64, periodOf func(T) *int64) T {
	for _, row := range rows {
		p := periodOf(row)
		if (p == nil && targetPeriodID == nil) ||
			(p != nil && targetPeriodID != nil && *p == *targetPeriodID) {
			return row
		}
	}
	return rows[0]
}

// unionSelectedWeekdays merges selected_weekdays across one student's carried
// rows. An empty/nil list means "all weekdays" and dominates the union; the
// result preserves ascending weekday order. With a single row this returns
// the preferred row's list unchanged.
func unionSelectedWeekdays(rows []*activitiesModel.StudentEnrollment, preferred *activitiesModel.StudentEnrollment) []int {
	if len(rows) == 1 {
		return preferred.SelectedWeekdays
	}
	seen := make(map[int]struct{})
	for _, e := range rows {
		if len(e.SelectedWeekdays) == 0 {
			return nil // "all weekdays" wins the union
		}
		for _, wd := range e.SelectedWeekdays {
			seen[wd] = struct{}{}
		}
	}
	out := make([]int, 0, len(seen))
	for wd := activitiesModel.WeekdayMonday; wd <= activitiesModel.WeekdaySunday; wd++ {
		if _, ok := seen[wd]; ok {
			out = append(out, wd)
		}
	}
	return out
}

// materializeWindow re-materializes the requested window, clamped so it never
// starts before the effective date. Requires both bounds; an inverted clamped
// window is a silent no-op (nothing to materialize before the split point).
func (s *TemplateSplitService) materializeWindow(ctx context.Context, in TemplateSplitInput) (*MaterializationResult, error) {
	if in.MaterializeFrom == nil || in.MaterializeTo == nil {
		return nil, nil
	}
	from := *in.MaterializeFrom
	if from.Before(in.EffectiveDate) {
		from = in.EffectiveDate
	}
	if from.After(*in.MaterializeTo) {
		s.getLogger().Debug("template split: materialization window empty after clamping to effective date",
			slog.String("from", from.String()),
			slog.String("to", in.MaterializeTo.String()),
		)
		return nil, nil
	}
	mat, err := s.deps.Materialization.MaterializeForTenant(ctx, from, *in.MaterializeTo, MaterializationSourceManual)
	if err != nil {
		return nil, &ScheduleError{Op: "split template: materialize", Err: err}
	}
	return mat, nil
}

// FindOrCreateTimeframe returns the id of an existing schedule.timeframes row
// matching the [start, end] clock window or inserts a fresh one. Description
// is set to descHint on first creation as a debug hint, but is informational
// only — lookups go by time window. Reusing existing timeframes keeps the
// schedule.timeframes table from growing one row per template — common slots
// (12:00–12:50) end up shared across templates.
//
// Shared by the POST /templates create handler, the PUT /templates/{id}
// update handler and the template split service so the find-or-create rule
// lives in exactly one place.
func FindOrCreateTimeframe(ctx context.Context, repo scheduleModel.TimeframeRepository, start, end time.Time, descHint string) (int64, error) {
	existing, err := repo.FindByTimeRange(ctx, start, end)
	if err == nil {
		for _, tf := range existing {
			if tf == nil {
				continue
			}
			// Match exact clock times; FindByTimeRange may return overlapping
			// windows depending on impl, so be precise. Do not use
			// time.Time.Equal here: schedule.timeframes stores SQL TIME, and
			// drivers may decode TIME with a different date anchor than the
			// caller's HH:MM parser uses.
			if timezone.SameClockTime(tf.StartTime, start) && tf.EndTime != nil && timezone.SameClockTime(*tf.EndTime, end) {
				return tf.ID, nil
			}
		}
	}

	endCopy := end
	tf := &scheduleModel.Timeframe{
		StartTime:   start,
		EndTime:     &endCopy,
		IsActive:    true,
		Description: fmt.Sprintf("auto: %s", descHint),
	}
	tf.SetTenantID(tenant.FromContext(ctx))
	if err := repo.Create(ctx, tf); err != nil {
		return 0, fmt.Errorf("create timeframe: %w", err)
	}
	return tf.ID, nil
}
