// Package parent holds the cross-tenant guardian-portal model types.
//
// Unlike the per-tenant domains (users, education, activities), parent
// types intentionally fan out across the tenant boundary — a single
// parent account can have students at multiple schools, and the
// portal lists them together. The repos in this domain therefore run
// inside admin transactions (BYPASSRLS) and validate the
// account-to-tenant scope explicitly via auth.account_tenants.
package parent

import (
	"context"
	"time"
)

// ChildSummary is the cross-tenant view of one student linked to a
// guardian's account. Carries enough school context for the parent
// dashboard to group + label without a second fetch.
type ChildSummary struct {
	// Student identity (per-tenant id; the (TenantID, StudentID) pair
	// is globally unique).
	StudentID   int64  `json:"student_id"`
	TenantID    int64  `json:"tenant_id"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	SchoolClass string `json:"school_class,omitempty"`

	// Lifecycle status as set by the activate-students scheduler:
	// pending → active → inactive (or alumnus).
	Status        string     `json:"status"`
	EnrolledFrom  *time.Time `json:"enrolled_from,omitempty"`
	EnrolledUntil *time.Time `json:"enrolled_until,omitempty"`

	// School context — repeated on every child of the same school so
	// the frontend can render grouped cards without an extra fetch.
	SchoolName string `json:"school_name"`
	SchoolSlug string `json:"school_slug"`
}

// ChildRepository is the contract for the cross-tenant lookup the
// parent portal needs. There's only one method today; future
// endpoints (per-child detail, attendance summary, etc.) get added
// here as the portal grows.
//
// Implementations MUST run inside a phoenix_admin transaction so the
// query crosses tenant_id boundaries safely. The caller is responsible
// for verifying the account is allowed to see the result — typically
// by reading claims.ID from a parent-scope JWT.
type ChildRepository interface {
	// ListByAccount returns every student linked to a guardian profile
	// owned by the given account, across every active tenant mapping
	// for that account. Sorted by school name, then student first
	// name, then last name. Soft-deleted persons are filtered out.
	ListByAccount(ctx context.Context, accountID int64) ([]*ChildSummary, error)
}
