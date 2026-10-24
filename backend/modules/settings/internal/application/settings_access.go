package config

import (
	"context"
	"strings"

	config "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// checkWritePermission verifies the caller has the required write permission.
// If userPermissions is nil, the check is skipped (system-level callers).
// Uses wildcard-aware matching (e.g. "admin:*" grants "config:update").
func checkWritePermission(def *config.Definition, userPermissions []string) error {
	if userPermissions == nil || def.WritePermission == "" {
		return nil
	}
	if hasPermission(def.WritePermission, userPermissions) {
		return nil
	}
	return &config.PermissionDeniedError{Key: def.Key, RequiredPermission: def.WritePermission}
}

func hasPermission(required string, permissions []string) bool {
	if required == "" {
		return true
	}
	requiredParts := strings.Split(required, ":")
	if len(requiredParts) != 2 {
		return false
	}
	for _, permission := range permissions {
		if permission == "admin:*" || permission == "*:*" {
			return true
		}
		parts := strings.Split(permission, ":")
		if len(parts) != 2 {
			continue
		}
		if permissionPartMatches(parts[0], requiredParts[0]) && permissionPartMatches(parts[1], requiredParts[1]) {
			return true
		}
	}
	return false
}

func permissionPartMatches(pattern, required string) bool {
	if pattern == required || pattern == "*" {
		return true
	}
	return strings.HasSuffix(pattern, "*") && strings.HasPrefix(required, strings.TrimSuffix(pattern, "*"))
}

func (s *settingsService) tenantID(ctx context.Context) int64 {
	if s.runtime == nil {
		return 0
	}
	return s.runtime.TenantID(ctx)
}

// GetSchema delegates to the schema builder for tenant admin callers.
func (s *settingsService) GetSchema(ctx context.Context, userPermissions []string) (*config.SettingsSchema, error) {
	return buildSchema(ctx, s, userPermissions)
}

// GetSchemaForOperator delegates to the operator-scoped schema builder.
func (s *settingsService) GetSchemaForOperator(ctx context.Context, userPermissions []string) (*config.SettingsSchema, error) {
	return buildSchemaForOperator(ctx, s, userPermissions)
}
