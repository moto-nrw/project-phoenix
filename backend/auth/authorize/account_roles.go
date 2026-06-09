package authorize

import (
	"strings"

	"github.com/moto-nrw/project-phoenix/models/auth"
)

// AccountHasRole reports whether the account's loaded role relations include a
// role with the given name (case-insensitive). The decision lives here rather
// than on the model: role membership is an authorization concern, not a data
// fact on the entity (issue #586, Rule 12).
func AccountHasRole(account *auth.Account, roleName string) bool {
	if account == nil {
		return false
	}
	for _, role := range account.Roles {
		if role != nil && strings.EqualFold(role.Name, roleName) {
			return true
		}
	}
	return false
}

// AccountHasPermission reports whether the account's loaded permission
// relations include a permission with the given name (case-insensitive). Use
// HasPermission (the wildcard-aware []string variant) for JWT-claim checks;
// this variant operates on the eagerly-loaded relation set.
func AccountHasPermission(account *auth.Account, permission string) bool {
	if account == nil {
		return false
	}
	for _, p := range account.Permissions {
		if p != nil && strings.EqualFold(p.Name, permission) {
			return true
		}
	}
	return false
}
