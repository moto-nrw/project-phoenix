package ports

import (
	"context"
	"encoding/json"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

// GuardianStore is the row-level persistence port over
// users.guardian_profiles and users.student_guardian_relationships. Reads honour the
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
	GuardianPortalWrites
}

// GuardianLinkOwners are the owners of the other two halves of a
// relationship (#2756): Care Plan's pickup permission and Identity & Access's
// portal access. A link is written as all three halves in one unit of work.
type GuardianLinkOwners interface {
	CreateGuardianPickupPermission(ctx context.Context, tenantID, relationshipID int64, canPickup bool, notes *string) error
	ChangeGuardianPickupPermission(ctx context.Context, tenantID, relationshipID int64, canPickup *bool, setNotes bool, notes *string) (bool, error)
	GrantGuardianStudentAccess(ctx context.Context, tenantID, relationshipID int64, accountID *int64, permissions json.RawMessage) error
}

// GuardianPortalWrites are the row writes of the parents-portal workflow.
// Every method writes in the tenant of the caller's transaction and refuses
// without one; a false or zero result means the tenant has no such row.
type GuardianPortalWrites interface {
	InsertContact(context.Context, domain.GuardianContact) (int64, domain.OperationStats, error)
	UpdateContact(context.Context, int64, domain.GuardianContact) (bool, domain.OperationStats, error)
	DeletePhones(ctx context.Context, guardianID int64) (domain.OperationStats, error)
	InsertPhone(ctx context.Context, guardianID int64, phone domain.GuardianPhoneRecord) (int64, domain.OperationStats, error)
	UpdatePhoneNumber(ctx context.Context, phoneID int64, number string) (bool, domain.OperationStats, error)
	DeletePhone(ctx context.Context, phoneID int64) (bool, domain.OperationStats, error)
	// InsertLinkIfAbsent writes the relationship half of a new link and
	// returns its id, or zero when the pair is linked already.
	InsertLinkIfAbsent(context.Context, domain.GuardianLinkRecord) (int64, domain.OperationStats, error)
	// LockLinkForPickup takes the relationship row lock, sets the emergency
	// contact flag when it is supplied and reports whether the tenant has the
	// relationship.
	LockLinkForPickup(ctx context.Context, linkID int64, isEmergencyContact *bool) (bool, domain.OperationStats, error)
	// GuardianAccount is the portal account the guardian profile names.
	GuardianAccount(ctx context.Context, guardianID int64) (*int64, domain.OperationStats, error)
	SetPortalLocale(ctx context.Context, accountID int64, locale string) (int64, domain.OperationStats, error)
}
