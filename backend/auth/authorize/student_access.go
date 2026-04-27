package authorize

import (
	"context"
	"log/slog"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// StudentReadUserContext is the narrow subset of the user-context service that
// CanReadStudent needs: identify the caller's staff record (if any) and list
// the education groups they supervise. Defined here (not imported) to keep
// this package a sibling of services/usercontext without a package cycle.
type StudentReadUserContext interface {
	GetCurrentStaff(ctx context.Context) (*users.Staff, error)
	GetMyGroups(ctx context.Context) ([]*education.Group, error)
}

// StudentReadSettings is the narrow subset of the settings service CanReadStudent
// needs: check whether a tenant override exists for the data-scope setting and
// resolve its string value. Declared locally to avoid importing services/config,
// which already imports this package.
type StudentReadSettings interface {
	HasTenantOverride(ctx context.Context, key string) (bool, error)
	ResolveString(ctx context.Context, key string) (string, error)
}

// CanReadStudent decides whether the caller is allowed to see unredacted
// student data (profile, location, pickup/arrival schedules, day plan).
//
// Order:
//  1. Admin permissions (admin:* / *:*) → always true.
//  2. Tenant setting gdpr.student_data_scope == "all_staff" AND caller is a
//     verified staff member → true. Other authenticated roles (guest,
//     guardian) with users:read must NOT pass this gate.
//  3. Caller supervises the student's education group → true.
//  4. Otherwise → false.
//
// Write operations must NOT use this function — the scope setting only
// relaxes READ access. Use isGroupSupervisorOrAdmin / canModifyStudent for
// writes.
func CanReadStudent(
	ctx context.Context,
	userPermissions []string,
	student *users.Student,
	userCtx StudentReadUserContext,
	settings StudentReadSettings,
	logger *slog.Logger,
) bool {
	if student == nil {
		return false
	}
	if hasAdminPermissions(userPermissions) {
		return true
	}

	scope := resolveStudentDataScope(ctx, settings, logger)
	if scope == configModel.StudentDataScopeAllStaff && userCtx != nil {
		if staff, err := userCtx.GetCurrentStaff(ctx); err == nil && staff != nil {
			return true
		}
	}

	if student.GroupID == nil || userCtx == nil {
		return false
	}

	educationGroups, err := userCtx.GetMyGroups(ctx)
	if err != nil {
		return false
	}
	for _, g := range educationGroups {
		if g.ID == *student.GroupID {
			return true
		}
	}
	return false
}

// hasAdminPermissions checks for the wildcard admin scopes. Mirrors the
// package-private helper in api/students so the extracted helper carries its
// own authority check and does not depend on the caller pre-filtering.
func hasAdminPermissions(permissions []string) bool {
	for _, p := range permissions {
		if p == "admin:*" || p == "*:*" {
			return true
		}
	}
	return false
}

// resolveStudentDataScope returns the effective gdpr.student_data_scope for
// the caller's tenant. Falls back to the restrictive default when there is
// no override, the settings service is nil, or the lookup errs — the
// permissive "all_staff" setting must only be honored when explicitly set.
func resolveStudentDataScope(ctx context.Context, settings StudentReadSettings, logger *slog.Logger) string {
	fallback := configModel.StudentDataScopeGroupSupervisorsOnly
	if settings == nil {
		return fallback
	}
	has, err := settings.HasTenantOverride(ctx, configModel.KeyStudentDataScope)
	if err != nil {
		if logger != nil {
			logger.Warn("settings override check failed in CanReadStudent",
				slog.String("key", configModel.KeyStudentDataScope),
				slog.String("error", err.Error()),
			)
		}
		return fallback
	}
	if !has {
		return fallback
	}
	val, err := settings.ResolveString(ctx, configModel.KeyStudentDataScope)
	if err != nil || val == "" {
		if err != nil && logger != nil {
			logger.Warn("settings resolve failed in CanReadStudent",
				slog.String("key", configModel.KeyStudentDataScope),
				slog.String("error", err.Error()),
			)
		}
		return fallback
	}
	return val
}
