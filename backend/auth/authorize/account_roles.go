package authorize

import (
	"strings"
)

type accountRoleSource interface{ AuthorizationRoleNames() []string }

// AccountHasRole reports whether the account's loaded role relations include a
// role with the given name (case-insensitive). The decision lives here rather
// than on the model: role membership is an authorization concern, not a data
// fact on the entity (issue #586, Rule 12).
func AccountHasRole(account accountRoleSource, roleName string) bool {
	if account == nil {
		return false
	}
	for _, role := range account.AuthorizationRoleNames() {
		if strings.EqualFold(role, roleName) {
			return true
		}
	}
	return false
}
