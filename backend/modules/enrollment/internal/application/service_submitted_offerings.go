package application

import (
	"context"

	"fmt"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// RecordSubmittedOfferingChoices accepts an identical retry, but never replaces
// an existing choice. The entire batch participates in the ambient UnitOfWork.
func (m *Service) RecordSubmittedOfferingChoices(ctx context.Context, childID int64, choices []enrollment.SubmittedOfferingChoice) (err error) {
	started := time.Now()
	defer func() { m.observeOfferingStorage("record", "command", started, int64(len(choices)), -1, err) }()
	if childID <= 0 {
		return fmt.Errorf("request_child_id is required")
	}
	seen := make(map[int64]bool, len(choices))
	for _, choice := range choices {
		if choice.CareOfferingID <= 0 || seen[choice.CareOfferingID] {
			return fmt.Errorf("a unique care_offering_id is required for each submitted choice")
		}
		seen[choice.CareOfferingID] = true
		if choice.RequestChildID != 0 && choice.RequestChildID != childID {
			return fmt.Errorf("submitted offering choice request child mismatch")
		}
		for _, day := range choice.SelectedDays {
			if !slices.Contains([]string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}, day) {
				return fmt.Errorf("invalid submitted offering day %q", day)
			}
		}
	}
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.store.RecordSubmittedOfferingChoices(ctx, childID, choices)
}

func (m *Service) SubmittedOfferingChoices(ctx context.Context, childIDs []int64) (choices []enrollment.SubmittedOfferingChoice, err error) {
	started := time.Now()
	defer func() {
		m.observeOfferingStorage("choices", "query", started, int64(len(childIDs)), int64(len(choices)), err)
	}()
	if len(childIDs) == 0 {
		return []enrollment.SubmittedOfferingChoice{}, nil
	}
	if err = m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.SubmittedOfferingChoices(ctx, childIDs)
}
