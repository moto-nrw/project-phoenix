package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// OperatorAudit appends operator actions to the Audit platform's
// platform-scoped ledger on the caller's transaction.
type OperatorAudit interface {
	RecordOperatorAction(ctx context.Context, entry identityaccess.OperatorAuditEntry) error
}

// OrganizationName is the organisation fact the access listing shows.
type OrganizationName struct {
	ID   int64
	Name string
}

// OrganizationDirectory reads the Organisation & Tenancy facts the access
// listing shows: the schools behind the mappings and the organisation
// names in the owner's name order.
type OrganizationDirectory interface {
	ListSchools(ctx context.Context, ids []int64) ([]identityaccess.School, error)
	ListOrganizationNames(ctx context.Context, ids []int64) ([]OrganizationName, error)
}

// AccountPersonIdentity is one person row that carries the account at any
// school, with the facts the name resolution decides on.
type AccountPersonIdentity struct {
	ID        int64
	TenantID  int64
	FirstName string
	LastName  string
	Deleted   bool
	IsStudent bool
}

// AccountIdentityFact reports whether a person and a staff record back the
// account at one school.
type AccountIdentityFact struct {
	TenantID  int64
	HasPerson bool
	HasStaff  bool
}

// SchoolIdentityRequest describes the identity chain a school access
// requires: person, staff and (for caregiver roles) teacher rows.
type SchoolIdentityRequest struct {
	AccountID    int64
	TenantID     int64
	Role         identityaccess.SchoolRole
	FirstName    string
	LastName     string
	Position     string
	CreatePerson bool
}

// SchoolIdentityProvisioner is the People Directory and School Membership
// identity chain a school access requires and the facts the listing shows
// about it. The tenant-scoped reads take the tenant from the context; the
// platform-wide reads run administratively. A request the caller can
// correct is reported as *identityaccess.InvalidInputError.
type SchoolIdentityProvisioner interface {
	HasLivePersonAtSchool(ctx context.Context, accountID int64) (bool, error)
	ListAccountPersons(ctx context.Context, accountID int64) ([]AccountPersonIdentity, error)
	HasLiveCaregiverProfile(ctx context.Context, accountID int64) (bool, error)
	EnsureSchoolIdentity(ctx context.Context, request SchoolIdentityRequest) error
	ListAccountIdentityFacts(ctx context.Context, accountID int64) ([]AccountIdentityFact, error)
}
