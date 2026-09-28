package compose

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
)

const (
	updateTemplateOp       = "update template"
	updateTemplateFieldsOp = "update template: update fields"
	archiveTemplateOp      = "archive template"
)

// TemplateUpdateInput carries the template fields, recurrence shape, and
// roster edited by PUT /timetable/templates/{id}. The validity envelope is
// deliberately not part of this input — it is an invariant of the existing
// split-series segment and survives an edit unchanged — with one narrow
// exception: StartDate may pull a not-yet-started segment's valid_from
// EARLIER (#2226).
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
	WeekdayAssignments []timetable.WeekdayRosterAssignment
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
	// StartDate pulls a not-yet-started series forward (#2226): the schedule
	// envelope's inclusive valid_from and the series-managed roster move to
	// this earlier date. Only earlier-than-stored, not-in-the-past dates that
	// stay clear of every capped predecessor segment are accepted; equal to
	// the stored start is an idempotent no-op. Nil keeps the stored envelope
	// untouched, exactly as before.
	StartDate *timezone.Date
}

// UpdateTemplate replaces a template's editable fields, schedules, and roster
// while preserving the segment's inclusive valid_from and exclusive
// valid_until boundaries across all three. All schedule rows of a segment must
// share one envelope; inconsistent existing rows are rejected before mutation.
func (s *TemplateService) UpdateTemplate(ctx context.Context, in TemplateUpdateInput) error {
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

func (s *TemplateService) validateTemplateUpdateRequest(ctx context.Context, in TemplateUpdateInput) (int64, error) {
	if err := validateTemplateUpdateInput(in, s.deps.SchoolClasses); err != nil {
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

// templateUpdateState is what the update read before its first write: the
// segment envelope, the previous schedules and the series values the later
// steps compare against.
type templateUpdateState struct {
	validFrom                 *timezone.Date
	validUntil                *timezone.Date
	previousSchedules         []*activitiesModel.Schedule
	previousListKind          *string
	previousSourceOfferingIDs []int64
	previousCalendarPeriodID  *int64
}

func (s *TemplateService) updateTemplateLocked(
	ctx context.Context,
	in TemplateUpdateInput,
	tenantID int64,
	targetsProvided bool,
) error {
	if err := s.lockRecurrence(ctx, updateTemplateOp); err != nil {
		return err
	}
	existing, err := s.loadUpdateTarget(ctx, in)
	if err != nil {
		return err
	}
	if err := s.resolveUpdateTargets(ctx, &in, existing, targetsProvided); err != nil {
		return err
	}
	state, err := s.loadTemplateUpdateState(ctx, in, existing)
	if err != nil {
		return err
	}
	if err := s.writeTemplateFields(ctx, in, state, targetsProvided); err != nil {
		return err
	}
	return s.writeTemplateRecurrenceAndRoster(ctx, in, tenantID, state)
}

// loadUpdateTarget loads the edited segment and validates the references the
// edit changes. An archived or missing segment is the active-CRUD 404.
func (s *TemplateService) loadUpdateTarget(ctx context.Context, in TemplateUpdateInput) (*activitiesModel.Group, error) {
	existing, err := s.deps.ActivityGroupRepo.FindByID(ctx, in.TemplateID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return nil, &ScheduleError{Op: "update template: load target", Err: timetable.ErrTemplateSegmentNotEditable}
		}
		return nil, &ScheduleError{Op: "update template: load target", Err: err}
	}
	if existing == nil || !existing.IsTemplate || existing.ArchivedAt != nil {
		return nil, &ScheduleError{Op: "update template: load target", Err: timetable.ErrTemplateSegmentNotEditable}
	}
	if in.Fields.CategoryID != existing.CategoryID {
		if err := validateAssignableCategory(ctx, s.deps.ActivityCategoryRepo, in.Fields.CategoryID, "update template: validate category"); err != nil {
			return nil, err
		}
	}
	if in.Fields.PlanningTrackIDProvided && !samePlanningTrackID(in.Fields.PlanningTrackID, existing.PlanningTrackID) {
		if err := validateAssignablePlanningTrack(ctx, s.deps.PlanningTracks, in.Fields.PlanningTrackID, existing.PlanningTrackID); err != nil {
			return nil, err
		}
	}
	return existing, nil
}

// resolveUpdateTargets keeps the stored dynamic targets when the edit sent
// none and checks the resulting targets against the grade cap and the
// education groups.
func (s *TemplateService) resolveUpdateTargets(
	ctx context.Context,
	in *TemplateUpdateInput,
	existing *activitiesModel.Group,
	targetsProvided bool,
) error {
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
	return nil
}

// loadTemplateUpdateState reads the segment envelope (pulled forward when the
// edit asks for it) and snapshots the values the later steps compare against
// before the field write changes them.
func (s *TemplateService) loadTemplateUpdateState(
	ctx context.Context,
	in TemplateUpdateInput,
	existing *activitiesModel.Group,
) (templateUpdateState, error) {
	validFrom, validUntil, err := s.loadEditableTemplateEnvelope(ctx, in.TemplateID)
	if err != nil {
		return templateUpdateState{}, err
	}
	validFrom, err = s.resolvePulledForwardStart(ctx, in, validFrom)
	if err != nil {
		return templateUpdateState{}, err
	}
	previousSchedules, err := s.deps.ActivityScheduleRepo.FindByGroupID(ctx, in.TemplateID)
	if err != nil {
		return templateUpdateState{}, &ScheduleError{Op: "update template: load previous schedules", Err: err}
	}
	if err := validateLegacyTemplateWeekdays(previousSchedules, in.Weekdays); err != nil {
		return templateUpdateState{}, &ScheduleError{Op: "update template: validate weekdays", Err: err}
	}
	// Capture the series' current Listenart before the field write so the
	// instance propagation can tell an untouched occurrence (still carrying the
	// series value) from a per-occurrence override.
	return templateUpdateState{
		validFrom:                 validFrom,
		validUntil:                validUntil,
		previousSchedules:         previousSchedules,
		previousListKind:          existing.ListKind,
		previousSourceOfferingIDs: append([]int64(nil), existing.SourceCareOfferingIDs...),
		previousCalendarPeriodID:  cloneOptionalInt64(existing.CalendarPeriodID),
	}, nil
}

// writeTemplateFields writes the template row, its targets and the Listenart
// of its future occurrences.
func (s *TemplateService) writeTemplateFields(ctx context.Context, in TemplateUpdateInput, state templateUpdateState, targetsProvided bool) error {
	// Same pre-write guard as on create: the merged source must resolve before
	// the field write stamps it onto the group row, otherwise an unknown
	// offering trips the FK (500) before the resync can classify it as
	// ErrOfferingSourceInvalid (400) (#2147 review round 18).
	if err := s.validateOfferingSourceReference(ctx, in.Fields.SourceCareOfferingIDs, state.previousSourceOfferingIDs, in.CalendarPeriodID, "update template: validate offering source"); err != nil {
		return err
	}
	if err := s.updateTemplateFields(ctx, in); err != nil {
		return err
	}
	if targetsProvided {
		targetRepo, ok := s.deps.ActivityGroupRepo.(activitiesModel.GroupTargetRepository)
		if !ok {
			return &ScheduleError{Op: "update template: replace targets", Err: errors.New("target repository is not configured")}
		}
		if err := targetRepo.ReplaceTargets(ctx, in.TemplateID, in.Targets); err != nil {
			return &ScheduleError{Op: "update template: replace targets", Err: err}
		}
	}
	return s.propagateListKindToInstances(ctx, in.TemplateID, state.previousListKind, in.Fields.ListKind)
}

// writeTemplateRecurrenceAndRoster replaces the schedules and the roster and
// realigns the occurrences the edit reaches.
func (s *TemplateService) writeTemplateRecurrenceAndRoster(ctx context.Context, in TemplateUpdateInput, tenantID int64, state templateUpdateState) error {
	if err := s.replaceTemplateSchedules(ctx, in, tenantID, state.validFrom, state.validUntil); err != nil {
		return err
	}
	// Resync AFTER the schedule replacement so a pulled-forward sourced series
	// sees the widened envelope, but BEFORE the roster replacement: sourced rows
	// are protected there
	// (EnrollmentRequestChildID != nil), so a removed source must clear its
	// rows first or a manually re-picked child would end up with no row at
	// all (the protected row suppresses the manual create, then a later
	// cleanup would delete it).
	if err := s.resyncUpdatedTemplateOfferingRoster(ctx, in, state.previousSourceOfferingIDs, state.validFrom); err != nil {
		return err
	}
	if err := s.deleteRemovedLegacyWeekendInstances(ctx, in.TemplateID, state.previousSchedules, in.Weekdays); err != nil {
		return err
	}
	retiredStudentIDs, err := s.replaceTemplateRoster(ctx, in, tenantID, state.validFrom, state.validUntil, state.previousCalendarPeriodID)
	if err != nil {
		return err
	}
	if err := s.reconcileManualRosterInstances(ctx, in, state.previousSourceOfferingIDs, state.validFrom, retiredStudentIDs); err != nil {
		return err
	}
	if err := s.reconcileSeriesPredecessorRoster(ctx, in, tenantID, state.validFrom); err != nil {
		return err
	}
	if s.deps.CareOfferings.ValidateSeries == nil {
		return nil
	}
	if err := s.deps.CareOfferings.ValidateSeries(ctx, in.TemplateID); err != nil {
		return s.deps.CareOfferings.validationError(
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
func (s *TemplateService) resyncUpdatedTemplateOfferingRoster(
	ctx context.Context,
	in TemplateUpdateInput,
	previousSourceOfferingIDs []int64,
	scheduleValidFrom *timezone.Date,
) error {
	if len(in.Fields.SourceCareOfferingIDs) == 0 && len(previousSourceOfferingIDs) == 0 {
		return nil
	}
	if s.deps.ResyncOfferingRoster == nil {
		return &ScheduleError{Op: updateTemplateOp, Err: errors.New("offering roster resync is not configured")}
	}
	if err := s.deps.ResyncOfferingRoster(ctx, timetable.OfferingRosterResyncInput{
		TemplateID:       in.TemplateID,
		OfferingIDs:      in.Fields.SourceCareOfferingIDs,
		GradeLevels:      in.Fields.SourceGradeLevels,
		SchoolClasses:    in.Fields.SourceSchoolClasses,
		CalendarPeriodID: in.CalendarPeriodID,
		EffectiveFrom:    offeringResyncBoundary(in.RosterValidFrom, scheduleValidFrom, s.deps.Today()),
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
func offeringResyncBoundary(rosterValidFrom timezone.Date, scheduleValidFrom *timezone.Date, today timezone.Date) timezone.Date {
	effectiveFrom := rosterValidFrom
	if scheduleValidFrom != nil {
		effectiveFrom = *scheduleValidFrom
	}
	if effectiveFrom.Before(today) {
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
func (s *TemplateService) reconcileManualRosterInstances(
	ctx context.Context,
	in TemplateUpdateInput,
	previousSourceOfferingIDs []int64,
	scheduleValidFrom *timezone.Date,
	retiredStudentIDs []int64,
) error {
	if len(in.Fields.SourceCareOfferingIDs) == 0 && len(previousSourceOfferingIDs) == 0 {
		return nil
	}
	if s.deps.ActivityInstanceRepo == nil || s.deps.InstanceStudentRepo == nil || s.deps.StudentEnrollmentRepo == nil {
		return nil // read-only test facades have no occurrences to reconcile
	}
	studentIDs := unionStudentIDs(manualRosterStudentIDs(in), retiredStudentIDs)
	if len(studentIDs) == 0 {
		return nil
	}
	// No prior-enrollment snapshot: both shapes re-establish coverage on
	// purpose. A re-picked child's instance rows were just removed by the
	// source resync and must come back; retired students only lose rows.
	if _, _, err := s.newRosterReconciler().reconcileSourcedTemplateRosters(
		ctx,
		in.TemplateID,
		studentIDs,
		offeringResyncBoundary(in.RosterValidFrom, scheduleValidFrom, s.deps.Today()),
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
				return timetable.ErrTemplateWeekendWeekday
			}
		}
	}
	return nil
}

type legacyWeekendInstanceCleaner interface {
	DeletePlannedMaterializedWeekendInstances(context.Context, int64, []int) (int64, error)
}

func (s *TemplateService) deleteRemovedLegacyWeekendInstances(ctx context.Context, templateID int64, previous []*activitiesModel.Schedule, requested []int) error {
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
		announceStaffingChanged(ctx, s.deps.Staffing, s.getLogger(), "template_legacy_weekend_cleanup")
	}
	return nil
}

// resolvePulledForwardStart applies the pull-forward series start (#2226) to
// the loaded schedule envelope. Without a requested StartDate — or with one
// equal to the stored start (idempotent PUT retries) — the stored valid_from
// passes through untouched. An accepted pull returns the earlier date, which
// the caller then stamps onto the replaced schedules and roster alike. The
// rules mirror the issue's scope: only earlier (a nil stored start means the
// series already begins with its period — nothing lies in front of it), never
// into the past, and never into a capped predecessor segment's window.
func (s *TemplateService) resolvePulledForwardStart(
	ctx context.Context,
	in TemplateUpdateInput,
	storedFrom *timezone.Date,
) (*timezone.Date, error) {
	if in.StartDate == nil {
		return storedFrom, nil
	}
	const op = "update template: pull series start forward"
	newStart := *in.StartDate
	if storedFrom != nil && newStart == *storedFrom {
		return storedFrom, nil
	}
	if newStart.Before(s.deps.Today()) {
		return nil, &ScheduleError{Op: op, Err: timetable.ErrTemplateStartInPast}
	}
	if storedFrom == nil || !newStart.Before(*storedFrom) {
		return nil, &ScheduleError{Op: op, Err: timetable.ErrTemplateStartNotEarlier}
	}
	segments, err := loadTemplateSeriesSegments(
		ctx,
		s.deps.ActivityGroupRepo,
		s.deps.ActivityScheduleRepo,
		s.getLogger(),
		in.TemplateID,
	)
	if err != nil {
		return nil, err
	}
	for i := range segments {
		if predecessorOverlapsPulledStart(&segments[i], in.TemplateID, *storedFrom, newStart) {
			return nil, &ScheduleError{Op: op, Err: timetable.ErrTemplateStartPredecessorOverlap}
		}
	}
	return in.StartDate, nil
}

// predecessorOverlapsPulledStart reports whether a capped predecessor
// segment's window reaches past the pulled-forward start.
func predecessorOverlapsPulledStart(seg *templateSeriesSegment, templateID int64, storedFrom, newStart timezone.Date) bool {
	if seg.Group == nil || seg.Group.ID == templateID || seg.ValidUntil == nil {
		return false
	}
	if seg.ValidFrom != nil && !seg.ValidFrom.Before(*seg.ValidUntil) {
		return false // empty window, nothing to overlap
	}
	// Only segments that begin BEFORE the edited segment are its
	// predecessors (same ordering rule as segmentIsReconcilablePredecessor).
	// The editable segment is the chain's open tail, so anything else
	// should satisfy this anyway — the filter guards against anomalies.
	if seg.ValidFrom != nil && !seg.ValidFrom.Before(storedFrom) {
		return false
	}
	// valid_until is exclusive: a new start ON the predecessor's end is
	// contiguous, anything before it overlaps.
	return seg.ValidUntil.After(newStart)
}

func (s *TemplateService) loadEditableTemplateEnvelope(
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
		return nil, nil, &ScheduleError{Op: "update template: inspect schedule validity", Err: timetable.ErrTemplateSegmentNotEditable}
	}
	return validFrom, validUntil, nil
}

func (s *TemplateService) updateTemplateFields(ctx context.Context, in TemplateUpdateInput) error {
	updated, err := s.deps.ActivityGroupRepo.UpdateTemplateFields(ctx, in.TemplateID, in.Fields)
	if err != nil {
		return &ScheduleError{Op: updateTemplateFieldsOp, Err: err}
	}
	if updated == 0 {
		// Archive can commit while the handler's read-only preflight waits for
		// this recurrence gate. Preserve the active-CRUD 404 contract.
		return &ScheduleError{Op: updateTemplateFieldsOp, Err: timetable.ErrTemplateSegmentNotEditable}
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
func (s *TemplateService) propagateListKindToInstances(
	ctx context.Context,
	templateID int64,
	previousKind, newKind *string,
) error {
	if s.deps.ActivityInstanceRepo == nil || sameListKind(previousKind, newKind) {
		return nil
	}
	if _, err := s.deps.ActivityInstanceRepo.PropagateListKindToFutureInstances(
		ctx, templateID, previousKind, newKind, scheduleModel.Date(s.deps.Today()),
	); err != nil {
		return &ScheduleError{Op: "update template: propagate list kind", Err: err}
	}
	return nil
}

func (s *TemplateService) replaceTemplateSchedules(
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
			ValidFrom:        activityDatePtr(validFrom),
			ValidUntil:       activityDatePtr(validUntil),
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
func (s *TemplateService) ArchiveTemplate(ctx context.Context, templateID int64) (int64, error) {
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
		if err := s.lockRecurrence(txCtx, archiveTemplateOp); err != nil {
			return err
		}
		var err error
		archived, err = s.deps.ActivityGroupRepo.ArchiveTemplate(txCtx, templateID)
		if err != nil {
			return &ScheduleError{Op: "archive template: update", Err: err}
		}
		if archived > 0 && s.deps.CareOfferings.ValidateSeries != nil {
			if err := s.deps.CareOfferings.ValidateSeries(txCtx, templateID); err != nil {
				return s.deps.CareOfferings.validationError(
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

func validateTemplateUpdateInput(in TemplateUpdateInput, rules SchoolClassRules) error {
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
	if in.Fields.MaxParticipants < 0 {
		return errors.New("max participants cannot be negative")
	}
	if in.CalendarPeriodID != nil && *in.CalendarPeriodID <= 0 {
		return errors.New("calendar period id must be positive when set")
	}
	if in.RosterValidFrom.IsZero() {
		return errors.New("roster valid_from is required")
	}
	if err := validateTemplateGradeLevelMax(rules, in.GradeLevelMax); err != nil {
		return err
	}
	return validateOfferingSourceInput(
		in.Fields.SourceCareOfferingIDs,
		in.Fields.SourceGradeLevels,
		in.Fields.SourceSchoolClasses,
		in.Fields.TargetGroupType,
		in.StudentIDs,
		in.WeekdayAssignments,
		rules,
	)
}
