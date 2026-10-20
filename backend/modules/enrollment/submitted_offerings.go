package enrollment

import (
	"context"
	"errors"

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
