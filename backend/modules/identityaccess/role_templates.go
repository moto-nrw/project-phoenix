package identityaccess

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Role templates and platform permissions (#4574). A school creates its own
// role from a template: the new role starts with a copy of the template
// system role's permissions and its tier, and is independent afterwards. The
// platform permissions never land on a school role, neither through the copy
// nor through a later selection.

// RoleTemplate names the system role a new school role copies. The values are
// the system role names on the wire.
type RoleTemplate string

const (
	// RoleTemplateCaregiver copies the platform caregiver role ("Betreuung").
	RoleTemplateCaregiver RoleTemplate = "user"
	// RoleTemplateAdministration copies the admin role ("Verwaltung") without
	// its platform permissions.
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

// platformOnlyPermissions are the permissions that only mean something for
// the platform operator. The wildcards grant everything including the
// operator routes; system:manage is the operator's system permission; the
// permission catalog mutations are mounted behind the platform scope, so a
// school session can never use them.
var platformOnlyPermissions = []string{
	"*:*",
	"admin:*",
	"system:manage",
	"permissions:create",
	"permissions:update",
	"permissions:delete",
}

// IsPlatformOnlyPermission reports whether the resource:action name is a
// platform permission.
func IsPlatformOnlyPermission(name string) bool {
	return slices.Contains(platformOnlyPermissions, strings.ToLower(strings.TrimSpace(name)))
}

// ErrPlatformPermissionNotAllowed refuses a platform permission on a school
// role. Errors carrying the permission name wrap it as
// PlatformPermissionError.
var ErrPlatformPermissionNotAllowed = errors.New("platform permissions cannot be granted to a school role")

// PlatformPermissionError names the platform permission a selection asked for.
type PlatformPermissionError struct {
	Permission string
}

func (e *PlatformPermissionError) Error() string {
	return fmt.Sprintf("%s: %s", ErrPlatformPermissionNotAllowed.Error(), e.Permission)
}

// Is matches ErrPlatformPermissionNotAllowed.
func (e *PlatformPermissionError) Is(target error) bool {
	return target == ErrPlatformPermissionNotAllowed
}
