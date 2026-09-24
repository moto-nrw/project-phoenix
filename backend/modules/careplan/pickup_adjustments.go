package careplan

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Pickup adjustments: a permanent change of a child's weekly pickup times is
// either a lasting exception to the booked offering or a switch to the
// offering that already carries those times. The preview names the
// deviation and the matching offerings; the apply re-checks the preview
// through its token.

// The two resolutions of a pickup adjustment.
const (
	PickupAdjustmentResolutionException = "exception"
	PickupAdjustmentResolutionOffering  = "offering"
)

// PickupAdjustmentSchedule is one proposed weekly pickup time.
type PickupAdjustmentSchedule struct {
	Weekday    int
	PickupTime string
	Notes      *string
}

// PickupAdjustmentArrivalSchedule is one proposed weekly arrival time.
type PickupAdjustmentArrivalSchedule struct {
	Weekday         int
	ExpectedArrival string
	Notes           *string
}

// PickupOfferingMatch is an offering whose pickup times match the proposed
// plan, with the complete selection that switches to it.
type PickupOfferingMatch struct {
	OfferingID   int64
	Name         string
	SelectedDays []string
	Selections   []OfferingChangeSelection
}

// PickupAdjustmentPreviewInput is a proposed permanent pickup plan.
type PickupAdjustmentPreviewInput struct {
	StudentID               int64
	Schedules               []PickupAdjustmentSchedule
	ArrivalSchedules        *[]PickupAdjustmentArrivalSchedule
	CareDays                []int
	EffectiveFrom           calendar.Date
	Selections              []OfferingChangeSelection
	ExcludedAutoOfferingIDs []int64
}

// PickupAdjustmentPreview is the non-writing projection of a pickup
// adjustment. PreviewToken binds the apply to exactly this projection.
type PickupAdjustmentPreview struct {
	PreviewToken         string
	EffectiveFrom        calendar.Date
	CurrentPlan          string
	ProposedPlan         string
	DeviatesFromOffering bool
	ResolutionRequired   bool
	MatchingOfferings    []PickupOfferingMatch
	OfferingCatalog      *OfferingChangeCatalog
	OfferingConsequences *OfferingChangePreview
	RemovedManualNotes   []PickupAdjustmentRemovedNote
}

// PickupAdjustmentRemovedNote is a note of a manual pickup row a switch to an
// offering removes.
type PickupAdjustmentRemovedNote struct {
	Weekday int
	Note    string
}

// PickupAdjustmentApplyInput applies a previewed pickup adjustment.
// Authorize re-checks the caller against the locked student.
type PickupAdjustmentApplyInput struct {
	PickupAdjustmentPreviewInput
	PreviewToken                string
	Resolution                  string
	Reason                      string
	ActorAccountID              int64
	ActorRole                   string
	CreatedByStaffID            int64
	CompleteWithdrawalConfirmed bool
	Authorize                   func(context.Context, ScheduleStudent) (bool, error)
}

// PickupAdjustmentResult names the resolution an apply took.
type PickupAdjustmentResult struct {
	Resolution string `json:"resolution"`
}

// PickupAdjustmentBulkInput is a lasting pickup exception for several
// children at once.
type PickupAdjustmentBulkInput struct {
	StudentIDs         []int64
	Schedules          []PickupScheduleInput
	ConfirmedException bool
	CreatedByStaffID   int64
	ActorAccountID     int64
	Authorize          func(context.Context, ScheduleStudent) (bool, error)
}

// PickupAdjustments previews and applies permanent pickup-time changes.
type PickupAdjustments interface {
	Preview(ctx context.Context, input PickupAdjustmentPreviewInput) (*PickupAdjustmentPreview, error)
	Apply(ctx context.Context, input PickupAdjustmentApplyInput) (*PickupAdjustmentResult, error)
	ApplyBulkExceptions(ctx context.Context, input PickupAdjustmentBulkInput) (*BulkUpsertResult, error)
}
