package compose

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// TemplateSplitInput mirrors the update-template request plus the split
// controls. StartTime/EndTime are wall-clock values (parsed from HH:MM).
// StudentIDs/StaffIDs follow tri-state semantics: nil = carry over the
// previously-active roster of the old template; non-nil (including empty) =
// use exactly the provided ids. PrimaryStaffID applies to an explicitly
// provided StaffIDs roster; carried-over supervisors keep their own
// is_primary flag.
type TemplateSplitInput struct {
	TemplateID              int64
	EffectiveDate           timezone.Date
	Name                    string
	Type                    string // care | activity | external
	Weekdays                []int  // ISO 8601, Mo=1 … Su=7
	StartTime               time.Time
	EndTime                 time.Time
	RoomID                  int64
	CategoryID              int64
	PlanningTrackID         *int64
	PlanningTrackIDProvided bool
	MaxParticipants         *int
	MaxParticipantsProvided bool
	// RequiredStaff is the manual Personalbedarf override (#1839) for the
	// successor Group, already normalized (nil = clear/derive). It is only
	// applied when RequiredStaffProvided is true; otherwise the successor
	// inherits the source template's override. This omitted-vs-null split is
	// what lets a "this and following" edit actually CLEAR an override.
	RequiredStaff         *int
	RequiredStaffProvided bool
	WeekPattern           *int // 0=every week, 1=A, 2=B; nil = 0
	CalendarPeriodID      *int64
	EducationGroupID      *int64
	// Zielgruppe (target-group) fields, carried onto the successor Group
	// (see createSuccessorGroup). "gruppe" reuses EducationGroupID above.
	TargetGroupType    string
	TargetGradeLevel   *int16
	TargetSchoolClass  *string
	Targets            []*activitiesModel.GroupTarget
	targetsProvided    bool
	targetsPresenceSet bool
	// Offering-source rule (#2137), presence-aware like RequiredStaff (#2147
	// review round 14): an omitted field inherits the old template's source
	// respectively filter — the pre-#2137 split body must not silently cut the
	// successor off its Betreuungsangebot feed — while a provided field is
	// authoritative (nil clears the source). Only meaningful while the split
	// keeps the 'angebot' Zielgruppe; every other type drops the rule (DB
	// CHECK). resolveSuccessorOfferingSource merges both pairs against the old
	// template before anything is written.
	SourceCareOfferingIDs         []int64
	SourceCareOfferingIDsProvided bool
	SourceGradeLevels             []int
	SourceGradeLevelsProvided     bool
	SourceSchoolClasses           []string
	SourceSchoolClassesProvided   bool
	// Notes is the durable Wochennotiz (#1837 follow-up) for the successor
	// Group, already normalized (nil = clear). Only applied when NotesProvided
	// is true; otherwise the successor inherits the source template's note. Same
	// omitted-vs-null split as RequiredStaff so a "this and following" edit can
	// actually CLEAR the series note.
	Notes         *string
	NotesProvided bool
	// ListKind classifies the successor for printable daily lists (#1565),
	// already normalized (nil = clear). Same omitted-vs-null split as
	// RequiredStaff/Notes: only applied when ListKindProvided is true,
	// otherwise the successor inherits the source template's list kind —
	// without this a plain "this and following" edit would silently drop the
	// series from its automatic Randstunden/Lernzeit/AG/Mensa list.
	ListKind         *string
	ListKindProvided bool
	StudentIDs       []int64
	StaffIDs         []int64
	PrimaryStaffID   *int64
	// WeekdayAssignments carries the per-weekday roster deviations (#2129)
	// onto the successor. It only applies to the explicit-roster path: a
	// carried-over roster keeps each row's own weekday scope.
	WeekdayAssignments []timetable.WeekdayRosterAssignment
	MaterializeFrom    *timezone.Date
	MaterializeTo      *timezone.Date
	// GradeLevelMax is the caller's validated snapshot of
	// enrollment.grade_level_max. Missing or out-of-range values are rejected.
	GradeLevelMax int
	// ActorAccountID stamps Änderungsprotokoll entries for deviations the
	// split drops (#1886); nil records actor-less events.
	ActorAccountID *int64
}

// validateSplitInput checks the semantic rules the handler's Bind cannot:
// dates, weekday range, week pattern, time order, activity type.
func validateSplitInput(in *TemplateSplitInput, today timezone.Date, rules SchoolClassRules) error {
	if err := validateSplitTemplateFields(*in, rules); err != nil {
		return err
	}
	if err := validateSplitRecurrence(*in, today); err != nil {
		return err
	}
	if err := validateSplitWeekdayAssignments(*in); err != nil {
		return err
	}
	if err := validateSplitTargetGroup(in); err != nil {
		return err
	}
	return validateSplitMaterializationWindow(*in)
}

func validateSplitWeekdayAssignments(in TemplateSplitInput) error {
	if len(in.WeekdayAssignments) == 0 {
		return nil
	}
	if in.StudentIDs == nil || in.StaffIDs == nil {
		return fmt.Errorf("%w: weekday assignments require explicit student and staff rosters", timetable.ErrSplitInvalidInput)
	}
	if _, err := indexWeekdayAssignments(in.Weekdays, in.WeekdayAssignments); err != nil {
		return fmt.Errorf("%w: %s", timetable.ErrSplitInvalidInput, err.Error())
	}
	return nil
}

func validateSplitTemplateFields(in TemplateSplitInput, rules SchoolClassRules) error {
	if in.TemplateID <= 0 {
		return fmt.Errorf("%w: template id is required", timetable.ErrSplitInvalidInput)
	}
	if in.Name == "" {
		return fmt.Errorf("%w: name is required", timetable.ErrSplitInvalidInput)
	}
	switch in.Type {
	case activitiesModel.GroupTypeCare, activitiesModel.GroupTypeActivity, activitiesModel.GroupTypeExternal:
	default:
		return fmt.Errorf("%w: invalid type %q (must be care, activity, or external)", timetable.ErrSplitInvalidInput, in.Type)
	}
	if in.RoomID <= 0 {
		return fmt.Errorf("%w: room_id is required", timetable.ErrSplitInvalidInput)
	}
	if in.CategoryID <= 0 {
		return fmt.Errorf("%w: category_id is required", timetable.ErrSplitInvalidInput)
	}
	if in.MaxParticipants != nil && *in.MaxParticipants <= 0 {
		return fmt.Errorf("%w: max_participants must be greater than zero when set", timetable.ErrSplitInvalidInput)
	}
	if err := validateTemplateGradeLevelMax(rules, in.GradeLevelMax); err != nil {
		return fmt.Errorf("%w: %s", timetable.ErrSplitInvalidInput, err.Error())
	}
	return nil
}

func validateSplitRecurrence(in TemplateSplitInput, today timezone.Date) error {
	if len(in.Weekdays) == 0 {
		return fmt.Errorf("%w: at least one weekday is required", timetable.ErrSplitInvalidInput)
	}
	for _, w := range in.Weekdays {
		if !activitiesModel.IsValidWeekday(w) {
			return fmt.Errorf("%w: invalid weekday %d (must be 1=Mon … 7=Sun)", timetable.ErrSplitInvalidInput, w)
		}
		if w > activitiesModel.WeekdayFriday {
			return fmt.Errorf("%w: timetable templates can only be scheduled from Monday to Friday", timetable.ErrSplitInvalidInput)
		}
	}
	if !in.EndTime.After(in.StartTime) {
		return fmt.Errorf("%w: end_time must be after start_time", timetable.ErrSplitInvalidInput)
	}
	if wp := in.WeekPattern; wp != nil && (*wp < 0 || *wp > 2) {
		return fmt.Errorf("%w: week_pattern must be 0 (every), 1 (A), or 2 (B)", timetable.ErrSplitInvalidInput)
	}
	if in.EffectiveDate.IsZero() {
		return fmt.Errorf("%w: effective_date is required", timetable.ErrSplitInvalidInput)
	}
	if in.EffectiveDate.Before(today) {
		return fmt.Errorf("%w: effective_date must not be in the past", timetable.ErrSplitInvalidInput)
	}
	return nil
}

func validateSplitTargetGroup(in *TemplateSplitInput) error {
	targets, err := normalizeDynamicTargets(
		in.TargetGroupType,
		in.TargetGradeLevel,
		in.TargetSchoolClass,
		in.EducationGroupID,
		in.Targets,
	)
	if err != nil {
		return fmt.Errorf("%w: %w", timetable.ErrSplitInvalidInput, err)
	}
	in.Targets = targets
	mirrorSplitTargets(in)
	target := &activitiesModel.Group{
		TargetGroupType:   in.TargetGroupType,
		TargetGradeLevel:  in.TargetGradeLevel,
		TargetSchoolClass: in.TargetSchoolClass,
		EducationGroupID:  in.EducationGroupID,
	}
	if err := target.ValidateTargetGroup(); err != nil {
		return fmt.Errorf("%w: %s", timetable.ErrSplitInvalidInput, err.Error())
	}
	in.TargetGroupType = target.TargetGroupType
	in.TargetSchoolClass = target.TargetSchoolClass
	return nil
}

func validateTemplateEndInput(in timetable.EndTemplateCommand, today timezone.Date) error {
	if in.TemplateID <= 0 {
		return fmt.Errorf("%w: template id is required", timetable.ErrSplitInvalidInput)
	}
	if in.EffectiveDate.IsZero() {
		return fmt.Errorf("%w: effective_date is required", timetable.ErrSplitInvalidInput)
	}
	if in.EffectiveDate.Before(today) {
		return fmt.Errorf("%w: effective_date must not be in the past", timetable.ErrSplitInvalidInput)
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
		return fmt.Errorf("%w: materialize_to must not be before effective_date", timetable.ErrSplitInvalidInput)
	}
	if in.MaterializeFrom == nil {
		return nil
	}
	if in.MaterializeFrom.After(*in.MaterializeTo) {
		return fmt.Errorf("%w: materialize_from must not be after materialize_to", timetable.ErrSplitInvalidInput)
	}
	clampedFrom := *in.MaterializeFrom
	if clampedFrom.Before(in.EffectiveDate) {
		clampedFrom = in.EffectiveDate
	}
	if clampedFrom.DaysUntil(*in.MaterializeTo)+1 > timetable.MaxMaterializationWindowDays {
		return fmt.Errorf("%w: materialization window exceeds %d days", timetable.ErrSplitInvalidInput, timetable.MaxMaterializationWindowDays)
	}
	return nil
}

// loadTemplate resolves the old template and enforces the split preconditions:
// it must exist in the current tenant, be a template, and not be archived.
func (s *TemplateSplitService) loadTemplate(ctx context.Context, id int64) (*activitiesModel.Group, error) {
	group, err := s.deps.GroupRepo.FindByID(ctx, id)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return nil, timetable.ErrSplitTemplateNotFound
		}
		return nil, &ScheduleError{Op: "split template: load template", Err: err}
	}
	if group == nil || !group.IsTemplate || group.ArchivedAt != nil {
		return nil, timetable.ErrSplitTemplateNotFound
	}
	return group, nil
}

// normalizeEffectiveDateInSegment enforces the editable segment interval
// [valid_from, valid_until). The check runs after acquiring the tenant
// recurrence gate, so a concurrent PUT/split/end cannot change the envelope
// between this read and the following cap. Split rejects a date before the
// segment start. End clamps it to valid_from, allowing the UI's default
// "today" to delete an entire future successor without inverting its rows.
// Both operations reject dates at or after the exclusive end because those
// dates do not belong to this segment and could create a second successor at
// an existing split boundary.
func (s *TemplateSplitService) normalizeEffectiveDateInSegment(
	ctx context.Context,
	templateID int64,
	effectiveDate timezone.Date,
	op string,
	clampBeforeStart bool,
) (timezone.Date, *timezone.Date, error) {
	schedules, err := s.deps.ScheduleRepo.FindByGroupID(ctx, templateID)
	if err != nil {
		return timezone.Date(""), nil, &ScheduleError{Op: op + ": load schedule envelope", Err: err}
	}
	validFrom, validUntil, err := commonScheduleValidityEnvelope(schedules)
	if err != nil {
		return timezone.Date(""), nil, &ScheduleError{Op: op + ": inspect schedule envelope", Err: err}
	}
	if validFrom != nil && effectiveDate.Before(*validFrom) {
		if !clampBeforeStart {
			return timezone.Date(""), nil, fmt.Errorf(
				"%w: effective_date %s is before segment valid_from %s",
				timetable.ErrSplitInvalidInput,
				effectiveDate.String(),
				validFrom.String(),
			)
		}
		effectiveDate = *validFrom
	}
	if validUntil != nil && !effectiveDate.Before(*validUntil) {
		return timezone.Date(""), nil, fmt.Errorf(
			"%w: effective_date %s must be before segment valid_until %s",
			timetable.ErrSplitInvalidInput,
			effectiveDate.String(),
			validUntil.String(),
		)
	}
	return effectiveDate, cloneOptionalDate(validUntil), nil
}
