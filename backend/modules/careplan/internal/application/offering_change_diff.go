package application

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"sort"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/enrollment/selection"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// offeringDecisionDiff is a request's review diff plus the override
// bookkeeping a staff approval with exclusions needs.
type offeringDecisionDiff struct {
	entries      []careplan.OfferingChangeDiffEntry
	overridden   []careplan.OfferingOverride
	current      []*careplan.BookedOffering
	base         []careplan.OfferingSelection
	selected     []careplan.OfferingSelection
	offeringByID map[int64]*careplan.CareOffering
	// phase, requested and effectiveFrom are the context the materialization
	// ran against, so a caller can re-check applicability without resolving
	// the same aggregate twice.
	phase         *ports.BookingPhase
	requested     []careplan.OfferingChangeSelection
	effectiveFrom calendar.Date
}

type offeringDecisionMaterialization struct {
	row       careplan.OfferingChangeRequest
	child     *ports.OfferingChangeChild
	base      []careplan.OfferingSelection
	selected  []careplan.OfferingSelection
	phase     *ports.BookingPhase
	requested []careplan.OfferingChangeSelection
}

// decisionDiffInput is everything buildDecisionDiff compares: the current
// booking, the unexcluded and the excluded materialization, and the course
// target the added lines are checked against.
type decisionDiffInput struct {
	excludedIDs []int64
	current     []*careplan.BookedOffering
	base        []careplan.OfferingSelection
	selected    []careplan.OfferingSelection
	course      *careplan.OfferingChangeCatalog
	requested   []careplan.OfferingChangeSelection
}

// diffForRequest renders "current → requested" per offering, including the
// dropped ones, so a reviewer never has to open two screens.
func (s *OfferingChanges) diffForRequest(ctx context.Context, row careplan.OfferingChangeRequest) ([]careplan.OfferingChangeDiffEntry, error) {
	diff, err := s.decisionDiff(ctx, row, nil, nil)
	if err != nil {
		return nil, err
	}
	return diff.entries, nil
}

// decisionDiff builds the "current → requested" diff. With exclusions it
// first validates each excluded id against the unexcluded materialization:
// only a target that actually gains rule-derived days can be overridden
// (#2370).
func (s *OfferingChanges) decisionDiff(
	ctx context.Context,
	row careplan.OfferingChangeRequest,
	excludedIDs []int64,
	effectiveFrom *calendar.Date,
) (*offeringDecisionDiff, error) {
	materialization, err := s.materializeDecisionSelections(ctx, row, excludedIDs, effectiveFrom)
	if err != nil {
		return nil, err
	}
	applied := offeringChangeEffectiveFrom(materialization.row)
	current, err := s.currentSelections(ctx, materialization.row.RequestChildID, applied)
	if err != nil {
		return nil, fmt.Errorf("offering change: list current offerings: %w", err)
	}
	diff, err := s.buildDecisionDiff(ctx, decisionDiffInput{
		excludedIDs: excludedIDs, current: current, base: materialization.base, selected: materialization.selected,
		course: courseTargetCatalog(materialization.child), requested: materialization.requested,
	})
	if err != nil {
		return nil, err
	}
	diff.phase = materialization.phase
	diff.requested = materialization.requested
	diff.effectiveFrom = applied
	return diff, nil
}

func (s *OfferingChanges) materializeDecisionSelections(
	ctx context.Context,
	row careplan.OfferingChangeRequest,
	excludedIDs []int64,
	effectiveFrom *calendar.Date,
) (*offeringDecisionMaterialization, error) {
	requested, child, phase, err := s.decisionRequestContext(ctx, row)
	if err != nil {
		return nil, err
	}
	confirmedDate, err := confirmedEffectiveFrom(effectiveFrom, offeringChangeEffectiveFrom(row), s.todayDate(), phase)
	row.EffectiveFrom = confirmedDate.String()
	if err != nil {
		return nil, err
	}
	allowCompleteWithdrawal, err := s.bookingsAuthoritative(ctx)
	if err != nil {
		return nil, err
	}
	in := selectionMaterialization{
		phase: phase, requestChildID: row.RequestChildID, effectiveFrom: confirmedDate,
		selections: requested, allowCompleteWithdrawal: allowCompleteWithdrawal,
	}
	base, err := s.materializedSelections(ctx, in)
	if err != nil {
		return nil, err
	}
	result := &offeringDecisionMaterialization{
		row: row, child: child, base: base, selected: base, phase: phase, requested: requested,
	}
	in.excluded = offeringIDSet(excludedIDs)
	if len(in.excluded) == 0 {
		return result, nil
	}
	result.selected, err = s.materializedSelections(ctx, in)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *OfferingChanges) decisionRequestContext(
	ctx context.Context,
	row careplan.OfferingChangeRequest,
) ([]careplan.OfferingChangeSelection, *ports.OfferingChangeChild, *ports.BookingPhase, error) {
	requested, err := requestSelections(row)
	if err != nil {
		return nil, nil, nil, err
	}
	child, err := s.deps.Enrollment.Child(ctx, row.RequestChildID)
	if err != nil || child == nil {
		return nil, nil, nil, fmt.Errorf("offering change: load request child for diff: %w", err)
	}
	request, err := s.deps.Enrollment.Request(ctx, child.RequestID)
	if err != nil || request == nil {
		return nil, nil, nil, fmt.Errorf("offering change: load request for diff: %w", err)
	}
	phase, err := s.deps.Enrollment.Phase(ctx, request.PhaseID)
	if err != nil || phase == nil {
		return nil, nil, nil, fmt.Errorf("offering change: load phase for diff: %w", err)
	}
	return requested, child, phase, nil
}

// appendMissingOfferingIDs adds the offerings of the base materialization
// the excluded diff no longer has; their names are still needed for the
// override record.
func appendMissingOfferingIDs(ids []int64, selections []careplan.OfferingSelection) []int64 {
	result := append([]int64(nil), ids...)
	for _, selected := range selections {
		if !slices.Contains(result, selected.OfferingID) {
			result = append(result, selected.OfferingID)
		}
	}
	return result
}

func (s *OfferingChanges) buildDecisionDiff(ctx context.Context, in decisionDiffInput) (*offeringDecisionDiff, error) {
	ids, currentByID, requestedByID := offeringChangeSides(in.current, offeringChangeSelections(in.selected))
	offerings, err := s.offeringsByIDs(ctx, appendMissingOfferingIDs(ids, in.base))
	if err != nil {
		return nil, fmt.Errorf("offering change: list offerings for diff: %w", err)
	}
	offeringByID := offeringsByID(offerings)
	overridden, err := careplan.ExcludedAutoTargetOverrides(in.excludedIDs, in.base, offeringByID)
	if err != nil {
		return nil, err
	}
	diff, err := materializedDecisionDiffFromSides(decisionSides{
		currentByID: currentByID, requestedByID: requestedByID, ids: ids,
	}, in, offeringByID, overridden)
	if err != nil {
		return nil, err
	}
	if err := s.markCourseDiffEntries(ctx, diff.entries, offeringByID, in.course, in.requested); err != nil {
		return nil, err
	}
	return diff, nil
}

// decisionSides are the two sides of a diff keyed by offering, with the
// offerings in first-seen order.
type decisionSides struct {
	currentByID   map[int64]*careplan.BookedOffering
	requestedByID map[int64]careplan.OfferingChangeSelection
	ids           []int64
}

// materializedDecisionDiff describes exactly the booking an adjustment
// persisted, so the decision snapshot never re-reads a moved catalog.
func materializedDecisionDiff(result *careplan.OfferingAdjustmentResult) (*offeringDecisionDiff, error) {
	current := bookedOfferingPointers(result.Before)
	offeringByID := make(map[int64]*careplan.CareOffering, len(result.Offerings))
	for id, offering := range result.Offerings {
		offeringByID[id] = &offering
	}
	ids, currentByID, requestedByID := offeringChangeSides(current, offeringChangeSelections(result.Selections))
	return materializedDecisionDiffFromSides(
		decisionSides{currentByID: currentByID, requestedByID: requestedByID, ids: ids},
		decisionDiffInput{current: current, selected: result.Selections},
		offeringByID, result.Overridden,
	)
}

func materializedDecisionDiffFromSides(
	sides decisionSides,
	in decisionDiffInput,
	offeringByID map[int64]*careplan.CareOffering,
	overridden []careplan.OfferingOverride,
) (*offeringDecisionDiff, error) {
	entries := offeringDiffEntries(sides, offeringByID)
	if err := annotateAutomaticShares(entries, in.selected, offeringByID); err != nil {
		return nil, err
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Label < entries[j].Label })
	return &offeringDecisionDiff{
		entries: entries, overridden: overridden, current: in.current, base: in.base,
		selected: in.selected, offeringByID: offeringByID,
	}, nil
}

func offeringDiffEntries(sides decisionSides, offeringByID map[int64]*careplan.CareOffering) []careplan.OfferingChangeDiffEntry {
	entries := make([]careplan.OfferingChangeDiffEntry, 0, len(sides.ids))
	for _, id := range sides.ids {
		name := ""
		if offering := offeringByID[id]; offering != nil {
			name = offering.Name
		}
		entry, changed := offeringDiffEntry(id, name, sides.currentByID[id], sides.requestedByID)
		if changed {
			entries = append(entries, entry)
		}
	}
	return entries
}

func offeringChangeSides(
	current []*careplan.BookedOffering,
	requested []careplan.OfferingChangeSelection,
) ([]int64, map[int64]*careplan.BookedOffering, map[int64]careplan.OfferingChangeSelection) {
	ids := make([]int64, 0, len(current)+len(requested))
	seen := make(map[int64]bool, len(current)+len(requested))
	addID := func(id int64) {
		if id > 0 && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	currentByID := make(map[int64]*careplan.BookedOffering, len(current))
	for _, link := range current {
		if link != nil {
			currentByID[link.CareOfferingID] = link
			addID(link.CareOfferingID)
		}
	}
	requestedByID := make(map[int64]careplan.OfferingChangeSelection, len(requested))
	for _, wanted := range requested {
		requestedByID[wanted.OfferingID] = wanted
		addID(wanted.OfferingID)
	}
	return ids, currentByID, requestedByID
}

// annotateAutomaticShares marks the diff lines whose new side carries days a
// Mitbuchungs-Regel (or the required lunch) added (#2365), named after the
// selected offerings that triggered them.
func annotateAutomaticShares(
	entries []careplan.OfferingChangeDiffEntry,
	materialized []careplan.OfferingSelection,
	offeringByID map[int64]*careplan.CareOffering,
) error {
	catalog, err := selectionCatalog(offeringByID)
	if err != nil {
		return err
	}
	shares := selection.AutomaticShares(nativeSelections(materialized), catalog)
	for i := range entries {
		entry := &entries[i]
		share, ok := shares[entry.OfferingID]
		if !ok {
			continue
		}
		entry.NewAutomaticDays = share.AutomaticDays
		entry.NewRuleDays = share.RuleDays
		entry.NewDaysWithoutRules = share.DaysWithoutRules
		entry.AutoTriggerIDs = share.TriggerIDs
		for _, triggerID := range share.TriggerIDs {
			name := fmt.Sprintf("Angebot %d", triggerID)
			if trigger := offeringByID[triggerID]; trigger != nil && trigger.Name != "" {
				name = trigger.Name
			}
			entry.AutoTriggerNames = append(entry.AutoTriggerNames, name)
		}
	}
	return nil
}

func offeringDiffEntry(
	id int64,
	name string,
	current *careplan.BookedOffering,
	requestedByID map[int64]careplan.OfferingChangeSelection,
) (careplan.OfferingChangeDiffEntry, bool) {
	if name == "" {
		name = fmt.Sprintf("Angebot %d", id)
	}
	oldState := "not_booked"
	var oldDays []string
	if current != nil {
		oldState = "booked"
		oldDays = canonicalDays(current.SelectedDays)
	}
	newState := "removed"
	var newDays []string
	if requested, wanted := requestedByID[id]; wanted {
		newState = "booked"
		newDays = canonicalDays(requested.SelectedDays)
	}
	changed := oldState != newState || !slices.Equal(oldDays, newDays)
	return careplan.OfferingChangeDiffEntry{
		OfferingID: id, Label: name, OldState: oldState, OldDays: oldDays, NewState: newState, NewDays: newDays,
	}, changed
}

func selectionPointersByOffering(materialized []careplan.OfferingSelection) map[int64]*careplan.OfferingSelection {
	byID := make(map[int64]*careplan.OfferingSelection, len(materialized))
	for i := range materialized {
		byID[materialized[i].OfferingID] = &materialized[i]
	}
	return byID
}

// rejectionDecisionDiff freezes what a rejection refused. A request that no
// longer materializes is frozen from its payload instead.
func (s *OfferingChanges) rejectionDecisionDiff(ctx context.Context, row careplan.OfferingChangeRequest) (*offeringDecisionDiff, error) {
	diff, err := s.decisionDiff(ctx, row, nil, nil)
	if err == nil {
		return diff, nil
	}
	fallback, fallbackErr := s.payloadDecisionDiff(ctx, row)
	if fallbackErr != nil {
		return nil, fmt.Errorf("offering change: build rejection snapshot: %v; payload fallback: %w", err, fallbackErr)
	}
	s.logger().Warn("offering change: rejection snapshot uses request payload",
		slog.Int64("request_id", row.ID),
		slog.String("materialization_error", err.Error()),
	)
	return fallback, nil
}

func (s *OfferingChanges) payloadDecisionDiff(ctx context.Context, row careplan.OfferingChangeRequest) (*offeringDecisionDiff, error) {
	requested, err := requestSelections(row)
	if err != nil {
		return nil, err
	}
	current, err := s.currentSelections(ctx, row.RequestChildID, appliedOfferingChangeDate(offeringChangeEffectiveFrom(row), s.todayDate()))
	if err != nil {
		return nil, fmt.Errorf("list current offerings: %w", err)
	}
	current = explicitOfferingLinks(current)
	ids, currentByID, requestedByID := offeringChangeSides(current, requested)
	offerings, err := s.offeringsByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list snapshot offerings: %w", err)
	}
	entries := offeringDiffEntries(decisionSides{currentByID: currentByID, requestedByID: requestedByID, ids: ids}, offeringsByID(offerings))
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Label < entries[j].Label })
	return &offeringDecisionDiff{entries: entries, current: current}, nil
}

// explicitOfferingLinks keeps the manual share of the bookings: a booking
// derived only from another offering is dropped, a mixed one keeps its
// manual days.
func explicitOfferingLinks(links []*careplan.BookedOffering) []*careplan.BookedOffering {
	explicit := make([]*careplan.BookedOffering, 0, len(links))
	for _, link := range links {
		if link == nil || (len(link.ManualSelectedDays) == 0 && len(link.AutomaticSelectedDays) > 0) {
			continue
		}
		clone := *link
		if len(link.AutomaticSelectedDays) > 0 {
			clone.SelectedDays = append([]string(nil), link.ManualSelectedDays...)
		}
		explicit = append(explicit, &clone)
	}
	return explicit
}

// storeDecisionSnapshot freezes the diff on the decided row (ADR 0002).
func (s *OfferingChanges) storeDecisionSnapshot(ctx context.Context, requestID int64, diff *offeringDecisionDiff) error {
	if diff == nil {
		return fmt.Errorf("offering change: decision snapshot diff is required")
	}
	snapshot, err := encodeDecisionSnapshot(diff)
	if err != nil {
		return fmt.Errorf("offering change: store decision snapshot: %w", err)
	}
	if err := s.deps.Rows.UpdateDecisionSnapshot(ctx, requestID, snapshot); err != nil {
		return fmt.Errorf("offering change: store decision snapshot: %w", err)
	}
	return nil
}
