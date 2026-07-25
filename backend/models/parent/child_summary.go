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

	"github.com/moto-nrw/project-phoenix/internal/timezone"
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
	Status        string         `json:"status"`
	EnrolledFrom  *timezone.Date `json:"enrolled_from,omitempty"`
	EnrolledUntil *timezone.Date `json:"enrolled_until,omitempty"`

	// School context — repeated on every child of the same school so
	// the frontend can render grouped cards without an extra fetch.
	SchoolName string `json:"school_name"`
	SchoolSlug string `json:"school_slug"`

	// GuardianPermissions is the matching users.students_guardians.permissions
	// map for the calling account's guardian relationship. It is internal to the
	// parent service; handlers must not serialize it.
	GuardianPermissions map[string]interface{} `json:"-"`
	GuardianProfileID   int64                  `json:"-"`
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
	// name, then last name. Soft-deleted persons and graduated
	// (alumnus) students are filtered out.
	ListByAccount(ctx context.Context, accountID int64) ([]*ChildSummary, error)

	// FindForAccount resolves a single child the account is a guardian
	// of, returning the same projection (crucially including TenantID so
	// callers can scope a follow-up tenant transaction). Returns nil,
	// nil when the student is not linked to the account — the caller MUST
	// treat that as a 403/404, never trusting the studentID alone. This
	// is the authorization gate for every per-child parent write, so
	// graduated (alumnus) children resolve to nil here too: a departed
	// child is neither visible nor writable in the portal.
	FindForAccount(ctx context.Context, accountID, studentID int64) (*ChildSummary, error)
}

// EnrollablePhase is one row in the parent's enrollment picker —
// (school, open phase) pair. A school with multiple open phases shows
// up multiple times; the frontend groups by SchoolID.
//
// "Open" here means: phase.is_active AND
//
//	(enrollment_open_at IS NULL OR enrollment_open_at <= now) AND
//	(enrollment_close_at IS NULL OR enrollment_close_at >= now)
//
// AlreadyLinked is true when the parent's account already has an
// active mapping to this school (i.e. they already have at least one
// child here). The frontend uses it to sort linked schools first and
// label them differently from "new" schools.
type EnrollablePhase struct {
	SchoolID          int64         `json:"school_id"`
	SchoolName        string        `json:"school_name"`
	SchoolSlug        string        `json:"school_slug"`
	PhaseID           int64         `json:"phase_id"`
	PhaseName         string        `json:"phase_name"`
	PhaseKind         string        `json:"phase_kind"`
	ServiceStartDate  timezone.Date `json:"service_start_date"`
	ServiceEndDate    timezone.Date `json:"service_end_date"`
	EnrollmentOpenAt  *time.Time    `json:"enrollment_open_at,omitempty"`
	EnrollmentCloseAt *time.Time    `json:"enrollment_close_at,omitempty"`
	AlreadyLinked     bool          `json:"already_linked"`
}

// EnrollablePhaseRepository is the cross-tenant lookup for the parent
// enrollment picker. Same admin-tx requirement as ChildRepository.
type EnrollablePhaseRepository interface {
	// ListEnrollable returns every (school, active+open phase) pair,
	// flagged with whether the given account already has a tenant
	// mapping to that school. Sorted by AlreadyLinked DESC (linked
	// schools first), then school name, then phase service start.
	ListEnrollable(ctx context.Context, accountID int64) ([]*EnrollablePhase, error)
}

// EnrollmentRequestSummary is the cross-tenant view of one
// enrollment.requests row owned by a guardian account, plus the
// minimal child + phase + school context the parent dashboard needs to
// render a status row. Multiple children produce multiple rows in
// Children — the request itself is one logical card.
type EnrollmentRequestSummary struct {
	RequestID   int64      `json:"request_id"`
	TenantID    int64      `json:"tenant_id"`
	StatusToken string     `json:"status_token"`
	SubmittedAt time.Time  `json:"submitted_at"`
	WithdrawnAt *time.Time `json:"withdrawn_at,omitempty"`

	PhaseID          int64         `json:"phase_id"`
	PhaseName        string        `json:"phase_name"`
	ServiceStartDate timezone.Date `json:"service_start_date"`
	ServiceEndDate   timezone.Date `json:"service_end_date"`

	// ShowStatusReasonToParent mirrors the owning phase flag. Internal
	// plumbing only (json:"-"): the parent service uses it to redact
	// admin-internal child status reasons before returning, and the API
	// handler maps an explicit response struct, so it never reaches the
	// wire. See services/parent ListEnrollmentsForAccount.
	ShowStatusReasonToParent bool `json:"-"`

	SchoolName string `json:"school_name"`
	SchoolSlug string `json:"school_slug"`

	Children []EnrollmentRequestChildSummary `json:"children"`
}

// EnrollmentRequestChildSummary is a slim view of one
// enrollment.request_children row — just enough for the dashboard to
// show per-child status badges next to the request.
type EnrollmentRequestChildSummary struct {
	ChildID      int64   `json:"child_id"`
	FirstName    string  `json:"first_name"`
	LastName     string  `json:"last_name"`
	Status       string  `json:"status"`
	StatusReason *string `json:"status_reason,omitempty"`
}

// EnrollmentRequestRepository lists every enrollment request a parent
// account owns. Same admin-tx + JWT-validation contract as
// ChildRepository / EnrollablePhaseRepository.
type EnrollmentRequestRepository interface {
	// ListByAccount returns every enrollment.requests row where
	// guardian_account_id matches the given account, newest first,
	// joined to phase + school + child rows. Cross-tenant — the
	// implementation MUST run under WithAdminTx.
	ListByAccount(ctx context.Context, accountID int64) ([]*EnrollmentRequestSummary, error)

	// BackfillGuardianAccountID stamps the given accountID onto every
	// enrollment.requests row with guardian_account_id IS NULL and a
	// case-insensitive match on guardian_email. Returns how many rows
	// were updated. Cross-tenant — implementation MUST run under
	// WithAdminTx. Called after a guardian invitation accept so
	// pre-account submissions surface in /me/enrollments.
	BackfillGuardianAccountID(ctx context.Context, accountID int64, email string) (int, error)
}
