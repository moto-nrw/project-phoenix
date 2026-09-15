package users

import "github.com/moto-nrw/project-phoenix/auth/authorize/permissions"

// DefaultStaffAccountPermission is granted to the login account of a newly
// created staff member so the colleague sees the group list. Lehrkraft
// accounts are excluded by the grant flow (#1772).
const DefaultStaffAccountPermission = permissions.GroupsRead
