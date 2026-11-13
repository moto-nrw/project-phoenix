package application

import (
	"context"
	"fmt"

	"github.com/moto-ogs/project-phoenix/modules/careplan"
	"github.com/moto-ogs/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// decisionSelectionFacts supplies one read-only decision projection. The
// confirmed date is at or after the phase start, so selections need neither
// the pre-service fallback nor the parents' submission notes.
func (s *OfferingChanges) decisionSelectionFacts(ctx context.Context, in selectionMaterialization, child *ports.OfferingChangeChild) (*directSelectionFacts, error) {
	active, err := s.activeOfferings(ctx, in.phase.ID)
	if err != nil {
		return nil, fmt.Errorf("offering change: list active offerings: %w", err)
	}
	values, err := s.deps.Enrollment.EffectiveSelectionsAt(ctx, map[int64]calendar.Date{in.requestChildID: in.effectiveFrom})
	if err != nil {
		return nil, fmt.Errorf("offering change: list current offerings: %w", err)
	}
	facts := &directSelectionFacts{allowed: offeringsByID(active), child: *child}
	facts.current, err = s.addHeldOfferings(ctx, bookedOfferingPointers(values), facts.allowed)
	return facts, err
}

// decisionDiffOfferings keeps the diff's original ID scope and order. Other
// callers can still build a diff without preloaded selection facts.
func (s *OfferingChanges) decisionDiffOfferings(ctx context.Context, ids []int64, known map[int64]*careplan.CareOffering) ([]careplan.CareOffering, error) {
	if known == nil {
		return s.offeringsByIDs(ctx, ids)
	}
	values := make([]careplan.CareOffering, 0, len(ids))
	for _, id := range ids {
		if offering := known[id]; offering != nil {
			values = append(values, *offering)
		}
	}
	return values, nil
}

func (s *OfferingChanges) capacityCurrentSelections(ctx context.Context, in selectionMaterialization) ([]*careplan.BookedOffering, error) {
	if in.facts != nil {
		return in.facts.current, nil
	}
	return s.currentSelections(ctx, in.requestChildID, in.effectiveFrom)
}

func selectionFactsChild(facts *directSelectionFacts) *ports.OfferingChangeChild {
	if facts == nil {
		return nil
	}
	return &facts.child
}
