package account

import "context"

// TenantSchool is the part of a school the tenant, invitation and passkey
// routes read.
type TenantSchool struct {
	ID               int64
	OrganizationID   int64
	OrganizationName string
	Name             string
	Slug             string
	Subdomain        string
	// Settings is the school's raw settings JSON the tenant shell renders.
	Settings string
	Active   bool
	Hidden   bool
	Deleted  bool
}

// SchoolDirectory reads schools for the auth routes. Organisation & Tenancy
// owns the rows; the root binds this port to its capability. A school that
// does not exist is (nil, nil). The list reads carry the organization name.
type SchoolDirectory interface {
	GetSchoolByID(ctx context.Context, id int64) (*TenantSchool, error)
	GetSchoolBySubdomain(ctx context.Context, subdomain string) (*TenantSchool, error)
	ListPublicSchools(ctx context.Context) ([]TenantSchool, error)
	ListActiveSchoolsByAccountID(ctx context.Context, accountID int64) ([]TenantSchool, error)
}
