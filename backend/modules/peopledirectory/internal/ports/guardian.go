package ports

import (
	"context"
	"encoding/json"
	"time"

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
	// ClearChildlessSince clears the retention marker as part of a new
	// relationship write, so a later unlink starts its retention window anew.
	// found is false when no profile remains in the caller's school.
	ClearChildlessSince(context.Context, int64) (found bool, stats domain.OperationStats, err error)
	// CountLinks counts the links per guardian profile.
	CountLinks(context.Context, []int64) (map[int64]int, domain.OperationStats, error)
	// ListAccountLinksByStudents returns the links of the children whose
	// guardian holds a portal account.
	ListAccountLinksByStudents(context.Context, []int64) ([]domain.GuardianLink, domain.OperationStats, error)
	GuardianPortalWrites
	GuardianLifecycleWrites
}

// GuardianLinkOwners are the owners of the other two halves of a
// relationship (#2756): Care Plan's pickup permission and Identity & Access's
// portal access. A link is written as all three halves in one unit of work.
type GuardianLinkOwners interface {
	// LockLiveGuardianAccount takes Identity & Access's shared account-row
	// lock and reports whether the account remains usable for a guardian link.
	LockLiveGuardianAccount(ctx context.Context, accountID int64) (bool, error)
	CreateGuardianPickupPermission(ctx context.Context, tenantID, relationshipID int64, canPickup bool, notes *string) error
	ChangeGuardianPickupPermission(ctx context.Context, tenantID, relationshipID int64, canPickup *bool, setNotes bool, notes *string) (bool, error)
	GrantGuardianStudentAccess(ctx context.Context, tenantID, relationshipID int64, accountID *int64, permissions json.RawMessage) error
	SetGuardianStudentPermissions(ctx context.Context, tenantID, relationshipID int64, permissions json.RawMessage) (bool, error)
}

// GuardianRolePresets is the Security Runtime's role-preset policy of a
// student-guardian relationship.
type GuardianRolePresets interface {
	// DefaultRole is the preset a new relationship takes when the caller
	// chose none.
	DefaultRole(relationshipType string, isPrimary, isEmergencyContact, canPickup bool) string
	// Preset returns the stored role of a preset and the parent-portal
	// permissions it grants.
	Preset(role string) (string, []string)
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

// GuardianLifecycleWrites are the row writes of the guardian invitation,
// approval and revocation flows. Every method writes in the tenant of the
// caller's transaction and refuses without one; a false result means the
// tenant has no such row.
type GuardianLifecycleWrites interface {
	// LockGuardianAccount serializes the account links of one parent account
	// in the school until the transaction ends, also while no profile holds
	// the account yet.
	LockGuardianAccount(ctx context.Context, accountID int64) (domain.OperationStats, error)
	// LockAccountHolders locks the profile and every profile holding the
	// account, ordered by ID.
	LockAccountHolders(ctx context.Context, guardianID, accountID int64) ([]domain.GuardianAccountHolder, domain.OperationStats, error)
	// GuardianHasLinks reports whether the profile has a relationship.
	GuardianHasLinks(ctx context.Context, guardianID int64) (bool, domain.OperationStats, error)
	// SetGuardianAccount binds the account to the profile, or clears the
	// binding when accountID is nil.
	SetGuardianAccount(ctx context.Context, guardianID int64, accountID *int64) (bool, domain.OperationStats, error)
	DeleteGuardian(ctx context.Context, guardianID int64) (bool, domain.OperationStats, error)
	// MarkChildlessGuardians keeps users.guardian_profiles.childless_since
	// in step for the tenant (#4043): it clears the mark of profiles with a
	// relationship and marks the profiles without one.
	MarkChildlessGuardians(ctx context.Context) (domain.OperationStats, error)
	// LockChildlessGuardiansBefore locks the tenant's profiles that have
	// been without a child since before cutoff (#4043), skipping rows a
	// concurrent transaction holds, and returns their IDs in order.
	LockChildlessGuardiansBefore(ctx context.Context, cutoff time.Time) ([]int64, domain.OperationStats, error)
	// DeleteChildlessGuardians deletes those of the profiles that are still
	// without a child since before cutoff and reports how many went.
	DeleteChildlessGuardians(ctx context.Context, guardianIDs []int64, cutoff time.Time) (int, domain.OperationStats, error)
	SetLinkRole(ctx context.Context, linkID int64, role string) (bool, domain.OperationStats, error)
	DeleteLink(ctx context.Context, linkID int64) (domain.OperationStats, error)
}
