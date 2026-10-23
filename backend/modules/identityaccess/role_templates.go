package identityaccess

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Role templates and reserved permissions (#4574). A school creates its own
// role from a template: the new role starts with a copy of the permissions the
// template system role effectively holds and its tier, and is independent
// afterwards. The reserved permissions never land on a school role, neither
// through the copy nor through a later selection.

// RoleTemplate names the system role a new school role copies. The values are
// the system role names on the wire.
type RoleTemplate string

const (
	// RoleTemplateCaregiver copies the platform caregiver role ("Betreuung").
	RoleTemplateCaregiver RoleTemplate = "user"
	// RoleTemplateAdministration copies what the admin role ("Verwaltung")
	// effectively holds: every catalog permission except the reserved ones.
	// admin reaches many permissions only through admin:*, so copying its
	// explicit grants would leave the new role short.
	RoleTemplateAdministration RoleTemplate = "admin"
	// RoleTemplateLehrkraft copies the Lehrkraft role.
	RoleTemplateLehrkraft RoleTemplate = "lehrkraft"
)

// RoleTemplates lists the templates a school may create a role from.
func RoleTemplates() []RoleTemplate {
	return []RoleTemplate{RoleTemplateCaregiver, RoleTemplateAdministration, RoleTemplateLehrkraft}
}

// Valid reports whether t is one of RoleTemplates.
func (t RoleTemplate) Valid() bool {
	return slices.Contains(RoleTemplates(), t)
}

// reservedPermissions are the permissions no school role may gain.
//   - *:* is the superuser wildcard of the platform operator.
//   - admin:* is the master key of the school administration: it implies every
//     permission, and several school features (parent announcements, staff
//     notices, staff view preview, care plan exceptions, guardian and activity
//     approvals) check it directly. Keeping it with the system role admin
//     alone is a product decision; the demo seed's ogs-leitung forgoes it too.
//     Operator routes do not depend on it: they require a platform session.
//   - system:manage is the operator's system permission.
//   - permissions:create/update/delete are mounted behind the platform scope,
//     so a school session can never use them.
var reservedPermissions = []string{
	"*:*",
	"admin:*",
	"system:manage",
	"permissions:create",
	"permissions:update",
	"permissions:delete",
}

// IsReservedPermission reports whether the resource:action name is a
// reserved permission.
func IsReservedPermission(name string) bool {
	return slices.Contains(reservedPermissions, strings.ToLower(strings.TrimSpace(name)))
}

// ErrReservedPermissionNotAllowed refuses a reserved permission on a school
// role. Errors carrying the permission name wrap it as
// ReservedPermissionError.
var ErrReservedPermissionNotAllowed = errors.New("reserved permissions cannot be granted to a school role")

// ReservedPermissionError names the reserved permission a selection asked for.
type ReservedPermissionError struct {
	Permission string
}

func (e *ReservedPermissionError) Error() string {
	return fmt.Sprintf("%s: %s", ErrReservedPermissionNotAllowed.Error(), e.Permission)
}

// Is matches ErrReservedPermissionNotAllowed.
func (e *ReservedPermissionError) Is(target error) bool {
	return target == ErrReservedPermissionNotAllowed
}
