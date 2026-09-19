package identityaccess

import (
	"context"
	"errors"
	"time"
)

// Operator-led school access (#3252, issue #1021): which schools an existing
// account may reach and with which role. Granting an account access to a
// school it does not belong to yet is a cross-tenant act, so the capability
// lives on the operator surface.
var (
	// ErrAccountTenantAccessNotFound reports an account without an active
	// mapping to the school an operation targets.
	ErrAccountTenantAccessNotFound = errors.New("account has no active access to this school")
	// ErrAccountTenantAccessExists refuses a grant for a school the account
	// already reaches.
	ErrAccountTenantAccessExists = errors.New("account already has access to this school")
	ErrSchoolNotFound            = errors.New("school not found")
	ErrSchoolDeleted             = errors.New("school is soft-deleted")
)

// AccountTenantRole is one role an account holds at one school.
type AccountTenantRole struct {
	ID       int64
	Name     string
	IsSystem bool
	BaseRole *string
}

// AccountTenantAccess is one school an account has (or had) access to,
// including the roles it holds there and whether a person and a staff
// record back it at that school.
type AccountTenantAccess struct {
	TenantID         int64
	SchoolName       string
	SchoolSlug       string
	SchoolActive     bool
	OrganizationID   int64
	OrganizationName string
	Status           string
	ActivatedAt      *time.Time
	DeactivatedAt    *time.Time
	HasPerson        bool
	HasStaff         bool
	Roles            []AccountTenantRole
}

// GrantAccountTenantAccess carries a grant: the school, the role and the
// optional person data used when the account has no person record anywhere
// yet. ClientIP is recorded in the operator audit ledger.
type GrantAccountTenantAccess struct {
	AccountID  int64
	SchoolID   int64
	RoleID     int64
	FirstName  string
	LastName   string
	Position   string
	OperatorID int64
	ClientIP   string
}

// OperatorAccountAccess is the capability the operator school-access
// routes consume. Every command runs in one administrative transaction,
// records the platform and the tenant-visible audit evidence and returns
// the account's complete access listing afterwards. Rejections the operator
// can correct are *InvalidInputError values carrying the reason.
type OperatorAccountAccess interface {
	ListAccountTenantAccess(ctx context.Context, accountID int64) ([]AccountTenantAccess, error)
	// ListAssignableSchoolRoles returns the system and target-school roles
	// the mutation paths would accept.
	ListAssignableSchoolRoles(ctx context.Context, schoolID int64) ([]AccountTenantRole, error)
	GrantAccountTenantAccess(ctx context.Context, request GrantAccountTenantAccess) ([]AccountTenantAccess, error)
	UpdateAccountTenantRole(ctx context.Context, accountID, schoolID, roleID, operatorID int64, clientIP string) ([]AccountTenantAccess, error)
	RevokeAccountTenantAccess(ctx context.Context, accountID, schoolID, operatorID int64, clientIP string) ([]AccountTenantAccess, error)
}

func (m *Module) ListAccountTenantAccess(ctx context.Context, accountID int64) ([]AccountTenantAccess, error) {
	return m.engine.ListAccountTenantAccess(ctx, accountID)
}

func (m *Module) ListAssignableSchoolRoles(ctx context.Context, schoolID int64) ([]AccountTenantRole, error) {
	return m.engine.ListAssignableSchoolRoles(ctx, schoolID)
}

func (m *Module) GrantAccountTenantAccess(ctx context.Context, request GrantAccountTenantAccess) ([]AccountTenantAccess, error) {
	return m.engine.GrantAccountTenantAccess(ctx, request)
}

func (m *Module) UpdateAccountTenantRole(ctx context.Context, accountID, schoolID, roleID, operatorID int64, clientIP string) ([]AccountTenantAccess, error) {
	return m.engine.UpdateAccountTenantRole(ctx, accountID, schoolID, roleID, operatorID, clientIP)
}

func (m *Module) RevokeAccountTenantAccess(ctx context.Context, accountID, schoolID, operatorID int64, clientIP string) ([]AccountTenantAccess, error) {
	return m.engine.RevokeAccountTenantAccess(ctx, accountID, schoolID, operatorID, clientIP)
}
