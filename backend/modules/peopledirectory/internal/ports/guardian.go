package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

// GuardianStore is the row-level persistence port over
// users.guardian_profiles and users.students_guardians. Reads honour the
// tenant in context when one is present; inside an admin transaction they
// span every tenant.
type GuardianStore interface {
	FindPortalMemberships(context.Context, []int64) (map[int64][]int64, error)
	ListPortalContacts(context.Context, []int64, []int64) ([]domain.GuardianPortalContact, domain.OperationStats, error)
	// ListLinksByAccount returns every link of the profiles whose account
	// is accountID, ordered by tenant then student.
	ListLinksByAccount(context.Context, int64) ([]domain.GuardianLink, domain.OperationStats, error)
	// ListByAccounts returns the profiles linked to the accounts.
	ListByAccounts(context.Context, []int64) ([]domain.Guardian, domain.OperationStats, error)
	// ListByIDs returns the profiles for the ids.
	ListByIDs(context.Context, []int64) ([]domain.Guardian, domain.OperationStats, error)
	// CountLinks counts the links per guardian profile.
	CountLinks(context.Context, []int64) (map[int64]int, domain.OperationStats, error)
	// ListAccountLinksByStudents returns the links of the children whose
	// guardian holds a portal account.
	ListAccountLinksByStudents(context.Context, []int64) ([]domain.GuardianLink, domain.OperationStats, error)
}
