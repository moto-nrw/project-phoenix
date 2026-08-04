package schedule

import (
	"context"
	"errors"
	"fmt"
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
	// SourceCareOfferingID/SourceGradeLevels declare the offering-source rule
	// (#2137). With a source set, the roster is seeded from the offering's
	// approved enrollments and StudentIDs must be empty.
	SourceCareOfferingID *int64
	SourceGradeLevels    []int
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
	if in.CalendarPeriodID != nil && *in.CalendarPeriodID <= 0 {
		return errors.New("calendar period id must be positive when set")
	}
	if in.RosterValidFrom.IsZero() {
		return errors.New("roster valid_from is required")
	}
	if err := validateOfferingSourceInput(in.SourceCareOfferingID, in.SourceGradeLevels, in.TargetGroupType, in.StudentIDs, in.WeekdayAssignments); err != nil {
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
func validateOfferingSourceInput(sourceOfferingID *int64, gradeLevels []int, targetGroupType string, studentIDs []int64, weekdayAssignments []WeekdayRosterAssignment) error {
	if sourceOfferingID == nil {
		if len(gradeLevels) > 0 {
			return fmt.Errorf("%w: source_grade_levels requires source_care_offering_id", ErrOfferingSourceInvalid)
		}
		return nil
	}
	if *sourceOfferingID <= 0 {
		return fmt.Errorf("%w: source_care_offering_id must be positive", ErrOfferingSourceInvalid)
	}
	if targetGroupType != activitiesModel.TargetGroupTypeAngebot {
		return fmt.Errorf("%w: source_care_offering_id requires target group type 'angebot'", ErrOfferingSourceInvalid)
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
	// Validation before any write: a rejected request must not strand a
	// timeframe or a half-built template.
	if err := ValidateTemplateTargetGradeLimit(in.GradeLevelMax, nil, in.TargetGroupType, in.TargetGradeLevel); err != nil {
		return err
	}
	if err := validateAssignableCategory(ctx, s.deps.ActivityCategoryRepo, in.CategoryID, "create template: validate category"); err != nil {
		return err
	}
	if err := validateAssignablePlanningTrack(ctx, s.deps.PlanningTrackRepo, in.PlanningTrackID, nil); err != nil {
		return err
	}

	timeframeID, err := s.FindOrCreateTimeframe(ctx, in.StartTime, in.EndTime, in.Name)
	if err != nil {
		return &ScheduleError{Op: "create template: resolve timeframe", Err: err}
	}

	roomID := in.RoomID
	group := &activitiesModel.Group{
		Name:                 in.Name,
		MaxParticipants:      in.MaxParticipants,
		RequiredStaff:        in.RequiredStaff,
		IsOpen:               true,
		CategoryID:           in.CategoryID,
		PlanningTrackID:      in.PlanningTrackID,
		PlannedRoomID:        &roomID,
		Type:                 in.Type,
		EducationGroupID:     in.EducationGroupID,
		IsTemplate:           true,
		CreatedBy:            in.CreatedBy,
		CalendarPeriodID:     in.CalendarPeriodID,
		TargetGroupType:      in.TargetGroupType,
		TargetGradeLevel:     in.TargetGradeLevel,
		TargetSchoolClass:    in.TargetSchoolClass,
		SourceCareOfferingID: in.SourceCareOfferingID,
		SourceGradeLevels:    in.SourceGradeLevels,
		ListKind:             in.ListKind,
		Notes:                in.Notes,
	}
	group.SetTenantID(tenantID)
	if err := s.deps.ActivityGroupRepo.Create(ctx, group); err != nil {
		return &ScheduleError{Op: "create template: create group", Err: err}
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
			ValidFrom:        cloneOptionalDate(in.ScheduleValidFrom),
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

	if in.SourceCareOfferingID != nil {
		if s.deps.ResyncOfferingRoster == nil {
			return &ScheduleError{Op: createTemplateOp, Err: errors.New("offering roster resync is not configured")}
		}
		if err := s.deps.ResyncOfferingRoster(ctx, OfferingRosterResyncInput{
			TemplateID:       group.ID,
			OfferingID:       in.SourceCareOfferingID,
			GradeLevels:      in.SourceGradeLevels,
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

	if err := s.deps.StudentEnrollmentRepo.CloseOpenByGroupAndPeriod(ctx, groupID, in.CalendarPeriodID, in.RosterValidFrom); err != nil {
		return &ScheduleError{Op: "create template: close enrollments", Err: err}
	}
	for _, row := range roster.Students {
		enrollment := &activitiesModel.StudentEnrollment{
			StudentID:        row.PersonID,
			ActivityGroupID:  groupID,
			ValidFrom:        in.RosterValidFrom,
			CalendarPeriodID: in.CalendarPeriodID,
			Weekday:          weekdayScopePtr(row.Weekday),
		}
		enrollment.SetTenantID(tenantID)
		if err := s.deps.StudentEnrollmentRepo.Create(ctx, enrollment); err != nil {
			return &ScheduleError{Op: "create template: create enrollment", Err: err}
		}
	}

	if err := s.deps.ActivitySupervisorRepo.CloseOpenByGroupAndPeriod(ctx, groupID, in.CalendarPeriodID, in.RosterValidFrom); err != nil {
		return &ScheduleError{Op: "create template: close supervisors", Err: err}
	}
	for _, row := range roster.Staff {
		supervisor := &activitiesModel.SupervisorPlanned{
			StaffID:          row.PersonID,
			GroupID:          groupID,
			IsPrimary:        row.IsPrimary,
			ValidFrom:        in.RosterValidFrom,
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
