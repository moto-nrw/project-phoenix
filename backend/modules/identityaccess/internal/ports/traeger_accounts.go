package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// TraegerAccountStore persists the Träger-Büro account management (#4650):
// the invitations, the role set of a member and the member's Träger
// sessions. Every method runs on the caller's administrative transaction.
type TraegerAccountStore interface {
	TraegerRoleStore
	// InsertTraegerInvitation stores the hash of invitation.Token and returns
	// the invitation with the raw token.
	InsertTraegerInvitation(ctx context.Context, invitation domain.TraegerInvitation) (domain.TraegerInvitation, domain.OperationStats, error)
	// RevokeOpenTraegerInvitationsForEmail takes the address lock of the
	// Träger until the caller's transaction ends, then spends every open
	// invitation of the address there.
	RevokeOpenTraegerInvitationsForEmail(ctx context.Context, organizationID int64, email string, at time.Time) (domain.OperationStats, error)
	// RevokeTraegerInvitation spends one open invitation of the Träger and
	// returns it; found is false for an unknown, foreign or spent one.
	RevokeTraegerInvitation(ctx context.Context, organizationID, id int64, at time.Time) (domain.TraegerInvitation, bool, domain.OperationStats, error)
	// ListOpenTraegerInvitations returns the open, unexpired invitations of
	// the Träger, newest first.
	ListOpenTraegerInvitations(ctx context.Context, organizationID int64, now time.Time) ([]domain.TraegerInvitation, domain.OperationStats, error)
	// FindTraegerInvitationByToken reads the invitation behind a raw token;
	// forUpdate locks it until the caller's transaction ends.
	FindTraegerInvitationByToken(ctx context.Context, token string, forUpdate bool) (domain.TraegerInvitation, bool, domain.OperationStats, error)
	// SpendTraegerInvitation marks an open invitation used and reports
	// whether it was open.
	SpendTraegerInvitation(ctx context.Context, id int64, at time.Time) (bool, domain.OperationStats, error)
	// ReplaceTraegerRoles makes roles the account's exact role set at the
	// Träger; an empty set removes every role.
	ReplaceTraegerRoles(ctx context.Context, organizationID, accountID int64, roles []string) (domain.OperationStats, error)
	// LockTraegerAdministrators locks the Verwaltung role rows of the Träger
	// whose accounts can sign in (active, not erased) and returns those
	// accounts: every change that could remove the last one queues behind
	// the others.
	LockTraegerAdministrators(ctx context.Context, organizationID int64) ([]int64, domain.OperationStats, error)
	// DeleteTraegerAccountSessions ends every session of the account at the Träger.
	DeleteTraegerAccountSessions(ctx context.Context, organizationID, accountID int64) (domain.OperationStats, error)
	FindLoginAccountByEmail(ctx context.Context, email string) (domain.LoginAccount, bool, domain.OperationStats, error)
	// FindLoginAccount reads a live account; the acting person's account
	// must still be switched on.
	FindLoginAccount(ctx context.Context, id int64, forUpdate bool) (domain.LoginAccount, bool, domain.OperationStats, error)
	// LockLoginAccountByEmail is the address lookup under the row lock that
	// serializes with account retention.
	LockLoginAccountByEmail(ctx context.Context, email string) (domain.LoginAccount, bool, domain.OperationStats, error)
	InsertAccount(ctx context.Context, email, passwordHash string) (domain.LoginAccount, domain.OperationStats, error)
}
