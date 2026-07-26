package enrollment

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// RequestGuardian is a row in enrollment.request_guardians - one row per
// ADDITIONAL guardian (Erziehungsberechtigte:r) included in a parent
// submission. The primary guardian stays flattened on the parent
// enrollment.requests row; these are the co-guardians the parent added
// via the "weitere:n Erziehungsberechtigte:n hinzufügen" button.
//
// Email and Phone are optional: a co-guardian may be a contact-only
// record (e.g. a grandparent without email). Only the primary guardian
// requires an email (status link + dedup hang off it).
//
// GuardianProfileID is NULL until the request's first child is approved.
// On approval the decision service resolves each co-guardian to a
// users.guardian_profiles row and stamps the id back here, so the next
// child of the same request reuses that profile instead of creating a
// duplicate (children are approved one at a time, and an email-less
// co-guardian cannot be deduped by email).
type RequestGuardian struct {
	base.Model `bun:"schema:enrollment,table:request_guardians"`
	base.TenantModel
	RequestID         int64   `bun:"request_id,notnull" json:"request_id"`
	FirstName         string  `bun:"first_name,notnull" json:"first_name"`
	LastName          string  `bun:"last_name,notnull" json:"last_name"`
	Email             *string `bun:"email" json:"email,omitempty"`
	Phone             *string `bun:"phone" json:"phone,omitempty"`
	GuardianProfileID *int64  `bun:"guardian_profile_id" json:"guardian_profile_id,omitempty"`
	SortOrder         int     `bun:"sort_order,notnull,default:0" json:"sort_order"`
}

// RequestGuardianRepository describes the DB operations the submit flow
// (Create) and the approval flow (ListByRequestID + StampResolvedProfile)
// need. Tenant-scoped via RLS.
type RequestGuardianRepository interface {
	Create(ctx context.Context, guardian *RequestGuardian) error
	ListByRequestID(ctx context.Context, requestID int64) ([]*RequestGuardian, error)
	DeleteByRequestID(ctx context.Context, requestID int64) error

	// ListByRequestIDs returns every co-guardian across the given requests
	// in a single query, so the phase export can attach co-guardians
	// without one query per request. Tenant-scoped via RLS.
	ListByRequestIDs(ctx context.Context, requestIDs []int64) ([]*RequestGuardian, error)

	// StampResolvedProfile back-links a co-guardian row to the
	// users.guardian_profiles row it resolved to on the first child
	// approval, so later approvals of the same request reuse it.
	StampResolvedProfile(ctx context.Context, id, guardianProfileID int64) error
}
