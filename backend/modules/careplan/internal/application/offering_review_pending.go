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
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/enrollment/selection"
)

type offeringPendingReview struct {
	applied, earliest, latest careplan.Date
	diff, unchanged           []careplan.OfferingReviewDiffEntry
	withdrawal                bool
}

type offeringPendingFacts struct {
	children  map[int64]ports.OfferingReviewChild
	requests  map[int64]ports.OfferingReviewRequest
	phases    map[int64]ports.OfferingReviewPhase
	current   map[int64][]ports.OfferingReviewBooking
	active    map[int64]map[int64]domain.CareOffering
	catalog   map[int64]domain.CareOffering
	requested map[int64][]careplan.OfferingReviewSelection
	reviews   map[int64]*offeringPendingReview
}

func (s *OfferingReviews) pendingFacts(ctx context.Context, rows []careplan.OfferingChangeRequest, today careplan.Date) (*offeringPendingFacts, error) {
	facts := &offeringPendingFacts{
		children: map[int64]ports.OfferingReviewChild{}, requests: map[int64]ports.OfferingReviewRequest{}, phases: map[int64]ports.OfferingReviewPhase{},
		current: map[int64][]ports.OfferingReviewBooking{}, active: map[int64]map[int64]domain.CareOffering{},
		catalog: map[int64]domain.CareOffering{}, requested: map[int64][]careplan.OfferingReviewSelection{}, reviews: map[int64]*offeringPendingReview{},
	}
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
	active, err := s.Catalog.ListCareOfferings(ctx, domain.CareOfferingFilter{PhaseIDs: phaseIDs, ActiveOnly: true})
	if err != nil {
		return nil, fmt.Errorf("load active offerings: %w", err)
	}
	allIDs := make([]int64, 0, len(active))
	for _, offering := range active {
		if facts.active[offering.PhaseID] == nil {
			facts.active[offering.PhaseID] = map[int64]domain.CareOffering{}
		}
		facts.active[offering.PhaseID][offering.ID] = offering
		allIDs = append(allIDs, offering.ID)
	}
	current, err := s.Enrollment.Selections(ctx, dates)
	if err != nil {
		return nil, fmt.Errorf("load current offerings: %w", err)
	}
	for _, booking := range current {
		facts.current[booking.ChildID] = append(facts.current[booking.ChildID], booking)
		allIDs = append(allIDs, booking.OfferingID)
	}
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
	catalog, err := s.Catalog.ListCareOfferings(ctx, domain.CareOfferingFilter{IDs: allIDs})
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

func (s *OfferingReviews) pendingReviews(ctx context.Context, rows []careplan.OfferingChangeRequest, today careplan.Date) (map[int64]*offeringPendingReview, error) {
	if len(rows) == 0 {
		return map[int64]*offeringPendingReview{}, nil
	}
	facts, err := s.pendingFacts(ctx, rows, today)
	if err != nil {
		return nil, err
	}
	type courseDiff struct {
		review    *offeringPendingReview
		entries   []careplan.OfferingReviewDiffEntry
		child     ports.OfferingReviewChild
		requested map[int64]bool
		changed   map[int64]bool
	}
	pending := make([]courseDiff, 0, len(rows))
	refs := make([]ports.OfferingReviewCourseRef, 0)
	for _, row := range rows {
		child, childOK := facts.children[row.RequestChildID]
		request, requestOK := facts.requests[child.RequestID]
		phase, phaseOK := facts.phases[request.PhaseID]
		requested, payloadOK := facts.requested[row.ID]
		if !childOK || !requestOK || !phaseOK || !payloadOK {
			continue
		}
		allowed := make(map[int64]domain.CareOffering, len(facts.active[phase.ID])+len(facts.current[child.ID]))
		for id, offering := range facts.active[phase.ID] {
			allowed[id] = offering
		}
		for _, booking := range facts.current[child.ID] {
			if offering, ok := facts.catalog[booking.OfferingID]; ok {
				allowed[offering.ID] = offering
			}
		}
		materialized, err := materializeOfferingReview(child, phase, requested, facts.current[child.ID], allowed, row.CompleteWithdrawalConfirmed)
		if err != nil {
			continue
		}
		entries, changed := offeringReviewDiff(facts.current[child.ID], materialized, facts.catalog)
		nativeCatalog, err := offeringSelectionCatalog(facts.catalog)
		if err != nil {
			continue
		}
		shares := selection.AutomaticShares(materialized, nativeCatalog)
		review := facts.reviews[row.ID]
		requestedIDs := make(map[int64]bool, len(requested))
		for _, item := range requested {
			requestedIDs[item.OfferingID] = true
		}
		held, remains := false, false
		for i := range entries {
			entry := &entries[i]
			share := shares[entry.OfferingID]
			entry.NewAutomaticDays, entry.NewRuleDays, entry.NewDaysWithoutRules, entry.AutoTriggerIDs = share.AutomaticDays, share.RuleDays, share.DaysWithoutRules, share.TriggerIDs
			for _, id := range share.TriggerIDs {
				entry.AutoTriggerNames = append(entry.AutoTriggerNames, offeringLabel(id, facts.catalog))
			}
			if facts.catalog[entry.OfferingID].CountsAsCare {
				held = held || entry.OldState == "booked"
				remains = remains || entry.NewState == "booked"
			}
			if !changed[entry.OfferingID] && len(entry.NewRuleDays) == 0 && entry.NewState == "booked" {
				review.unchanged = append(review.unchanged, *entry)
			}
			if entry.OldState == "not_booked" && entry.NewState == "booked" && requestedIDs[entry.OfferingID] {
				refs = append(refs, ports.OfferingReviewCourseRef{OfferingID: entry.OfferingID, LegacyGroupID: facts.catalog[entry.OfferingID].ActivityGroupID})
			}
		}
		review.withdrawal = held && !remains
		sort.SliceStable(review.unchanged, func(i, j int) bool { return review.unchanged[i].Label < review.unchanged[j].Label })
		pending = append(pending, courseDiff{review: review, entries: entries, child: child, requested: requestedIDs, changed: changed})
	}
	groups := map[int64][]ports.OfferingReviewCourse{}
	if len(refs) > 0 {
		groups, err = s.Courses.CourseGroups(ctx, refs, today)
		if err != nil {
			return nil, fmt.Errorf("offering change: mark course diff lines: %w", err)
		}
	}
	for _, diff := range pending {
		for i := range diff.entries {
			entry := &diff.entries[i]
			if entry.OldState != "not_booked" || entry.NewState != "booked" || !diff.requested[entry.OfferingID] {
				continue
			}
			for _, group := range groups[entry.OfferingID] {
				if offeringCourseMatches(group, diff.child) {
					entry.IsCourse = true
					break
				}
			}
		}
		entries := slices.DeleteFunc(diff.entries, func(entry careplan.OfferingReviewDiffEntry) bool {
			return !diff.changed[entry.OfferingID] && len(entry.NewRuleDays) == 0
		})
		sort.SliceStable(entries, func(i, j int) bool { return entries[i].Label < entries[j].Label })
		diff.review.diff = entries
	}
	return facts.reviews, nil
}

func offeringSelectionCatalog(catalog map[int64]domain.CareOffering) (map[int64]*selection.Offering, error) {
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

func materializeOfferingReview(child ports.OfferingReviewChild, phase ports.OfferingReviewPhase, requested []careplan.OfferingReviewSelection, current []ports.OfferingReviewBooking, catalog map[int64]domain.CareOffering, confirmed bool) ([]selection.Selection, error) {
	submit := selection.Child{TargetGradeLevel: child.Grade}
	for _, item := range requested {
		if item.OfferingID <= 0 {
			return nil, fmt.Errorf("offering id is required")
		}
		if slices.Contains(submit.OfferingIDs, item.OfferingID) {
			continue
		}
		if _, ok := catalog[item.OfferingID]; !ok {
			return nil, fmt.Errorf("care offering %d cannot be booked for this child", item.OfferingID)
		}
		submit.OfferingIDs = append(submit.OfferingIDs, item.OfferingID)
		if len(item.SelectedDays) > 0 {
			submit.OfferingDays = append(submit.OfferingDays, selection.DaySelection{OfferingID: item.OfferingID, SelectedDays: item.SelectedDays})
		}
	}
	grandfathered := selection.Grandfathered{Manual: map[int64]bool{}, Automatic: map[int64]bool{}}
	hadCareDays := false
	for _, booking := range current {
		if len(booking.AutomaticDays) > 0 {
			grandfathered.Automatic[booking.OfferingID] = true
		}
		if len(booking.ManualDays) > 0 || len(booking.AutomaticDays) == 0 {
			grandfathered.Manual[booking.OfferingID] = true
		}
		offering := catalog[booking.OfferingID]
		if offering.CountsAsCare && ((offering.DaysOfWeekMode == "fixed" && len(offering.AvailableDays) > 0) || (offering.DaysOfWeekMode != "fixed" && len(booking.SelectedDays) > 0)) {
			hadCareDays = true
		}
	}
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

func offeringReviewDiff(current []ports.OfferingReviewBooking, desired []selection.Selection, catalog map[int64]domain.CareOffering) ([]careplan.OfferingReviewDiffEntry, map[int64]bool) {
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

func offeringCourseMatches(group ports.OfferingReviewCourse, child ports.OfferingReviewChild) bool {
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
