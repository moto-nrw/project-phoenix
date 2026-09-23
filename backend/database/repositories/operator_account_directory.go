package repositories

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// TenantAccountInfo holds flattened account data for a given tenant, used by operator dashboard.
type TenantAccountInfo struct {
	AccountID           int64
	Email               string
	Active              bool
	FirstName           string
	LastName            string
	RoleName            string
	PedagogicRole       string
	Status              string
	HasAdminRole        bool
	HasUserRole         bool
	HasCaregiverProfile bool
	IsActiveCaregiver   bool
}

// OrgAccountInfo extends TenantAccountInfo with school context for org-level listings.
type OrgAccountInfo struct {
	TenantAccountInfo
	SchoolID   int64
	SchoolName string
}

// CaregiverChain is the staff and teacher record behind one person at one
// school. The account listings combine it with the People Directory's
// person rows to derive the caregiver facts (#2661).
type CaregiverChain struct {
	PersonID    int64
	TenantID    int64
	StaffID     int64
	TeacherID   int64
	TeacherRole string
}

// OperatorAccountDirectory is the operator listing projection composed from
// identity, person, membership and school facts, with no persistence rows.
type OperatorAccountDirectory interface {
	ListAccountsByTenantID(context.Context, int64) ([]TenantAccountInfo, error)
	ListAccountsByOrganizationID(context.Context, int64) ([]OrgAccountInfo, error)
	ListAllAccounts(context.Context) ([]OrgAccountInfo, error)
}

type operatorAccountDirectory struct {
	accounts identityaccess.SchoolAccountListings
	persons  peopledirectory.Query
	chains   caregiverChainQuery
	schools  organizationtenancy.Query
}

// NewOperatorAccountDirectory reuses the serving owners; it constructs no
// repositories or modules and does not own a database connection.
func NewOperatorAccountDirectory(accounts identityaccess.SchoolAccountListings, persons peopledirectory.Query, membership staffLookup, schools organizationtenancy.Query) OperatorAccountDirectory {
	return operatorAccountDirectory{accounts: accounts, persons: persons, chains: caregiverChainsFromMembership(membership), schools: schools}
}

func (r operatorAccountDirectory) identityRows(ctx context.Context, schoolIDs []int64) ([]OrgAccountInfo, error) {
	rows, err := r.accounts.ListSchoolAccountListings(ctx, schoolIDs)
	if err != nil {
		return nil, err
	}
	result := make([]OrgAccountInfo, 0, len(rows))
	for _, row := range rows {
		result = append(result, OrgAccountInfo{SchoolID: row.SchoolID, TenantAccountInfo: TenantAccountInfo{
			AccountID: row.AccountID, Email: row.Email, Active: row.Active,
			FirstName: row.FirstName, LastName: row.LastName, RoleName: row.RoleName,
			Status: row.Status, HasAdminRole: row.HasAdminRole, HasUserRole: row.HasUserRole,
		}})
	}
	return result, nil
}
