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
	Targets          []*activitiesModel.GroupTarget
	// WeekdayAssignments carries the per-weekday deviations from the shared
	// roster above (issue #2129). Empty = identical roster on every weekday.
	WeekdayAssignments []WeekdayRosterAssignment
	// GradeLevelMax is the caller's validated snapshot of
	// enrollment.grade_level_max. Missing or out-of-range values are rejected.
	GradeLevelMax int
	// SeriesRosterFrom extends the saved roster backwards over the split
	// series (#2187): when set, the roster is additionally reconciled onto the
	// bounded predecessor segments overlapping [SeriesRosterFrom, this
	// segment's valid_from). It does NOT change the living segment's own
	// roster anchor (RosterValidFrom stays authoritative there). Nil = the
	// update touches only this segment, exactly as before.
	SeriesRosterFrom *timezone.Date
	// SeriesRosterScopeStudentIDs / SeriesRosterScopeStaffIDs name the people
	// whose membership this edit actually changed. Only they are reconciled on
	// the predecessor segments; everyone else keeps their predecessor rows.
	// StudentIDs/StaffIDs describe the LIVING segment and may legitimately
	// differ from a predecessor's roster (a split can change the roster), so
	// treating them as the predecessor's absolute target set would silently
	// drop predecessor-only members. Empty scopes = nothing to mirror.
	SeriesRosterScopeStudentIDs []int64
	SeriesRosterScopeStaffIDs   []int64
	// SeriesRosterScopeWeekdays narrows the mirroring to the weekdays this
	// edit describes. The occurrence editor of a per-weekday series (#2129)
	// shows ONE weekday's roster, so the predecessor's other weekdays must not
	// be judged against it. Empty = every weekday the segment shares with the
	// submitted recurrence.
	SeriesRosterScopeWeekdays []int
	// SeriesRosterPrimaryChanged marks the Hauptbetreuung itself as part of
	// this edit. PrimaryStaffID always names the LIVING segment's lead, so
	// without this flag a newly mirrored supervisor row would stamp that lead
	// onto the predecessor and outrank its own.
	SeriesRosterPrimaryChanged bool
}

// UpdateTemplate replaces a template's editable fields, schedules, and roster
// while preserving the segment's inclusive valid_from and exclusive
// valid_until boundaries across all three. All schedule rows of a segment must
// share one envelope; inconsistent existing rows are rejected before mutation.
func (s *TimetableDataService) UpdateTemplate(ctx context.Context, in TemplateUpdateInput) error {
	targetsProvided := in.Targets != nil
	if err := normalizeTemplateUpdateTarget(&in); err != nil {
		return &ScheduleError{Op: updateTemplateOp, Err: err}
	}
	tenantID, err := s.validateTemplateUpdateRequest(ctx, in)
	if err != nil {
		return err
	}

	return tenant.WithTenantTx(ctx, s.deps.DB, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		return s.updateTemplateLocked(txCtx, in, tenantID, targetsProvided)
	})
}

func normalizeTemplateUpdateTarget(in *TemplateUpdateInput) error {
	targets, err := normalizeDynamicTargets(in.Fields.TargetGroupType, in.Fields.TargetGradeLevel, in.Fields.TargetSchoolClass, in.Fields.EducationGroupID, in.Targets)
	if err != nil {
		return err
	}
	in.Targets = targets
	in.Fields.TargetGradeLevel = nil
	in.Fields.TargetSchoolClass = nil
	if len(targets) > 0 {
		in.Fields.TargetGradeLevel = targets[0].TargetGradeLevel
		in.Fields.TargetSchoolClass = targets[0].TargetSchoolClass
		if in.Fields.TargetGroupType == activitiesModel.TargetGroupTypeGruppe {
			in.Fields.EducationGroupID = targets[0].EducationGroupID
		}
	}
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

func (s *TimetableDataService) updateTemplateLocked(
	ctx context.Context,
	in TemplateUpdateInput,
	tenantID int64,
	targetsProvided bool,
) error {
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
	if in.Fields.PlanningTrackIDProvided && !samePlanningTrackID(in.Fields.PlanningTrackID, existing.PlanningTrackID) {
		if err := validateAssignablePlanningTrack(ctx, s.deps.PlanningTrackRepo, in.Fields.PlanningTrackID, existing.PlanningTrackID); err != nil {
			return err
		}
	}
	existingTargets, err := loadExistingDynamicTargets(ctx, s.deps.ActivityGroupRepo, in.TemplateID)
	if err != nil {
		return &ScheduleError{Op: "update template: load targets", Err: err}
	}
	if !targetsProvided && len(existingTargets) > 0 {
		in.Targets = existingTargets
		in.Fields.TargetGroupType = existingTargets[0].TargetGroupType
		in.Fields.TargetGradeLevel = existingTargets[0].TargetGradeLevel
		in.Fields.TargetSchoolClass = existingTargets[0].TargetSchoolClass
		if in.Fields.TargetGroupType == activitiesModel.TargetGroupTypeGruppe {
			in.Fields.EducationGroupID = existingTargets[0].EducationGroupID
		}
	}
	if err := validateDynamicTargets(ctx, s, in.GradeLevelMax, existing, existingTargets, in.Targets); err != nil {
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
	previousSourceOfferingID := cloneOptionalInt64(existing.SourceCareOfferingID)
	previousCalendarPeriodID := cloneOptionalInt64(existing.CalendarPeriodID)
	// Same pre-write guard as on create: the merged source must resolve before
	// the field write stamps it onto the group row, otherwise an unknown
	// offering trips the FK (500) before the resync can classify it as
	// ErrOfferingSourceInvalid (400) (#2147 review round 18).
	if err := s.validateOfferingSourceReference(ctx, in.Fields.SourceCareOfferingID, in.CalendarPeriodID, "update template: validate offering source"); err != nil {
		return err
	}
	if err := s.updateTemplateFields(ctx, in); err != nil {
		return err
	}
	if targetsProvided {
		targetRepo, ok := s.deps.ActivityGroupRepo.(activitiesModel.GroupTargetRepository)
		if !ok {
			return &ScheduleError{Op: "update template: replace targets", Err: errors.New("target repository is not configured")}
		} else {
			if err := targetRepo.ReplaceTargets(ctx, in.TemplateID, in.Targets); err != nil {
				return &ScheduleError{Op: "update template: replace targets", Err: err}
			}
		}
	}
	if err := s.propagateListKindToInstances(ctx, in.TemplateID, previousListKind, in.Fields.ListKind); err != nil {
		return err
	}
	// Resync BEFORE the roster replacement: sourced rows are protected there
	// (EnrollmentRequestChildID != nil), so a removed source must clear its
	// rows first or a manually re-picked child would end up with no row at
	// all (the protected row suppresses the manual create, then a later
	// cleanup would delete it).
	if err := s.resyncUpdatedTemplateOfferingRoster(ctx, in, previousSourceOfferingID, validFrom); err != nil {
		return err
	}
	if err := s.replaceTemplateSchedules(ctx, in, tenantID, validFrom, validUntil); err != nil {
		return err
	}
	if err := s.deleteRemovedLegacyWeekendInstances(ctx, in.TemplateID, previousSchedules, in.Weekdays); err != nil {
		return err
	}
	retiredStudentIDs, err := s.replaceTemplateRoster(ctx, in, tenantID, validFrom, validUntil, previousCalendarPeriodID)
	if err != nil {
		return err
	}
	if err := s.reconcileManualRosterInstances(ctx, in, previousSourceOfferingID, validFrom, retiredStudentIDs); err != nil {
		return err
	}
	if err := s.reconcileSeriesPredecessorRoster(ctx, in, tenantID, validFrom); err != nil {
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

// resyncUpdatedTemplateOfferingRoster runs the offering-source reconcile when
// the edit involves a source (kept, changed, added, or removed). A template
// that never had a source and gets none skips the hook entirely.
func (s *TimetableDataService) resyncUpdatedTemplateOfferingRoster(
	ctx context.Context,
	in TemplateUpdateInput,
	previousSourceOfferingID *int64,
	scheduleValidFrom *timezone.Date,
) error {
	if in.Fields.SourceCareOfferingID == nil && previousSourceOfferingID == nil {
		return nil
	}
	if s.deps.ResyncOfferingRoster == nil {
		return &ScheduleError{Op: updateTemplateOp, Err: errors.New("offering roster resync is not configured")}
	}
	if err := s.deps.ResyncOfferingRoster(ctx, OfferingRosterResyncInput{
		TemplateID:         in.TemplateID,
		PreviousOfferingID: previousSourceOfferingID,
		OfferingID:         in.Fields.SourceCareOfferingID,
		GradeLevels:        in.Fields.SourceGradeLevels,
		CalendarPeriodID:   in.CalendarPeriodID,
		EffectiveFrom:      offeringResyncBoundary(in.RosterValidFrom, scheduleValidFrom),
	}); err != nil {
		return &ScheduleError{Op: "update template: resync offering roster", Err: err}
	}
	return nil
}

// offeringResyncBoundary is the date from which a template edit may rewrite
// its offering-sourced roster: the series start when one exists, else the
// roster valid_from. An already-started series must not use its schedule
// start as the rewrite boundary: the resync deletes rows starting on or after
// EffectiveFrom and caps earlier ones AT it, so a past boundary would rewrite
// roster history that was already effective. Today is the earliest honest
// edit boundary; a future schedule start stays as-is (#2147 review).
func offeringResyncBoundary(rosterValidFrom timezone.Date, scheduleValidFrom *timezone.Date) timezone.Date {
	effectiveFrom := rosterValidFrom
	if scheduleValidFrom != nil {
		effectiveFrom = *scheduleValidFrom
	}
	if today := timezone.TodayDate(); effectiveFrom.Before(today) {
		effectiveFrom = today
	}
	return effectiveFrom
}

// reconcileManualRosterInstances re-aligns the template's already-
// materialized future occurrences with the manual-roster changes the update
// just wrote. Only edits that involved an offering source need it, in two
// shapes (#2147 review):
//
//   - source removed, child re-picked by hand in the same save: the source
//     resync runs BEFORE the roster replacement (see the ordering comment at
//     its call site) and removes the departing child's still-planned instance
//     rows — nothing after replaceTemplateRoster would put the child back on
//     existing occurrences until a manual re-plan. Covered by the retained
//     manual roster (in.StudentIDs / weekday assignments).
//   - manual template converted to a sourced one: replaceTemplateRoster
//     retires the old manual enrollment rows, but the materializer never
//     revisits existing instances — retired students not re-covered by the
//     new source would stay planned on them. Covered by retiredStudentIDs.
//
// A manual roster can only coexist with a source edit in the removal shape,
// because validateOfferingSourceInput rejects student_ids next to a set
// source.
func (s *TimetableDataService) reconcileManualRosterInstances(
	ctx context.Context,
	in TemplateUpdateInput,
	previousSourceOfferingID *int64,
	scheduleValidFrom *timezone.Date,
	retiredStudentIDs []int64,
) error {
	if in.Fields.SourceCareOfferingID == nil && previousSourceOfferingID == nil {
		return nil
	}
	if s.deps.ActivityInstanceRepo == nil || s.deps.InstanceStudentRepo == nil || s.deps.StudentEnrollmentRepo == nil {
		return nil // read-only test facades have no occurrences to reconcile
	}
	studentIDs := unionStudentIDs(manualRosterStudentIDs(in), retiredStudentIDs)
	if len(studentIDs) == 0 {
		return nil
	}
	reconciler := NewRosterReconciler(s.deps.ActivityInstanceRepo, s.deps.InstanceStudentRepo, s.deps.StudentEnrollmentRepo, s.deps.Logger)
	// No prior-enrollment snapshot: both shapes re-establish coverage on
	// purpose. A re-picked child's instance rows were just removed by the
	// source resync and must come back; retired students only lose rows.
	if _, _, err := reconciler.ReconcileSourcedTemplateRosters(
		ctx,
		in.TemplateID,
		studentIDs,
		offeringResyncBoundary(in.RosterValidFrom, scheduleValidFrom),
		nil,
	); err != nil {
		return &ScheduleError{Op: "update template: reconcile manual roster occurrences", Err: err}
	}
	return nil
}

// manualRosterStudentIDs collects the distinct students the editor manages by
// hand — the shared roster plus every per-weekday assignment.
func manualRosterStudentIDs(in TemplateUpdateInput) []int64 {
	seen := make(map[int64]bool, len(in.StudentIDs))
	ids := make([]int64, 0, len(in.StudentIDs))
	appendID := func(id int64) {
		if id > 0 && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	for _, id := range in.StudentIDs {
		appendID(id)
	}
	for _, assignment := range in.WeekdayAssignments {
		for _, id := range assignment.StudentIDs {
			appendID(id)
		}
	}
	return ids
}

// unionStudentIDs merges two ID lists without duplicates, preserving order.
func unionStudentIDs(left, right []int64) []int64 {
	seen := make(map[int64]bool, len(left)+len(right))
	ids := make([]int64, 0, len(left)+len(right))
	for _, list := range [][]int64{left, right} {
		for _, id := range list {
			if id > 0 && !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	return ids
}

func loadExistingDynamicTargets(ctx context.Context, repo activitiesModel.GroupRepository, groupID int64) ([]*activitiesModel.GroupTarget, error) {
	targetRepo, ok := repo.(activitiesModel.GroupTargetRepository)
	if !ok {
		return nil, nil
	}
	byGroup, err := targetRepo.FindTargetsByGroupIDs(ctx, []int64{groupID})
	if err != nil {
		return nil, err
	}
	return byGroup[groupID], nil
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
	if err := validateOfferingSourceInput(
		in.Fields.SourceCareOfferingID,
		in.Fields.SourceGradeLevels,
		in.Fields.TargetGroupType,
		in.StudentIDs,
		in.WeekdayAssignments,
	); err != nil {
		return err
	}
	return nil
}

// replaceTemplateRoster rewrites the template's planned roster and returns
// the students whose enrollment rows the rewrite retired (deleted or closed)
// — the caller reconciles their already-materialized occurrences when the
// edit involved an offering source (#2147 review).
func (s *TimetableDataService) replaceTemplateRoster(
	ctx context.Context,
	in TemplateUpdateInput,
	tenantID int64,
	scheduleValidFrom, scheduleValidUntil *timezone.Date,
	previousCalendarPeriodID *int64,
) ([]int64, error) {
	rosterValidFrom := in.RosterValidFrom
	if scheduleValidFrom != nil {
		rosterValidFrom = *scheduleValidFrom
	}
	if scheduleValidUntil != nil && rosterValidFrom.After(*scheduleValidUntil) {
		return nil, &ScheduleError{
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
		return nil, &ScheduleError{Op: "update template: resolve roster", Err: err}
	}

	protectedCoverage, retiredStudentIDs, err := s.retireTemplateEnrollments(
		ctx,
		in.TemplateID,
		in.CalendarPeriodID,
		in.Weekdays,
		rosterValidFrom,
		scheduleValidUntil,
		previousCalendarPeriodID,
	)
	if err != nil {
		return nil, err
	}
	for _, row := range excludeProtectedStudentWeekdays(roster.Students, protectedCoverage) {
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
			return nil, &ScheduleError{Op: "update template: create enrollment", Err: err}
		}
	}

	if err := s.retireTemplateSupervisors(ctx, in.TemplateID, in.CalendarPeriodID, rosterValidFrom, scheduleValidUntil, previousCalendarPeriodID); err != nil {
		return nil, err
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
			return nil, &ScheduleError{Op: "update template: create supervisor", Err: err}
		}
	}
	return retiredStudentIDs, nil
}

type rosterRetirementAction uint8

const (
	rosterRetirementSkip rosterRetirementAction = iota
	rosterRetirementPreserve
	rosterRetirementDelete
	rosterRetirementClose
)

// retireTemplateEnrollments retires the roster rows the replacement is about
// to rewrite. The second return value lists the students whose rows were
// actually deleted or closed (#2147 review) — their future coverage shrank,
// so already-materialized occurrences may need reconciling.
func (s *TimetableDataService) retireTemplateEnrollments(
	ctx context.Context,
	templateID int64,
	calendarPeriodID *int64,
	weekdays []int,
	replacementFrom timezone.Date,
	replacementUntil *timezone.Date,
	previousCalendarPeriodID *int64,
) (map[int64]protectedStudentCoverage, []int64, error) {
	rows, err := s.deps.StudentEnrollmentRepo.FindByGroupID(ctx, templateID)
	if err != nil {
		return nil, nil, &ScheduleError{Op: "update template: load enrollments", Err: err}
	}
	protected, retiredStudentIDs, err := s.retireUnprotectedTemplateEnrollments(
		ctx,
		rows,
		calendarPeriodID,
		replacementFrom,
		replacementUntil,
		previousCalendarPeriodID,
	)
	if err != nil {
		return nil, nil, err
	}
	coverage, err := s.rebaseProtectedTemplateEnrollments(ctx, protected, calendarPeriodID, weekdays)
	if err != nil {
		return nil, nil, err
	}
	return coverage, retiredStudentIDs, nil
}

func (s *TimetableDataService) retireUnprotectedTemplateEnrollments(
	ctx context.Context,
	rows []*activitiesModel.StudentEnrollment,
	calendarPeriodID *int64,
	replacementFrom timezone.Date,
	replacementUntil *timezone.Date,
	previousCalendarPeriodID *int64,
) ([]*activitiesModel.StudentEnrollment, []int64, error) {
	protected := make([]*activitiesModel.StudentEnrollment, 0)
	retiredStudentIDs := make([]int64, 0)
	retiredSeen := make(map[int64]bool)
	for _, row := range rows {
		if row != nil && enrollmentIsProtected(row) &&
			validityWindowsOverlap(row.ValidFrom, row.ValidUntil, replacementFrom, replacementUntil) {
			protected = append(protected, row)
			continue
		}
		action := classifyEnrollmentRetirement(row, calendarPeriodID, replacementFrom, replacementUntil, previousCalendarPeriodID)
		if err := s.applyEnrollmentRetirement(ctx, row, action, replacementFrom); err != nil {
			return nil, nil, err
		}
		if (action == rosterRetirementDelete || action == rosterRetirementClose) && !retiredSeen[row.StudentID] {
			retiredSeen[row.StudentID] = true
			retiredStudentIDs = append(retiredStudentIDs, row.StudentID)
		}
	}
	return protected, retiredStudentIDs, nil
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
	previousCalendarPeriodID *int64,
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
		ownedRosterPeriodMatches(row.CalendarPeriodID, calendarPeriodID, previousCalendarPeriodID),
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
	previousCalendarPeriodID *int64,
) error {
	rows, err := s.deps.ActivitySupervisorRepo.FindByGroupID(ctx, templateID)
	if err != nil {
		return &ScheduleError{Op: "update template: load supervisors", Err: err}
	}
	for _, row := range rows {
		switch classifySupervisorRetirement(row, calendarPeriodID, replacementFrom, replacementUntil, previousCalendarPeriodID) {
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
	previousCalendarPeriodID *int64,
) rosterRetirementAction {
	if row == nil || !validityWindowsOverlap(row.ValidFrom, row.ValidUntil, replacementFrom, replacementUntil) {
		return rosterRetirementSkip
	}
	return classifyOwnedRosterRetirement(
		row.ValidFrom,
		row.ValidUntil,
		ownedRosterPeriodMatches(row.CalendarPeriodID, calendarPeriodID, previousCalendarPeriodID),
		replacementFrom,
		replacementUntil,
	)
}

func ownedRosterPeriodMatches(rowPeriodID, targetPeriodID, previousPeriodID *int64) bool {
	return optionalInt64sEqual(rowPeriodID, targetPeriodID) ||
		optionalInt64sEqual(rowPeriodID, previousPeriodID)
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
