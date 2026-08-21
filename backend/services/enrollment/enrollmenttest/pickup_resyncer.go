// Package enrollmenttest contains small test doubles shared by enrollment
// service tests.
package enrollmenttest

import "context"

// PickupResyncer records offering projection refreshes.
type PickupResyncer struct {
	OfferingIDs []int64
	Err         error
}

func (r *PickupResyncer) ReconcileOfferingPickupForOffering(_ context.Context, offeringID int64) error {
	r.OfferingIDs = append(r.OfferingIDs, offeringID)
	return r.Err
}
