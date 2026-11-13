package enrollment

import (
	"context"
	"errors"
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
	// Terms are the catalog terms the parents submitted the choice under
	// (#6837); nil for a choice stored before or without a catalog row.
	Terms     *SubmittedOfferingTerms
	CreatedAt time.Time
}

// SubmittedOfferingTerms is the catalog entry of an offering as it read when
// the parents submitted and signed their choice (#6837). The catalog stays
// editable; the approval compares against this snapshot.
type SubmittedOfferingTerms struct {
	Name            string   `json:"name"`
	PriceCents      *int     `json:"price_cents"`
	DaysOfWeekMode  string   `json:"days_of_week_mode"`
	AvailableDays   []string `json:"available_days"`
	ActivityGroupID *int64   `json:"activity_group_id"`
}

// Changed terms of an offering since the submission, as the approval and the
// office's detail view name them.
const (
	OfferingTermPrice    = "price"
	OfferingTermDays     = "days"
	OfferingTermActivity = "activity"
)

// ChangedOfferingTerms names the booking-relevant terms in which current
// differs from the submitted snapshot: price, day mode or offered days, and
// the linked activity. A renamed offering books the same; the name is shown,
// not compared. Without a snapshot nothing can be compared.
func ChangedOfferingTerms(submitted *SubmittedOfferingTerms, current SubmittedOfferingTerms) []string {
	if submitted == nil {
		return nil
	}
	var changed []string
	if !sameOptionalValue(submitted.PriceCents, current.PriceCents) {
		changed = append(changed, OfferingTermPrice)
	}
	if submitted.DaysOfWeekMode != current.DaysOfWeekMode || !slices.Equal(sortedDays(submitted.AvailableDays), sortedDays(current.AvailableDays)) {
		changed = append(changed, OfferingTermDays)
	}
	if !sameOptionalValue(submitted.ActivityGroupID, current.ActivityGroupID) {
		changed = append(changed, OfferingTermActivity)
	}
	return changed
}

func sameOptionalValue[T comparable](left, right *T) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func sortedDays(days []string) []string {
	return slices.Sorted(slices.Values(days))
}

type SubmittedOfferingQueries interface {
	SubmittedOfferingChoices(context.Context, []int64) ([]SubmittedOfferingChoice, error)
}

type SubmittedOfferingCommands interface {
	RecordSubmittedOfferingChoices(context.Context, int64, []SubmittedOfferingChoice) error
}
