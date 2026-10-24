package ports

import (
	"context"
	"errors"

	"time"
)

// ErrGuardianEmailInUse marks a guardian e-mail that another profile of the
// school already uses, so the HTTP layer can answer with its own code (#2517).
var ErrGuardianEmailInUse = errors.New("guardian e-mail is already in use")

// GuardianInvitationRecord is one invitation as the guardian list reads it
// from the owner module (#2722).
type GuardianInvitationRecord struct {
	ID                int64
	TenantID          int64
	Token             string
	GuardianProfileID int64
	CreatedBy         int64
	ExpiresAt         time.Time
	AcceptedAt        *time.Time
	EmailSentAt       *time.Time
	EmailError        *string
	StudentID         *int64
	ApprovalStatus    string
	CreatedAt         time.Time
}

// GuardianInvitations is the consumer-owned port over the Identity & Access
// guardian invitation capability (#2722): the guardian list shows who has an
// open invitation, and the invitation overview lists the redeemable ones.
// The composition root binds it; auth.guardian_invitations belongs to the
// owner module.
type GuardianInvitations interface {
	// ListOpen returns the invitations of those contacts that are still
	// going somewhere: redeemable, or awaiting a staff decision.
	ListOpen(ctx context.Context, guardianProfileIDs []int64) ([]GuardianInvitationRecord, error)
	// ListRedeemable returns the invitations of the school in context whose
	// link can still be spent.
	ListRedeemable(ctx context.Context) ([]GuardianInvitationRecord, error)
}

// GuardianServiceDependencies contains all dependencies required by the guardian service
type GuardianServiceDependencies struct {
	// Repository dependencies
	GuardianProfileRepo     GuardianProfileRepository
	GuardianPhoneNumberRepo GuardianPhoneNumberRepository
	StudentGuardianRepo     StudentGuardianRepository
	GuardianInvitations     GuardianInvitations
	StudentRepo             StudentRepository
	PersonRepo              PersonRepository

	// Payment data (#2608). Kept as separate, narrow dependencies so the
	// guardians:financial code path is the only one holding a reference to
	// the bank rows. Both audit writers are mandatory for that path: it
	// refuses to serve or change an IBAN when it cannot write the trail.
	GuardianFinancialRepo  GuardianFinancialDataRepository
	GuardianFinancialAudit GuardianFinancialAudit
	DataAccessLog          GuardianDataAccessLog

	// RolePresets assigns a relationship's role and the parent-portal
	// permissions it grants.
	RolePresets GuardianRolePresets
}

// GuardianTransactions runs the guardian writes in the request's tenant
// transaction and names the caller's tenant. The composition binds it to the
// tenant runtime and the database.
type GuardianTransactions interface {
	// RunInTx joins the caller's transaction or opens one.
	RunInTx(ctx context.Context, fn func(context.Context) error) error
	// WithTenantTx runs fn in a transaction of the caller's tenant.
	WithTenantTx(ctx context.Context, fn func(context.Context) error) error
	// TenantID is the caller's tenant.
	TenantID(ctx context.Context) int64
}

// GuardianDisplay is the People Directory projection used outside this domain.
type GuardianDisplay struct {
	GuardianProfileID int64
	FirstName         string
	LastName          string
}
