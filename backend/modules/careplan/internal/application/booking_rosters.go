package application

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// MaterializeApprovedBookings writes one roster row per booked offering whose
// care offering points at an activity group, or whose offering a template
// sources. Offerings without either are skipped. It leaves out the
// multi-source union resync: the approval runs BEFORE the child's status
// flips to approved, so the union — which resolves children by their approved
// status — cannot see the child yet; the decision runs
// ResyncMultiSourceTemplatesForChild after the flip.
func (m *BookingMaterialization) MaterializeApprovedBookings(ctx context.Context, requestChildID, studentID int64, phase careplan.OfferingPhase) error {
	return m.materializeEnrollmentsFrom(ctx, requestChildID, studentID, phase, nil, false)
}

// materializeEnrollmentsFrom writes the drafts of the child's bookings with an
// optional start override. startFrom is set when a dated adjustment replaces
// only the part of the phase window from that date onward; the rows before it
// were capped, not deleted, so the new rows must not reach back over them.
// resyncMultiSource is false only on the approval paths.
func (m *BookingMaterialization) materializeEnrollmentsFrom(
	ctx context.Context,
	requestChildID, studentID int64,
	phase careplan.OfferingPhase,
	startFrom *calendar.Date,
	resyncMultiSource bool,
) error {
	if err := m.lockTemplateRecurrence(ctx); err != nil {
		return err
	}
	drafts, multiSource, err := m.careEnrollmentDraftsForChild(ctx, requestChildID, studentID, phase)
	if err != nil {
		return err
	}
	if err := m.persistCareEnrollmentDrafts(ctx, requestChildID, studentID, phase, drafts, startFrom); err != nil {
		return err
	}
	// The materializer is insert-only and never revisits an existing
	// occurrence, so an approval after materialization must add the child to
	// the drafted templates' already-materialized future occurrences itself
	// (#2147 review).
	if err := m.reconcileEnrollmentInstanceRosters(ctx, studentID, draftGroupIDSet(drafts), m.enrollmentRewriteBoundary(startFrom)); err != nil {
		return err
	}
	// Multi-source templates were deliberately not drafted above; the resync
	// unions their offerings' children (including this one) and reconciles
	// materialized occurrences itself. Scoped to THIS child with a
	// phase-anchored boundary, so its rows start at the phase's service start
	// like the single-source drafts — an approval or undated correction into a
	// running phase must not lose the already-elapsed window. The scope keeps
	// the early boundary away from other children's capped history, and the
	// occurrence reconcile clamps to today internally.
	if !resyncMultiSource {
		return nil
	}
	multiSourceFrom := phase.ServiceStart
	if startFrom != nil && startFrom.After(multiSourceFrom) {
		multiSourceFrom = *startFrom
	}
	return m.resyncMultiSourceTemplates(ctx, multiSource, multiSourceFrom, []int64{requestChildID})
}

// enrollmentRewriteBoundary is the date from which a decision/adjustment flow
// may rewrite materialized occurrences: the flow's own start override, never
// earlier than today (history is observation, not plan).
func (m *BookingMaterialization) enrollmentRewriteBoundary(startFrom *calendar.Date) calendar.Date {
	boundary := m.todayDate()
	if startFrom != nil && startFrom.After(boundary) {
		boundary = *startFrom
	}
	return boundary
}

func draftGroupIDSet(drafts map[int64]*careEnrollmentDraft) map[int64]bool {
	groupIDs := make(map[int64]bool, len(drafts))
	for groupID := range drafts {
		groupIDs[groupID] = true
	}
	return groupIDs
}

// reconcileEnrollmentInstanceRosters propagates the enrollment rows a
// decision or adjustment flow just wrote, capped, or removed for ONE student
// onto the affected templates' already-materialized future occurrences
// (#2147 review). Without it, added or removed children keep stale
// schedule.instance_students rows — and stale staffing counts — until a
// manual re-plan; only the template-save resync reconciled them before.
func (m *BookingMaterialization) reconcileEnrollmentInstanceRosters(
	ctx context.Context,
	studentID int64,
	groupIDs map[int64]bool,
	from calendar.Date,
) error {
	if studentID <= 0 || len(groupIDs) == 0 {
		return nil
	}
	students := map[int64]bool{studentID: true}
	for _, groupID := range sortedIDSet(groupIDs) {
		// No prior-enrollment snapshot: a decision/adjustment writes THIS
		// student's coverage on purpose, so desired-but-missing rows are
		// (re)created rather than read as per-occurrence hand removals.
		if err := m.reconcileSourcedInstanceRosters(ctx, groupID, students, from, nil); err != nil {
			return err
		}
	}
	return nil
}

// reconcileSourcedInstanceRosters propagates the resync's enrollment changes
// onto the template's already-materialized future occurrences (#2147 review).
// The materializer is insert-only and never revisits an existing instance, so
// without this pass schedule.instance_students — and every children/staffing
// count derived from it — stays stale until a manual re-plan.
func (m *BookingMaterialization) reconcileSourcedInstanceRosters(
	ctx context.Context,
	templateID int64,
	students map[int64]bool,
	effectiveFrom calendar.Date,
	priorEnrollments []ports.RosterEnrollment,
) error {
	if len(students) == 0 {
		return nil
	}
	if err := m.deps.Rosters.ReconcileTemplateRosters(ctx, templateID, sortedIDSet(students), effectiveFrom, priorEnrollments); err != nil {
		return fmt.Errorf("offering roster resync: reconcile materialized occurrences: %w", err)
	}
	return nil
}

func (m *BookingMaterialization) careEnrollmentDraftsForChild(
	ctx context.Context,
	requestChildID, studentID int64,
	phase careplan.OfferingPhase,
) (map[int64]*careEnrollmentDraft, map[int64]ports.SourcedTemplate, error) {
	// A future phase's selections are bounded to its service window and are
	// therefore not active today while staff approve the request.
	links, err := m.deps.Enrollment.SelectionsAt(ctx, requestChildID, phase.ServiceStart)
	if err != nil {
		return nil, nil, fmt.Errorf("decision: list child offerings: %w", err)
	}
	return m.careEnrollmentDraftsForLinks(ctx, requestChildID, studentID, bookedOfferingPointers(links), phase)
}

// careEnrollmentDraftsForLinks materializes an explicitly supplied selection.
// Dated offering changes use it after scheduling their future links: reading
// the current links at that point would correctly return the old selection,
// but incorrectly materialize that old selection at the switch date. The
// second return value carries the multi-source templates the selection feeds;
// the caller must resync them after persisting the drafts.
func (m *BookingMaterialization) careEnrollmentDraftsForLinks(
	ctx context.Context,
	requestChildID, studentID int64,
	links []*careplan.BookedOffering,
	phase careplan.OfferingPhase,
) (map[int64]*careEnrollmentDraft, map[int64]ports.SourcedTemplate, error) {
	if len(links) == 0 {
		return map[int64]*careEnrollmentDraft{}, map[int64]ports.SourcedTemplate{}, nil
	}
	offerings, err := m.catalog.listByIDs(ctx, bookedOfferingIDs(links))
	if err != nil {
		return nil, nil, fmt.Errorf("decision: list linked care offerings: %w", err)
	}
	return m.buildCareEnrollmentDrafts(ctx, requestChildID, studentID, links, offerings, phase)
}

// ResyncMultiSourceTemplatesForChild re-reconciles every multi-source
// template fed by one of the child's offerings. Called AFTER the child's
// status flipped to approved: the union resync resolves children through
// their approved status, so the resync inside the approval's materialization
// pass cannot see the child yet.
//
// Mirrors the approval's careOfferingsEnabled gate: with the setting off the
// approval wrote no enrollment rows, so this post-flip pass must not start
// materializing either. It also takes the tenant recurrence lock itself,
// because with the setting off the locking materialization pass never ran in
// this transaction.
func (m *BookingMaterialization) ResyncMultiSourceTemplatesForChild(ctx context.Context, requestChildID int64, phase careplan.OfferingPhase) error {
	careOfferingsEnabled, err := m.careOfferingsEnabled(ctx)
	if err != nil {
		return fmt.Errorf("decision: resolve care offerings setting: %w", err)
	}
	if !careOfferingsEnabled {
		return nil
	}
	if err := m.lockTemplateRecurrence(ctx); err != nil {
		return err
	}
	links, err := m.deps.Enrollment.SelectionsAt(ctx, requestChildID, phase.ServiceStart)
	if err != nil {
		return fmt.Errorf("decision: list child offerings for multi-source resync: %w", err)
	}
	templates, err := m.deps.Templates.TemplatesSourcedFrom(ctx, bookedOfferingIDs(bookedOfferingPointers(links)))
	if err != nil {
		return fmt.Errorf("decision: list sourced templates: %w", err)
	}
	multiSource := make(map[int64]ports.SourcedTemplate)
	for _, tmpl := range templates {
		if len(tmpl.SourceCareOfferingIDs) > 1 {
			multiSource[tmpl.ID] = tmpl
		}
	}
	// Same scoped, phase-anchored boundary as the materialization pass: the
	// child's union rows must start at the phase start even when the phase is
	// already running, matching the single-source drafts.
	return m.resyncMultiSourceTemplates(ctx, multiSource, phase.ServiceStart, []int64{requestChildID})
}

// resyncMultiSourceTemplates reconciles the multi-source templates a decision
// or adjustment flow touched, AFTER its drafts were persisted: the resync is
// the single authoritative writer of the union roster (see
// addSourcedTemplateDrafts). scopeRequestChildIDs restricts the rewrite to
// the flow's own child(ren) — the per-child flows pass a phase-anchored
// effectiveFrom so the child's rows match the single-source drafts, and the
// scope keeps that early boundary away from every other child's capped
// history. Drifted-invalid templates are skipped with a warning, mirroring
// the single-source fan-out — one broken template must not fail an approval.
// The caller must hold the tenant recurrence lock.
func (m *BookingMaterialization) resyncMultiSourceTemplates(
	ctx context.Context,
	templates map[int64]ports.SourcedTemplate,
	effectiveFrom calendar.Date,
	scopeRequestChildIDs []int64,
) error {
	templateIDs := make([]int64, 0, len(templates))
	for templateID := range templates {
		templateIDs = append(templateIDs, templateID)
	}
	slices.Sort(templateIDs)
	for _, templateID := range templateIDs {
		tmpl := templates[templateID]
		in := templateResync(tmpl, tmpl.SourceCareOfferingIDs, effectiveFrom)
		in.GradeLevels, in.SchoolClasses = tmpl.SourceGradeLevels, tmpl.SourceSchoolClasses
		in.ScopeRequestChildIDs = scopeRequestChildIDs
		if err := m.ResyncTemplateOfferingRoster(ctx, in); err != nil {
			if m.catalog.deps.SourceRules.IsRejection(err) {
				m.logSkippedSourcedTemplate(tmpl.ID, tmpl.SourceCareOfferingIDs, "decision fan-out: multi-source template invalid", err)
				continue
			}
			return fmt.Errorf("decision: resync multi-source template %d: %w", tmpl.ID, err)
		}
	}
	return nil
}

// templateResync is the resync input of a template's stored source rule for
// the given sources.
func templateResync(tmpl ports.SourcedTemplate, offeringIDs []int64, effectiveFrom calendar.Date) careplan.OfferingRosterResync {
	return careplan.OfferingRosterResync{
		TemplateID:       tmpl.ID,
		OfferingIDs:      offeringIDs,
		CalendarPeriodID: tmpl.CalendarPeriodID,
		EffectiveFrom:    effectiveFrom,
	}
}

func (m *BookingMaterialization) persistCareEnrollmentDrafts(
	ctx context.Context,
	requestChildID, studentID int64,
	phase careplan.OfferingPhase,
	drafts map[int64]*careEnrollmentDraft,
	startFrom *calendar.Date,
) error {
	groupIDs := make([]int64, 0, len(drafts))
	for groupID := range drafts {
		groupIDs = append(groupIDs, groupID)
	}
	slices.Sort(groupIDs)
	for _, groupID := range groupIDs {
		row := rosterRowFromCareDraft(requestChildID, studentID, phase, drafts[groupID], startFrom)
		if row.ValidUntil != nil && !row.ValidFrom.Before(*row.ValidUntil) {
			// The draft's segment ended before the row would begin (approval
			// after a split): the capped predecessor has nothing left to plan.
			continue
		}
		if err := validateRosterRow(row); err != nil {
			return fmt.Errorf("decision: validate enrollment: %w", err)
		}
		if err := m.deps.Rosters.CreateEnrollment(ctx, row); err != nil {
			return fmt.Errorf("decision: create enrollment: %w", err)
		}
	}
	return nil
}

func rosterRowFromCareDraft(
	requestChildID, studentID int64,
	phase careplan.OfferingPhase,
	draft *careEnrollmentDraft,
	startFrom *calendar.Date,
) ports.RosterEnrollment {
	validUntil := careDraftValidUntil(draft, phase)
	validFrom := phase.ServiceStart
	// A dated switch may start mid-phase; a phase that already began must not
	// pull the new row back to its service start. Clamped so an effective date
	// before the phase window cannot widen it either.
	if startFrom != nil && startFrom.After(validFrom) {
		validFrom = *startFrom
	}
	// A sourced draft is additionally bounded by its segment's recurrence
	// envelope and by the child's offering-link window (#2147 review):
	// approving after a split must not give the capped predecessor coverage
	// past its valid_until, and a link starting mid-phase must not plan the
	// child before it. Callers skip rows whose window collapses to empty.
	if draft.scheduleValidFrom != nil && draft.scheduleValidFrom.After(validFrom) {
		validFrom = *draft.scheduleValidFrom
	}
	if draft.linkValidFrom != nil && draft.linkValidFrom.After(validFrom) {
		validFrom = *draft.linkValidFrom
	}
	childID := requestChildID
	row := ports.RosterEnrollment{
		StudentID:                studentID,
		ActivityGroupID:          draft.activityGroupID,
		ValidFrom:                validFrom,
		ValidUntil:               &validUntil,
		CalendarPeriodID:         draft.calendarPeriodID,
		EnrollmentRequestChildID: &childID,
	}
	if !draft.allWeekdays && len(draft.selectedWeekday) > 0 {
		row.SelectedWeekdays = sortedWeekdaySet(draft.selectedWeekday)
	}
	return row
}

// careDraftValidUntil is the exclusive end a draft's rows may reach: the
// phase's service window, clamped by the sourced segment's recurrence
// envelope and by the child's offering-link validity (#2147 review). The
// dated adjustment's retained-row extension uses the same bound — extending a
// capped split predecessor back to the phase end would overlap its successor,
// and extending past the link end would plan the child after leaving the
// offering.
func careDraftValidUntil(draft *careEnrollmentDraft, phase careplan.OfferingPhase) calendar.Date {
	validUntil := phase.ServiceEnd.AddDays(1)
	for _, bound := range []*calendar.Date{draft.scheduleValidUntil, draft.linkValidUntil, draft.studentValidUntil} {
		if bound != nil && bound.Before(validUntil) {
			validUntil = *bound
		}
	}
	return validUntil
}

// validateRosterRow applies the Timetable roster row invariants before a
// write, with the texts the row validation has always reported.
func validateRosterRow(row ports.RosterEnrollment) error {
	if row.StudentID <= 0 {
		return errors.New("student ID is required")
	}
	if row.ActivityGroupID <= 0 {
		return errors.New("activity group ID is required")
	}
	seen := make(map[int]bool, len(row.SelectedWeekdays))
	for _, weekday := range row.SelectedWeekdays {
		if weekday < 1 || weekday > 7 {
			return errors.New("selected weekdays must be between 1 and 7")
		}
		if seen[weekday] {
			return errors.New("selected weekdays must not contain duplicates")
		}
		seen[weekday] = true
	}
	if row.Weekday != nil && (*row.Weekday < 1 || *row.Weekday > 7) {
		return errors.New("weekday must be between 1 and 7")
	}
	return nil
}
