package organizationtenancy

import (
	"context"
	"fmt"
)

// DefaultChildQuotaBundleSize is the bundle size of a normal contract.
const DefaultChildQuotaBundleSize = 50

// Bounds of the contract model; they keep the Kinderkontingent a plausible
// contract value rather than a typo.
const (
	MaxChildQuotaBundles    = 1000
	MaxChildQuotaBundleSize = 1000
)

// ChildQuota is the Kinderkontingent of one school (#3567): the contracted
// maximum number of children, booked in whole bundles. The Kontingentzahl it
// limits is counted by School Membership.
type ChildQuota struct {
	Bundles    int
	BundleSize int
}

// Limit is the number of children the contract allows.
func (q ChildQuota) Limit() int { return q.Bundles * q.BundleSize }

// Validate rejects a quota outside the contract model.
func (q ChildQuota) Validate() error {
	if q.Bundles < 1 || q.Bundles > MaxChildQuotaBundles {
		return invalidSchool(fmt.Sprintf("child quota bundles must be between 1 and %d", MaxChildQuotaBundles))
	}
	if q.BundleSize < 1 || q.BundleSize > MaxChildQuotaBundleSize {
		return invalidSchool(fmt.Sprintf("child quota bundle size must be between 1 and %d", MaxChildQuotaBundleSize))
	}
	return nil
}

// ChildQuotaLimits is the named tenant-safe read of the Kinderkontingent
// (#3567). It answers only for the school of the tenant in context, on the
// caller's transaction, so a membership write sees the same contract value it
// is checked against. Limited is false when the school has no Kinderkontingent.
type ChildQuotaLimits interface {
	ChildQuotaLimit(ctx context.Context) (limit int, limited bool, err error)
}
