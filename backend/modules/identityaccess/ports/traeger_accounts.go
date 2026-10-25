package ports

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// TraegerMembership is an active Träger membership as Organisation & Tenancy
// reports it to the Träger-Büro account management.
type TraegerMembership struct {
	AccountID   int64
	ActivatedAt *time.Time
}

// TraegerBueroMemberships are the Organisation & Tenancy membership facts
// and commands the Träger-Büro account management works with (ADR 0084). The
// membership commands participate in the administrative transaction the
// account management opens (ADR 0067, command participation); every call
// joins it.
type TraegerBueroMemberships interface {
	// FindLiveTraeger reports the Träger when it is active and undeleted.
	FindLiveTraeger(ctx context.Context, organizationID int64) (identityaccess.TraegerOrganization, bool, error)
	// ListActiveTraegerMemberships returns the active memberships of the
	// Träger.
	ListActiveTraegerMemberships(ctx context.Context, organizationID int64) ([]TraegerMembership, error)
	// ActivateTraegerMembership creates or reactivates the account's
	// membership at an active Träger.
	ActivateTraegerMembership(ctx context.Context, organizationID, accountID int64) error
	// DeactivateTraegerMembership ends an active membership; an inactive
	// one stays as it is.
	DeactivateTraegerMembership(ctx context.Context, organizationID, accountID int64) error
}

// TraegerInvitationDelivery queues the invitation mail in the caller's
// transaction: the invitation and its mail commit together.
type TraegerInvitationDelivery interface {
	DispatchTraegerInvitation(ctx context.Context, invitation identityaccess.TraegerInvitation, traegerName string, expiry time.Duration) error
}

// TraegerAccountAudit appends to the Audit Platform's Träger account ledger
// in the caller's transaction; a failed append aborts the change.
type TraegerAccountAudit interface {
	RecordTraegerAccountChange(ctx context.Context, change identityaccess.TraegerAccountChange) error
}

// TraegerAccountDependencies compose the Träger-Büro account management
// (#4650). It requires the session dependencies: their password verifier
// checks an existing account at acceptance.
type TraegerAccountDependencies struct {
	Memberships TraegerBueroMemberships
	Delivery    TraegerInvitationDelivery
	Audit       TraegerAccountAudit
	Passwords   PasswordPolicy
	// Expiry is the invitation link lifetime.
	Expiry time.Duration
	Logger *slog.Logger
}
