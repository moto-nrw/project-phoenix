package ports

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/domain"
)

// The ports other owners and the root bind: directories, ledgers and cleanups outside the module.

// SchoolDirectory is the consumer-owned port over the Organisation & Tenancy
// school facts login resolves. Missing schools report found=false; every
// other failure is an error.
type SchoolDirectory interface {
	FindSchool(ctx context.Context, id int64) (domain.School, bool, error)
	FindSchoolBySubdomain(ctx context.Context, subdomain string) (domain.School, bool, error)
	// LockSchoolShared reads the school under a FOR SHARE lock inside the
	// caller's transaction.
	LockSchoolShared(ctx context.Context, id int64) (domain.School, bool, error)
	// ListActiveSchoolsByID returns the live, active schools among the supplied IDs.
	// Identity has already selected the account's active memberships.
	ListActiveSchoolsByID(ctx context.Context, schoolIDs []int64) ([]domain.School, error)
}

// PersonDirectory is the consumer-owned port over the People Directory name
// of an account's person row in the tenant the context carries.
type PersonDirectory interface {
	FindPersonName(ctx context.Context, accountID int64) (firstName, lastName string, found bool, err error)
}

// AuthAudit is the consumer-owned port over the Audit platform's
// authentication ledger.
type AuthAudit interface {
	// RecordAuthEvent appends the event on the caller's transaction.
	RecordAuthEvent(ctx context.Context, event domain.AuthEvent) error
	ListPendingAccountWideWipes(ctx context.Context) ([]domain.PendingAccountWideWipe, error)
	ClaimPendingAccountWideWipes(ctx context.Context, accountID int64) ([]domain.PendingAccountWideWipe, error)
}

// PushSubscriptionCleanup is the consumer-owned port over the Delivery
// platform's push subscription rows a session revocation orphans.
type PushSubscriptionCleanup interface {
	DeleteStaffByAccount(ctx context.Context, accountID int64) error
	DeleteSchoolByAccount(ctx context.Context, accountID int64) error
	DeleteParentByAccount(ctx context.Context, accountID int64) error
	DeleteByTokenFamily(ctx context.Context, accountID int64, familyID string) error
	DeleteUnboundByAccount(ctx context.Context, accountID, tenantID int64, portal string) error
	DeleteOrphaned(ctx context.Context) error
}

// OrganizationDirectory is the consumer-owned port over the Organisation &
// Tenancy facts the access listing shows: the schools behind the mappings
// and the organisation names, in the owner's name order.
type OrganizationDirectory interface {
	ListSchools(ctx context.Context, ids []int64) ([]domain.School, error)
	ListOrganizationNames(ctx context.Context, ids []int64) ([]domain.OrganizationName, error)
}

// SchoolIdentityProvisioner is the consumer-owned port over the People
// Directory and School Membership identity chain a school access requires
// (persons, staff, teachers) and the facts the listing shows about it. The
// tenant-scoped reads take the tenant from the context; the platform-wide
// reads run administratively.
type SchoolIdentityProvisioner interface {
	// HasLivePersonAtSchool reports a live person carrying the account at
	// the school in context.
	HasLivePersonAtSchool(ctx context.Context, accountID int64) (bool, error)
	// ListAccountPersons returns every person row carrying the account at
	// any school.
	ListAccountPersons(ctx context.Context, accountID int64) ([]domain.AccountPersonIdentity, error)
	// HasLiveCaregiverProfile reports a live users.teachers row behind the
	// account's identity at the school in context.
	HasLiveCaregiverProfile(ctx context.Context, accountID int64) (bool, error)
	// EnsureSchoolIdentity creates the person, staff and (for caregiver
	// roles) teacher rows idempotently on the caller's tenant transaction.
	// A request the caller can correct is reported as
	// *domain.InvalidInputError.
	EnsureSchoolIdentity(ctx context.Context, request domain.SchoolIdentityRequest) error
	// ListAccountIdentityFacts reports, per school, whether a person and a
	// staff record back the account.
	ListAccountIdentityFacts(ctx context.Context, accountID int64) ([]domain.AccountIdentityFact, error)
}

// OperatorAudit is the consumer-owned port over the Audit platform's
// operator action ledger. Entries append on the caller's transaction.
type OperatorAudit interface {
	RecordOperatorAction(ctx context.Context, entry domain.OperatorAuditEntry) error
}

// OperatorCredentialCleanup invalidates the bearer-style controls a password
// rotation must not leave alive besides the refresh sessions: the pending
// e-mail change links. The composition binds it to the module's own link
// operations (#2722), so the revocation joins the password change.
type OperatorCredentialCleanup interface {
	// RevokePasswordBoundCredentials ends the operator's e-mail change links
	// and remembered devices when the password changes (#6289).
	RevokePasswordBoundCredentials(ctx context.Context, operatorID int64) error
}
