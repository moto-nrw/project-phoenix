package dataimport

import "github.com/moto-nrw/project-phoenix/modules/identityaccess"

// SchoolRole and SchoolRoleQuery name the owner's public role facts and reads.
type SchoolRole = identityaccess.SchoolRole
type SchoolRoleQuery = identityaccess.SchoolRoleQuery

var ErrRoleNotFound = identityaccess.ErrRoleNotFound

// SchoolRolePolicy keeps assignment and caregiver-profile decisions with the
// Identity & Access owner, independently of the role lookup.
type SchoolRolePolicy interface {
	Validate(*SchoolRole, int64) error
	NeedsCaregiver(*SchoolRole) bool
}
