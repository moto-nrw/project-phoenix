package application

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// sourcedRosterTarget is one wanted roster row of an offering-sourced
// template: the child's student, the draft describing period + weekdays, and
// the window the child actually holds the offering in. validUntil is
// exclusive, matching the roster rows. A child with non-contiguous offering
// links (left and later re-joined) yields one target per disjoint window —
// bridging the gap would plan the child on days it does not hold the
// offering.
type sourcedRosterTarget struct {
	studentID  int64
	draft      *careEnrollmentDraft
	validFrom  calendar.Date
	validUntil calendar.Date
}

// ResyncTemplateOfferingRoster reconciles the offering-sourced roster rows of
// one template with the union of the source offerings' currently approved
// bookings, applying the template's Jahrgang or Klassen filter (#2137).
//
//   - rows for children that still match keep their row when start and
//     weekday set line up with a wanted window; the end is then set exactly
//     to where the child's offering window reaches (extended OR shrunk)
//   - rows for children that no longer match — including rows whose validity
//     no longer fits any wanted window, e.g. after a source switch — are
//     deleted (not yet effective) or capped at EffectiveFrom (already
//     started); the reconcile diffs EVERY tagged row of the template, so no
//     cleanup scoped to the previous source set is needed
//   - several source offerings union per child: the FIRST offering (array
//     order) contributes its windows unchanged, later offerings contribute
//     only the weekday×date coverage the earlier ones do not already plan —
//     a child in "bis 16:00" (Mo–Fr) and "montags Musik" is planned once on
//     Monday, not twice. All offerings must share one enrollment phase.
//   - missing children are seeded via the same draft/persist shapes the
//     decision fan-out uses, so both write paths stay byte-compatible
//   - children fed by a legacy CareOffering.ActivityGroupID link pointing at
//     the same template are owned by that feed WITHIN the link's weekday and
//     validity footprint: rows fully inside it are never touched and the
//     source never seeds a duplicate there. Outside that footprint — other
//     weekdays, or dates before/after the legacy link's window — the child's
//     source contribution is seeded and reconciled normally: protection by
//     child identity alone would let a legacy Monday suppress a source
//     Friday and let stale source rows outlive a filter change (#2147 review)
//   - wanted windows stop at the template's own schedule envelope, so a
//     capped split predecessor is never planned past its segment end
//   - finally, the touched students' already-materialized future occurrences
//     are reconciled (the materializer never revisits existing instances)
//
// Runs atomically in a tenant transaction, joining the template save's
// transaction when present. The caller holds the tenant recurrence lock.
func (m *BookingMaterialization) ResyncTemplateOfferingRoster(ctx context.Context, in careplan.OfferingRosterResync) error {
	if in.TemplateID <= 0 {
		return m.catalog.deps.SourceRules.Reject("template id is required")
	}
	if in.EffectiveFrom.IsZero() {
		return m.catalog.deps.SourceRules.Reject("effective_from is required")
	}
	return m.deps.RunInTx(ctx, func(txCtx context.Context) error {
		return m.resyncTemplateOfferingRoster(txCtx, in)
	})
}

func (m *BookingMaterialization) resyncTemplateOfferingRoster(ctx context.Context, in careplan.OfferingRosterResync) error {
	phase, wanted, err := m.wantedSourcedRoster(ctx, in)
	if err != nil {
		return err
	}
	scope := newRequestChildScope(in.ScopeRequestChildIDs)
	scope.restrict(wanted)
	protectedChildren, err := m.legacyLinkedChildCoverage(ctx, in.TemplateID, in.EffectiveFrom)
	if err != nil {
		return err
	}
	// A weekday×date the legacy feed already plans must not get a second,
	// source-fed row — but only that overlapping contribution is dropped: a
	// child legacy-planned on Monday can still be source-fed on Friday, and a
	// legacy link starting mid-phase suppresses nothing before it begins
	// (#2147 review).
	for childID, coverage := range protectedChildren {
		remaining := subtractTargetCoverage(wanted[childID], coverage)
		if len(remaining) == 0 {
			delete(wanted, childID)
			continue
		}
		wanted[childID] = remaining
	}
	rows, err := m.deps.Rosters.GroupEnrollments(ctx, in.TemplateID)
	if err != nil {
		return fmt.Errorf("offering roster resync: load template enrollments: %w", err)
	}
	// Students whose rows this resync creates, caps, deletes, or resizes.
	// Their already-materialized future occurrences must be reconciled
	// afterwards — scoped to exactly these students so per-occurrence hand
	// edits of untouched children survive (#2147 review).
	touchedStudents, err := m.reconcileSourcedRosterRows(ctx, rows, scope, wanted, protectedChildren, in.EffectiveFrom)
	if err != nil {
		return err
	}
	if err := m.seedSourcedRosterTargets(ctx, wanted, phase, touchedStudents); err != nil {
		return err
	}
	// rows still holds the pre-write state (every write above goes by id):
	// the reconciler uses it to tell newly gained coverage from occurrences a
	// human removed a still-covered child from (#2147 review round 11).
	return m.reconcileSourcedInstanceRosters(ctx, in.TemplateID, touchedStudents, in.EffectiveFrom, rows)
}

// wantedSourcedRoster resolves the source offerings and builds the wanted
// roster of their union. A template without surviving sources wants nobody,
// which turns the resync into a full cleanup.
func (m *BookingMaterialization) wantedSourcedRoster(
	ctx context.Context,
	in careplan.OfferingRosterResync,
) (*careplan.OfferingPhase, map[int64][]*sourcedRosterTarget, error) {
	wanted := make(map[int64][]*sourcedRosterTarget)
	if len(in.OfferingIDs) == 0 {
		return nil, wanted, nil
	}
	sources, err := m.catalog.LoadOfferingSources(ctx, in.OfferingIDs, in.CalendarPeriodID, in.TolerateDriftedSources)
	if err != nil {
		return nil, nil, err
	}
	if len(sources.Dropped) > 0 {
		// The array carries no FK; an offering deleted without the detach
		// flow leaves a dangling id. Dropping it here (instead of failing)
		// keeps the template editable and lets the resync degrade to the
		// surviving sources, or to a full cleanup when none survive.
		m.deps.Logger.Warn("offering roster resync: ignoring vanished source offerings",
			slog.Int64("template_id", in.TemplateID),
			slog.Any("care_offering_ids", sources.Dropped),
		)
	}
	if len(sources.Offerings) == 0 {
		return nil, wanted, nil
	}
	// The wanted windows are additionally bounded by the template's own
	// schedule envelope: the tenant-wide resync also visits capped split
	// predecessors, whose rows must never be extended past the segment end
	// (#2147 review). Open envelopes leave the windows untouched.
	schedules, err := m.catalog.deps.Timetable.GroupSchedules(ctx, []int64{in.TemplateID})
	if err != nil {
		return nil, nil, fmt.Errorf("offering roster resync: load template schedules: %w", err)
	}
	envelopeFrom, envelopeUntil := scheduleValidityBounds(schedules)
	window := rosterWindow{phase: *sources.Phase, effectiveFrom: in.EffectiveFrom, envelopeFrom: envelopeFrom, envelopeUntil: envelopeUntil}
	wanted, err = m.unionWantedSourcedRosterTargets(ctx, sources.Offerings, in, window)
	if err != nil {
		return nil, nil, err
	}
	return sources.Phase, wanted, nil
}

// requestChildScope restricts a resync to the given request children; an
// empty scope covers the whole template.
type requestChildScope map[int64]bool

func newRequestChildScope(childIDs []int64) requestChildScope {
	scope := make(requestChildScope, len(childIDs))
	for _, childID := range childIDs {
		scope[childID] = true
	}
	return scope
}

func (s requestChildScope) restrict(wanted map[int64][]*sourcedRosterTarget) {
	if len(s) == 0 {
		return
	}
	for childID := range wanted {
		if !s[childID] {
			delete(wanted, childID)
		}
	}
}

func (s requestChildScope) covers(row ports.RosterEnrollment) bool {
	if len(s) == 0 {
		return true
	}
	return row.EnrollmentRequestChildID != nil && s[*row.EnrollmentRequestChildID]
}

func (m *BookingMaterialization) reconcileSourcedRosterRows(
	ctx context.Context,
	rows []ports.RosterEnrollment,
	scope requestChildScope,
	wanted map[int64][]*sourcedRosterTarget,
	protectedChildren map[int64]*legacyChildCoverage,
	effectiveFrom calendar.Date,
) (map[int64]bool, error) {
	touched := make(map[int64]bool)
	for _, row := range rows {
		if !scope.covers(row) {
			continue
		}
		changed, err := m.reconcileSourcedRosterRow(ctx, row, wanted, protectedChildren, effectiveFrom)
		if err != nil {
			return nil, err
		}
		if changed {
			touched[row.StudentID] = true
		}
	}
	return touched, nil
}

// seedSourcedRosterTargets creates the rows of every wanted target no
// existing row serves, child by child in id order.
func (m *BookingMaterialization) seedSourcedRosterTargets(
	ctx context.Context,
	wanted map[int64][]*sourcedRosterTarget,
	phase *careplan.OfferingPhase,
	touched map[int64]bool,
) error {
	childIDs := make([]int64, 0, len(wanted))
	for childID := range wanted {
		childIDs = append(childIDs, childID)
	}
	sort.Slice(childIDs, func(i, j int) bool { return childIDs[i] < childIDs[j] })
	for _, childID := range childIDs {
		for _, target := range wanted[childID] {
			row := rosterRowFromCareDraft(childID, target.studentID, *phase, target.draft, &target.validFrom)
			// The child's offering link can start after the template's effective
			// date and end before the phase does (dated switch into or out of the
			// offering). Planning them for the full phase window would place them
			// on days they do not hold the offering.
			validUntil := target.validUntil
			row.ValidUntil = &validUntil
			if err := validateRosterRow(row); err != nil {
				return fmt.Errorf("offering roster resync: validate seeded enrollment: %w", err)
			}
			if err := m.deps.Rosters.CreateEnrollment(ctx, row); err != nil {
				return fmt.Errorf("offering roster resync: create seeded enrollment: %w", err)
			}
			touched[target.studentID] = true
		}
	}
	return nil
}

// rosterWindow is what bounds a sourced template's wanted rows besides the
// child's own offering link: the phase's service window, the resync's
// effective date and the template's schedule envelope.
type rosterWindow struct {
	phase         careplan.OfferingPhase
	effectiveFrom calendar.Date
	envelopeFrom  *calendar.Date
	envelopeUntil *calendar.Date
}

// unionWantedSourcedRosterTargets builds the union roster across the source
// offerings in array order: the first offering's targets are taken as-is,
// every later offering contributes only what the accumulated coverage does
// not already plan (subtractTargetCoverage — the same segment-splitting
// machinery the legacy-feed protection uses). The order dependence is
// deliberate and stable: the stored id array is the order, so repeated
// resyncs produce byte-identical rows and the row reuse in
// reconcileSourcedRosterRow keeps matching.
func (m *BookingMaterialization) unionWantedSourcedRosterTargets(
	ctx context.Context,
	offerings []careplan.CareOffering,
	in careplan.OfferingRosterResync,
	window rosterWindow,
) (map[int64][]*sourcedRosterTarget, error) {
	wanted := make(map[int64][]*sourcedRosterTarget)
	for i := range offerings {
		perOffering, err := m.wantedSourcedRosterTargets(ctx, &offerings[i], in, window)
		if err != nil {
			return nil, err
		}
		childIDs := make([]int64, 0, len(perOffering))
		for childID := range perOffering {
			childIDs = append(childIDs, childID)
		}
		sort.Slice(childIDs, func(i, j int) bool { return childIDs[i] < childIDs[j] })
		for _, childID := range childIDs {
			targets := perOffering[childID]
			existing := wanted[childID]
			if len(existing) > 0 {
				targets = subtractTargetCoverage(targets, coverageFromSourcedTargets(existing))
				if len(targets) == 0 {
					continue
				}
			}
			wanted[childID] = coalesceSourcedRosterTargets(append(existing, targets...))
		}
	}
	return wanted, nil
}

// coverageFromSourcedTargets converts a child's already-accepted union
// targets into the coverage shape the subtraction machinery consumes: one
// window per target, carrying the target's weekday set.
func coverageFromSourcedTargets(targets []*sourcedRosterTarget) *legacyChildCoverage {
	coverage := &legacyChildCoverage{windows: make([]legacyCoverageWindow, 0, len(targets))}
	for _, target := range targets {
		from := target.validFrom
		until := target.validUntil
		window := legacyCoverageWindow{from: &from, until: &until}
		if target.draft.allWeekdays || len(target.draft.selectedWeekday) == 0 {
			window.allWeekdays = true
		} else {
			window.weekdays = target.draft.selectedWeekday
		}
		coverage.windows = append(coverage.windows, window)
	}
	return coverage
}

// wantedSourcedRosterTargets builds the desired roster of the sourced
// template: every approved, grade-matching child of the offering with the
// weekday set their booking selects, bounded by the window their offering
// link is actually valid in. Each child maps to one target per disjoint link
// window (see coalesceSourcedRosterTargets).
func (m *BookingMaterialization) wantedSourcedRosterTargets(
	ctx context.Context,
	offering *careplan.CareOffering,
	in careplan.OfferingRosterResync,
	window rosterWindow,
) (map[int64][]*sourcedRosterTarget, error) {
	children, err := m.deps.Enrollment.ApprovedChildren(ctx, []int64{offering.ID}, in.EffectiveFrom)
	if err != nil {
		return nil, fmt.Errorf("offering roster resync: list approved children: %w", err)
	}
	wanted := make(map[int64][]*sourcedRosterTarget, len(children))
	for i := range children {
		child := &children[i]
		// The two filters are mutually exclusive by validation, so at most one
		// of the two guards is ever active (#2482).
		if !gradeFilterMatches(in.GradeLevels, child.GradeLevel) || !sourceClassFilterMatches(in.SchoolClasses, child.SchoolClass) {
			continue
		}
		validFrom, validUntil, ok := sourcedRosterWindow(&child.Link, window)
		if !ok {
			// A link that only covers days before the template takes effect
			// (or outside the phase) has nothing left to plan.
			continue
		}
		days, err := effectiveOfferingDaysForEnrollment(offering, &child.Link)
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
func sourcedRosterWindow(link *careplan.BookedOffering, window rosterWindow) (validFrom, validUntil calendar.Date, ok bool) {
	validFrom = latestDate(window.phase.ServiceStart, &window.effectiveFrom, window.envelopeFrom)
	validUntil = window.phase.ServiceEnd.AddDays(1)
	if window.envelopeUntil != nil && window.envelopeUntil.Before(validUntil) {
		validUntil = *window.envelopeUntil
	}
	if link != nil {
		validFrom = latestDate(validFrom, link.ValidFrom)
		if link.ValidUntil != nil && link.ValidUntil.Before(validUntil) {
			validUntil = *link.ValidUntil
		}
	}
	if !validFrom.Before(validUntil) {
		return calendar.Date(""), calendar.Date(""), false
	}
	return validFrom, validUntil, true
}

// latestDate is the latest of a date and the set optional dates.
func latestDate(date calendar.Date, others ...*calendar.Date) calendar.Date {
	for _, other := range others {
		if other != nil && other.After(date) {
			date = *other
		}
	}
	return date
}

// reconcileSourcedRosterRow applies keep/cap/delete to one existing row.
// Manual rows (no request-child tag), history, and legacy-owned rows (see
// legacyChildCoverage.coversRow) are left untouched.
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
func (m *BookingMaterialization) reconcileSourcedRosterRow(
	ctx context.Context,
	row ports.RosterEnrollment,
	wanted map[int64][]*sourcedRosterTarget,
	protectedChildren map[int64]*legacyChildCoverage,
	effectiveFrom calendar.Date,
) (bool, error) {
	if row.EnrollmentRequestChildID == nil {
		return false, nil
	}
	if row.ValidUntil != nil && !row.ValidUntil.After(effectiveFrom) {
		return false, nil // history stays
	}
	childID := *row.EnrollmentRequestChildID
	if coverage := protectedChildren[childID]; coverage.coversRow(row) {
		// The row plans nothing the legacy CareOffering.ActivityGroupID feed
		// would not plan itself: its lifecycle belongs to THAT offering's
		// flows and is never rewritten here. A row of the same child reaching
		// outside the legacy footprint — other weekdays or dates the legacy
		// links do not cover — falls through: it carries a source
		// contribution the resync must reconcile (#2147 review).
		return false, nil
	}
	if target := takeServableTarget(wanted, childID, row, effectiveFrom); target != nil {
		if row.ValidUntil != nil && *row.ValidUntil == target.validUntil {
			return false, nil
		}
		if err := m.deps.Rosters.SetEnrollmentValidUntil(ctx, row.ID, target.validUntil); err != nil {
			return false, fmt.Errorf("offering roster resync: adjust retained enrollment: %w", err)
		}
		return true, nil
	}
	if !row.ValidFrom.Before(effectiveFrom) {
		if err := m.deps.Rosters.DeleteEnrollment(ctx, row.ID); err != nil {
			return false, fmt.Errorf("offering roster resync: delete enrollment: %w", err)
		}
		return true, nil
	}
	if err := m.deps.Rosters.SetEnrollmentValidUntil(ctx, row.ID, effectiveFrom); err != nil {
		return false, fmt.Errorf("offering roster resync: cap enrollment: %w", err)
	}
	return true, nil
}

// takeServableTarget removes and returns the child's first target the row can
// serve; nil when the row serves none.
func takeServableTarget(wanted map[int64][]*sourcedRosterTarget, childID int64, row ports.RosterEnrollment, effectiveFrom calendar.Date) *sourcedRosterTarget {
	idx := indexOfServableTarget(wanted[childID], row, effectiveFrom)
	if idx < 0 {
		return nil
	}
	target := wanted[childID][idx]
	wanted[childID] = append(wanted[childID][:idx], wanted[childID][idx+1:]...)
	if len(wanted[childID]) == 0 {
		delete(wanted, childID)
	}
	return target
}

// indexOfServableTarget returns the first target the row can serve without
// misrepresenting any day: draft (weekdays + period) equal and validity start
// aligned. A row whose ValidFrom lies before effectiveFrom carries immutable
// history, so it may serve a target starting exactly at the rewrite boundary;
// any other start mismatch means cap-and-reseed.
func indexOfServableTarget(targets []*sourcedRosterTarget, row ports.RosterEnrollment, effectiveFrom calendar.Date) int {
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

// careDraftMatchesEnrollment reports whether an existing row already is what
// the new selection asks for, so the switch can leave it alone. Weekday sets
// are compared as sets; an empty stored set means every weekday, which is how
// materialization writes an all-days draft.
func careDraftMatchesEnrollment(draft *careEnrollmentDraft, row ports.RosterEnrollment) bool {
	if !sameOptionalInt64(draft.calendarPeriodID, row.CalendarPeriodID) {
		return false
	}
	if draft.allWeekdays || len(draft.selectedWeekday) == 0 {
		return len(row.SelectedWeekdays) == 0
	}
	if len(row.SelectedWeekdays) != len(draft.selectedWeekday) {
		return false
	}
	for _, weekday := range row.SelectedWeekdays {
		if !draft.selectedWeekday[weekday] {
			return false
		}
	}
	return true
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
		weekday, ok := offeringDayWeekday(day)
		if !ok {
			return nil, fmt.Errorf("offering roster resync: invalid selected day %q", day)
		}
		draft.selectedWeekday[weekday] = true
	}
	return draft, nil
}
