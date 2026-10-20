package application

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/careplan/selection"
)

type offeringPendingReview struct {
	applied, earliest, latest careplan.Date
	diff, unchanged           []careplan.OfferingReviewDiffEntry
	withdrawal                bool
}

type offeringPendingFacts struct {
	children  map[int64]careplan.OfferingReviewChild
	requests  map[int64]careplan.OfferingReviewRequest
	phases    map[int64]careplan.OfferingReviewPhase
	current   map[int64][]careplan.OfferingReviewBooking
	active    map[int64]map[int64]careplan.CareOffering
	catalog   map[int64]careplan.CareOffering
	requested map[int64][]careplan.OfferingReviewSelection
	reviews   map[int64]*offeringPendingReview
}

func newOfferingPendingFacts() *offeringPendingFacts {
	return &offeringPendingFacts{
		children: map[int64]careplan.OfferingReviewChild{}, requests: map[int64]careplan.OfferingReviewRequest{}, phases: map[int64]careplan.OfferingReviewPhase{},
		current: map[int64][]careplan.OfferingReviewBooking{}, active: map[int64]map[int64]careplan.CareOffering{},
		catalog: map[int64]careplan.CareOffering{}, requested: map[int64][]careplan.OfferingReviewSelection{}, reviews: map[int64]*offeringPendingReview{},
	}
}

func (s *OfferingReviews) pendingFacts(ctx context.Context, rows []careplan.OfferingChangeRequest, today careplan.Date) (*offeringPendingFacts, error) {
	facts := newOfferingPendingFacts()
	phaseIDs, err := s.loadPendingEnrollment(ctx, facts, rows)
	if err != nil {
		return nil, err
	}
	dates := facts.seedReviews(rows, today)
	active, err := s.Catalog.ListCareOfferings(ctx, careplan.CareOfferingFilter{PhaseIDs: phaseIDs, ActiveOnly: true})
	if err != nil {
		return nil, fmt.Errorf("load active offerings: %w", err)
	}
	allIDs := facts.indexActive(active)
	current, err := s.Enrollment.Selections(ctx, dates)
	if err != nil {
		return nil, fmt.Errorf("load current offerings: %w", err)
	}
	allIDs = facts.indexCurrent(current, allIDs)
	allIDs = facts.indexRequested(rows, allIDs)
	catalog, err := s.Catalog.ListCareOfferings(ctx, careplan.CareOfferingFilter{IDs: allIDs})
	if err != nil {
		return nil, fmt.Errorf("load queue offerings: %w", err)
	}
	for _, offering := range catalog {
		facts.catalog[offering.ID] = offering
	}
	for _, offering := range active {
		facts.catalog[offering.ID] = offering
	}
	return facts, nil
}

// loadPendingEnrollment indexes the request children, their enrollment
// requests and phases, and returns the phase IDs for the catalog read.
func (s *OfferingReviews) loadPendingEnrollment(ctx context.Context, facts *offeringPendingFacts, rows []careplan.OfferingChangeRequest) ([]int64, error) {
	childIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		if row.RequestChildID > 0 {
			childIDs = append(childIDs, row.RequestChildID)
		}
	}
	children, err := s.Enrollment.Children(ctx, childIDs)
	if err != nil {
		return nil, fmt.Errorf("load request children: %w", err)
	}
	requestIDs := make([]int64, 0, len(children))
	for _, child := range children {
		facts.children[child.ID] = child
		requestIDs = append(requestIDs, child.RequestID)
	}
	requests, err := s.Enrollment.Requests(ctx, requestIDs)
	if err != nil {
		return nil, fmt.Errorf("load enrollment requests: %w", err)
	}
	phaseIDs := make([]int64, 0, len(requests))
	for _, request := range requests {
		facts.requests[request.ID] = request
		phaseIDs = append(phaseIDs, request.PhaseID)
	}
	phases, err := s.Enrollment.Phases(ctx, phaseIDs)
	if err != nil {
		return nil, fmt.Errorf("load phases: %w", err)
	}
	for _, phase := range phases {
		facts.phases[phase.ID] = phase
	}
	return phaseIDs, nil
}

// seedReviews creates one review per request row with its applied date and
// window, and returns the applied date per request child.
func (facts *offeringPendingFacts) seedReviews(rows []careplan.OfferingChangeRequest, today careplan.Date) map[int64]domain.Date {
	dates := make(map[int64]domain.Date, len(rows))
	for _, row := range rows {
		if row.RequestChildID <= 0 {
			continue
		}
		phase, ok := facts.phases[facts.requests[facts.children[row.RequestChildID].RequestID].PhaseID]
		applied := offeringAppliedDate(careplan.Date(row.EffectiveFrom), today, phase.Start)
		review := &offeringPendingReview{applied: applied}
		if ok {
			review.earliest, review.latest = offeringAppliedDate(today, today, phase.Start), phase.End
		}
		facts.reviews[row.ID], dates[row.RequestChildID] = review, applied
	}
	return dates
}

func (facts *offeringPendingFacts) indexActive(active []careplan.CareOffering) []int64 {
	allIDs := make([]int64, 0, len(active))
	for _, offering := range active {
		if facts.active[offering.PhaseID] == nil {
			facts.active[offering.PhaseID] = map[int64]careplan.CareOffering{}
		}
		facts.active[offering.PhaseID][offering.ID] = offering
		allIDs = append(allIDs, offering.ID)
	}
	return allIDs
}

func (facts *offeringPendingFacts) indexCurrent(current []careplan.OfferingReviewBooking, allIDs []int64) []int64 {
	for _, booking := range current {
		facts.current[booking.ChildID] = append(facts.current[booking.ChildID], booking)
		allIDs = append(allIDs, booking.OfferingID)
	}
	return allIDs
}

// indexRequested parses each row's requested selections; rows with an
// unreadable payload stay without selections.
func (facts *offeringPendingFacts) indexRequested(rows []careplan.OfferingChangeRequest, allIDs []int64) []int64 {
	for _, row := range rows {
		requested, err := careplan.ParseOfferingReviewSelections(row.Payload)
		if err != nil {
			continue
		}
		facts.requested[row.ID] = requested
		for _, item := range requested {
			allIDs = append(allIDs, item.OfferingID)
		}
	}
	return allIDs
}

type offeringCourseDiff struct {
	review    *offeringPendingReview
	entries   []careplan.OfferingReviewDiffEntry
	child     careplan.OfferingReviewChild
	requested map[int64]bool
	changed   map[int64]bool
}

func (s *OfferingReviews) pendingReviews(ctx context.Context, rows []careplan.OfferingChangeRequest, today careplan.Date) (map[int64]*offeringPendingReview, error) {
	if len(rows) == 0 {
		return map[int64]*offeringPendingReview{}, nil
	}
	facts, err := s.pendingFacts(ctx, rows, today)
	if err != nil {
		return nil, err
	}
	pending := make([]offeringCourseDiff, 0, len(rows))
	refs := make([]careplan.OfferingReviewCourseRef, 0)
	for _, row := range rows {
		diff, ok := facts.rowDiff(row)
		if !ok {
			continue
		}
		refs = diff.annotate(facts.catalog, refs)
		pending = append(pending, diff)
	}
	groups := map[int64][]careplan.OfferingReviewCourse{}
	if len(refs) > 0 {
		groups, err = s.Courses.CourseGroups(ctx, refs, today)
		if err != nil {
			return nil, fmt.Errorf("offering change: mark course diff lines: %w", err)
		}
	}
	for _, diff := range pending {
		diff.finish(groups)
	}
	return facts.reviews, nil
}

// allowedOfferings is the phase's active catalog plus every offering the
// child currently holds.
func (facts *offeringPendingFacts) allowedOfferings(phaseID, childID int64) map[int64]careplan.CareOffering {
	allowed := make(map[int64]careplan.CareOffering, len(facts.active[phaseID])+len(facts.current[childID]))
	for id, offering := range facts.active[phaseID] {
		allowed[id] = offering
	}
	for _, booking := range facts.current[childID] {
		if offering, ok := facts.catalog[booking.OfferingID]; ok {
			allowed[offering.ID] = offering
		}
	}
	return allowed
}

// rowDiff materializes one request against the child's current bookings. It
// reports false when the row's facts are incomplete or do not materialize.
func (facts *offeringPendingFacts) rowDiff(row careplan.OfferingChangeRequest) (offeringCourseDiff, bool) {
	child, childOK := facts.children[row.RequestChildID]
	request, requestOK := facts.requests[child.RequestID]
	phase, phaseOK := facts.phases[request.PhaseID]
	requested, payloadOK := facts.requested[row.ID]
	if !childOK || !requestOK || !phaseOK || !payloadOK {
		return offeringCourseDiff{}, false
	}
	allowed := facts.allowedOfferings(phase.ID, child.ID)
	materialized, err := materializeOfferingReview(child, phase, requested, facts.current[child.ID], allowed, row.CompleteWithdrawalConfirmed)
	if err != nil {
		return offeringCourseDiff{}, false
	}
	entries, changed := offeringReviewDiff(facts.current[child.ID], materialized, facts.catalog)
	nativeCatalog, err := offeringSelectionCatalog(facts.catalog)
	if err != nil {
		return offeringCourseDiff{}, false
	}
	shares := selection.AutomaticShares(materialized, nativeCatalog)
	requestedIDs := make(map[int64]bool, len(requested))
	for _, item := range requested {
		requestedIDs[item.OfferingID] = true
	}
	diff := offeringCourseDiff{review: facts.reviews[row.ID], entries: entries, child: child, requested: requestedIDs, changed: changed}
	diff.applyShares(shares, facts.catalog)
	return diff, true
}

func (d *offeringCourseDiff) applyShares(shares map[int64]selection.AutomaticShare, catalog map[int64]careplan.CareOffering) {
	for i := range d.entries {
		entry := &d.entries[i]
		share := shares[entry.OfferingID]
		entry.NewAutomaticDays, entry.NewRuleDays, entry.NewDaysWithoutRules, entry.AutoTriggerIDs = share.AutomaticDays, share.RuleDays, share.DaysWithoutRules, share.TriggerIDs
		for _, id := range share.TriggerIDs {
			entry.AutoTriggerNames = append(entry.AutoTriggerNames, offeringLabel(id, catalog))
		}
	}
}

// annotate records unchanged booked lines and the withdrawal flag on the
// review, and appends a course ref for every newly booked requested line.
func (d *offeringCourseDiff) annotate(catalog map[int64]careplan.CareOffering, refs []careplan.OfferingReviewCourseRef) []careplan.OfferingReviewCourseRef {
	review := d.review
	held, remains := false, false
	for i := range d.entries {
		entry := &d.entries[i]
		if catalog[entry.OfferingID].CountsAsCare {
			held = held || entry.OldState == "booked"
			remains = remains || entry.NewState == "booked"
		}
		if !d.changed[entry.OfferingID] && len(entry.NewRuleDays) == 0 && entry.NewState == "booked" {
			review.unchanged = append(review.unchanged, *entry)
		}
		if d.newlyRequested(entry) {
			refs = append(refs, careplan.OfferingReviewCourseRef{OfferingID: entry.OfferingID, LegacyGroupID: catalog[entry.OfferingID].ActivityGroupID})
		}
	}
	review.withdrawal = held && !remains
	sort.SliceStable(review.unchanged, func(i, j int) bool { return review.unchanged[i].Label < review.unchanged[j].Label })
	return refs
}

func (d *offeringCourseDiff) newlyRequested(entry *careplan.OfferingReviewDiffEntry) bool {
	return entry.OldState == "not_booked" && entry.NewState == "booked" && d.requested[entry.OfferingID]
}

// finish marks newly requested course lines and stores the sorted changed
// lines on the review.
func (d offeringCourseDiff) finish(groups map[int64][]careplan.OfferingReviewCourse) {
	for i := range d.entries {
		entry := &d.entries[i]
		if !d.newlyRequested(entry) {
			continue
		}
		if slices.ContainsFunc(groups[entry.OfferingID], func(group careplan.OfferingReviewCourse) bool {
			return offeringCourseMatches(group, d.child)
		}) {
			entry.IsCourse = true
		}
	}
	entries := slices.DeleteFunc(d.entries, func(entry careplan.OfferingReviewDiffEntry) bool {
		return !d.changed[entry.OfferingID] && len(entry.NewRuleDays) == 0
	})
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Label < entries[j].Label })
	d.review.diff = entries
}

func offeringSelectionCatalog(catalog map[int64]careplan.CareOffering) (map[int64]*selection.Offering, error) {
	result := make(map[int64]*selection.Offering, len(catalog))
	for id, offering := range catalog {
		var rule *selection.AvailabilityRule
		if len(offering.AvailabilityRule) > 0 {
			if err := json.Unmarshal(offering.AvailabilityRule, &rule); err != nil {
				return nil, err
			}
		}
		result[id] = &selection.Offering{
			ID: id, SortOrder: offering.SortOrder, DaysOfWeekMode: offering.DaysOfWeekMode, AvailableDays: offering.AvailableDays,
			CountsAsCare: offering.CountsAsCare, IncludesLunch: offering.IncludesLunch, IsRequired: offering.IsRequired,
			SelectionGroup: offering.SelectionGroup, SelectionRule: offering.SelectionRule,
			AutoAddTriggerOfferingIDs: offering.AutoAddTriggerOfferingIDs, AutoAddGradeLevels: offering.AutoAddGradeLevels, AvailabilityRule: rule,
		}
	}
	return result, nil
}

func materializeOfferingReview(child careplan.OfferingReviewChild, phase careplan.OfferingReviewPhase, requested []careplan.OfferingReviewSelection, current []careplan.OfferingReviewBooking, catalog map[int64]careplan.CareOffering, confirmed bool) ([]selection.Selection, error) {
	submit, err := offeringReviewSubmission(child, requested, catalog)
	if err != nil {
		return nil, err
	}
	grandfathered, hadCareDays := offeringReviewGrandfathered(current, catalog)
	native, err := offeringSelectionCatalog(catalog)
	if err != nil {
		return nil, err
	}
	result, err := selection.MaterializeAdjustments([]selection.Child{submit}, native, phase.SelectionMode, grandfathered, confirmed && hadCareDays)
	if err != nil {
		return nil, err
	}
	return result[0], nil
}

// offeringReviewSubmission turns the requested selections into one child
// submission; every offering must be in the bookable catalog.
func offeringReviewSubmission(child careplan.OfferingReviewChild, requested []careplan.OfferingReviewSelection, catalog map[int64]careplan.CareOffering) (selection.Child, error) {
	submit := selection.Child{TargetGradeLevel: child.Grade}
	for _, item := range requested {
		if item.OfferingID <= 0 {
			return selection.Child{}, fmt.Errorf("offering id is required")
		}
		if slices.Contains(submit.OfferingIDs, item.OfferingID) {
			continue
		}
		if _, ok := catalog[item.OfferingID]; !ok {
			return selection.Child{}, fmt.Errorf("care offering %d cannot be booked for this child", item.OfferingID)
		}
		submit.OfferingIDs = append(submit.OfferingIDs, item.OfferingID)
		if len(item.SelectedDays) > 0 {
			submit.OfferingDays = append(submit.OfferingDays, selection.DaySelection{OfferingID: item.OfferingID, SelectedDays: item.SelectedDays})
		}
	}
	return submit, nil
}

// offeringReviewGrandfathered marks the child's current manual and automatic
// bookings and reports whether any current care booking covers days.
func offeringReviewGrandfathered(current []careplan.OfferingReviewBooking, catalog map[int64]careplan.CareOffering) (selection.Grandfathered, bool) {
	grandfathered := selection.Grandfathered{Manual: map[int64]bool{}, Automatic: map[int64]bool{}}
	hadCareDays := false
	for _, booking := range current {
		if len(booking.AutomaticDays) > 0 {
			grandfathered.Automatic[booking.OfferingID] = true
		}
		if len(booking.ManualDays) > 0 || len(booking.AutomaticDays) == 0 {
			grandfathered.Manual[booking.OfferingID] = true
		}
		if offeringBookingHasCareDays(catalog[booking.OfferingID], booking) {
			hadCareDays = true
		}
	}
	return grandfathered, hadCareDays
}

func offeringBookingHasCareDays(offering careplan.CareOffering, booking careplan.OfferingReviewBooking) bool {
	if !offering.CountsAsCare {
		return false
	}
	if offering.DaysOfWeekMode == "fixed" {
		return len(offering.AvailableDays) > 0
	}
	return len(booking.SelectedDays) > 0
}

func offeringReviewDiff(current []careplan.OfferingReviewBooking, desired []selection.Selection, catalog map[int64]careplan.CareOffering) ([]careplan.OfferingReviewDiffEntry, map[int64]bool) {
	ids := make([]int64, 0, len(current)+len(desired))
	before, after := map[int64][]string{}, map[int64][]string{}
	for _, booking := range current {
		if !slices.Contains(ids, booking.OfferingID) {
			ids = append(ids, booking.OfferingID)
		}
		before[booking.OfferingID] = booking.SelectedDays
	}
	for _, selected := range desired {
		if !slices.Contains(ids, selected.OfferingID) {
			ids = append(ids, selected.OfferingID)
		}
		after[selected.OfferingID] = selected.SelectedDays
	}
	result := make([]careplan.OfferingReviewDiffEntry, 0, len(ids))
	changed := make(map[int64]bool, len(ids))
	for _, id := range ids {
		entry := careplan.OfferingReviewDiffEntry{OfferingID: id, Label: offeringLabel(id, catalog), OldState: "not_booked", NewState: "removed"}
		if days, ok := before[id]; ok {
			entry.OldState, entry.OldDays = "booked", careplan.CanonicalOfferingReviewDays(days)
		}
		if days, ok := after[id]; ok {
			entry.NewState, entry.NewDays = "booked", careplan.CanonicalOfferingReviewDays(days)
		}
		changed[id] = entry.OldState != entry.NewState || !slices.Equal(entry.OldDays, entry.NewDays)
		result = append(result, entry)
	}
	return result, changed
}

func offeringCourseMatches(group careplan.OfferingReviewCourse, child careplan.OfferingReviewChild) bool {
	if !group.Active {
		return false
	}
	if len(group.Grades) > 0 && (child.Grade == nil || !slices.Contains(group.Grades, int(*child.Grade))) {
		return false
	}
	if len(group.SchoolClasses) == 0 {
		return true
	}
	class := strings.ToLower(strings.TrimSpace(child.SchoolClass))
	if class == "" {
		return false
	}
	for _, value := range group.SchoolClasses {
		if strings.ToLower(strings.TrimSpace(value)) == class {
			return true
		}
	}
	return false
}
