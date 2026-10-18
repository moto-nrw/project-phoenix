package importpkg

import (
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/dataimport"
)

// importAuthorizationDouble supplies the grant outcomes used by the import
// decision fixtures. Security Runtime's own suites exercise the real policy.
type importAuthorizationDouble struct{}

func (importAuthorizationDouble) HasPermission(required string, granted []string) bool {
	for _, value := range granted {
		if value == required || value == "*:*" || value == "admin:*" {
			return true
		}
		if strings.HasSuffix(value, ":*") && strings.HasPrefix(required, strings.TrimSuffix(value, "*")) {
			return true
		}
	}
	return false
}
func (a importAuthorizationDouble) CanGrantRole(role dataimport.RoleGrantSource, granted []string) bool {
	if role == nil {
		return false
	}
	present, name, base, system, bound, permissions := role.AuthorizationGrantData()
	if !present {
		return false
	}
	if a.HasPermission("users:manage", granted) {
		return true
	}
	tier := ""
	if base != nil {
		tier = strings.ToLower(strings.TrimSpace(*base))
	}
	if tier == "" && system {
		tier = strings.ToLower(strings.TrimSpace(name))
	}
	if tier != "user" && tier != "guardian" && (tier != "" || !bound || base != nil) {
		return false
	}
	for _, value := range permissions {
		if value == "" || !a.HasPermission(value, granted) {
			return false
		}
	}
	return true
}
