package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// directSelectionFacts is an immutable bulk input, local to one alternatives
// projection, including Apply's revalidation under its existing locks. It is
// never persisted or attached to the request context.
type directSelectionFacts struct {
	allowed map[int64]*careplan.CareOffering
	current []*careplan.BookedOffering
	child   ports.OfferingChangeChild
}

// PreviewDirectOfferingAlternatives evaluates replacements for the same
// student and date in order. Scope and selection inputs are read once;
// each alternative still runs the selection engine and consequence guards.
func (s *OfferingChanges) PreviewDirectOfferingAlternatives(ctx context.Context, input careplan.DirectOfferingAdjustmentInput, today calendar.Date, alternatives [][]careplan.OfferingChangeSelection) ([]*careplan.DirectOfferingAdjustmentPreview, error) {
	result := make([]*careplan.DirectOfferingAdjustmentPreview, 0, len(alternatives))
	if len(alternatives) == 0 {
		return result, nil
	}
	scope, err := s.directAdjustmentScope(ctx, input, today)
	if err != nil {
		return nil, err
	}
	scope.selectionFacts, err = s.directSelectionFacts(ctx, scope, alternatives[0])
	if err != nil {
		return nil, err
	}
	for _, selections := range alternatives {
		candidate := input
		candidate.Selections = selections
		preview, err := s.previewDirectOfferingInScope(ctx, candidate, scope, false)
		if err != nil {
			return nil, err
		}
		result = append(result, preview)
	}
	return result, nil
}

func (s *OfferingChanges) directSelectionFacts(ctx context.Context, scope *directAdjustmentScope, first []careplan.OfferingChangeSelection) (*directSelectionFacts, error) {
	active, err := s.activeOfferings(ctx, scope.phase.ID)
	if err != nil {
		return nil, fmt.Errorf("offering change: list active offerings: %w", err)
	}
	// The resolved care period covers this date, so no pre-service upcoming
	// interval fallback is needed. Materialization never uses submission notes.
	values, err := s.deps.Enrollment.EffectiveSelectionsAt(ctx, map[int64]calendar.Date{
		scope.period.RequestChildID: scope.effectiveFrom,
	})
	if err != nil {
		return nil, fmt.Errorf("offering change: list current offerings: %w", err)
	}
	facts := &directSelectionFacts{allowed: offeringsByID(active)}
	facts.current, err = s.addHeldOfferings(ctx, bookedOfferingPointers(values), facts.allowed)
	if err != nil {
		return nil, err
	}
	// Preserve the scalar path's first validation before the child lookup.
	if _, _, err := normalizeOfferingSelections(first, facts.allowed); err != nil {
		return nil, err
	}
	children, err := s.deps.Enrollment.Children(ctx, []int64{scope.period.RequestChildID})
	if err != nil {
		return nil, fmt.Errorf("offering change: load request child: %w", err)
	}
	for _, child := range children {
		if child.ID == scope.period.RequestChildID {
			facts.child = child
			return facts, nil
		}
	}
	return nil, fmt.Errorf("offering change: load request child: %w", err)
}

func (s *OfferingChanges) selectionOfferings(ctx context.Context, in selectionMaterialization) (map[int64]*careplan.CareOffering, []*careplan.BookedOffering, error) {
	if in.facts != nil {
		return in.facts.allowed, in.facts.current, nil
	}
	active, err := s.activeOfferings(ctx, in.phase.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("offering change: list active offerings: %w", err)
	}
	allowed := offeringsByID(active)
	current, err := s.addHeldOfferingsAtDate(ctx, in.requestChildID, in.effectiveFrom, allowed)
	return allowed, current, err
}

func (s *OfferingChanges) selectionChildGrade(ctx context.Context, in selectionMaterialization, child *selectionChild) error {
	if in.facts != nil {
		child.TargetGradeLevel = in.facts.child.TargetGradeLevel
		return nil
	}
	return s.applyChildGrade(ctx, in.requestChildID, child)
}

func (s *OfferingChanges) directCurrentSelections(ctx context.Context, scope *directAdjustmentScope) ([]*careplan.BookedOffering, error) {
	if scope.selectionFacts != nil {
		return scope.selectionFacts.current, nil
	}
	return s.currentSelections(ctx, scope.period.RequestChildID, scope.effectiveFrom)
}
