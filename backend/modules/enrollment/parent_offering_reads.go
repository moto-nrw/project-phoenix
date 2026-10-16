package enrollment

import "context"

// ParentCareOfferingQueries exposes the catalog values behind booked care,
// preserving the retained reader's decoding and error contract.
type ParentCareOfferingQueries interface {
	CareOfferingsByIDs(context.Context, []int64) ([]*CareOffering, error)
}

// ParentOfferingChangeQueries identifies the child and submitting account of
// an offering change. The caller must still authorize the relationship.
type ParentOfferingChangeQueries interface {
	OfferingChangeIdentity(context.Context, int64) (*OfferingChangeIdentity, error)
}

type OfferingChangeIdentity struct {
	StudentID   int64
	SubmittedBy int64
}

// CareOfferingDaysFixed is the catalog mode that books its available days
// rather than a guardian's selected days.
const CareOfferingDaysFixed = "fixed"
