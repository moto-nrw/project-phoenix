package schedule

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// ErrInconsistentTemplateScheduleValidity reports corrupt template data where
// schedule rows belonging to one template do not share the same recurrence
// boundary. Replacing such rows must fail instead of silently widening one of
// the recurrences.
var ErrInconsistentTemplateScheduleValidity = errors.New("template schedules have inconsistent validity bounds")

// ErrTemplateWeekendWeekday is returned when an update tries to introduce a
// weekend weekday that was not already present on a legacy template.
var ErrTemplateWeekendWeekday = errors.New("timetable templates can only be scheduled from Monday to Friday")

// ErrTemplateSegmentNotEditable is returned when a full-series PUT reaches a
// segment that has already been capped by Split/End. The active CRUD contract
// exposes only open segments, so handlers map this race-safe service check to
// the same 404 as their preflight lookup.
var ErrTemplateSegmentNotEditable = errors.New("template segment is not editable")

const (
	updateTemplateOp       = "update template"
	updateTemplateFieldsOp = "update template: update fields"
	archiveTemplateOp      = "archive template"
)

// TemplateUpdateInput carries the template fields, recurrence shape, and
// roster edited by PUT /timetable/templates/{id}. The validity envelope is
// deliberately not part of this input: it is an invariant of the existing
// split-series segment and must survive an edit unchanged.
type TemplateUpdateInput struct {
	TemplateID       int64
	Fields           activitiesModel.TemplateFieldsUpdate
	Weekdays         []int
	TimeframeID      int64
	WeekPattern      int
	CalendarPeriodID *int64
	RosterValidFrom  timezone.Date
	StudentIDs       []int64
	StaffIDs         []int64
	PrimaryStaffID   *int64
	// WeekdayAssignments carries the per-weekday deviations from the shared
	// roster above (issue #2129). Empty = identical roster on every weekday.
	WeekdayAssignments []WeekdayRosterAssignment
	// GradeLevelMax is the caller's validated snapshot of
	// enrollment.grade_level_max. Missing or out-of-range values are rejected.
	GradeLevelMax int
}

// UpdateTemplate replaces a template's editable fields, schedules, and roster
// while preserving the segment's inclusive valid_from and exclusive
// valid_until boundaries across all three. All schedule rows of a segment must
// share one envelope; inconsistent existing rows are rejected before mutation.
func (s *TimetableDataService) UpdateTemplate(ctx context.Context, in TemplateUpdateInput) error {
	if err := normalizeTemplateUpdateTarget(&in); err != nil {
		return &ScheduleError{Op: updateTemplateOp, Err: err}
	}
	tenantID, err := s.validateTemplateUpdateRequest(ctx, in)
	if err != nil {
		return err
	}

	return tenant.WithTenantTx(ctx, s.deps.DB, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		return s.updateTemplateLocked(txCtx, in, tenantID)
	})
}

func normalizeTemplateUpdateTarget(in *TemplateUpdateInput) error {
	target := &activitiesModel.Group{
		TargetGroupType:   in.Fields.TargetGroupType,
		TargetGradeLevel:  in.Fields.TargetGradeLevel,
		TargetSchoolClass: in.Fields.TargetSchoolClass,
		EducationGroupID:  in.Fields.EducationGroupID,
	}
	if err := target.ValidateTargetGroup(); err != nil {
		return err
	}
	in.Fields.TargetGroupType = target.TargetGroupType
	in.Fields.TargetSchoolClass = target.TargetSchoolClass
	return nil
}

func (s *TimetableDataService) validateTemplateUpdateRequest(ctx context.Context, in TemplateUpdateInput) (int64, error) {
	if err := validateTemplateUpdateInput(in); err != nil {
		return 0, &ScheduleError{Op: updateTemplateOp, Err: err}
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return 0, &ScheduleError{Op: updateTemplateOp, Err: errors.New("no tenant in context")}
	}
	if s.deps.DB == nil {
		return 0, &ScheduleError{Op: updateTemplateOp, Err: errors.New("database is not configured")}
	}
	if s.deps.ActivityGroupRepo == nil || s.deps.ActivityScheduleRepo == nil ||
		s.deps.StudentEnrollmentRepo == nil || s.deps.ActivitySupervisorRepo == nil ||
		s.deps.ActivityCategoryRepo == nil {
		return 0, &ScheduleError{Op: updateTemplateOp, Err: errors.New("template repositories are not configured")}
	}
	return tenantID, nil
}

func (s *TimetableDataService) updateTemplateLocked(ctx context.Context, in TemplateUpdateInput, tenantID int64) error {
	if err := lockTenantRecurrenceWrites(ctx, s.deps.DB); err != nil {
		return &ScheduleError{Op: "update template: lock recurrence", Err: err}
	}
	existing, err := s.deps.ActivityGroupRepo.FindByID(ctx, in.TemplateID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return &ScheduleError{Op: "update template: load target", Err: ErrTemplateSegmentNotEditable}
		}
		return &ScheduleError{Op: "update template: load target", Err: err}
	}
	if existing == nil || !existing.IsTemplate || existing.ArchivedAt != nil {
		return &ScheduleError{Op: "update template: load target", Err: ErrTemplateSegmentNotEditable}
	}
	if in.Fields.CategoryID != existing.CategoryID {
		if err := validateAssignableCategory(ctx, s.deps.ActivityCategoryRepo, in.Fields.CategoryID, "update template: validate category"); err != nil {
			return err
		}
	}
	if err := ValidateTemplateTargetGradeLimit(
		in.GradeLevelMax,
		existing,
		in.Fields.TargetGroupType,
		in.Fields.TargetGradeLevel,
	); err != nil {
		return &ScheduleError{Op: "update template: validate target grade", Err: err}
	}
	validFrom, validUntil, err := s.loadEditableTemplateEnvelope(ctx, in.TemplateID)
	if err != nil {
		return err
	}
	previousSchedules, err := s.deps.ActivityScheduleRepo.FindByGroupID(ctx, in.TemplateID)
	if err != nil {
		return &ScheduleError{Op: "update template: load previous schedules", Err: err}
	}
	if err := validateLegacyTemplateWeekdays(previousSchedules, in.Weekdays); err != nil {
		return &ScheduleError{Op: "update template: validate weekdays", Err: err}
	}
	// Capture the series' current Listenart before the field write so the
	// instance propagation can tell an untouched occurrence (still carrying the
	// series value) from a per-occurrence override.
	previousListKind := existing.ListKind
	if err := s.updateTemplateFields(ctx, in); err != nil {
		return err
	}
	if err := s.propagateListKindToInstances(ctx, in.TemplateID, previousListKind, in.Fields.ListKind); err != nil {
		return err
	}
	if err := s.replaceTemplateSchedules(ctx, in, tenantID, validFrom, validUntil); err != nil {
		return err
	}
	if err := s.deleteRemovedLegacyWeekendInstances(ctx, in.TemplateID, previousSchedules, in.Weekdays); err != nil {
		return err
	}
	if err := s.replaceTemplateRoster(ctx, in, tenantID, validFrom, validUntil); err != nil {
		return err
	}
	if s.deps.ValidateCareOfferingSeries == nil {
		return nil
	}
	if err := s.deps.ValidateCareOfferingSeries(ctx, in.TemplateID); err != nil {
		return templateCareOfferingValidationError(
			ctx,
			"update template: validate linked care offerings",
			"updated recurrence is incompatible with an existing care offering",
			err,
		)
	}
	return nil
}

func validateLegacyTemplateWeekdays(existing []*activitiesModel.Schedule, requested []int) error {
	legacy := make(map[int]struct{})
	for _, schedule := range existing {
		if schedule != nil && schedule.Weekday > activitiesModel.WeekdayFriday {
			legacy[schedule.Weekday] = struct{}{}
		}
	}
	for _, weekday := range requested {
		if weekday > activitiesModel.WeekdayFriday {
			if _, ok := legacy[weekday]; !ok {
				return ErrTemplateWeekendWeekday
			}
		}
	}
	return nil
}

type legacyWeekendInstanceCleaner interface {
	DeletePlannedMaterializedWeekendInstances(context.Context, int64, []int) (int64, error)
}

func (s *TimetableDataService) deleteRemovedLegacyWeekendInstances(ctx context.Context, templateID int64, previous []*activitiesModel.Schedule, requested []int) error {
	requestedWeekdays := make(map[int]struct{}, len(requested))
	for _, weekday := range requested {
		requestedWeekdays[weekday] = struct{}{}
	}
	removed := make([]int, 0, 2)
	for _, schedule := range previous {
		if schedule.Weekday > activitiesModel.WeekdayFriday {
			if _, retained := requestedWeekdays[schedule.Weekday]; !retained {
				removed = append(removed, schedule.Weekday)
			}
		}
	}
	cleaner, ok := s.deps.ActivityInstanceRepo.(legacyWeekendInstanceCleaner)
	if !ok || len(removed) == 0 {
		return nil
	}
	deleted, err := cleaner.DeletePlannedMaterializedWeekendInstances(ctx, templateID, removed)
	if err != nil {
		return &ScheduleError{Op: "update template: delete removed legacy weekend instances", Err: err}
	}
	if deleted > 0 {
		// This bulk deletion bypasses the planned-instance CRUD flow, so it must
		// invalidate clients after the surrounding transaction commits.
		broadcastStaffingChanged(ctx, s.deps.Broadcaster, s.getLogger(), "template_legacy_weekend_cleanup")
	}
	return nil
}

func (s *TimetableDataService) loadEditableTemplateEnvelope(
	ctx context.Context,
	templateID int64,
) (*timezone.Date, *timezone.Date, error) {
	existing, err := s.deps.ActivityScheduleRepo.FindByGroupID(ctx, templateID)
	if err != nil {
		return nil, nil, &ScheduleError{Op: "update template: load schedules", Err: err}
	}
	validFrom, validUntil, err := commonScheduleValidityEnvelope(existing)
	if err != nil {
		return nil, nil, &ScheduleError{Op: "update template: inspect schedule validity", Err: err}
	}
	if validUntil != nil {
		return nil, nil, &ScheduleError{Op: "update template: inspect schedule validity", Err: ErrTemplateSegmentNotEditable}
	}
	return validFrom, validUntil, nil
}

func (s *TimetableDataService) updateTemplateFields(ctx context.Context, in TemplateUpdateInput) error {
	updated, err := s.deps.ActivityGroupRepo.UpdateTemplateFields(ctx, in.TemplateID, in.Fields)
	if err != nil {
		return &ScheduleError{Op: updateTemplateFieldsOp, Err: err}
	}
	if updated == 0 {
		// Archive can commit while the handler's read-only preflight waits for
		// this recurrence gate. Preserve the active-CRUD 404 contract.
		return &ScheduleError{Op: updateTemplateFieldsOp, Err: ErrTemplateSegmentNotEditable}
	}
	if updated > 1 {
		return &ScheduleError{
			Op:  updateTemplateFieldsOp,
			Err: fmt.Errorf("expected one template row to change, got %d", updated),
		}
	}
	return nil
}

// propagateListKindToInstances carries a series Listenart change onto the
// template's already-materialized future occurrences. Without it a list_kind
// edit reaches only occurrences materialized AFTER the edit, so the classified
// daily lists (#1565) omitted the series until a manual re-plan. It is a no-op
// when the classification is unchanged or the instance repository is not wired
// (read-only test facades). Runs inside the caller's tenant transaction and
// recurrence gate; the repository predicate preserves today/past rows,
// non-planned/spontaneous rows, and per-occurrence classification overrides.
func (s *TimetableDataService) propagateListKindToInstances(
	ctx context.Context,
	templateID int64,
	previousKind, newKind *string,
) error {
	if s.deps.ActivityInstanceRepo == nil || sameListKind(previousKind, newKind) {
		return nil
	}
	if _, err := s.deps.ActivityInstanceRepo.PropagateListKindToFutureInstances(
		ctx, templateID, previousKind, newKind, timezone.TodayDate(),
	); err != nil {
		return &ScheduleError{Op: "update template: propagate list kind", Err: err}
	}
	return nil
}

func (s *TimetableDataService) replaceTemplateSchedules(
	ctx context.Context,
	in TemplateUpdateInput,
	tenantID int64,
	validFrom, validUntil *timezone.Date,
) error {
	if err := s.deps.ActivityScheduleRepo.DeleteByGroupID(ctx, in.TemplateID); err != nil {
		return &ScheduleError{Op: "update template: delete schedules", Err: err}
	}
	for _, weekday := range in.Weekdays {
		timeframeID := in.TimeframeID
		schedule := &activitiesModel.Schedule{
			Weekday:          weekday,
			TimeframeID:      &timeframeID,
			ActivityGroupID:  in.TemplateID,
			WeekPattern:      in.WeekPattern,
			CalendarPeriodID: in.CalendarPeriodID,
			ValidFrom:        cloneOptionalDate(validFrom),
			ValidUntil:       cloneOptionalDate(validUntil),
		}
		schedule.SetTenantID(tenantID)
		if err := s.deps.ActivityScheduleRepo.Create(ctx, schedule); err != nil {
			return &ScheduleError{Op: "update template: create schedule", Err: err}
		}
	}
	return nil
}

// ArchiveTemplate removes a template from future planner reads while holding
// the same tenant recurrence gate as materialization. Without the gate a
// materializer can load the unarchived template, wait for archive to commit,
// then insert a stale future occurrence.
func (s *TimetableDataService) ArchiveTemplate(ctx context.Context, templateID int64) (int64, error) {
	if templateID <= 0 {
		return 0, &ScheduleError{Op: archiveTemplateOp, Err: errors.New("template id is required")}
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return 0, &ScheduleError{Op: archiveTemplateOp, Err: errors.New("no tenant in context")}
	}
	if s.deps.DB == nil || s.deps.ActivityGroupRepo == nil {
		return 0, &ScheduleError{Op: archiveTemplateOp, Err: errors.New("template service is not configured")}
	}

	var archived int64
	err := tenant.WithTenantTx(ctx, s.deps.DB, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		if err := lockTenantRecurrenceWrites(txCtx, s.deps.DB); err != nil {
			return &ScheduleError{Op: "archive template: lock recurrence", Err: err}
		}
		var err error
		archived, err = s.deps.ActivityGroupRepo.ArchiveTemplate(txCtx, templateID)
		if err != nil {
			return &ScheduleError{Op: "archive template: update", Err: err}
		}
		if archived > 0 && s.deps.ValidateCareOfferingSeries != nil {
			if err := s.deps.ValidateCareOfferingSeries(txCtx, templateID); err != nil {
				return templateCareOfferingValidationError(
					txCtx,
					"archive template: validate linked care offerings",
					"archiving the template is incompatible with an existing care offering",
					err,
				)
			}
		}
		return nil
	})
	return archived, err
}

func validateTemplateUpdateInput(in TemplateUpdateInput) error {
	if in.TemplateID <= 0 {
		return errors.New("template id is required")
	}
	if in.TimeframeID <= 0 {
		return errors.New("timeframe id is required")
	}
	if len(in.Weekdays) == 0 {
		return errors.New("at least one weekday is required")
	}
	for _, weekday := range in.Weekdays {
		if !activitiesModel.IsValidWeekday(weekday) {
			return fmt.Errorf("invalid weekday %d", weekday)
		}
	}
	if in.WeekPattern < 0 || in.WeekPattern > 2 {
		return errors.New("week pattern must be 0, 1, or 2")
	}
	if in.CalendarPeriodID != nil && *in.CalendarPeriodID <= 0 {
		return errors.New("calendar period id must be positive when set")
	}
	if in.RosterValidFrom.IsZero() {
		return errors.New("roster valid_from is required")
	}
	if err := validateTemplateGradeLevelMax(in.GradeLevelMax); err != nil {
		return err
	}
	return nil
}

func (s *TimetableDataService) replaceTemplateRoster(
	ctx context.Context,
	in TemplateUpdateInput,
	tenantID int64,
	scheduleValidFrom, scheduleValidUntil *timezone.Date,
) error {
	rosterValidFrom := in.RosterValidFrom
	if scheduleValidFrom != nil {
		rosterValidFrom = *scheduleValidFrom
	}
	if scheduleValidUntil != nil && rosterValidFrom.After(*scheduleValidUntil) {
		return &ScheduleError{
			Op: "update template: replace roster",
			Err: fmt.Errorf(
				"roster valid_from %s is after segment valid_until %s",
				rosterValidFrom.String(),
				scheduleValidUntil.String(),
			),
		}
	}

	roster, err := resolveTemplateRoster(in.Weekdays, in.StudentIDs, in.StaffIDs, in.PrimaryStaffID, in.WeekdayAssignments)
	if err != nil {
		return &ScheduleError{Op: "update template: resolve roster", Err: err}
	}

	protectedCoverage, err := s.retireTemplateEnrollments(
		ctx,
		in.TemplateID,
		in.CalendarPeriodID,
		in.Weekdays,
		rosterValidFrom,
		scheduleValidUntil,
	)
	if err != nil {
		return err
	}
	for _, row := range excludeProtectedStudentWeekdays(roster.Students, in.Weekdays, protectedCoverage) {
		// A child kept by a care-offering row is already on the roster with a
		// provenance this editor must not overwrite on the weekdays it covers.
		enrollment := &activitiesModel.StudentEnrollment{
			StudentID:        row.PersonID,
			ActivityGroupID:  in.TemplateID,
			ValidFrom:        rosterValidFrom,
			ValidUntil:       cloneOptionalDate(scheduleValidUntil),
			CalendarPeriodID: in.CalendarPeriodID,
			Weekday:          weekdayScopePtr(row.Weekday),
		}
		enrollment.SetTenantID(tenantID)
		if err := s.deps.StudentEnrollmentRepo.Create(ctx, enrollment); err != nil {
			return &ScheduleError{Op: "update template: create enrollment", Err: err}
		}
	}

	if err := s.retireTemplateSupervisors(ctx, in.TemplateID, in.CalendarPeriodID, rosterValidFrom, scheduleValidUntil); err != nil {
		return err
	}
	for _, row := range roster.Staff {
		supervisor := &activitiesModel.SupervisorPlanned{
			StaffID:          row.PersonID,
			GroupID:          in.TemplateID,
			IsPrimary:        row.IsPrimary,
			ValidFrom:        rosterValidFrom,
			ValidUntil:       cloneOptionalDate(scheduleValidUntil),
			CalendarPeriodID: in.CalendarPeriodID,
			Weekday:          weekdayScopePtr(row.Weekday),
		}
		supervisor.SetTenantID(tenantID)
		if err := s.deps.ActivitySupervisorRepo.Create(ctx, supervisor); err != nil {
			return &ScheduleError{Op: "update template: create supervisor", Err: err}
		}
	}
	return nil
}

type rosterRetirementAction uint8

const (
	rosterRetirementSkip rosterRetirementAction = iota
	rosterRetirementPreserve
	rosterRetirementDelete
	rosterRetirementClose
)

func (s *TimetableDataService) retireTemplateEnrollments(
	ctx context.Context,
	templateID int64,
	calendarPeriodID *int64,
	weekdays []int,
	replacementFrom timezone.Date,
	replacementUntil *timezone.Date,
) (map[int64]protectedStudentCoverage, error) {
	rows, err := s.deps.StudentEnrollmentRepo.FindByGroupID(ctx, templateID)
	if err != nil {
		return nil, &ScheduleError{Op: "update template: load enrollments", Err: err}
	}
	protected, err := s.retireUnprotectedTemplateEnrollments(
		ctx,
		rows,
		calendarPeriodID,
		replacementFrom,
		replacementUntil,
	)
	if err != nil {
		return nil, err
	}
	return s.rebaseProtectedTemplateEnrollments(ctx, protected, calendarPeriodID, weekdays)
}

func (s *TimetableDataService) retireUnprotectedTemplateEnrollments(
	ctx context.Context,
	rows []*activitiesModel.StudentEnrollment,
	calendarPeriodID *int64,
	replacementFrom timezone.Date,
	replacementUntil *timezone.Date,
) ([]*activitiesModel.StudentEnrollment, error) {
	protected := make([]*activitiesModel.StudentEnrollment, 0)
	for _, row := range rows {
		if row != nil && enrollmentIsProtected(row) &&
			validityWindowsOverlap(row.ValidFrom, row.ValidUntil, replacementFrom, replacementUntil) {
			protected = append(protected, row)
			continue
		}
		action := classifyEnrollmentRetirement(row, calendarPeriodID, replacementFrom, replacementUntil)
		if err := s.applyEnrollmentRetirement(ctx, row, action, replacementFrom); err != nil {
			return nil, err
		}
	}
	return protected, nil
}

func (s *TimetableDataService) applyEnrollmentRetirement(
	ctx context.Context,
	row *activitiesModel.StudentEnrollment,
	action rosterRetirementAction,
	replacementFrom timezone.Date,
) error {
	switch action {
	case rosterRetirementDelete:
		if err := s.deps.StudentEnrollmentRepo.Delete(ctx, row.ID); err != nil {
			return &ScheduleError{Op: "update template: delete future enrollment", Err: err}
		}
	case rosterRetirementClose:
		if err := s.deps.StudentEnrollmentRepo.SetValidUntilByID(ctx, row.ID, replacementFrom); err != nil {
			return &ScheduleError{Op: "update template: close enrollment", Err: err}
		}
	}
	return nil
}

func (s *TimetableDataService) rebaseProtectedTemplateEnrollments(
	ctx context.Context,
	protected []*activitiesModel.StudentEnrollment,
	calendarPeriodID *int64,
	weekdays []int,
) (map[int64]protectedStudentCoverage, error) {
	if err := validateProtectedEnrollmentRebase(protected, calendarPeriodID); err != nil {
		return nil, &ScheduleError{Op: "update template: rebase protected enrollments", Err: err}
	}
	for _, row := range protected {
		if err := s.rebaseProtectedEnrollmentPeriod(ctx, row, calendarPeriodID); err != nil {
			return nil, err
		}
	}
	return buildProtectedStudentCoverage(
		protected,
		weekdays,
		func(row *activitiesModel.StudentEnrollment) bool {
			return rosterPeriodApplies(row.CalendarPeriodID, calendarPeriodID)
		},
	), nil
}

func (s *TimetableDataService) rebaseProtectedEnrollmentPeriod(
	ctx context.Context,
	row *activitiesModel.StudentEnrollment,
	calendarPeriodID *int64,
) error {
	if row.CalendarPeriodID == nil || calendarPeriodID == nil ||
		*row.CalendarPeriodID == *calendarPeriodID {
		return nil
	}
	row.CalendarPeriodID = cloneOptionalInt64(calendarPeriodID)
	if err := s.deps.StudentEnrollmentRepo.Update(ctx, row); err != nil {
		return &ScheduleError{Op: "update template: persist protected enrollment period", Err: err}
	}
	return nil
}

func classifyEnrollmentRetirement(
	row *activitiesModel.StudentEnrollment,
	calendarPeriodID *int64,
	replacementFrom timezone.Date,
	replacementUntil *timezone.Date,
) rosterRetirementAction {
	if row == nil || !validityWindowsOverlap(row.ValidFrom, row.ValidUntil, replacementFrom, replacementUntil) {
		return rosterRetirementSkip
	}
	// Enrollment-offer rows and weekday-specific legacy rows are managed
	// outside this template editor. The request does not expose their
	// provenance or selected weekdays, so replacing them would silently
	// discard data and widen/narrow a care-offer assignment.
	if row.EnrollmentRequestChildID != nil || len(row.SelectedWeekdays) > 0 {
		if rosterPeriodApplies(row.CalendarPeriodID, calendarPeriodID) {
			return rosterRetirementPreserve
		}
		return rosterRetirementSkip
	}
	return classifyOwnedRosterRetirement(
		row.ValidFrom,
		row.ValidUntil,
		optionalInt64sEqual(row.CalendarPeriodID, calendarPeriodID),
		replacementFrom,
		replacementUntil,
	)
}

func (s *TimetableDataService) retireTemplateSupervisors(
	ctx context.Context,
	templateID int64,
	calendarPeriodID *int64,
	replacementFrom timezone.Date,
	replacementUntil *timezone.Date,
) error {
	rows, err := s.deps.ActivitySupervisorRepo.FindByGroupID(ctx, templateID)
	if err != nil {
		return &ScheduleError{Op: "update template: load supervisors", Err: err}
	}
	for _, row := range rows {
		switch classifySupervisorRetirement(row, calendarPeriodID, replacementFrom, replacementUntil) {
		case rosterRetirementDelete:
			if err := s.deps.ActivitySupervisorRepo.Delete(ctx, row.ID); err != nil {
				return &ScheduleError{Op: "update template: delete future supervisor", Err: err}
			}
		case rosterRetirementClose:
			if err := s.deps.ActivitySupervisorRepo.SetValidUntilByID(ctx, row.ID, replacementFrom); err != nil {
				return &ScheduleError{Op: "update template: close supervisor", Err: err}
			}
		}
	}
	return nil
}

func classifySupervisorRetirement(
	row *activitiesModel.SupervisorPlanned,
	calendarPeriodID *int64,
	replacementFrom timezone.Date,
	replacementUntil *timezone.Date,
) rosterRetirementAction {
	if row == nil || !validityWindowsOverlap(row.ValidFrom, row.ValidUntil, replacementFrom, replacementUntil) {
		return rosterRetirementSkip
	}
	return classifyOwnedRosterRetirement(
		row.ValidFrom,
		row.ValidUntil,
		optionalInt64sEqual(row.CalendarPeriodID, calendarPeriodID),
		replacementFrom,
		replacementUntil,
	)
}

func classifyOwnedRosterRetirement(
	validFrom timezone.Date,
	validUntil *timezone.Date,
	periodMatches bool,
	replacementFrom timezone.Date,
	replacementUntil *timezone.Date,
) rosterRetirementAction {
	if !periodMatches {
		return rosterRetirementSkip
	}
	// Open rows are editor-managed. A bounded plain roster row is replaceable
	// only when it shares this segment's end; unrelated phase windows survive.
	if validUntil != nil && (replacementUntil == nil || *validUntil != *replacementUntil) {
		return rosterRetirementSkip
	}
	if validFrom.After(replacementFrom) {
		return rosterRetirementDelete
	}
	return rosterRetirementClose
}

func validityWindowsOverlap(
	leftFrom timezone.Date,
	leftUntil *timezone.Date,
	rightFrom timezone.Date,
	rightUntil *timezone.Date,
) bool {
	if leftUntil != nil && !rightFrom.Before(*leftUntil) {
		return false
	}
	if rightUntil != nil && !leftFrom.Before(*rightUntil) {
		return false
	}
	return true
}

func optionalInt64sEqual(left, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

// rosterPeriodApplies mirrors materialization semantics: an unscoped roster
// row applies to every selected period, while a scoped row applies only to the
// same period. A protected unscoped row therefore suppresses creation of a
// broad period-specific duplicate.
func rosterPeriodApplies(rowPeriodID, targetPeriodID *int64) bool {
	if rowPeriodID == nil {
		return true
	}
	return targetPeriodID != nil && *rowPeriodID == *targetPeriodID
}

func commonScheduleValidityEnvelope(schedules []*activitiesModel.Schedule) (*timezone.Date, *timezone.Date, error) {
	if len(schedules) == 0 {
		return nil, nil, nil
	}
	if schedules[0] == nil {
		return nil, nil, fmt.Errorf("%w: nil schedule row", ErrInconsistentTemplateScheduleValidity)
	}
	validFrom := cloneOptionalDate(schedules[0].ValidFrom)
	validUntil := cloneOptionalDate(schedules[0].ValidUntil)
	if validFrom != nil && validUntil != nil && validFrom.After(*validUntil) {
		return nil, nil, fmt.Errorf(
			"%w: segment valid_from %s is after valid_until %s",
			ErrInconsistentTemplateScheduleValidity,
			validFrom.String(),
			validUntil.String(),
		)
	}
	for _, schedule := range schedules[1:] {
		if schedule == nil {
			return nil, nil, fmt.Errorf("%w: nil schedule row", ErrInconsistentTemplateScheduleValidity)
		}
		if !optionalDatesEqual(validFrom, schedule.ValidFrom) || !optionalDatesEqual(validUntil, schedule.ValidUntil) {
			return nil, nil, fmt.Errorf(
				"%w: schedule %d does not match the segment envelope",
				ErrInconsistentTemplateScheduleValidity,
				schedule.ID,
			)
		}
	}
	return validFrom, validUntil, nil
}

func optionalDatesEqual(left, right *timezone.Date) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func cloneOptionalDate(date *timezone.Date) *timezone.Date {
	if date == nil {
		return nil
	}
	cloned := *date
	return &cloned
}
