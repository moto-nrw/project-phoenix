package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// CatererInvitationDelivery mails the invitation; the link exists only in
// the mail.
type CatererInvitationDelivery interface {
	EnqueueCatererInvitation(ctx context.Context, invitationID, schoolID int64, email, catererName, token string, expiresAt time.Time) error
}

// CatererStore holds the caterer, its accounts, the schools' supplies and
// the schools' invitations (ADR 0077). Writes run in the school's
// transaction; a caterer's schools are read through the global identity
// read, filtered on that caterer.
type CatererStore interface {
	FindCatererOfAccount(ctx context.Context, accountID int64) (domain.Caterer, bool, domain.OperationStats, error)
	// CreateInvitedCaterer creates the caterer of an open invitation of the
	// school in scope, for the account of the invited address.
	CreateInvitedCaterer(ctx context.Context, invitationID, accountID int64) (int64, domain.OperationStats, error)
	InsertCatererInvitation(ctx context.Context, invitation domain.CatererInvitation) (domain.CatererInvitation, domain.OperationStats, error)
	// ListCatererInvitations lists the school's invitations, newest first.
	ListCatererInvitations(ctx context.Context) ([]domain.CatererInvitation, domain.OperationStats, error)
	// RevokeCatererInvitation takes back an open invitation of the school.
	RevokeCatererInvitation(ctx context.Context, invitationID int64, at time.Time) (bool, domain.OperationStats, error)
	// FindCatererInvitationByTokenHash locks the school's invitation for its
	// acceptance.
	FindCatererInvitationByTokenHash(ctx context.Context, tokenHash string) (domain.CatererInvitation, bool, domain.OperationStats, error)
	AcceptCatererInvitation(ctx context.Context, invitationID, accountID int64, at time.Time) (bool, domain.OperationStats, error)
	// InsertCatererSchool starts the school's supply by the caterer;
	// EndCatererSchool ends the school's running supply.
	InsertCatererSchool(ctx context.Context, catererID int64, deliveryAddress string, startedBy int64, at time.Time) (domain.OperationStats, error)
	EndCatererSchool(ctx context.Context, endedBy int64, at time.Time) (bool, domain.OperationStats, error)
	// FindSchoolCaterer reads the running supply of the school in scope.
	FindSchoolCaterer(ctx context.Context) (domain.CatererSupply, bool, domain.OperationStats, error)
	// ListCatererSchools lists the schools the caterer supplies now, in the
	// order they started. It is the one read of the supply across schools:
	// it filters on the caterer.
	ListCatererSchools(ctx context.Context, catererID int64) ([]int64, domain.OperationStats, error)
	// FindLoginAccountByEmail serves the invitation preview.
	FindLoginAccountByEmail(ctx context.Context, email string) (domain.LoginAccount, bool, domain.OperationStats, error)
	// LockLoginAccountByEmail serializes acceptance with account retention.
	LockLoginAccountByEmail(ctx context.Context, email string) (domain.LoginAccount, bool, domain.OperationStats, error)
	// InsertAccount creates the account for a new caterer address.
	InsertAccount(ctx context.Context, email, passwordHash string) (domain.LoginAccount, domain.OperationStats, error)
}
