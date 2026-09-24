package application

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/enrollment/selection"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// materializeOfferingAdjustment validates the requested offerings against the
// phase's catalog and materializes the selection the adjustment books,
// including the days a Mitbuchungs-Regel or the required lunch derives.
func (m *BookingMaterialization) materializeOfferingAdjustment(ctx context.Context, work *offeringAdjustmentWork) error {
	child, err := adjustmentSelectionChild(work)
	if err != nil {
		return err
	}
	if err := m.resolveAdjustmentAuthority(ctx, work); err != nil {
		return err
	}
	allowed := make(map[int64]*careplan.CareOffering, len(work.activeOfferingByID)+len(work.beforeOfferingByID))
	addOfferingMap(allowed, work.activeOfferingByID)
	addOfferingMap(allowed, work.beforeOfferingByID)
	catalog, err := selectionCatalog(allowed)
	if err != nil {
		return err
	}
	grandfathered := grandfatheredBookings(work.beforeLinks)
	allowWithdrawal := work.authoritative && bookingsHaveCareDays(work.beforeLinks, work.offeringByID)
	overridden, err := appliedOfferingOverrides(work.input.ExcludedAutoAddTargetIDs, child, catalog, allowed, work.phase.CareOfferingSelectionMode, grandfathered)
	if err != nil {
		return err
	}
	work.reason = adjustmentReasonWithOverrides(work.reason, overridden)
	materialized, err := selection.MaterializeAdjustments(
		[]selection.Child{child}, catalog, work.phase.CareOfferingSelectionMode, grandfathered, allowWithdrawal,
	)
	if err != nil {
		return fmt.Errorf("%w: %v", careplan.ErrOfferingAdjustmentInvalid, err)
	}
	work.selections, work.overridden = offeringSelections(materialized[0]), overridden
	work.afterHasCareDays = selectionsHaveCareDays(work.selections, work.offeringByID)
	work.isCompleteWithdrawal = allowWithdrawal && !work.afterHasCareDays
	if work.isCompleteWithdrawal && !work.input.CompleteWithdrawalConfirmed {
		return careplan.ErrCompleteWithdrawalConfirmationRequired
	}
	return buildOfferingAdjustmentReplacement(work)
}

// adjustmentSelectionChild turns the requested offerings into the selection
// engine's child, in catalog order. An offering must be active in the phase
// or already held by the child.
func adjustmentSelectionChild(work *offeringAdjustmentWork) (selection.Child, error) {
	input := work.input
	child := selection.Child{
		TargetGradeLevel:         work.child.TargetGradeLevel,
		OfferingIDs:              make([]int64, 0, len(input.Offerings)),
		OfferingDays:             make([]selection.DaySelection, 0, len(input.Offerings)),
		ExcludedAutoAddTargetIDs: input.ExcludedAutoAddTargetIDs,
	}
	seen := make(map[int64]bool, len(input.Offerings))
	for _, row := range input.Offerings {
		if row.OfferingID <= 0 {
			return selection.Child{}, fmt.Errorf("%w: offering_id is required", careplan.ErrOfferingAdjustmentInvalid)
		}
		if work.activeOfferingByID[row.OfferingID] == nil && work.beforeOfferingByID[row.OfferingID] == nil {
			return selection.Child{}, fmt.Errorf("%w: care offering %d is not available for this child adjustment", selection.ErrCareOfferingMissing, row.OfferingID)
		}
		if seen[row.OfferingID] {
			continue
		}
		seen[row.OfferingID] = true
		child.OfferingIDs = append(child.OfferingIDs, row.OfferingID)
		if len(row.SelectedDays) > 0 {
			child.OfferingDays = append(child.OfferingDays, selection.DaySelection{
				OfferingID:   row.OfferingID,
				SelectedDays: copyDays(row.SelectedDays),
			})
		}
	}
	sortAdjustmentOfferingIDs(child.OfferingIDs, work.offeringByID)
	return child, nil
}

func sortAdjustmentOfferingIDs(ids []int64, offerings map[int64]*careplan.CareOffering) {
	sort.SliceStable(ids, func(i, j int) bool {
		left, right := offerings[ids[i]], offerings[ids[j]]
		if left == nil || right == nil || left.SortOrder == right.SortOrder {
			return ids[i] < ids[j]
		}
		return left.SortOrder < right.SortOrder
	})
}

func (m *BookingMaterialization) resolveAdjustmentAuthority(ctx context.Context, work *offeringAdjustmentWork) error {
	authoritative, err := m.deps.Settings.BookingsAuthoritative(ctx)
	if err != nil {
		return fmt.Errorf("offering adjustment: resolve authoritative bookings setting: %w", err)
	}
	if authoritative && m.deps.Withdrawals == nil {
		return errors.New("offering adjustment: authoritative booking lifecycle is not configured")
	}
	work.authoritative = authoritative
	return nil
}

// selectionCatalog describes the offerings in the selection engine's terms.
func selectionCatalog(offerings map[int64]*careplan.CareOffering) (map[int64]*selection.Offering, error) {
	result := make(map[int64]*selection.Offering, len(offerings))
	for id, offering := range offerings {
		native := &selection.Offering{
			ID: offering.ID, SortOrder: offering.SortOrder, DaysOfWeekMode: offering.DaysOfWeekMode,
			AvailableDays: offering.AvailableDays, CountsAsCare: offering.CountsAsCare, IncludesLunch: offering.IncludesLunch,
			IsRequired: offering.IsRequired, SelectionGroup: offering.SelectionGroup, SelectionRule: offering.SelectionRule,
			AutoAddTriggerOfferingIDs: offering.AutoAddTriggerOfferingIDs, AutoAddGradeLevels: offering.AutoAddGradeLevels,
		}
		if len(offering.AvailabilityRule) > 0 && string(offering.AvailabilityRule) != "null" {
			rule := new(selection.AvailabilityRule)
			if err := json.Unmarshal(offering.AvailabilityRule, rule); err != nil {
				return nil, fmt.Errorf("decode care offering availability rule: %w", err)
			}
			native.AvailabilityRule = rule
		}
		result[id] = native
	}
	return result, nil
}

func offeringSelections(values []selection.Selection) []careplan.OfferingSelection {
	selections := make([]careplan.OfferingSelection, 0, len(values))
	for _, value := range values {
		selections = append(selections, careplan.OfferingSelection{
			OfferingID: value.OfferingID, SelectedDays: value.SelectedDays,
			ManualSelectedDays: value.ManualSelectedDays, AutomaticSelectedDays: value.AutomaticSelectedDays,
		})
	}
	return selections
}

// appliedOfferingOverrides validates the co-booking targets the reviewer
// excluded against the materialization without exclusions.
func appliedOfferingOverrides(
	excluded map[int64]bool,
	child selection.Child,
	catalog map[int64]*selection.Offering,
	allowed map[int64]*careplan.CareOffering,
	selectionMode string,
	grandfathered selection.Grandfathered,
) ([]careplan.OfferingOverride, error) {
	if len(excluded) == 0 {
		return nil, nil
	}
	child.ExcludedAutoAddTargetIDs = nil
	base, err := selection.MaterializeAdjustments([]selection.Child{child}, catalog, selectionMode, grandfathered, false)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", careplan.ErrOfferingAdjustmentInvalid, err)
	}
	excludedIDs := make([]int64, 0, len(excluded))
	for offeringID := range excluded {
		excludedIDs = append(excludedIDs, offeringID)
	}
	sort.Slice(excludedIDs, func(i, j int) bool { return excludedIDs[i] < excludedIDs[j] })
	return careplan.ExcludedAutoTargetOverrides(excludedIDs, offeringSelections(base[0]), allowed)
}

func adjustmentReasonWithOverrides(reason string, overridden []careplan.OfferingOverride) string {
	if len(overridden) == 0 {
		return reason
	}
	names := make([]string, 0, len(overridden))
	for _, offering := range overridden {
		names = append(names, offering.Name)
	}
	return reason + " · Mitbuchung nicht angewendet: " + strings.Join(names, ", ")
}

// grandfatheredBookings classifies the bookings a child already holds for
// the Bestandsschutz exemption (#2186): an availability rule tightened after
// the fact does not revoke what is already on file for THIS child, even when
// the grade rule now excludes it. Newly added blocked offerings are not on
// file and are still rejected.
//
// The split follows the DAYS on each link, not the link as a whole: manual
// days are what the admin ticked and can untick, automatic days are derived
// from a trigger and never appear in a payload at all. A link can carry both,
// so the two buckets deliberately OVERLAP (#2186 review) — unticking such a
// booking withdraws only its manual half, and its automatic half must still be
// re-derived while the trigger stays selected.
func grandfatheredBookings(links []*careplan.BookedOffering) selection.Grandfathered {
	grandfathered := selection.Grandfathered{
		Manual:    make(map[int64]bool, len(links)),
		Automatic: make(map[int64]bool, len(links)),
	}
	for _, link := range links {
		if link == nil {
			continue
		}
		if len(link.AutomaticSelectedDays) > 0 {
			grandfathered.Automatic[link.CareOfferingID] = true
		}
		// Legacy links carry neither breakdown, only SelectedDays. Nothing
		// derived those, so they are manual by construction.
		if len(link.ManualSelectedDays) > 0 || len(link.AutomaticSelectedDays) == 0 {
			grandfathered.Manual[link.CareOfferingID] = true
		}
	}
	return grandfathered
}

// bookingsHaveCareDays reports whether any of the links books a care day of
// an offering that counts as care.
func bookingsHaveCareDays(links []*careplan.BookedOffering, offerings map[int64]*careplan.CareOffering) bool {
	for _, link := range links {
		if link != nil && offeringBooksCareDays(offerings[link.CareOfferingID], link.SelectedDays) {
			return true
		}
	}
	return false
}

func selectionsHaveCareDays(selections []careplan.OfferingSelection, offerings map[int64]*careplan.CareOffering) bool {
	for _, selected := range selections {
		if offeringBooksCareDays(offerings[selected.OfferingID], selected.SelectedDays) {
			return true
		}
	}
	return false
}

// offeringBooksCareDays reports whether the selected days of an offering that
// counts as care book at least one day. A fixed offering books its days.
func offeringBooksCareDays(offering *careplan.CareOffering, selectedDays []string) bool {
	if offering == nil || !offering.CountsAsCare {
		return false
	}
	if offering.DaysOfWeekMode == daysOfWeekModeFixed {
		return len(offering.AvailableDays) > 0
	}
	return len(selectedDays) > 0
}

func buildOfferingAdjustmentReplacement(work *offeringAdjustmentWork) error {
	replacement := make([]*careplan.BookedOffering, 0, len(work.selections))
	for _, selected := range work.selections {
		replacement = append(replacement, &careplan.BookedOffering{
			RequestChildID:        work.child.ID,
			CareOfferingID:        selected.OfferingID,
			SelectedDays:          selected.SelectedDays,
			ManualSelectedDays:    selected.ManualSelectedDays,
			AutomaticSelectedDays: selected.AutomaticSelectedDays,
		})
	}
	replacement = capOfferingReplacementAtCareEnd(replacement, work.student.EnrolledUntil, work.phase.OfferingPhase, work.effectiveFrom)
	afterJSON, err := adjustmentSnapshotJSON(replacement, work.offeringByID)
	if err != nil {
		return err
	}
	work.replacement, work.afterJSON = replacement, afterJSON
	return nil
}

// capOfferingReplacementAtCareEnd bounds the new selection by the child's
// last care day. A replacement that would start after it plans nothing.
func capOfferingReplacementAtCareEnd(
	rows []*careplan.BookedOffering,
	lastCareDay *calendar.Date,
	phase careplan.OfferingPhase,
	effectiveFrom *calendar.Date,
) []*careplan.BookedOffering {
	if lastCareDay == nil {
		return rows
	}
	validFrom := phase.ServiceStart
	if effectiveFrom != nil {
		validFrom = *effectiveFrom
	}
	validUntil := lastCareDay.AddDays(1)
	if !validFrom.Before(validUntil) {
		return nil
	}
	for _, row := range rows {
		from, until := validFrom, validUntil
		row.ValidFrom, row.ValidUntil = &from, &until
	}
	return rows
}

func adjustmentSnapshotJSON(links []*careplan.BookedOffering, offeringByID map[int64]*careplan.CareOffering) ([]byte, error) {
	rows := make([]careplan.OfferingAdjustmentSnapshot, 0, len(links))
	for _, link := range links {
		if link == nil {
			continue
		}
		row := careplan.OfferingAdjustmentSnapshot{
			OfferingID:            strconv.FormatInt(link.CareOfferingID, 10),
			SelectedDays:          copyDays(link.SelectedDays),
			ManualSelectedDays:    copyDays(link.ManualSelectedDays),
			AutomaticSelectedDays: copyDays(link.AutomaticSelectedDays),
		}
		if offering := offeringByID[link.CareOfferingID]; offering != nil {
			row.OfferingName = offering.Name
			row.DaysOfWeekMode = offering.DaysOfWeekMode
			row.AvailableDays = copyDays(offering.AvailableDays)
		}
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return lessNumericString(rows[i].OfferingID, rows[j].OfferingID)
	})
	raw, err := json.Marshal(rows)
	if err != nil {
		return nil, fmt.Errorf("decision: marshal offering adjustment snapshot: %w", err)
	}
	return raw, nil
}

func lessNumericString(left, right string) bool {
	leftID, leftErr := strconv.ParseInt(left, 10, 64)
	rightID, rightErr := strconv.ParseInt(right, 10, 64)
	if leftErr == nil && rightErr == nil {
		return leftID < rightID
	}
	return left < right
}

// careExitSourceOfferingsFromLinks summarizes the bookings a complete
// withdrawal ends, for the binding care-exit preview.
func careExitSourceOfferingsFromLinks(links []*careplan.BookedOffering, offerings map[int64]*careplan.CareOffering) []careplan.CareExitSourceOffering {
	result := make([]careplan.CareExitSourceOffering, 0, len(links))
	for _, link := range links {
		if link == nil {
			continue
		}
		offering := offerings[link.CareOfferingID]
		if offering == nil {
			continue
		}
		days := link.SelectedDays
		if offering.DaysOfWeekMode == daysOfWeekModeFixed && len(days) == 0 {
			days = offering.AvailableDays
		}
		result = append(result, careplan.CareExitSourceOffering{Name: offering.Name, Days: copyDays(days)})
	}
	return result
}
