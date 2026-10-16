package carerequests

import "context"

// ParentCareRequestQueries reads only the identity needed for parent sharing.
// The caller still checks the relationship and request ownership.
type ParentCareRequestQueries interface {
	CareRequestIdentity(context.Context, int64) (*ParentCareRequestIdentity, error)
}

// ParentCareRequestIdentity identifies the submitting parent, child and request kind.
type ParentCareRequestIdentity struct {
	SubmittedBy int64
	StudentID   int64
	RequestKind string
}
