package identityaccess

import (
	"context"
	"errors"
	"strings"
	"time"
)

// Role and permission administration (#3314): role and permission CRUD,
// account role assignment with the school identity it owes, direct account
// grants and the role-permission selection. The error messages are the wire
// contract the retained auth service established; the HTTP layers switch on
// the sentinels. System roles are immutable, and every account mutation
// serializes on the account row and, in organisation scope, on the
// account's active school membership.
var (
	ErrSystemRoleImmutable = errors.New("system roles cannot be modified")
	ErrPermissionNotFound  = errors.New("permission not found")

	// ErrRoleNotAssignable is returned when the requested role does not exist
	// (or is not a role that may be handed out for a school at all).
	ErrRoleNotAssignable = errors.New("Die angegebene Rolle existiert nicht") //nolint:staticcheck // ST1005: user-facing German message
	// ErrRoleForeignTenant is returned when a tenant-scoped role belongs to a
	// different school than the one being granted.
	ErrRoleForeignTenant = errors.New("Diese Rolle existiert an der Zielschule nicht") //nolint:staticcheck // ST1005: user-facing German message
	// ErrRoleGuardianNotAssignable is returned for the guardian role, which is
	// granted exclusively through the guardian invitation flow.
	ErrRoleGuardianNotAssignable = errors.New("Sorgeberechtigten-Zugänge werden über den Einladungs-Flow für Sorgeberechtigte vergeben") //nolint:staticcheck // ST1005: user-facing German message
	// ErrRoleLegacyTeacherNotAssignable is returned for the retired teacher
	// role; caregiver accounts use the user role instead.
	ErrRoleLegacyTeacherNotAssignable = errors.New("Die alte Rolle 'teacher' wird nicht mehr vergeben; bitte die Rolle 'user' verwenden") //nolint:staticcheck // ST1005: user-facing German message

	// ErrRoleCaregiverNeedsProfile refuses a caregiver role on a Lehrkraft
	// account: it has person and staff records but deliberately no caregiver
	// profile, so the caregiver permissions would own no groups (#1772).
	ErrRoleCaregiverNeedsProfile = errors.New("Ein Lehrkraft-Konto hat kein Betreuungsprofil und kann nicht auf eine Betreuer-Rolle umgestellt werden") //nolint:staticcheck // ST1005: user-facing German message
	// ErrLehrkraftRoleImmutable keeps a Lehrkraft account from receiving any
	// other role. Changing it goes through offboarding plus a new account.
	ErrLehrkraftRoleImmutable = errors.New("Ein Lehrkraft-Konto kann nicht umgestellt werden") //nolint:staticcheck // ST1005: user-facing German message
	// ErrRoleLehrkraftCaregiverProfile refuses the Lehrkraft role on an
	// account whose identity at the school still carries a live caregiver
	// profile: the swap would strand the profile and its group supervisions
	// under class_day-only permissions (#1772).
	ErrRoleLehrkraftCaregiverProfile = errors.New("Das Konto hat ein Betreuungsprofil an dieser Schule und kann nicht auf Lehrkraft umgestellt werden") //nolint:staticcheck // ST1005: user-facing German message

	// ErrRoleAdministrationUnavailable reports a module composed without the
	// account-lifecycle dependencies the role administration needs.
	ErrRoleAdministrationUnavailable = errors.New("role administration is not composed")
)

// ValidateAssignableSchoolRole applies the school-role assignment policy to a
// resolved role. A role qualifies when it is a platform system role
// (tenant_id NULL) or a custom role of that very school. Guardian-tier roles
// belong to the guardian invitation flow and the retired teacher role is
// never handed out again. Shared by every path that grants school access, so
// the rules cannot drift apart per entry point.
func ValidateAssignableSchoolRole(role *RoleFacts, tenantID int64) error {
	if role == nil {
		return ErrRoleNotAssignable
	}
	if role.TenantID != nil && *role.TenantID != tenantID {
		return ErrRoleForeignTenant
	}
	if role.TenantID == nil && !role.IsSystem {
		return ErrRoleNotAssignable
	}
	// Guardian is decided by tier, not by name: a school role labelled
	// "Guardian" is not the parent-portal role, and the parent-portal role may
	// carry a custom label.
	if IsGuardianTierRole(role) {
		return ErrRoleGuardianNotAssignable
	}
	// The retired teacher role predates base_role and was never backfilled, so
	// it stays a name match narrowed to system roles. A school's own custom
	// "teacher" role remains assignable.
	if role.IsSystem && strings.EqualFold(strings.TrimSpace(role.Name), legacyTeacherRoleName) {
		return ErrRoleLegacyTeacherNotAssignable
	}
	return nil
}

// IsGuardianTierRole reports whether a role hands out guardian privileges.
func IsGuardianTierRole(role *RoleFacts) bool {
	return EffectiveBaseRole(role) == BaseRoleGuardian
}

// Role is one role as the administration reports it. TenantID is nil for the
// platform system roles.
type Role struct {
	ID          int64
	TenantID    *int64
	Name        string
	Description string
	IsSystem    bool
	BaseRole    *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// AssignableSchoolRole is a role the policy allows for a school, with the
// permission names the role-grant check compares against the caller's.
type AssignableSchoolRole struct {
	Role
	Permissions []string
}

// AuthorizationGrantData exposes the stored facts the role-grant policy reads.
func (r *AssignableSchoolRole) AuthorizationGrantData() (bool, string, *string, bool, bool, []string) {
	if r == nil {
		return false, "", nil, false, false, nil
	}
	return true, r.Name, r.BaseRole, r.IsSystem, r.TenantID != nil, append([]string(nil), r.Permissions...)
}

// Permission is one permission as the administration reports it.
type Permission struct {
	ID          int64
	Name        string
	Description string
	Resource    string
	Action      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// FullName is the resource:action form role details list.
func (p Permission) FullName() string {
	return p.Resource + ":" + p.Action
}

// RoleFilter narrows ListRoles. Zero values do not filter.
type RoleFilter struct {
	Name string
}

// PermissionFilter narrows ListPermissions. Zero values do not filter.
type PermissionFilter struct {
	Resource string
	Action   string
}

// RoleQuery reads roles and the roles accounts hold. Account reads apply the
// tenant in context and, in organisation scope, require the account's
// membership in that school.
type RoleQuery interface {
	GetRole(ctx context.Context, id int64) (Role, error)
	ListRoles(ctx context.Context, filter RoleFilter) ([]Role, error)
	// ResolveAssignableSchoolRole returns the role only if the school-role
	// policy allows it for tenantID. A failed lookup other than a missing
	// role is reported as is.
	ResolveAssignableSchoolRole(ctx context.Context, roleID, tenantID int64) (*AssignableSchoolRole, error)
	GetAccountRoles(ctx context.Context, accountID int64) ([]Role, error)
	// AccountHoldsLehrkraftRole reports whether the account holds the
	// Lehrkraft system role at the tenant in context.
	AccountHoldsLehrkraftRole(ctx context.Context, accountID int64) (bool, error)
	// GetAccountRoleNames maps each account to its primary role name.
	GetAccountRoleNames(ctx context.Context, accountIDs []int64) (map[int64]string, error)
	GetAccountEmails(ctx context.Context, accountIDs []int64) (map[int64]string, error)
	GetAccountAvatars(ctx context.Context, accountIDs []int64) (map[int64]string, error)
}

// RoleCommand changes roles and account role assignments. Every assignment
// change revokes the account's sessions at the school.
type RoleCommand interface {
	// CreateRole creates a custom role of the tenant in context; baseRole maps
	// it to a system role for announcement targeting and is required.
	CreateRole(ctx context.Context, name, description string, baseRole *string) (Role, error)
	// UpdateRole writes name, description and base role of a custom role.
	UpdateRole(ctx context.Context, role Role) error
	DeleteRole(ctx context.Context, id int64) error
	// AssignRoleToAccount assigns the role and completes the school identity
	// a staff-tier role owes, in one transaction.
	AssignRoleToAccount(ctx context.Context, accountID, roleID int64) error
	// ReplaceAccountRole makes roleID the account's only staff role at the
	// school; guardian-tier roles stay.
	ReplaceAccountRole(ctx context.Context, accountID, roleID int64) error
	RemoveRoleFromAccount(ctx context.Context, accountID, roleID int64) error
}

// PermissionQuery reads permissions, role selections and account grants.
type PermissionQuery interface {
	GetPermission(ctx context.Context, id int64) (Permission, error)
	GetPermissionByName(ctx context.Context, name string) (Permission, error)
	ListPermissions(ctx context.Context, filter PermissionFilter) ([]Permission, error)
	GetRolePermissions(ctx context.Context, roleID int64) ([]Permission, error)
	// GetAccountPermissions returns the direct and the role-based permissions.
	GetAccountPermissions(ctx context.Context, accountID int64) ([]Permission, error)
	GetAccountDirectPermissions(ctx context.Context, accountID int64) ([]Permission, error)
}

// PermissionCommand changes permissions, direct account grants and role
// selections. Role selections of system roles are immutable.
type PermissionCommand interface {
	CreatePermission(ctx context.Context, name, description, resource, action string) (Permission, error)
	UpdatePermission(ctx context.Context, permission Permission) error
	DeletePermission(ctx context.Context, id int64) error
	GrantPermissionToAccount(ctx context.Context, accountID, permissionID int64) error
	DenyPermissionToAccount(ctx context.Context, accountID, permissionID int64) error
	RemovePermissionFromAccount(ctx context.Context, accountID, permissionID int64) error
	AssignPermissionToRole(ctx context.Context, roleID, permissionID int64) error
	// ReplaceRolePermissions applies a complete selection atomically; an
	// invalid request leaves the current selection untouched.
	ReplaceRolePermissions(ctx context.Context, roleID int64, permissionIDs []int64) error
	RemovePermissionFromRole(ctx context.Context, roleID, permissionID int64) error
	// GrantStaffDefaultPermission grants permissionName to the account of a
	// newly created staff member unless the account holds the Lehrkraft role.
	// Failures are logged, never returned: the staff record stands either way.
	GrantStaffDefaultPermission(ctx context.Context, accountID int64, isTeacher bool, permissionName string)
}

// RoleAdministration is the capability the RBAC routes, the staff membership
// runtime, operator provisioning, the caregiver capability and staff
// offboarding consume (#3314).
type RoleAdministration interface {
	RoleQuery
	RoleCommand
	PermissionQuery
	PermissionCommand
}

func (m *Module) GetRole(ctx context.Context, id int64) (Role, error) {
	return m.engine.GetRole(ctx, id)
}

func (m *Module) ListRoles(ctx context.Context, filter RoleFilter) ([]Role, error) {
	return m.engine.ListRoles(ctx, filter)
}

func (m *Module) ResolveAssignableSchoolRole(ctx context.Context, roleID, tenantID int64) (*AssignableSchoolRole, error) {
	return m.engine.ResolveAssignableSchoolRole(ctx, roleID, tenantID)
}

func (m *Module) GetAccountRoles(ctx context.Context, accountID int64) ([]Role, error) {
	return m.engine.GetAccountRoles(ctx, accountID)
}

func (m *Module) AccountHoldsLehrkraftRole(ctx context.Context, accountID int64) (bool, error) {
	return m.engine.AccountHoldsLehrkraftRole(ctx, accountID)
}

func (m *Module) GetAccountRoleNames(ctx context.Context, accountIDs []int64) (map[int64]string, error) {
	return m.engine.GetAccountRoleNames(ctx, accountIDs)
}

func (m *Module) GetAccountEmails(ctx context.Context, accountIDs []int64) (map[int64]string, error) {
	return m.engine.GetAccountEmails(ctx, accountIDs)
}

func (m *Module) GetAccountAvatars(ctx context.Context, accountIDs []int64) (map[int64]string, error) {
	return m.engine.GetAccountAvatars(ctx, accountIDs)
}

func (m *Module) CreateRole(ctx context.Context, name, description string, baseRole *string) (Role, error) {
	return m.engine.CreateRole(ctx, name, description, baseRole)
}

func (m *Module) UpdateRole(ctx context.Context, role Role) error {
	return m.engine.UpdateRole(ctx, role)
}

func (m *Module) DeleteRole(ctx context.Context, id int64) error {
	return m.engine.DeleteRole(ctx, id)
}

func (m *Module) AssignRoleToAccount(ctx context.Context, accountID, roleID int64) error {
	return m.engine.AssignRoleToAccount(ctx, accountID, roleID)
}

func (m *Module) ReplaceAccountRole(ctx context.Context, accountID, roleID int64) error {
	return m.engine.ReplaceAccountRole(ctx, accountID, roleID)
}

func (m *Module) RemoveRoleFromAccount(ctx context.Context, accountID, roleID int64) error {
	return m.engine.RemoveRoleFromAccount(ctx, accountID, roleID)
}

func (m *Module) GetPermission(ctx context.Context, id int64) (Permission, error) {
	return m.engine.GetPermission(ctx, id)
}

func (m *Module) GetPermissionByName(ctx context.Context, name string) (Permission, error) {
	return m.engine.GetPermissionByName(ctx, name)
}

func (m *Module) ListPermissions(ctx context.Context, filter PermissionFilter) ([]Permission, error) {
	return m.engine.ListPermissions(ctx, filter)
}

func (m *Module) GetRolePermissions(ctx context.Context, roleID int64) ([]Permission, error) {
	return m.engine.GetRolePermissions(ctx, roleID)
}

func (m *Module) GetAccountPermissions(ctx context.Context, accountID int64) ([]Permission, error) {
	return m.engine.GetAccountPermissions(ctx, accountID)
}

func (m *Module) GetAccountDirectPermissions(ctx context.Context, accountID int64) ([]Permission, error) {
	return m.engine.GetAccountDirectPermissions(ctx, accountID)
}

func (m *Module) CreatePermission(ctx context.Context, name, description, resource, action string) (Permission, error) {
	return m.engine.CreatePermission(ctx, name, description, resource, action)
}

func (m *Module) UpdatePermission(ctx context.Context, permission Permission) error {
	return m.engine.UpdatePermission(ctx, permission)
}

func (m *Module) DeletePermission(ctx context.Context, id int64) error {
	return m.engine.DeletePermission(ctx, id)
}

func (m *Module) GrantPermissionToAccount(ctx context.Context, accountID, permissionID int64) error {
	return m.engine.GrantPermissionToAccount(ctx, accountID, permissionID)
}

func (m *Module) DenyPermissionToAccount(ctx context.Context, accountID, permissionID int64) error {
	return m.engine.DenyPermissionToAccount(ctx, accountID, permissionID)
}

func (m *Module) RemovePermissionFromAccount(ctx context.Context, accountID, permissionID int64) error {
	return m.engine.RemovePermissionFromAccount(ctx, accountID, permissionID)
}

func (m *Module) AssignPermissionToRole(ctx context.Context, roleID, permissionID int64) error {
	return m.engine.AssignPermissionToRole(ctx, roleID, permissionID)
}

func (m *Module) ReplaceRolePermissions(ctx context.Context, roleID int64, permissionIDs []int64) error {
	return m.engine.ReplaceRolePermissions(ctx, roleID, permissionIDs)
}

func (m *Module) RemovePermissionFromRole(ctx context.Context, roleID, permissionID int64) error {
	return m.engine.RemovePermissionFromRole(ctx, roleID, permissionID)
}

func (m *Module) GrantStaffDefaultPermission(ctx context.Context, accountID int64, isTeacher bool, permissionName string) {
	m.engine.GrantStaffDefaultPermission(ctx, accountID, isTeacher, permissionName)
}
