package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// legacyChildCoverage is what the legacy CareOffering.ActivityGroupID feed
// (#1651) plans for one child on the template being resynced, as one window
// per approved link. The resync uses it to tell legacy-owned rows (protected)
// from source-owned contributions of the SAME child (reconciled normally).
// Both the weekday set AND each link's validity window matter (#2147 review):
// weekday-only protection would let a legacy Monday suppress a source Friday,
// and window-less protection would let a future or partial legacy link guard
// dates it never plans.
type legacyChildCoverage struct {
	windows []legacyCoverageWindow
}

// legacyCoverageWindow is one legacy link's contribution: the weekdays it
// selects during [from, until). Open sides are nil; allWeekdays marks a link
// whose day selection resolves to every weekday — or could not be resolved,
// which protects conservatively.
type legacyCoverageWindow struct {
	from        *calendar.Date
	until       *calendar.Date // exclusive
	allWeekdays bool
	weekdays    map[int]bool
}

func (w *legacyCoverageWindow) coversWeekday(weekday int) bool {
	return w.allWeekdays || w.weekdays[weekday]
}

func (w *legacyCoverageWindow) activeOn(date calendar.Date) bool {
	if w.from != nil && w.from.After(date) {
		return false
	}
	return w.until == nil || w.until.After(date)
}

func allISOWeekdays() []int { return []int{1, 2, 3, 4, 5, 6, 7} }

// coversRow reports whether a roster row's weekday selection AND validity lie
// fully inside what the legacy feed plans. Such a row plans nothing the
// legacy feed would not plan itself, so the resync leaves it alone; a row
// reaching outside the legacy windows carries a source contribution and is
// reconciled normally (#2147 review).
func (c *legacyChildCoverage) coversRow(row ports.RosterEnrollment) bool {
	if c == nil || len(c.windows) == 0 {
		return false
	}
	weekdays := row.SelectedWeekdays
	if len(weekdays) == 0 {
		// An empty selection means every weekday.
		weekdays = allISOWeekdays()
	}
	for _, weekday := range weekdays {
		if !c.coversSpan(weekday, row.ValidFrom, row.ValidUntil) {
			return false
		}
	}
	return true
}

// coversSpan reports whether the union of the legacy windows selecting the
// weekday covers all of [from, until). An open row end (until == nil) is only
// covered by a chain reaching an open-ended window. The sweep extends a
// cursor through overlapping windows; it terminates because every extension
// strictly advances the cursor to one of finitely many window ends.
func (c *legacyChildCoverage) coversSpan(weekday int, from calendar.Date, until *calendar.Date) bool {
	cursor := from
	for progressed := true; progressed; {
		progressed = false
		for i := range c.windows {
			reached, advanced, done := c.windows[i].extend(weekday, cursor, until)
			if done {
				return true
			}
			if advanced {
				cursor, progressed = reached, true
			}
		}
	}
	return false
}

// extend applies one window to the coverage sweep: done when the window
// covers the rest of the span, advanced when it moves the cursor forward.
func (w *legacyCoverageWindow) extend(weekday int, cursor calendar.Date, until *calendar.Date) (reached calendar.Date, advanced, done bool) {
	if !w.coversWeekday(weekday) || (w.from != nil && w.from.After(cursor)) {
		return cursor, false, false
	}
	if w.until == nil || (until != nil && !w.until.Before(*until)) {
		return cursor, false, true
	}
	if w.until.After(cursor) {
		return *w.until, true, false
	}
	return cursor, false, false
}

// subtractTargetCoverage removes an existing weekday×date coverage from a
// child's wanted source targets, splitting each target at the coverage
// windows' boundaries. Segments outside every window keep the original draft;
// segments with a partial weekday overlap keep only the weekdays the coverage
// does not plan there; fully covered segments drop. Touching segments that
// end up with the same weekday set coalesce back into one row. Two coverages
// consume it: the legacy CareOffering.ActivityGroupID feed's footprint, and —
// with several source offerings — the union coverage the earlier offerings
// already contributed (multi-source follow-up to #2137).
func subtractTargetCoverage(targets []*sourcedRosterTarget, coverage *legacyChildCoverage) []*sourcedRosterTarget {
	if coverage == nil || len(coverage.windows) == 0 {
		return targets
	}
	remaining := make([]*sourcedRosterTarget, 0, len(targets))
	for _, target := range targets {
		remaining = append(remaining, subtractTargetCoverageFromTarget(target, coverage)...)
	}
	return coalesceSourcedRosterTargets(remaining)
}

func subtractTargetCoverageFromTarget(target *sourcedRosterTarget, coverage *legacyChildCoverage) []*sourcedRosterTarget {
	bounds := legacySegmentBoundaries(target.validFrom, target.validUntil, coverage)
	out := make([]*sourcedRosterTarget, 0, len(bounds)-1)
	for i := 0; i+1 < len(bounds); i++ {
		segFrom, segUntil := bounds[i], bounds[i+1]
		draft, ok := draftMinusLegacyWeekdays(target.draft, coverage, segFrom)
		if !ok {
			continue
		}
		out = append(out, &sourcedRosterTarget{
			studentID:  target.studentID,
			draft:      draft,
			validFrom:  segFrom,
			validUntil: segUntil,
		})
	}
	return out
}

// legacySegmentBoundaries returns the sorted cut points of [from, until):
// both ends plus every legacy window edge falling strictly inside. Between
// two consecutive points the set of active legacy windows is constant.
func legacySegmentBoundaries(from, until calendar.Date, coverage *legacyChildCoverage) []calendar.Date {
	set := map[calendar.Date]bool{from: true, until: true}
	for _, w := range coverage.windows {
		for _, edge := range []*calendar.Date{w.from, w.until} {
			if edge != nil && edge.After(from) && edge.Before(until) {
				set[*edge] = true
			}
		}
	}
	bounds := make([]calendar.Date, 0, len(set))
	for bound := range set {
		bounds = append(bounds, bound)
	}
	sort.Slice(bounds, func(i, j int) bool { return bounds[i].Before(bounds[j]) })
	return bounds
}

// legacyWeekdaysOn returns the weekdays the legacy windows active on at plan;
// all is true when one of them plans every weekday.
func (c *legacyChildCoverage) legacyWeekdaysOn(at calendar.Date) (days map[int]bool, all bool) {
	days = map[int]bool{}
	for i := range c.windows {
		w := &c.windows[i]
		if !w.activeOn(at) {
			continue
		}
		if w.allWeekdays {
			return nil, true
		}
		for weekday := range w.weekdays {
			days[weekday] = true
		}
	}
	return days, false
}

// draftMinusLegacyWeekdays reduces a draft by the weekdays the legacy windows
// active on `at` plan there. ok is false when nothing remains. A draft none
// of whose weekdays overlap is returned untouched — including its all-weekday
// shape — so unaffected rows keep matching byte-for-byte.
func draftMinusLegacyWeekdays(draft *careEnrollmentDraft, coverage *legacyChildCoverage, at calendar.Date) (*careEnrollmentDraft, bool) {
	legacyDays, legacyAll := coverage.legacyWeekdaysOn(at)
	if legacyAll {
		return nil, false
	}
	if len(legacyDays) == 0 {
		return draft, true
	}
	draftAllDays := draft.allWeekdays || len(draft.selectedWeekday) == 0
	remaining := make(map[int]bool)
	overlapped := false
	for _, weekday := range allISOWeekdays() {
		if !draftAllDays && !draft.selectedWeekday[weekday] {
			continue
		}
		if legacyDays[weekday] {
			overlapped = true
			continue
		}
		remaining[weekday] = true
	}
	if !overlapped {
		return draft, true
	}
	if len(remaining) == 0 {
		return nil, false
	}
	return &careEnrollmentDraft{
		activityGroupID:   draft.activityGroupID,
		calendarPeriodID:  draft.calendarPeriodID,
		selectedWeekday:   remaining,
		studentValidUntil: cloneOptionalDate(draft.studentValidUntil),
	}, true
}

// legacyLinkedChildCoverage returns, per request child, the windows the
// legacy CareOffering.ActivityGroupID feed (#1651) plans on this template.
// The resync must never rewrite rows inside that footprint: their lifecycle
// belongs to the decision/adjustment flows of THAT offering.
//
// The lookup follows the template's whole split lineage, not just the direct
// link: the legacy fan-out writes rows on EVERY overlapping series segment
// (addLegacyLinkedGroupDrafts), so a link pointing at a predecessor still
// plans children on the successor being resynced. Matching only the direct
// link would let a resync treat those rows as source-owned and delete them
// (#2147 review).
func (m *BookingMaterialization) legacyLinkedChildCoverage(
	ctx context.Context,
	templateID int64,
	onOrAfter calendar.Date,
) (map[int64]*legacyChildCoverage, error) {
	series, err := m.catalog.deps.Timetable.TemplateSeries(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("offering roster resync: load template series for legacy coverage: %w", err)
	}
	offerings, err := m.catalog.listByActivityGroupIDs(ctx, careOfferingSeriesGroupIDs(templateID, series))
	if err != nil {
		return nil, fmt.Errorf("offering roster resync: list legacy-linked offerings: %w", err)
	}
	if len(offerings) == 0 {
		return map[int64]*legacyChildCoverage{}, nil
	}
	offeringByID := offeringsByID(offerings)
	offeringIDs := make([]int64, 0, len(offerings))
	for _, offering := range offerings {
		offeringIDs = append(offeringIDs, offering.ID)
	}
	children, err := m.deps.Enrollment.ApprovedChildren(ctx, offeringIDs, onOrAfter)
	if err != nil {
		return nil, fmt.Errorf("offering roster resync: list legacy-linked children: %w", err)
	}
	phases := make(map[int64]*careplan.OfferingPhase)
	protected := make(map[int64]*legacyChildCoverage, len(children))
	for i := range children {
		link := &children[i].Link
		coverage := protected[link.RequestChildID]
		if coverage == nil {
			coverage = &legacyChildCoverage{}
			protected[link.RequestChildID] = coverage
		}
		window, err := m.legacyCoverageWindowForLink(ctx, phases, offeringByID[link.CareOfferingID], link, templateID)
		if err != nil {
			return nil, err
		}
		if window != nil {
			coverage.windows = append(coverage.windows, *window)
		}
	}
	return protected, nil
}

// legacyCoverageWindowForLink resolves one legacy link into its coverage
// window: the link's validity clipped to its offering's phase service window
// (the rows the legacy feed writes never reach outside either), carrying the
// weekday set the link selects. Unresolvable offerings, phases, or day
// selections protect conservatively — an open, all-weekday window — instead
// of failing the whole resync; ambiguity must never remove a child the
// legacy link still plans. nil without error means the link plans nothing.
func (m *BookingMaterialization) legacyCoverageWindowForLink(
	ctx context.Context,
	phases map[int64]*careplan.OfferingPhase,
	offering *careplan.CareOffering,
	link *careplan.BookedOffering,
	templateID int64,
) (*legacyCoverageWindow, error) {
	if offering == nil {
		return &legacyCoverageWindow{allWeekdays: true}, nil
	}
	phase, err := m.legacyCoveragePhase(ctx, phases, offering)
	if err != nil {
		return nil, err
	}
	if phase == nil {
		m.deps.Logger.Warn("offering roster resync: cannot resolve legacy-linked phase; protecting the full window",
			slog.Int64("template_id", templateID),
			slog.Int64("care_offering_id", offering.ID),
			slog.Int64("request_child_id", link.RequestChildID),
		)
		return &legacyCoverageWindow{allWeekdays: true}, nil
	}
	serviceStart := phase.ServiceStart
	window := &legacyCoverageWindow{from: &serviceStart, until: cloneOptionalDate(link.ValidUntil)}
	if link.ValidFrom != nil && link.ValidFrom.After(serviceStart) {
		window.from = cloneOptionalDate(link.ValidFrom)
	}
	serviceEnd := phase.ServiceEnd.AddDays(1)
	if window.until == nil || serviceEnd.Before(*window.until) {
		window.until = &serviceEnd
	}
	if !window.from.Before(*window.until) {
		return nil, nil
	}
	m.applyLegacyCoverageDays(window, offering, link, templateID)
	return window, nil
}

// applyLegacyCoverageDays sets the weekdays a legacy link selects on its
// window. An empty or unresolvable selection protects every weekday, the same
// way mergeCareEnrollmentDraft treats an empty day list.
func (m *BookingMaterialization) applyLegacyCoverageDays(window *legacyCoverageWindow, offering *careplan.CareOffering, link *careplan.BookedOffering, templateID int64) {
	days, err := effectiveOfferingDaysForEnrollment(offering, link)
	if err != nil {
		m.deps.Logger.Warn("offering roster resync: cannot resolve legacy-linked days; protecting all weekdays",
			slog.Int64("template_id", templateID),
			slog.Int64("care_offering_id", offering.ID),
			slog.Int64("request_child_id", link.RequestChildID),
			slog.String("error", err.Error()),
		)
	}
	window.weekdays = make(map[int]bool, len(days))
	for _, day := range days {
		if weekday, ok := offeringDayWeekday(day); ok {
			window.weekdays[weekday] = true
		}
	}
	if len(window.weekdays) == 0 {
		window.allWeekdays = true
	}
}

// legacyCoveragePhase loads (and memoizes) the phase of a legacy-linked
// offering. A missing phase row yields nil so the caller can protect
// conservatively instead of failing the resync.
func (m *BookingMaterialization) legacyCoveragePhase(
	ctx context.Context,
	phases map[int64]*careplan.OfferingPhase,
	offering *careplan.CareOffering,
) (*careplan.OfferingPhase, error) {
	if phase, ok := phases[offering.PhaseID]; ok {
		return phase, nil
	}
	phase, err := m.deps.Enrollment.Phase(ctx, offering.PhaseID)
	if err != nil {
		if errors.Is(err, ports.ErrBookingRowNotFound) {
			phases[offering.PhaseID] = nil
			return nil, nil
		}
		return nil, fmt.Errorf("offering roster resync: load legacy-linked phase: %w", err)
	}
	phases[offering.PhaseID] = &phase.OfferingPhase
	return &phase.OfferingPhase, nil
}
