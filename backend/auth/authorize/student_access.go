package authorize

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/models/active"
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

// StudentModifyUserContext is the narrow subset CanModifyStudent needs.
// Adds GetMyActiveGroups so spontaneous-session supervision counts the
// same way as it does for the reads.
type StudentModifyUserContext interface {
	StudentReadUserContext
	GetMyActiveGroups(ctx context.Context) ([]*active.Group, error)
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
	if HasAdminWildcard(userPermissions) {
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

// CanModifyStudent decides whether the caller may write to this student row
// (update, delete, photo change, ...). The scope setting only relaxes READS,
// so this gate is stricter than CanReadStudent: admin OR group supervisor
// only. The `operation` parameter shapes the human-readable error so the
// caller can map it to the right HTTP message ("update", "delete", ...).
//
// Returns (true, nil) when allowed; (false, error) with a stable error
// message otherwise — handlers map the error string to the user-facing
// 403 reason.
func CanModifyStudent(
	ctx context.Context,
	userPermissions []string,
	student *users.Student,
	userCtx StudentModifyUserContext,
	operation string,
) (bool, error) {
	if HasAdminWildcard(userPermissions) {
		return true, nil
	}
	if student == nil || student.GroupID == nil {
		return false, fmt.Errorf("only administrators can %s students without assigned groups", operation)
	}
	if userCtx == nil {
		return false, fmt.Errorf("insufficient permissions to %s this student's data", operation)
	}
	staff, err := userCtx.GetCurrentStaff(ctx)
	if err != nil || staff == nil {
		return false, fmt.Errorf("insufficient permissions to %s this student's data", operation)
	}
	if IsGroupSupervisor(ctx, *student.GroupID, userCtx) {
		return true, nil
	}
	return false, fmt.Errorf("you can only %s students in groups you supervise", operation)
}

// CanUpdateStudent is a convenience wrapper for update operations.
func CanUpdateStudent(
	ctx context.Context,
	userPermissions []string,
	student *users.Student,
	userCtx StudentModifyUserContext,
) (bool, error) {
	return CanModifyStudent(ctx, userPermissions, student, userCtx, "update")
}

// CanDeleteStudent is a convenience wrapper for delete operations.
func CanDeleteStudent(
	ctx context.Context,
	userPermissions []string,
	student *users.Student,
	userCtx StudentModifyUserContext,
) (bool, error) {
	return CanModifyStudent(ctx, userPermissions, student, userCtx, "delete")
}

// WritableStudentFilter returns a predicate reporting whether the caller may
// WRITE a given student, precomputing the caller's supervised-group set once so
// a caller-side loop (e.g. filtering a review queue) does not re-resolve it per
// student. The predicate is the set form of CanModifyStudent: admin → every
// student; otherwise only students in a group the caller supervises (persistent
// education group or matching active-session template). A student with no group
// is admin-only, exactly like CanModifyStudent.
//
// Deliberately does NOT consult gdpr.student_data_scope: that setting relaxes
// READS only. Both this filter and CanModifyStudent are write gates, so an
// "all_staff" read scope must never widen who can see/decide change requests.
func WritableStudentFilter(ctx context.Context, userPermissions []string, userCtx StudentModifyUserContext) func(*users.Student) bool {
	if HasAdminWildcard(userPermissions) {
		return func(*users.Student) bool { return true }
	}
	if userCtx == nil {
		return func(*users.Student) bool { return false }
	}
	allowed := make(map[int64]bool)
	if educationGroups, err := userCtx.GetMyGroups(ctx); err == nil {
		for _, g := range educationGroups {
			allowed[g.ID] = true
		}
	}
	if activeGroups, err := userCtx.GetMyActiveGroups(ctx); err == nil {
		for _, ag := range activeGroups {
			if templateID, ok := ag.TemplateID(); ok {
				allowed[templateID] = true
			}
		}
	}
	return func(student *users.Student) bool {
		return student != nil && student.GroupID != nil && allowed[*student.GroupID]
	}
}

// IsGroupSupervisor reports whether the caller supervises the given
// education group, counting both the persistent education-group assignment
// and any active group whose template ID matches. Spontaneous active
// sessions (no parent template) are skipped because they cannot match a
// caller-provided template ID.
func IsGroupSupervisor(ctx context.Context, groupID int64, userCtx StudentModifyUserContext) bool {
	if userCtx == nil {
		return false
	}
	if educationGroups, err := userCtx.GetMyGroups(ctx); err == nil {
		for _, g := range educationGroups {
			if g.ID == groupID {
				return true
			}
		}
	}
	if activeGroups, err := userCtx.GetMyActiveGroups(ctx); err == nil {
		for _, ag := range activeGroups {
			if templateID, ok := ag.TemplateID(); ok && templateID == groupID {
				return true
			}
		}
	}
	return false
}
