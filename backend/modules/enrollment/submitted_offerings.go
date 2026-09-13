package enrollment

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"
)

var ErrSubmittedOfferingChoiceConflict = errors.New("submitted offering choice is immutable")

// SubmittedOfferingChoice is the submitted choice, not the effective care plan.
// Booking adjustments cannot change its days or submission notes.
type SubmittedOfferingChoice struct {
	ID             int64
	TenantID       int64
	RequestChildID int64
	CareOfferingID int64
	SelectedDays   []string
	Notes          *string
	CreatedAt      time.Time
}

type SubmittedOfferingQueries interface {
	SubmittedOfferingChoices(context.Context, []int64) ([]SubmittedOfferingChoice, error)
}

type SubmittedOfferingCommands interface {
	RecordSubmittedOfferingChoices(context.Context, int64, []SubmittedOfferingChoice) error
}

// RecordSubmittedOfferingChoices accepts an identical retry, but never replaces
// an existing choice. The entire batch participates in the ambient UnitOfWork.
func (m *Module) RecordSubmittedOfferingChoices(ctx context.Context, childID int64, choices []SubmittedOfferingChoice) (err error) {
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
	return m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		return m.engine.RecordSubmittedOfferingChoices(ctx, childID, choices)
	})
}

func (m *Module) SubmittedOfferingChoices(ctx context.Context, childIDs []int64) (choices []SubmittedOfferingChoice, err error) {
	started := time.Now()
	defer func() {
		m.observeOfferingStorage("choices", "query", started, int64(len(childIDs)), int64(len(choices)), err)
	}()
	if len(childIDs) == 0 {
		return []SubmittedOfferingChoice{}, nil
	}
	err = m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		choices, err = m.engine.SubmittedOfferingChoices(ctx, childIDs)
		return err
	})
	return choices, err
}
