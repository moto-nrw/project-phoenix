package application

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/enrollment/selection"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// A requested selection is validated and materialized with the very same
// selection engine the enrollment form and the staff adjustment use, so a
// request cannot be accepted that an approval would then refuse.

// The engine's input and the rule failures a review reports as invalid.
type (
	selectionChild = selection.Child
	daySelection   = selection.DaySelection
)

var (
	errCareOfferingRule               = selection.ErrCareOfferingRule
	errCareOfferingExactlyOneRequired = selection.ErrCareOfferingExactlyOneRequired
)

// selectionMaterialization is what one materialization runs against: the
// child's phase, the request child whose bookings it replaces, and the day
// the replacement starts.
type selectionMaterialization struct {
	phase                   *ports.BookingPhase
	requestChildID          int64
	effectiveFrom           calendar.Date
	selections              []careplan.OfferingChangeSelection
	excluded                map[int64]bool
	allowCompleteWithdrawal bool
}

// currentSelections reads the child's bookings in force on the date.
func (s *OfferingChanges) currentSelections(ctx context.Context, requestChildID int64, on calendar.Date) ([]*careplan.BookedOffering, error) {
	values, err := s.deps.Enrollment.SelectionsAt(ctx, requestChildID, on)
	if err != nil {
		return nil, err
	}
	return bookedOfferingPointers(values), nil
}

// materializedSelections materializes the selection with every
// Mitbuchungs-Regel in force. Excluded targets (#2370) keep their manual days
// but gain no rule-derived ones.
func (s *OfferingChanges) materializedSelections(ctx context.Context, in selectionMaterialization) ([]careplan.OfferingSelection, error) {
	active, err := s.activeOfferings(ctx, in.phase.ID)
	if err != nil {
		return nil, fmt.Errorf("offering change: list active offerings: %w", err)
	}
	allowed := offeringsByID(active)
	current, err := s.addHeldOfferingsAtDate(ctx, in.requestChildID, in.effectiveFrom, allowed)
	if err != nil {
		return nil, err
	}
	allowWithdrawal := in.allowCompleteWithdrawal && bookingsHaveCareDays(current, allowed)
	_, child, err := normalizeOfferingSelections(in.selections, allowed)
	if err != nil {
		return nil, err
	}
	if err := s.applyChildGrade(ctx, in.requestChildID, &child); err != nil {
		return nil, err
	}
	child.ExcludedAutoAddTargetIDs = in.excluded
	materialized, err := materializeForAdjustment(child, allowed, in.phase.CareOfferingSelectionMode, current, allowWithdrawal)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", careplan.ErrOfferingChangeInvalid, err)
	}
	return materialized, nil
}

// validateSelections checks the desired selection against the phase catalog
// and its selection rules and returns the guardian's explicit selection in
// offering order. The dependent offerings the engine adds are not stored:
// they would turn into manual bookings on approval.
func (s *OfferingChanges) validateSelections(ctx context.Context, in selectionMaterialization) ([]careplan.OfferingChangeSelection, error) {
	active, err := s.activeOfferings(ctx, in.phase.ID)
	if err != nil {
		return nil, fmt.Errorf("offering change: list active offerings: %w", err)
	}
	allowed := offeringsByID(active)
	current, err := s.addHeldOfferingsAtDate(ctx, in.requestChildID, in.effectiveFrom, allowed)
	if err != nil {
		return nil, err
	}
	allowWithdrawal := in.allowCompleteWithdrawal && bookingsHaveCareDays(current, allowed)
	normalized, child, err := normalizeOfferingSelections(in.selections, allowed)
	if err != nil {
		return nil, err
	}
	if err := s.applyChildGrade(ctx, in.requestChildID, &child); err != nil {
		return nil, err
	}
	if _, err := materializeForAdjustment(child, allowed, in.phase.CareOfferingSelectionMode, current, allowWithdrawal); err != nil {
		return nil, fmt.Errorf("%w: %v", careplan.ErrOfferingChangeInvalid, err)
	}
	sort.SliceStable(normalized, func(i, j int) bool {
		return normalized[i].OfferingID < normalized[j].OfferingID
	})
	return normalized, nil
}

func (s *OfferingChanges) applyChildGrade(ctx context.Context, requestChildID int64, child *selectionChild) error {
	requestChild, err := s.deps.Enrollment.Child(ctx, requestChildID)
	if err != nil || requestChild == nil {
		return fmt.Errorf("offering change: load request child: %w", err)
	}
	child.TargetGradeLevel = requestChild.TargetGradeLevel
	return nil
}

// materializeForAdjustment runs the selection engine for one child. Held
// bookings keep their Bestandsschutz (#2186).
func materializeForAdjustment(
	child selectionChild,
	allowed map[int64]*careplan.CareOffering,
	selectionMode string,
	current []*careplan.BookedOffering,
	allowCompleteWithdrawal bool,
) ([]careplan.OfferingSelection, error) {
	catalog, err := selectionCatalog(allowed)
	if err != nil {
		return nil, err
	}
	materialized, err := selection.MaterializeAdjustments(
		[]selectionChild{child}, catalog, selectionMode, grandfatheredBookings(current), allowCompleteWithdrawal,
	)
	if err != nil {
		return nil, err
	}
	return offeringSelections(materialized[0]), nil
}

// addHeldOfferingsAtDate adds the offerings the child holds on the date to
// the allowed catalog, even when the admin deactivated them since.
func (s *OfferingChanges) addHeldOfferingsAtDate(
	ctx context.Context,
	requestChildID int64,
	onDate calendar.Date,
	allowed map[int64]*careplan.CareOffering,
) ([]*careplan.BookedOffering, error) {
	if requestChildID <= 0 {
		return nil, nil
	}
	current, err := s.currentSelections(ctx, requestChildID, onDate)
	if err != nil {
		return nil, fmt.Errorf("offering change: list current offerings: %w", err)
	}
	heldIDs := make([]int64, 0, len(current))
	for _, link := range current {
		if link != nil && allowed[link.CareOfferingID] == nil {
			heldIDs = append(heldIDs, link.CareOfferingID)
		}
	}
	if len(heldIDs) == 0 {
		return current, nil
	}
	held, err := s.offeringsByIDs(ctx, heldIDs)
	if err != nil {
		return nil, fmt.Errorf("offering change: list held offerings: %w", err)
	}
	maps.Copy(allowed, offeringsByID(held))
	return current, nil
}

// normalizeOfferingSelections validates the requested offerings and days and
// returns them canonicalized, together with the selection engine's child.
func normalizeOfferingSelections(
	selections []careplan.OfferingChangeSelection,
	allowed map[int64]*careplan.CareOffering,
) ([]careplan.OfferingChangeSelection, selectionChild, error) {
	normalized := make([]careplan.OfferingChangeSelection, 0, len(selections))
	child := selectionChild{
		OfferingIDs:  make([]int64, 0, len(selections)),
		OfferingDays: make([]daySelection, 0, len(selections)),
	}
	seen := make(map[int64]bool, len(selections))
	for _, requested := range selections {
		if requested.OfferingID <= 0 {
			return nil, selectionChild{}, fmt.Errorf("%w: offering id is required", careplan.ErrOfferingChangeInvalid)
		}
		if seen[requested.OfferingID] {
			continue
		}
		if allowed[requested.OfferingID] == nil {
			return nil, selectionChild{}, fmt.Errorf("%w: care offering %d cannot be booked for this child", careplan.ErrOfferingChangeInvalid, requested.OfferingID)
		}
		seen[requested.OfferingID] = true
		if err := validateSelectedDays(requested.SelectedDays); err != nil {
			return nil, selectionChild{}, err
		}
		days := canonicalDays(requested.SelectedDays)
		normalized = append(normalized, careplan.OfferingChangeSelection{OfferingID: requested.OfferingID, SelectedDays: days})
		child.OfferingIDs = append(child.OfferingIDs, requested.OfferingID)
		if len(days) > 0 {
			child.OfferingDays = append(child.OfferingDays, daySelection{OfferingID: requested.OfferingID, SelectedDays: days})
		}
	}
	return normalized, child, nil
}

func validateSelectedDays(days []string) error {
	for _, day := range days {
		normalizedDay := strings.ToLower(strings.TrimSpace(day))
		if normalizedDay == "" {
			continue
		}
		if _, ok := offeringDayWeekday(normalizedDay); !ok {
			return fmt.Errorf("%w: unknown selected day %q", careplan.ErrOfferingChangeInvalid, day)
		}
	}
	return nil
}

// withoutAutomaticSelections ignores guardian input for offerings currently
// derived from another selection. Automatic offerings are visible in the form
// but cannot be changed on their own or turned into manual bookings.
func withoutAutomaticSelections(current []*careplan.BookedOffering, selections []careplan.OfferingChangeSelection) []careplan.OfferingChangeSelection {
	automatic := make(map[int64]bool, len(current))
	for _, link := range current {
		if link != nil && len(link.ManualSelectedDays) == 0 && len(link.AutomaticSelectedDays) > 0 {
			automatic[link.CareOfferingID] = true
		}
	}
	manual := make([]careplan.OfferingChangeSelection, 0, len(selections))
	for _, requested := range selections {
		if !automatic[requested.OfferingID] {
			manual = append(manual, requested)
		}
	}
	return manual
}

func sameMaterializedOfferingSelections(current []*careplan.BookedOffering, selections []careplan.OfferingSelection) bool {
	if len(current) != len(selections) {
		return false
	}
	byID := make(map[int64]*careplan.BookedOffering, len(current))
	for _, link := range current {
		if link == nil {
			return false
		}
		byID[link.CareOfferingID] = link
	}
	for _, selected := range selections {
		link := byID[selected.OfferingID]
		if link == nil || !slices.Equal(canonicalDays(link.SelectedDays), canonicalDays(selected.SelectedDays)) {
			return false
		}
	}
	return true
}

func heldOfferingIDs(links []*careplan.BookedOffering) map[int64]bool {
	held := make(map[int64]bool, len(links))
	for _, link := range links {
		if link != nil {
			held[link.CareOfferingID] = true
		}
	}
	return held
}

func heldOfferingCoversRange(links []*careplan.BookedOffering, offeringID int64, until calendar.Date) bool {
	for _, link := range links {
		if link != nil && link.CareOfferingID == offeringID && (link.ValidUntil == nil || !link.ValidUntil.Before(until)) {
			return true
		}
	}
	return false
}

func offeringChangeSelections(materialized []careplan.OfferingSelection) []careplan.OfferingChangeSelection {
	selections := make([]careplan.OfferingChangeSelection, 0, len(materialized))
	for _, selected := range materialized {
		selections = append(selections, careplan.OfferingChangeSelection{
			OfferingID: selected.OfferingID, SelectedDays: append([]string(nil), selected.SelectedDays...),
		})
	}
	return selections
}

func nativeSelections(values []careplan.OfferingSelection) []selection.Selection {
	result := make([]selection.Selection, 0, len(values))
	for _, value := range values {
		result = append(result, selection.Selection{
			OfferingID: value.OfferingID, SelectedDays: value.SelectedDays,
			ManualSelectedDays: value.ManualSelectedDays, AutomaticSelectedDays: value.AutomaticSelectedDays,
		})
	}
	return result
}
