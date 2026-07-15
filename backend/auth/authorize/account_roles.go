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
