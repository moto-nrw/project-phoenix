package application

import (
	"context"

	offeringContract "github.com/moto-nrw/project-phoenix/modules/enrollment/offeringcontract"
)

// CareOfferingRepository is the DB contract used by the service layer.
type CareOfferingRepository interface {
	Create(ctx context.Context, offering *offeringContract.CareOfferingRecord) error
	FindByID(ctx context.Context, id int64) (*offeringContract.CareOfferingRecord, error)
	// ListByIDsForUpdate locks the referenced offerings in ascending id order.
	// Capacity checks use this to serialize competing approvals.
	ListByIDsForUpdate(ctx context.Context, ids []int64) ([]*offeringContract.CareOfferingRecord, error)
	Update(ctx context.Context, offering *offeringContract.CareOfferingRecord) error
	Delete(ctx context.Context, id int64) error
	ReplaceAutoAddTriggers(ctx context.Context, targetOfferingID int64, triggerOfferingIDs []int64) error

	// ListByTenant returns every care offering for the tenant in
	// context, sorted by sort_order. Admin endpoint uses this.
	ListByTenant(ctx context.Context) ([]*offeringContract.CareOfferingRecord, error)

	// ListByPhase returns offerings filtered to a single phase -
	// admin pages pivot on the phase dropdown, and the public-form
	// endpoint uses it after resolving the parent's selected phase.
	ListByPhase(ctx context.Context, phaseID int64) ([]*offeringContract.CareOfferingRecord, error)

	// ListActiveByPhase returns is_active=true offerings for a phase.
	// Used by the parent-facing endpoint; the phase's own enrollment
	// window gates the surrounding flow, so individual offerings
	// don't need their own time filter.
	ListActiveByPhase(ctx context.Context, phaseID int64) ([]*offeringContract.CareOfferingRecord, error)
	// ListActiveByPhaseIDs is the batched variant for queue projections.
	ListActiveByPhaseIDs(ctx context.Context, phaseIDs []int64) ([]*offeringContract.CareOfferingRecord, error)

	// ListByIDs returns the exact care offerings referenced by ids,
	// regardless of phase. Empty input returns an empty slice.
	ListByIDs(ctx context.Context, ids []int64) ([]*offeringContract.CareOfferingRecord, error)

	// ListByActivityGroupIDs returns offerings whose timetable link points at
	// one of the supplied groups. Split validation uses it to find every care
	// offering attached anywhere in one recurring-template series.
	ListByActivityGroupIDs(ctx context.Context, activityGroupIDs []int64) ([]*offeringContract.CareOfferingRecord, error)

	// CountByPhaseID returns how many care offerings belong to the phase.
	// Powers the phase-delete confirmation modal.
	CountByPhaseID(ctx context.Context, phaseID int64) (int, error)
}
