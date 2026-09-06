package schedule

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/internal/schoolclass"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/tenant"
)

const createTemplateOp = "create template"

// CreateTemplateInput carries everything POST /timetable/templates needs to
// materialize a recurring template in one atomic write: the template fields,
// its weekday recurrence, an underlying timeframe (resolved by clock window),
// and the initial roster. The caller resolves the tenant-scoped preconditions
// (grade-level cap, roster valid_from) and passes them in.
type CreateTemplateInput struct {
	Name              string
	Type              string
	Weekdays          []int
	StartTime         time.Time
	EndTime           time.Time
	RoomID            int64
	CategoryID        int64
	PlanningTrackID   *int64
	MaxParticipants   int
	RequiredStaff     *int
	WeekPattern       int
	CalendarPeriodID  *int64
	EducationGroupID  *int64
	TargetGroupType   string
	TargetGradeLevel  *int16
	TargetSchoolClass *string
	Targets           []*activitiesModel.GroupTarget
	// SourceCareOfferingIDs/SourceGradeLevels/SourceSchoolClasses declare the
	// offering-source rule (#2137, multi-source follow-up, #2482). With
	// sources set, the roster is seeded from the union of the offerings'
	// approved enrollments and StudentIDs must be empty. Grade and class
	// filter are mutually exclusive.
	SourceCareOfferingIDs []int64
	SourceGradeLevels     []int
	SourceSchoolClasses   []string
	// ListKind classifies the template for printable daily lists (#1565);
	// nil = no list kind.
	ListKind       *string
	Notes          *string
	StudentIDs     []int64
	StaffIDs       []int64
	PrimaryStaffID *int64
	// WeekdayAssignments carries the per-weekday deviations from the shared
	// roster above (issue #2129). Empty = the roster is identical on every
	// weekday of the series, which is the pre-#2129 shape.
	WeekdayAssignments []WeekdayRosterAssignment
	CreatedBy          *int64
	RosterValidFrom    timezone.Date
	// ScheduleValidFrom is the optional series start (#2135): every schedule
	// row gets it as valid_from, so the materializer skips earlier dates
	// (scheduleNotStartedOn). nil = the series starts with the planning period.
	ScheduleValidFrom *timezone.Date
	// GradeLevelMax is the caller's validated snapshot of
	// enrollment.grade_level_max, used to cap Jahrgang targets.
	GradeLevelMax int
}

// CreateTemplateResult reports what the planner UI needs after a create: the
// new template id, the (possibly reused) timeframe id, and the schedule ids.
type CreateTemplateResult struct {
	TemplateID  int64
	TimeframeID int64
	ScheduleIDs []int64
}

// ConvertInstanceToSeriesInput turns one existing planned occurrence into the
// seed of a new recurring template. Template contains the complete series
// definition; InstanceNotes is the per-occurrence note that stays on the seed.
type ConvertInstanceToSeriesInput struct {
	InstanceID     int64
	Template       CreateTemplateInput
	InstanceNotes  *string
	ActorAccountID *int64
}

// ConvertInstanceToSeriesResult identifies both sides of a successful atomic
// conversion. LinkedInstanceID is always the pre-existing occurrence ID.
type ConvertInstanceToSeriesResult struct {
	TemplateID       int64
	TimeframeID      int64
	ScheduleIDs      []int64
	LinkedInstanceID int64
}

// InstanceSeriesConverter is the single write seam used by the timetable API
// when a planner changes a one-off occurrence into a recurring series.
type InstanceSeriesConverter interface {
	ConvertInstanceToSeries(context.Context, ConvertInstanceToSeriesInput) (*ConvertInstanceToSeriesResult, error)
}

// CreateTemplate persists a recurring template (activities.groups with
// is_template=true) together with its weekday schedules, underlying timeframe,
// and initial roster in one tenant transaction. Grade-limit and education-group
// validation run before any write, so a rejected request leaves no orphan rows.
func (s *TimetableDataService) CreateTemplate(ctx context.Context, in CreateTemplateInput) (*CreateTemplateResult, error) {
	tenantID, err := s.validateTemplateCreateRequest(ctx, in)
	if err != nil {
		return nil, err
	}

	var result CreateTemplateResult
	err = tenant.WithTenantTx(ctx, s.deps.DB, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		return s.createTemplateLocked(txCtx, in, tenantID, &result)
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *TimetableDataService) validateTemplateCreateRequest(ctx context.Context, in CreateTemplateInput) (int64, error) {
	if err := validateTemplateCreateInput(in); err != nil {
		return 0, &ScheduleError{Op: createTemplateOp, Err: err}
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return 0, &ScheduleError{Op: createTemplateOp, Err: errors.New("no tenant in context")}
	}
	if s.deps.DB == nil {
		return 0, &ScheduleError{Op: createTemplateOp, Err: errors.New("database is not configured")}
	}
	if s.deps.ActivityGroupRepo == nil || s.deps.ActivityScheduleRepo == nil ||
		s.deps.StudentEnrollmentRepo == nil || s.deps.ActivitySupervisorRepo == nil ||
		s.deps.TimeframeRepo == nil || s.deps.EducationGroupRepo == nil ||
		s.deps.ActivityCategoryRepo == nil {
		return 0, &ScheduleError{Op: createTemplateOp, Err: errors.New("template repositories are not configured")}
	}
	if err := s.ValidateTemplateEducationGroup(ctx, in.EducationGroupID); err != nil {
		return 0, err
	}
	return tenantID, nil
}

func validateTemplateCreateInput(in CreateTemplateInput) error {
	if in.Name == "" {
		return errors.New("name is required")
	}
	if len(in.Weekdays) == 0 {
		return errors.New("at least one weekday is required")
	}
	for _, weekday := range in.Weekdays {
		if !activitiesModel.IsValidWeekday(weekday) {
			return fmt.Errorf("invalid weekday %d", weekday)
		}
		if weekday > activitiesModel.WeekdayFriday {
			return errors.New("timetable templates can only be scheduled from Monday to Friday")
		}
	}
	if in.WeekPattern < 0 || in.WeekPattern > 2 {
		return errors.New("week pattern must be 0, 1, or 2")
	}
	if in.CategoryID <= 0 {
		return errors.New("category id is required")
	}
	if in.RoomID <= 0 {
		return errors.New("room id is required")
	}
	if in.MaxParticipants < 0 {
		return errors.New("max participants cannot be negative")
	}
	if in.CalendarPeriodID != nil && *in.CalendarPeriodID <= 0 {
		return errors.New("calendar period id must be positive when set")
	}
	if in.RosterValidFrom.IsZero() {
		return errors.New("roster valid_from is required")
	}
	if err := validateOfferingSourceInput(
		in.SourceCareOfferingIDs, in.SourceGradeLevels, in.SourceSchoolClasses,
		in.TargetGroupType, in.StudentIDs, in.WeekdayAssignments,
	); err != nil {
		return err
	}
	return nil
}

// validateOfferingSourceInput enforces the offering-source request contract
// shared by template create and update (#2137): a grade filter only together
// with a source, a source only on 'angebot' templates, filter values within
// the supported grade bounds and free of duplicates (mirroring
// Group.ValidateTargetGroup, which direct service callers bypass), and no
// manual roster next to a source (the roster is derived, a snapshot would
// silently drift).
// Callers persist the class filter as given; trimming and the nil-for-empty
// canonicalization happen in activities.Group.ValidateTargetGroup (create)
// and in the API layer (update), and matching normalizes at compare time.
func validateOfferingSourceInput(
	sourceOfferingIDs []int64,
	gradeLevels []int,
	schoolClasses []string,
	targetGroupType string,
	studentIDs []int64,
	weekdayAssignments []WeekdayRosterAssignment,
) error {
	if len(sourceOfferingIDs) == 0 {
		if len(gradeLevels) > 0 {
			return fmt.Errorf("%w: source_grade_levels requires source_care_offering_ids", ErrOfferingSourceInvalid)
		}
		if len(schoolClasses) > 0 {
			return fmt.Errorf("%w: source_school_classes requires source_care_offering_ids", ErrOfferingSourceInvalid)
		}
		return nil
	}
	seenOfferings := make(map[int64]bool, len(sourceOfferingIDs))
	for _, offeringID := range sourceOfferingIDs {
		if offeringID <= 0 {
			return fmt.Errorf("%w: source_care_offering_ids entries must be positive", ErrOfferingSourceInvalid)
		}
		if seenOfferings[offeringID] {
			return fmt.Errorf("%w: source_care_offering_ids must not contain duplicates", ErrOfferingSourceInvalid)
		}
		seenOfferings[offeringID] = true
	}
	if targetGroupType != activitiesModel.TargetGroupTypeAngebot {
		return fmt.Errorf("%w: source_care_offering_ids requires target group type 'angebot'", ErrOfferingSourceInvalid)
	}
	seenGrades := make(map[int]bool, len(gradeLevels))
	for _, level := range gradeLevels {
		if level < schoolclass.MinGradeLevel || level > schoolclass.MaxGradeLevel {
			return fmt.Errorf(
				"%w: source_grade_levels entries must be between %d and %d",
				ErrOfferingSourceInvalid, schoolclass.MinGradeLevel, schoolclass.MaxGradeLevel,
			)
		}
		if seenGrades[level] {
			return fmt.Errorf("%w: source_grade_levels must not contain duplicates", ErrOfferingSourceInvalid)
		}
		seenGrades[level] = true
	}
	// Same rule and wording as activities.Group.ValidateTargetGroup, which
	// direct service callers bypass (#2482): the class filter is mutually
	// exclusive with the Jahrgang filter, so no caller ever has to decide
	// whether the two AND or OR.
	normalizedClasses, err := activitiesModel.NormalizeSourceSchoolClasses(schoolClasses)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrOfferingSourceInvalid, err.Error())
	}
	if len(gradeLevels) > 0 && len(normalizedClasses) > 0 {
		return fmt.Errorf("%w: source_school_classes and source_grade_levels cannot be combined", ErrOfferingSourceInvalid)
	}
	if len(studentIDs) > 0 {
		return fmt.Errorf("%w: student_ids must be empty when an offering source is set", ErrOfferingSourceInvalid)
	}
	// Per-weekday CHILD lists (#2129) are editor-owned snapshots; a sourced
	// roster is server-managed. Letting both in would plan children twice.
	// Per-weekday staff on sourced templates is a possible follow-up — the
	// editor hides the whole weekday section for sourced templates today, so
	// reject staff rows too instead of silently accepting a half.
	if len(weekdayAssignments) > 0 {
		return fmt.Errorf("%w: weekday_assignments must be empty when an offering source is set", ErrOfferingSourceInvalid)
	}
	return nil
}

func (s *TimetableDataService) createTemplateLocked(
	ctx context.Context,
	in CreateTemplateInput,
	tenantID int64,
	result *CreateTemplateResult,
) error {
	// The tenant-wide recurrence gate serializes this create with every other
	// recurrence writer — update, split, materialization, and the enrollment
	// decision fan-out. An offering-sourced template seeds its roster from the
	// approved enrollments inside this transaction; without the gate a
	// concurrent approval and this create can each miss the other's
	// uncommitted rows, leaving the child off the new template until an
	// unrelated resync (#2147 review).
	if err := lockTenantRecurrenceWrites(ctx, s.deps.DB); err != nil {
		return &ScheduleError{Op: "create template: lock recurrence", Err: err}
	}
	// Validation before any write: a rejected request must not strand a
	// timeframe or a half-built template.
	if err := s.ValidateTemplateEducationGroup(ctx, in.EducationGroupID); err != nil {
		return err
	}
	targets, err := normalizeDynamicTargets(in.TargetGroupType, in.TargetGradeLevel, in.TargetSchoolClass, in.EducationGroupID, in.Targets)
	if err != nil {
		return err
	}
	if err := validateDynamicTargets(ctx, s, in.GradeLevelMax, nil, nil, targets); err != nil {
		return err
	}
	if err := validateAssignableCategory(ctx, s.deps.ActivityCategoryRepo, in.CategoryID, "create template: validate category"); err != nil {
		return err
	}
	if err := validateAssignablePlanningTrack(ctx, s.deps.PlanningTrackRepo, in.PlanningTrackID, nil); err != nil {
		return err
	}
	applyTargetMirrorToCreateInput(&in, targets)
	// Before any write: an unknown or period-incompatible source would
	// otherwise only fail on the group insert's FK (500) instead of the
	// client-correctable 400 the resync produces (#2147 review round 18).
	if err := s.validateOfferingSourceReference(ctx, in.SourceCareOfferingIDs, nil, in.CalendarPeriodID, "create template: validate offering source"); err != nil {
		return err
	}

	timeframeID, err := s.FindOrCreateTimeframe(ctx, in.StartTime, in.EndTime, in.Name)
	if err != nil {
		return &ScheduleError{Op: "create template: resolve timeframe", Err: err}
	}

	roomID := in.RoomID
	group := &activitiesModel.Group{
		Name:                  in.Name,
		MaxParticipants:       in.MaxParticipants,
		RequiredStaff:         in.RequiredStaff,
		IsOpen:                true,
		CategoryID:            in.CategoryID,
		PlanningTrackID:       in.PlanningTrackID,
		PlannedRoomID:         &roomID,
		Type:                  in.Type,
		EducationGroupID:      in.EducationGroupID,
		IsTemplate:            true,
		CreatedBy:             in.CreatedBy,
		CalendarPeriodID:      in.CalendarPeriodID,
		TargetGroupType:       in.TargetGroupType,
		TargetGradeLevel:      in.TargetGradeLevel,
		TargetSchoolClass:     in.TargetSchoolClass,
		SourceCareOfferingIDs: in.SourceCareOfferingIDs,
		SourceGradeLevels:     in.SourceGradeLevels,
		SourceSchoolClasses:   in.SourceSchoolClasses,
		ListKind:              in.ListKind,
		Notes:                 in.Notes,
	}
	group.SetTenantID(tenantID)
	if err := s.deps.ActivityGroupRepo.Create(ctx, group); err != nil {
		return &ScheduleError{Op: "create template: create group", Err: err}
	}
	targetRepo, ok := s.deps.ActivityGroupRepo.(activitiesModel.GroupTargetRepository)
	if !ok && len(targets) > 0 {
		return &ScheduleError{Op: "create template: replace targets", Err: errors.New("target repository is not configured")}
	}
	if ok {
		if err := targetRepo.ReplaceTargets(ctx, group.ID, targets); err != nil {
			return &ScheduleError{Op: "create template: replace targets", Err: err}
		}
	}

	scheduleIDs := make([]int64, 0, len(in.Weekdays))
	for _, weekday := range in.Weekdays {
		timeframe := timeframeID
		sched := &activitiesModel.Schedule{
			Weekday:          weekday,
			TimeframeID:      &timeframe,
			ActivityGroupID:  group.ID,
			WeekPattern:      in.WeekPattern,
			CalendarPeriodID: in.CalendarPeriodID,
			ValidFrom:        activityDatePtr(in.ScheduleValidFrom),
		}
		sched.SetTenantID(tenantID)
		if err := s.deps.ActivityScheduleRepo.Create(ctx, sched); err != nil {
			return &ScheduleError{Op: "create template: create schedule", Err: err}
		}
		scheduleIDs = append(scheduleIDs, sched.ID)
	}

	if err := s.createTemplateRoster(ctx, group.ID, in, tenantID); err != nil {
		return err
	}

	if len(in.SourceCareOfferingIDs) > 0 {
		if s.deps.ResyncOfferingRoster == nil {
			return &ScheduleError{Op: createTemplateOp, Err: errors.New("offering roster resync is not configured")}
		}
		if err := s.deps.ResyncOfferingRoster(ctx, OfferingRosterResyncInput{
			TemplateID:       group.ID,
			OfferingIDs:      in.SourceCareOfferingIDs,
			GradeLevels:      in.SourceGradeLevels,
			SchoolClasses:    in.SourceSchoolClasses,
			CalendarPeriodID: in.CalendarPeriodID,
			EffectiveFrom:    in.RosterValidFrom,
		}); err != nil {
			return &ScheduleError{Op: "create template: seed offering roster", Err: err}
		}
	}

	result.TemplateID = group.ID
	result.TimeframeID = timeframeID
	result.ScheduleIDs = scheduleIDs
	return nil
}

func normalizeDynamicTargets(targetType string, grade *int16, class *string, groupID *int64, targets []*activitiesModel.GroupTarget) ([]*activitiesModel.GroupTarget, error) {
	if len(targets) == 0 {
		switch targetType {
		case activitiesModel.TargetGroupTypeJahrgang:
			if grade != nil {
				targets = []*activitiesModel.GroupTarget{{TargetGroupType: targetType, TargetGradeLevel: grade}}
			}
		case activitiesModel.TargetGroupTypeKlasse:
			if class != nil {
				targets = []*activitiesModel.GroupTarget{{TargetGroupType: targetType, TargetSchoolClass: class}}
			}
		case activitiesModel.TargetGroupTypeGruppe:
			if groupID != nil {
				targets = []*activitiesModel.GroupTarget{{TargetGroupType: targetType, EducationGroupID: groupID}}
			}
		}
	}
	seen := make(map[string]struct{}, len(targets))
	normalized := make([]*activitiesModel.GroupTarget, 0, len(targets))
	for _, target := range targets {
		if target == nil {
			return nil, errors.New("target cannot be null")
		}
		copy := *target
		if copy.TargetGroupType == "" {
			copy.TargetGroupType = targetType
		}
		if copy.TargetGroupType != targetType {
			return nil, errors.New("all dynamic targets must match target_group_type")
		}
		if err := copy.Validate(); err != nil {
			return nil, err
		}
		key := copy.TargetGroupType
		switch copy.TargetGroupType {
		case activitiesModel.TargetGroupTypeJahrgang:
			key += fmt.Sprintf(":%d", *copy.TargetGradeLevel)
		case activitiesModel.TargetGroupTypeKlasse:
			key += ":" + strings.ToLower(*copy.TargetSchoolClass)
		case activitiesModel.TargetGroupTypeGruppe:
			key += fmt.Sprintf(":%d", *copy.EducationGroupID)
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, &copy)
	}
	if targetType == activitiesModel.TargetGroupTypeJahrgang || targetType == activitiesModel.TargetGroupTypeKlasse || targetType == activitiesModel.TargetGroupTypeGruppe {
		if len(normalized) == 0 {
			return nil, errors.New("at least one dynamic target is required")
		}
	} else if len(normalized) > 0 {
		return nil, errors.New("dynamic targets require jahrgang, klasse, or gruppe")
	}
	return normalized, nil
}

func validateDynamicTargets(ctx context.Context, s *TimetableDataService, gradeLevelMax int, existing *activitiesModel.Group, existingTargets, targets []*activitiesModel.GroupTarget) error {
	if err := ValidateTemplateTargetsGradeLimit(gradeLevelMax, existing, existingTargets, targets); err != nil {
		return err
	}
	for _, target := range targets {
		if err := s.ValidateTemplateEducationGroup(ctx, target.EducationGroupID); err != nil {
			return err
		}
	}
	return nil
}

func applyTargetMirrorToCreateInput(in *CreateTemplateInput, targets []*activitiesModel.GroupTarget) {
	in.TargetGradeLevel = nil
	in.TargetSchoolClass = nil
	if len(targets) == 0 {
		return
	}
	in.TargetGradeLevel = targets[0].TargetGradeLevel
	in.TargetSchoolClass = targets[0].TargetSchoolClass
	if in.TargetGroupType == activitiesModel.TargetGroupTypeGruppe {
		in.EducationGroupID = targets[0].EducationGroupID
	}
}

// createTemplateRoster seeds the fresh template's period-scoped roster. The
// close-open calls are defensive no-ops for a brand-new group but keep the
// write path identical to a roster replacement.
func (s *TimetableDataService) createTemplateRoster(
	ctx context.Context,
	groupID int64,
	in CreateTemplateInput,
	tenantID int64,
) error {
	roster, err := resolveTemplateRoster(in.Weekdays, in.StudentIDs, in.StaffIDs, in.PrimaryStaffID, in.WeekdayAssignments)
	if err != nil {
		return &ScheduleError{Op: "create template: resolve roster", Err: err}
	}

	if err := s.deps.StudentEnrollmentRepo.CloseOpenByGroupAndPeriod(ctx, groupID, in.CalendarPeriodID, activitiesModel.Date(in.RosterValidFrom)); err != nil {
		return &ScheduleError{Op: "create template: close enrollments", Err: err}
	}
	for _, row := range roster.Students {
		enrollment := &activitiesModel.StudentEnrollment{
			StudentID:        row.PersonID,
			ActivityGroupID:  groupID,
			ValidFrom:        activitiesModel.Date(in.RosterValidFrom),
			CalendarPeriodID: in.CalendarPeriodID,
			Weekday:          weekdayScopePtr(row.Weekday),
		}
		enrollment.SetTenantID(tenantID)
		if err := s.deps.StudentEnrollmentRepo.Create(ctx, enrollment); err != nil {
			return &ScheduleError{Op: "create template: create enrollment", Err: err}
		}
	}

	if err := s.deps.ActivitySupervisorRepo.CloseOpenByGroupAndPeriod(ctx, groupID, in.CalendarPeriodID, activitiesModel.Date(in.RosterValidFrom)); err != nil {
		return &ScheduleError{Op: "create template: close supervisors", Err: err}
	}
	for _, row := range roster.Staff {
		supervisor := &activitiesModel.SupervisorPlanned{
			StaffID:          row.PersonID,
			GroupID:          groupID,
			IsPrimary:        row.IsPrimary,
			ValidFrom:        activitiesModel.Date(in.RosterValidFrom),
			CalendarPeriodID: in.CalendarPeriodID,
			Weekday:          weekdayScopePtr(row.Weekday),
		}
		supervisor.SetTenantID(tenantID)
		if err := s.deps.ActivitySupervisorRepo.Create(ctx, supervisor); err != nil {
			return &ScheduleError{Op: "create template: create supervisor", Err: err}
		}
	}
	return nil
}
