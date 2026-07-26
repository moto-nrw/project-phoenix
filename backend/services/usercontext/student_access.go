package usercontext

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/users"
	configService "github.com/moto-nrw/project-phoenix/services/config"
)

// StudentAccessContext caches the per-request access decisions so per-student
// checks during list iteration stay O(1).
//
// The rules are:
//   - Admin permissions (`admin:*` or `*:*`)             → full access
//   - Tenant scope `gdpr.student_data_scope = all_staff` → full access for verified staff
//   - Otherwise                                          → full access only when the
//     caller supervises the student's education group
type StudentAccessContext struct {
	IsAdmin       bool
	AllStaffScope bool
	MyGroupIDs    map[int64]struct{}
}

// ResolveStudentAccess resolves the access context for the current caller.
// Admin status comes from JWT permissions; the all_staff scope and supervised
// group set are looked up only for non-admin staff (other roles never receive
// unredacted access regardless of scope).
func ResolveStudentAccess(
	ctx context.Context,
	userCtx UserContextService,
	settings configService.SettingsService,
	logger *slog.Logger,
) *StudentAccessContext {
	access := &StudentAccessContext{
		IsAdmin: authorize.HasAdminWildcard(jwt.PermissionsFromCtx(ctx)),
	}

	if access.IsAdmin {
		return access
	}

	staff, err := userCtx.GetCurrentStaff(ctx)
	if err != nil || staff == nil {
		return access
	}

	scope := configService.ResolveStringOrDefault(
		ctx,
		settings,
		configModel.KeyStudentDataScope,
		configModel.StudentDataScopeGroupSupervisorsOnly,
		logger,
	)
	access.AllStaffScope = scope == configModel.StudentDataScopeAllStaff

	if educationGroups, err := userCtx.GetMyGroups(ctx); err == nil {
		access.MyGroupIDs = make(map[int64]struct{}, len(educationGroups))
		for _, g := range educationGroups {
			access.MyGroupIDs[g.ID] = struct{}{}
		}
	}

	return access
}

// HasFullAccessByGroupID returns true when the caller may see unredacted data
// for a student whose education group_id is the given pointer (nil = no group).
// Group-less students are accessible only to admins / all_staff scope callers.
func (a *StudentAccessContext) HasFullAccessByGroupID(groupID *int64) bool {
	if a == nil {
		return false
	}
	if a.IsAdmin || a.AllStaffScope {
		return true
	}
	if groupID == nil || a.MyGroupIDs == nil {
		return false
	}
	_, ok := a.MyGroupIDs[*groupID]
	return ok
}

// HasFullAccessToStudent is a convenience wrapper around HasFullAccessByGroupID
// for callers that already hold the full *users.Student.
func (a *StudentAccessContext) HasFullAccessToStudent(student *users.Student) bool {
	if student == nil {
		return false
	}
	return a.HasFullAccessByGroupID(student.GroupID)
}
