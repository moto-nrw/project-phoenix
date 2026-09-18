package auth

import (
	"context"
	"strings"
)

// ResolveSystemRoleByName looks up the platform system role with that name,
// matching case-insensitively; a school's own role never matches. Returns
// (nil, nil) when no system role has the name. Callers that grant a tier by
// name resolve it through here, so the "system role, no tenant" rule is
// stated once (#3364).
func ResolveSystemRoleByName(ctx context.Context, repo RoleRepository, name string) (*Role, error) {
	roles, err := repo.List(ctx, map[string]interface{}{
		"name":      strings.TrimSpace(strings.ToLower(name)),
		"is_system": true,
	})
	if err != nil {
		return nil, err
	}
	for _, role := range roles {
		if role == nil {
			continue
		}
		if role.TenantID == nil && role.IsSystem && strings.EqualFold(role.Name, name) {
			return role, nil
		}
	}
	return nil, nil
}
