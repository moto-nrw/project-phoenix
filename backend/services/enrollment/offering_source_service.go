package enrollment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"

	"github.com/moto-nrw/project-phoenix/internal/schoolclass"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
)

// OfferingRosterResyncer is the factory-facing view of the offering-source
// roster resync (#2137). The decision service implements it; the factory
// injects it into the schedule layer's TimetableDataDependencies as a func
// hook to avoid an enrollment→schedule package cycle.
type OfferingRosterResyncer interface {
	ResyncTemplateOfferingRoster(ctx context.Context, in scheduleService.OfferingRosterResyncInput) error
}

// sourcedRosterTarget is one wanted roster row of an offering-sourced
// template: the child's student, the draft describing period + weekdays, and
// the window the child actually holds the offering in. validUntil is
// exclusive, matching activities.StudentEnrollment. A child with
// non-contiguous offering links (left and later re-joined) yields one target
// per disjoint window — bridging the gap would plan the child on days it does
// not hold the offering.
type sourcedRosterTarget struct {
	studentID  int64
	draft      *careEnrollmentDraft
	validFrom  timezone.Date
	validUntil timezone.Date
}

// ResyncTemplateOfferingRoster implements the schedule layer's
// ResyncOfferingRoster hook (#2137): it reconciles the offering-sourced
// student_enrollments rows of one template with the offering's currently
// approved enrollments, applying the template's Jahrgang filter.
//
//   - rows for children that still match keep their row when start and
//     weekday set line up with a wanted window; the end is then set exactly
//     to where the child's offering window reaches (extended OR shrunk)
//   - rows for children that no longer match — including rows whose validity
//     no longer fits any wanted window, e.g. after a source switch — are
//     deleted (not yet effective) or capped at EffectiveFrom (already
//     started); the reconcile diffs EVERY tagged row of the template, so no
//     offering-scoped cleanup via PreviousOfferingID is needed
//   - missing children are seeded via the same draft/persist shapes the
//     decision fan-out uses, so both write paths stay byte-compatible
//   - rows fed by a legacy CareOffering.ActivityGroupID link pointing at the
//     same template are protected and never touched
//   - wanted windows stop at the template's own schedule envelope, so a
//     capped split predecessor is never planned past its segment end
//   - finally, the touched students' already-materialized future occurrences
//     are reconciled (the materializer never revisits existing instances)
//
// Runs inside the template save's tenant transaction; the caller already
// holds the tenant recurrence lock.
func (s *decisionService) ResyncTemplateOfferingRoster(ctx context.Context, in scheduleService.OfferingRosterResyncInput) error {
	if in.TemplateID <= 0 {
		return fmt.Errorf("%w: template id is required", scheduleService.ErrOfferingSourceInvalid)
	}
	if in.EffectiveFrom.IsZero() {
		return fmt.Errorf("%w: effective_from is required", scheduleService.ErrOfferingSourceInvalid)
	}
	if !s.hasEnrollmentMaterializationDependencies() {
		return fmt.Errorf("offering roster resync: enrollment repositories are not configured")
	}

	var phase *enrollmentModels.Phase
	wanted := make(map[int64][]*sourcedRosterTarget)
	if in.OfferingID != nil {
		offering, offeringPhase, err := s.loadOfferingSource(ctx, *in.OfferingID, in.CalendarPeriodID)
		if err != nil {
			return err
		}
		phase = offeringPhase
		// The wanted windows are additionally bounded by the template's own
		// schedule envelope: the tenant-wide resync also visits capped split
		// predecessors, whose rows must never be extended past the segment end
		// (#2147 review). Open envelopes leave the windows untouched.
		schedules, err := s.ActivityScheduleRepo.FindByGroupID(ctx, in.TemplateID)
		if err != nil {
			return fmt.Errorf("offering roster resync: load template schedules: %w", err)
		}
		envelopeFrom, envelopeUntil := scheduleValidityBounds(schedules)
		wanted, err = s.wantedSourcedRosterTargets(ctx, offering, phase, in, envelopeFrom, envelopeUntil)
		if err != nil {
			return err
		}
	}

	protectedChildren, err := s.legacyLinkedChildIDs(ctx, in.TemplateID, in.EffectiveFrom)
	if err != nil {
		return err
	}

	// Students whose rows this resync creates, caps, deletes, or resizes. Their
	// already-materialized future occurrences must be reconciled afterwards —
	// scoped to exactly these students so per-occurrence hand edits of
	// untouched children survive (#2147 review).
	touchedStudents := make(map[int64]bool)

	rows, err := s.StudentEnrollmentRepo.FindByGroupID(ctx, in.TemplateID)
	if err != nil {
		return fmt.Errorf("offering roster resync: load template enrollments: %w", err)
	}
	for _, row := range rows {
		changed, err := s.reconcileSourcedRosterRow(ctx, row, wanted, protectedChildren, in.EffectiveFrom)
		if err != nil {
			return err
		}
		if changed {
			touchedStudents[row.StudentID] = true
		}
	}

	childIDs := make([]int64, 0, len(wanted))
	for childID := range wanted {
		childIDs = append(childIDs, childID)
	}
	sort.Slice(childIDs, func(i, j int) bool { return childIDs[i] < childIDs[j] })
	for _, childID := range childIDs {
		for _, target := range wanted[childID] {
			row := studentEnrollmentFromCareDraft(childID, target.studentID, phase, target.draft, &target.validFrom)
			// The child's offering link can start after the template's effective
			// date and end before the phase does (dated switch into or out of the
			// offering). Planning them for the full phase window would place them
			// on days they do not hold the offering.
			validUntil := target.validUntil
			row.ValidUntil = &validUntil
			if err := row.Validate(); err != nil {
				return fmt.Errorf("offering roster resync: validate seeded enrollment: %w", err)
			}
			if err := s.StudentEnrollmentRepo.Create(ctx, row); err != nil {
				return fmt.Errorf("offering roster resync: create seeded enrollment: %w", err)
			}
			touchedStudents[target.studentID] = true
		}
	}
	return s.reconcileSourcedInstanceRosters(ctx, in.TemplateID, touchedStudents, in.EffectiveFrom)
}

// reconcileSourcedInstanceRosters propagates the resync's enrollment changes
// onto the template's already-materialized future occurrences (#2147 review).
// The materializer is insert-only and never revisits an existing instance, so
// without this pass schedule.instance_students — and every children/staffing
// count derived from it — stays stale until a manual re-plan.
func (s *decisionService) reconcileSourcedInstanceRosters(
	ctx context.Context,
	templateID int64,
	students map[int64]bool,
	effectiveFrom timezone.Date,
) error {
	if len(students) == 0 {
		return nil
	}
	if s.InstanceRosters == nil {
		// Mock-only wirings (unit tests) may omit the reconciler; production
		// always injects it via the factory.
		s.Logger.Warn("offering roster resync: instance roster reconciler not configured; materialized occurrences may be stale",
			slog.Int64("template_id", templateID))
		return nil
	}
	studentIDs := make([]int64, 0, len(students))
	for studentID := range students {
		studentIDs = append(studentIDs, studentID)
	}
	sort.Slice(studentIDs, func(i, j int) bool { return studentIDs[i] < studentIDs[j] })
	if _, _, err := s.InstanceRosters.ReconcileSourcedTemplateRosters(ctx, templateID, studentIDs, effectiveFrom); err != nil {
		return fmt.Errorf("offering roster resync: reconcile materialized occurrences: %w", err)
	}
	return nil
}

// SourcedInstanceRosterReconciler propagates sourced-roster changes onto
// already-materialized future timetable occurrences (#2147 review).
// Implemented by services/schedule.RosterReconciler; injected here to avoid
// widening the decision service's repo surface.
type SourcedInstanceRosterReconciler interface {
	ReconcileSourcedTemplateRosters(ctx context.Context, templateID int64, studentIDs []int64, from timezone.Date) (int, int, error)
}

// scheduleValidityBounds returns the union recurrence envelope of a template's
// schedule rows. A side is nil (open) when any row is open on that side;
// otherwise the widest bound wins. Segment rows share one envelope per the
// split-series invariant — the union keeps drifted rows honest instead of
// trusting an arbitrary one.
func scheduleValidityBounds(schedules []*activities.Schedule) (validFrom, validUntil *timezone.Date) {
	fromOpen, untilOpen := false, false
	for _, sch := range schedules {
		if sch == nil {
			continue
		}
		switch {
		case sch.ValidFrom == nil:
			fromOpen, validFrom = true, nil
		case !fromOpen && (validFrom == nil || sch.ValidFrom.Before(*validFrom)):
			cloned := *sch.ValidFrom
			validFrom = &cloned
		}
		switch {
		case sch.ValidUntil == nil:
			untilOpen, validUntil = true, nil
		case !untilOpen && (validUntil == nil || sch.ValidUntil.After(*validUntil)):
			cloned := *sch.ValidUntil
			validUntil = &cloned
		}
	}
	return validFrom, validUntil
}

// ResyncOfferingSourcedTemplates re-reconciles EVERY offering-sourced
// template of the tenant against its offering's approved children. Grade
// transitions call it (apply AND revert) after rewriting school classes:
// promotions move children between Jahrgänge, so a template with a grade
// filter must lose the children that left its filter and gain the ones that
// entered it — and graduations turn children into alumni, whose rows are
// capped. History before effectiveFrom stays untouched.
//
// A template whose source has drifted invalid (offering deleted mid-flight,
// phase outside the period) is skipped with a warning, mirroring the decision
// fan-out: one broken template must not block a whole grade transition.
// The caller must hold the tenant recurrence lock.
func (s *decisionService) ResyncOfferingSourcedTemplates(ctx context.Context, effectiveFrom timezone.Date) error {
	if !s.hasEnrollmentMaterializationDependencies() {
		s.Logger.Warn("offering roster resync: enrollment repositories not configured; skipping sourced-template resync")
		return nil
	}
	templates, err := s.ActivityGroupRepo.FindTemplatesWithOfferingSource(ctx)
	if err != nil {
		return fmt.Errorf("offering roster resync: list sourced templates: %w", err)
	}
	for _, tmpl := range templates {
		if tmpl == nil || tmpl.SourceCareOfferingID == nil {
			continue
		}
		err := s.ResyncTemplateOfferingRoster(ctx, scheduleService.OfferingRosterResyncInput{
			TemplateID:         tmpl.ID,
			PreviousOfferingID: tmpl.SourceCareOfferingID,
			OfferingID:         tmpl.SourceCareOfferingID,
			GradeLevels:        tmpl.SourceGradeLevels,
			CalendarPeriodID:   tmpl.CalendarPeriodID,
			EffectiveFrom:      effectiveFrom,
		})
		if err != nil {
			if errors.Is(err, scheduleService.ErrOfferingSourceInvalid) {
				s.logSkippedSourcedTemplate(tmpl.ID, *tmpl.SourceCareOfferingID, "tenant-wide resync: source invalid", err)
				continue
			}
			return fmt.Errorf("offering roster resync: template %d: %w", tmpl.ID, err)
		}
	}
	return nil
}

// loadOfferingSource resolves and validates the source offering: it must
// exist in this tenant, and its phase's service window must lie within the
// template's calendar period (when the template is period-pinned) — otherwise
// the seeded rows could never materialize and the editor would silently save
// a dead rule.
func (s *decisionService) loadOfferingSource(
	ctx context.Context,
	offeringID int64,
	calendarPeriodID *int64,
) (*enrollmentModels.CareOffering, *enrollmentModels.Phase, error) {
	return loadValidatedOfferingSource(ctx, s.CareOfferingRepo, s.PhaseRepo, s.CalendarPeriodRepo, offeringID, calendarPeriodID)
}

// loadValidatedOfferingSource is the shared offering-source guard (#2137):
// the offering must exist in this tenant and its phase's service window must
// lie within the given calendar period (nil skips the period check). Every
// declaration path uses it — template create/update via the resync, and the
// split via CareOfferingSeriesValidator.ValidateTemplateOfferingSource — so
// no path can persist a source rule the others would reject.
func loadValidatedOfferingSource(
	ctx context.Context,
	offeringRepo enrollmentModels.CareOfferingRepository,
	phaseRepo enrollmentModels.PhaseRepository,
	periodRepo scheduleModels.CalendarPeriodRepository,
	offeringID int64,
	calendarPeriodID *int64,
) (*enrollmentModels.CareOffering, *enrollmentModels.Phase, error) {
	offering, err := offeringRepo.FindByID(ctx, offeringID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return nil, nil, fmt.Errorf("%w: care offering %d not found", scheduleService.ErrOfferingSourceInvalid, offeringID)
		}
		return nil, nil, fmt.Errorf("offering roster resync: load offering: %w", err)
	}
	if offering == nil {
		return nil, nil, fmt.Errorf("%w: care offering %d not found", scheduleService.ErrOfferingSourceInvalid, offeringID)
	}
	phase, err := phaseRepo.FindByID(ctx, offering.PhaseID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return nil, nil, fmt.Errorf("%w: enrollment phase of care offering %d not found", scheduleService.ErrOfferingSourceInvalid, offeringID)
		}
		return nil, nil, fmt.Errorf("offering roster resync: load phase: %w", err)
	}
	if calendarPeriodID != nil {
		period, err := periodRepo.FindByID(ctx, *calendarPeriodID)
		if err != nil {
			if modelBase.IsNoRows(err) {
				return nil, nil, fmt.Errorf("%w: calendar period %d not found", scheduleService.ErrOfferingSourceInvalid, *calendarPeriodID)
			}
			return nil, nil, fmt.Errorf("offering roster resync: load calendar period: %w", err)
		}
		if err := validatePhaseWithinTemplatePeriod(phase, period); err != nil {
			return nil, nil, fmt.Errorf("%w: %s", scheduleService.ErrOfferingSourceInvalid, err.Error())
		}
	}
	return offering, phase, nil
}

// wantedSourcedRosterTargets builds the desired roster of the sourced
// template: every approved, grade-matching child of the offering with the
// weekday set their enrollment selects, bounded by the window their offering
// link is actually valid in. Each child maps to one target per disjoint
// link window (see coalesceSourcedRosterTargets).
func (s *decisionService) wantedSourcedRosterTargets(
	ctx context.Context,
	offering *enrollmentModels.CareOffering,
	phase *enrollmentModels.Phase,
	in scheduleService.OfferingRosterResyncInput,
	envelopeFrom, envelopeUntil *timezone.Date,
) (map[int64][]*sourcedRosterTarget, error) {
	children, err := s.RequestChildOfferingRepo.ListApprovedChildrenByCareOfferingIDs(ctx, []int64{offering.ID}, in.EffectiveFrom)
	if err != nil {
		return nil, fmt.Errorf("offering roster resync: list approved children: %w", err)
	}
	wanted := make(map[int64][]*sourcedRosterTarget, len(children))
	for _, child := range children {
		grade := gradeLevelFromSchoolClass(child.SchoolClass)
		if !gradeFilterMatches(in.GradeLevels, grade) {
			continue
		}
		validFrom, validUntil, ok := sourcedRosterWindow(child.Link, phase, in.EffectiveFrom, envelopeFrom, envelopeUntil)
		if !ok {
			// A link that only covers days before the template takes effect
			// (or outside the phase) has nothing left to plan.
			continue
		}
		days, err := effectiveOfferingDaysForEnrollment(offering, child.Link)
		if err != nil {
			return nil, fmt.Errorf("offering roster resync: resolve days for request child %d: %w", child.Link.RequestChildID, err)
		}
		draft, err := sourcedTemplateDraft(in.TemplateID, in.CalendarPeriodID, days)
		if err != nil {
			return nil, err
		}
		wanted[child.Link.RequestChildID] = append(wanted[child.Link.RequestChildID], &sourcedRosterTarget{
			studentID:  child.StudentID,
			draft:      draft,
			validFrom:  validFrom,
			validUntil: validUntil,
		})
	}
	for childID := range wanted {
		wanted[childID] = coalesceSourcedRosterTargets(wanted[childID])
	}
	return wanted, nil
}

// coalesceSourcedRosterTargets reduces a child's per-link windows to the
// minimal row set. Windows that overlap or touch AND select the same weekday
// set collapse into one row — the common dated-switch shape of consecutive
// links with unchanged days. Everything else stays a separate row: bridging a
// gap between two links would plan the child in a window it left the offering
// for, and unifying different weekday sets would plan days only one of the
// links selects.
func coalesceSourcedRosterTargets(targets []*sourcedRosterTarget) []*sourcedRosterTarget {
	sort.Slice(targets, func(i, j int) bool {
		if targets[i].validFrom != targets[j].validFrom {
			return targets[i].validFrom.Before(targets[j].validFrom)
		}
		return targets[i].validUntil.Before(targets[j].validUntil)
	})
	merged := make([]*sourcedRosterTarget, 0, len(targets))
	for _, target := range targets {
		if len(merged) > 0 {
			last := merged[len(merged)-1]
			if !target.validFrom.After(last.validUntil) && sameSourcedDraftDays(last.draft, target.draft) {
				if target.validUntil.After(last.validUntil) {
					last.validUntil = target.validUntil
				}
				continue
			}
		}
		merged = append(merged, target)
	}
	return merged
}

// sameSourcedDraftDays reports whether two drafts select the same weekday set.
func sameSourcedDraftDays(left, right *careEnrollmentDraft) bool {
	if left.allWeekdays != right.allWeekdays {
		return false
	}
	if left.allWeekdays {
		return true
	}
	if len(left.selectedWeekday) != len(right.selectedWeekday) {
		return false
	}
	for weekday := range left.selectedWeekday {
		if !right.selectedWeekday[weekday] {
			return false
		}
	}
	return true
}

// sourcedRosterWindow intersects four windows into the validity of one
// seeded roster row: the phase's service window, the template's effective
// date (history is never rewritten), the template's own schedule envelope
// (a capped split segment must not plan past its end nor before its start,
// #2147 review), and the child's offering link. The link bound is what keeps
// a dated switch honest — a selection starting in September must not plan the
// child from the template's save date onward, and one ending in September
// must not outlive it. ok is false when the intersection is empty.
func sourcedRosterWindow(
	link *enrollmentModels.RequestChildOffering,
	phase *enrollmentModels.Phase,
	effectiveFrom timezone.Date,
	envelopeFrom, envelopeUntil *timezone.Date,
) (validFrom, validUntil timezone.Date, ok bool) {
	validFrom = phase.ServiceStartDate
	if effectiveFrom.After(validFrom) {
		validFrom = effectiveFrom
	}
	if envelopeFrom != nil && envelopeFrom.After(validFrom) {
		validFrom = *envelopeFrom
	}
	validUntil = phase.ServiceEndDate.AddDays(1)
	if envelopeUntil != nil && envelopeUntil.Before(validUntil) {
		validUntil = *envelopeUntil
	}
	if link != nil {
		if link.ValidFrom != nil && link.ValidFrom.After(validFrom) {
			validFrom = *link.ValidFrom
		}
		if link.ValidUntil != nil && link.ValidUntil.Before(validUntil) {
			validUntil = *link.ValidUntil
		}
	}
	if !validFrom.Before(validUntil) {
		return timezone.Date{}, timezone.Date{}, false
	}
	return validFrom, validUntil, true
}

// reconcileSourcedRosterRow applies keep/cap/delete to one existing row.
// Manual rows (no request-child tag), history, and rows protected by a
// legacy offering link are left untouched.
//
// A row is only reused for a target whose window it can actually serve: the
// weekday/period draft must match AND the validity start must line up (either
// exactly, or the row already started before the rewrite boundary and the
// target begins at that boundary). Matching on the draft alone would let a
// row survive a source switch with a start or end outside the new offering
// link's validity. The end is then set EXACTLY — shrinking is as necessary as
// extending, because the child's offering window may have contracted.
//
// The returned bool reports whether the row was actually written (capped,
// deleted, or resized) — the callers feed those students into the
// materialized-instance reconcile (#2147 review).
func (s *decisionService) reconcileSourcedRosterRow(
	ctx context.Context,
	row *activities.StudentEnrollment,
	wanted map[int64][]*sourcedRosterTarget,
	protectedChildren map[int64]bool,
	effectiveFrom timezone.Date,
) (bool, error) {
	if row == nil || row.EnrollmentRequestChildID == nil {
		return false, nil
	}
	if row.ValidUntil != nil && !row.ValidUntil.After(effectiveFrom) {
		return false, nil // history stays
	}
	childID := *row.EnrollmentRequestChildID
	if idx := indexOfServableTarget(wanted[childID], row, effectiveFrom); idx >= 0 {
		target := wanted[childID][idx]
		wanted[childID] = append(wanted[childID][:idx], wanted[childID][idx+1:]...)
		if len(wanted[childID]) == 0 {
			delete(wanted, childID)
		}
		if protectedChildren[childID] {
			// The row is owned by the legacy CareOffering.ActivityGroupID feed:
			// consuming the target avoids seeding a duplicate, but its window
			// belongs to THAT offering's lifecycle and is never resized here.
			return false, nil
		}
		if row.ValidUntil == nil || *row.ValidUntil != target.validUntil {
			if err := s.StudentEnrollmentRepo.SetValidUntilByID(ctx, row.ID, target.validUntil); err != nil {
				return false, fmt.Errorf("offering roster resync: adjust retained enrollment: %w", err)
			}
			return true, nil
		}
		return false, nil
	}
	if protectedChildren[childID] {
		return false, nil
	}
	if !row.ValidFrom.Before(effectiveFrom) {
		if err := s.StudentEnrollmentRepo.Delete(ctx, row.ID); err != nil {
			return false, fmt.Errorf("offering roster resync: delete enrollment: %w", err)
		}
		return true, nil
	}
	if err := s.StudentEnrollmentRepo.SetValidUntilByID(ctx, row.ID, effectiveFrom); err != nil {
		return false, fmt.Errorf("offering roster resync: cap enrollment: %w", err)
	}
	return true, nil
}

// indexOfServableTarget returns the first target the row can serve without
// misrepresenting any day: draft (weekdays + period) equal and validity start
// aligned. A row whose ValidFrom lies before effectiveFrom carries immutable
// history, so it may serve a target starting exactly at the rewrite boundary;
// any other start mismatch means cap-and-reseed.
func indexOfServableTarget(
	targets []*sourcedRosterTarget,
	row *activities.StudentEnrollment,
	effectiveFrom timezone.Date,
) int {
	for i, target := range targets {
		if !careDraftMatchesEnrollment(target.draft, row) {
			continue
		}
		if row.ValidFrom == target.validFrom {
			return i
		}
		if row.ValidFrom.Before(effectiveFrom) && target.validFrom == effectiveFrom {
			return i
		}
	}
	return -1
}

// legacyLinkedChildIDs returns the request children whose roster rows on this
// template are owned by the legacy CareOffering.ActivityGroupID feed (#1651).
// The resync must never rewrite those: their lifecycle belongs to the
// decision/adjustment flows of THAT offering.
func (s *decisionService) legacyLinkedChildIDs(
	ctx context.Context,
	templateID int64,
	onOrAfter timezone.Date,
) (map[int64]bool, error) {
	offerings, err := s.CareOfferingRepo.ListByActivityGroupIDs(ctx, []int64{templateID})
	if err != nil {
		return nil, fmt.Errorf("offering roster resync: list legacy-linked offerings: %w", err)
	}
	if len(offerings) == 0 {
		return map[int64]bool{}, nil
	}
	offeringIDs := make([]int64, 0, len(offerings))
	for _, offering := range offerings {
		if offering != nil {
			offeringIDs = append(offeringIDs, offering.ID)
		}
	}
	children, err := s.RequestChildOfferingRepo.ListApprovedChildrenByCareOfferingIDs(ctx, offeringIDs, onOrAfter)
	if err != nil {
		return nil, fmt.Errorf("offering roster resync: list legacy-linked children: %w", err)
	}
	protected := make(map[int64]bool, len(children))
	for _, child := range children {
		protected[child.Link.RequestChildID] = true
	}
	return protected, nil
}

// sourcedTemplateDraft builds the careEnrollmentDraft for one child on one
// sourced template, mirroring mergeCareEnrollmentDraft's weekday rules: an
// empty day list means every weekday.
func sourcedTemplateDraft(templateID int64, calendarPeriodID *int64, days []string) (*careEnrollmentDraft, error) {
	var periodID *int64
	if calendarPeriodID != nil {
		cloned := *calendarPeriodID
		periodID = &cloned
	}
	draft := &careEnrollmentDraft{
		activityGroupID:  templateID,
		calendarPeriodID: periodID,
		selectedWeekday:  make(map[int]bool, len(days)),
	}
	if len(days) == 0 {
		draft.allWeekdays = true
		return draft, nil
	}
	for _, day := range days {
		weekday, ok := enrollmentDayToISOWeekday(day)
		if !ok {
			return nil, fmt.Errorf("offering roster resync: invalid selected day %q", day)
		}
		draft.selectedWeekday[weekday] = true
	}
	return draft, nil
}

// gradeLevelFromSchoolClass derives the numeric Jahrgang from the free-text
// school class ("3a" -> 3). Classes without a grade number ("Bienen") yield
// nil — a set filter then never matches, mirroring
// Group.MatchesSourceGradeFilter.
func gradeLevelFromSchoolClass(schoolClass string) *int16 {
	prefix := schoolclass.GradePrefix(schoolClass)
	if prefix == "" {
		return nil
	}
	parsed, err := strconv.Atoi(prefix)
	if err != nil || parsed < schoolclass.MinGradeLevel || parsed > schoolclass.MaxGradeLevel {
		return nil
	}
	grade := int16(parsed)
	return &grade
}

// gradeFilterMatches mirrors activities.Group.MatchesSourceGradeFilter for a
// raw filter slice.
func gradeFilterMatches(levels []int, grade *int16) bool {
	if len(levels) == 0 {
		return true
	}
	if grade == nil {
		return false
	}
	for _, level := range levels {
		if level == int(*grade) {
			return true
		}
	}
	return false
}

// logSkippedSourcedTemplate reports a sourced template the decision fan-out
// had to skip (misconfigured period/schedules). Approvals must not fail on a
// template drifted since its save; the editor surfaces the mismatch.
func (s *decisionService) logSkippedSourcedTemplate(templateID, offeringID int64, reason string, err error) {
	attrs := []any{
		slog.Int64("template_id", templateID),
		slog.Int64("care_offering_id", offeringID),
		slog.String("reason", reason),
	}
	if err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))
	}
	s.Logger.Warn("decision: skipping offering-sourced template", attrs...)
}

// OfferingSourcedTemplate is one template already sourcing an offering,
// exposed so the editor can warn about overlapping Jahrgang subsets before
// the admin saves a second Regeltermin over the same children.
type OfferingSourcedTemplate struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	GradeLevels []int  `json:"grade_levels,omitempty"`
}

// OfferingSourceOption is one selectable Betreuungsangebot in the Regeltermin
// editor (#2137), with per-grade counts of approved children (live filter
// preview) and the templates already sourcing it (overlap Hinweis).
type OfferingSourceOption struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	PhaseID   int64  `json:"phase_id"`
	PhaseName string `json:"phase_name"`
	// TotalCount is the number of distinct approved children currently or
	// prospectively enrolled in the offering.
	TotalCount int `json:"total_count"`
	// GradeCounts maps Jahrgang → approved children; key 0 collects children
	// whose school class carries no derivable grade number.
	GradeCounts            map[int]int               `json:"grade_counts"`
	SourcedTemplates       []OfferingSourcedTemplate `json:"sourced_templates"`
	LegacyLinkedTemplateID *int64                    `json:"legacy_linked_template_id,omitempty"`
}

// OfferingSourceOptionLister is the api/timetable-facing view of the editor
// support endpoint.
type OfferingSourceOptionLister interface {
	ListOfferingSourceOptions(ctx context.Context, calendarPeriodID *int64) ([]OfferingSourceOption, error)
}

// ListOfferingSourceOptions returns the offerings an admin may pick as a
// Regeltermin source. With a calendar period given, only offerings whose
// phase's service window lies within that period qualify — a source outside
// the Planungszeitraum could never materialize a single occurrence.
func (s *decisionService) ListOfferingSourceOptions(ctx context.Context, calendarPeriodID *int64) ([]OfferingSourceOption, error) {
	if s.CareOfferingRepo == nil || s.PhaseRepo == nil || s.RequestChildOfferingRepo == nil || s.ActivityGroupRepo == nil {
		return nil, fmt.Errorf("offering source options: repositories are not configured")
	}
	offerings, err := s.CareOfferingRepo.ListByTenant(ctx)
	if err != nil {
		return nil, fmt.Errorf("offering source options: list offerings: %w", err)
	}
	phases, err := s.offeringSourcePhases(ctx, calendarPeriodID)
	if err != nil {
		return nil, err
	}

	selected := make([]*enrollmentModels.CareOffering, 0, len(offerings))
	offeringIDs := make([]int64, 0, len(offerings))
	for _, offering := range offerings {
		if offering == nil || !offering.IsActive {
			continue
		}
		if _, ok := phases[offering.PhaseID]; !ok {
			continue
		}
		selected = append(selected, offering)
		offeringIDs = append(offeringIDs, offering.ID)
	}
	children, err := s.RequestChildOfferingRepo.ListApprovedChildrenByCareOfferingIDs(ctx, offeringIDs, timezone.TodayDate())
	if err != nil {
		return nil, fmt.Errorf("offering source options: list approved children: %w", err)
	}
	counts := groupOfferingGradeCounts(children)

	options := make([]OfferingSourceOption, 0, len(selected))
	for _, offering := range selected {
		option := OfferingSourceOption{
			ID:                     offering.ID,
			Name:                   offering.Name,
			PhaseID:                offering.PhaseID,
			GradeCounts:            map[int]int{},
			SourcedTemplates:       []OfferingSourcedTemplate{},
			LegacyLinkedTemplateID: offering.ActivityGroupID,
		}
		if phase := phases[offering.PhaseID]; phase != nil {
			option.PhaseName = phase.Name
		}
		if c := counts[offering.ID]; c != nil {
			option.TotalCount = c.total
			option.GradeCounts = c.byGrade
		}
		templates, err := s.ActivityGroupRepo.FindTemplatesBySourceOffering(ctx, offering.ID)
		if err != nil {
			return nil, fmt.Errorf("offering source options: list sourced templates: %w", err)
		}
		for _, tmpl := range templates {
			if tmpl == nil {
				continue
			}
			option.SourcedTemplates = append(option.SourcedTemplates, OfferingSourcedTemplate{
				ID:          tmpl.ID,
				Name:        tmpl.Name,
				GradeLevels: tmpl.SourceGradeLevels,
			})
		}
		options = append(options, option)
	}
	return options, nil
}

// offeringSourcePhases returns the tenant's phases keyed by id, restricted to
// those fitting the calendar period when one is given.
func (s *decisionService) offeringSourcePhases(ctx context.Context, calendarPeriodID *int64) (map[int64]*enrollmentModels.Phase, error) {
	phases, err := s.PhaseRepo.ListByTenant(ctx)
	if err != nil {
		return nil, fmt.Errorf("offering source options: list phases: %w", err)
	}
	var period *scheduleModels.CalendarPeriod
	if calendarPeriodID != nil {
		if s.CalendarPeriodRepo == nil {
			return nil, fmt.Errorf("offering source options: calendar period repository is not configured")
		}
		period, err = s.CalendarPeriodRepo.FindByID(ctx, *calendarPeriodID)
		if err != nil {
			if modelBase.IsNoRows(err) {
				return nil, fmt.Errorf("%w: calendar period %d not found", scheduleService.ErrOfferingSourceInvalid, *calendarPeriodID)
			}
			return nil, fmt.Errorf("offering source options: load calendar period: %w", err)
		}
	}
	byID := make(map[int64]*enrollmentModels.Phase, len(phases))
	for _, phase := range phases {
		if phase == nil {
			continue
		}
		if period != nil && validatePhaseWithinTemplatePeriod(phase, period) != nil {
			continue
		}
		byID[phase.ID] = phase
	}
	return byID, nil
}

type offeringGradeCount struct {
	total   int
	byGrade map[int]int
}

// groupOfferingGradeCounts aggregates approved children per offering into
// distinct-child totals and per-grade buckets (0 = no derivable grade).
func groupOfferingGradeCounts(children []*enrollmentModels.ApprovedOfferingChild) map[int64]*offeringGradeCount {
	counts := make(map[int64]*offeringGradeCount)
	seen := make(map[int64]map[int64]bool)
	for _, child := range children {
		if child == nil || child.Link == nil {
			continue
		}
		offeringID := child.Link.CareOfferingID
		if seen[offeringID] == nil {
			seen[offeringID] = make(map[int64]bool)
		}
		if seen[offeringID][child.Link.RequestChildID] {
			continue
		}
		seen[offeringID][child.Link.RequestChildID] = true
		count := counts[offeringID]
		if count == nil {
			count = &offeringGradeCount{byGrade: map[int]int{}}
			counts[offeringID] = count
		}
		count.total++
		bucket := 0
		if grade := gradeLevelFromSchoolClass(child.SchoolClass); grade != nil {
			bucket = int(*grade)
		}
		count.byGrade[bucket]++
	}
	return counts
}
