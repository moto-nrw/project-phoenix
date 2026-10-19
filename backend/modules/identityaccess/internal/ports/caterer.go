package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// CatererStore holds the caterer, its accounts, the schools it supplies and
// the schools' invitations (ADR 0074). Writes run in the school's
// transaction; the caterer's schools are read through the global identity
// read (#4088).
type CatererStore interface {
	FindCatererOfAccount(ctx context.Context, accountID int64) (domain.Caterer, bool, domain.OperationStats, error)
	InsertCaterer(ctx context.Context, name string) (domain.Caterer, domain.OperationStats, error)
	InsertCatererAccount(ctx context.Context, accountID, catererID int64) (domain.OperationStats, error)
	InsertCatererInvitation(ctx context.Context, invitation domain.CatererInvitation) (domain.CatererInvitation, domain.OperationStats, error)
	// ListCatererInvitations lists the school's invitations, newest first.
	ListCatererInvitations(ctx context.Context) ([]domain.CatererInvitation, domain.OperationStats, error)
	// RevokeCatererInvitation takes back an open invitation of the school.
	RevokeCatererInvitation(ctx context.Context, invitationID int64, at time.Time) (bool, domain.OperationStats, error)
	// FindCatererInvitationByTokenHash locks the school's invitation for its
	// acceptance.
	FindCatererInvitationByTokenHash(ctx context.Context, tokenHash string) (domain.CatererInvitation, bool, domain.OperationStats, error)
	AcceptCatererInvitation(ctx context.Context, invitationID, accountID int64, at time.Time) (bool, domain.OperationStats, error)
	// InsertCatererSchool links the caterer to the school in context;
	// EndCatererSchool ends the school's active link.
	InsertCatererSchool(ctx context.Context, catererID int64, at time.Time) (domain.OperationStats, error)
	EndCatererSchool(ctx context.Context, at time.Time) (bool, domain.OperationStats, error)
	// HasActiveCatererSchool reports whether a caterer supplies the school
	// in context.
	HasActiveCatererSchool(ctx context.Context) (bool, domain.OperationStats, error)
	// ListCatererSchools lists the schools the caterer supplies now, in the
	// order they started.
	ListCatererSchools(ctx context.Context, catererID int64) ([]int64, domain.OperationStats, error)
	// FindLoginAccountByEmail and InsertAccount are the account rows an
	// acceptance finds or creates.
	FindLoginAccountByEmail(ctx context.Context, email string) (domain.LoginAccount, bool, domain.OperationStats, error)
	InsertAccount(ctx context.Context, email, passwordHash string) (domain.LoginAccount, domain.OperationStats, error)
}
