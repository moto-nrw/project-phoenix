package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// SchoolInvitationStore is the persistence port over auth.invitation_tokens
// and the account rows an acceptance creates: the account itself and its
// school mapping. Every statement runs on the connection the caller's
// context carries; the acceptance holds the administrative transaction.
type SchoolInvitationStore interface {
	InsertSchoolInvitation(ctx context.Context, invitation domain.SchoolInvitation) (domain.SchoolInvitation, domain.OperationStats, error)
	FindSchoolInvitation(ctx context.Context, id int64) (domain.SchoolInvitation, bool, domain.OperationStats, error)
	FindSchoolInvitationByToken(ctx context.Context, token string) (domain.SchoolInvitation, bool, domain.OperationStats, error)
	// ListRedeemableSchoolInvitations returns the pending invitations of the
	// tenant in context.
	ListRedeemableSchoolInvitations(ctx context.Context, now time.Time) ([]domain.SchoolInvitation, domain.OperationStats, error)
	// SpendSchoolInvitation spends the invitation and reports whether it was
	// still unused. Both redeeming and revoking spend it; only the reason
	// differs.
	SpendSchoolInvitation(ctx context.Context, id int64) (bool, domain.OperationStats, error)
	// RevokeSchoolInvitationsForEmail spends every unused invitation for the
	// address in the caller's tenant scope.
	RevokeSchoolInvitationsForEmail(ctx context.Context, email string) (int, domain.OperationStats, error)
	RevokeSchoolInvitationsForTenant(ctx context.Context, tenantID int64) (int, domain.OperationStats, error)
	ExtendSchoolInvitation(ctx context.Context, id int64, expiresAt, now time.Time) (bool, domain.OperationStats, error)
	RecordSchoolInvitationDelivery(ctx context.Context, id int64, delivery domain.TokenDelivery) (domain.OperationStats, error)
	DeleteExpiredSchoolInvitations(ctx context.Context, now time.Time) (int, domain.OperationStats, error)

	AccountProvisioning

	// EnsureAccountTenant activates the account's mapping to the school,
	// reactivating a deactivated one so a re-invitation after offboarding
	// works.
	EnsureAccountTenant(ctx context.Context, accountID, tenantID int64) (domain.OperationStats, error)
}

// AccountProvisioning creates the account an accepted invitation gives
// access to. Both invitation flows write the same auth.accounts row.
type AccountProvisioning interface {
	// InsertAccount creates the invited account with its credential and
	// returns it.
	InsertAccount(ctx context.Context, email, passwordHash string) (domain.LoginAccount, domain.OperationStats, error)
}

// InvitationGrantPolicy answers whether the inviting account may hand out
// the role; Security Runtime owns the decision.
type InvitationGrantPolicy interface {
	CanGrantRole(role domain.RoleFacts, permissions []string, rolePermissions []string) bool
}

// InvitationOwnerTokens verifies that the caller holds a live session of the
// account an invitation was addressed to.
type InvitationOwnerTokens interface {
	// AccountOfAccessToken returns the account id a usable access token
	// proves. A token that is expired, read-only, a preview, an unfinished
	// MFA challenge or of an operator scope proves nothing (0, nil).
	AccountOfAccessToken(token string) (int64, error)
}

// SchoolInvitationDelivery mails an invitation. The module decides the
// portal; the root resolves its host, the school name and the reply-to
// identity, and records the outcome back through the module.
type SchoolInvitationDelivery interface {
	DispatchSchoolInvitation(ctx context.Context, invitation domain.SchoolInvitation, schoolName string, portal domain.InvitationPortal, expiry time.Duration)
}
