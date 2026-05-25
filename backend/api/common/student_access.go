// Package common - student_access.go
//
// Shared per-request access context used to decide whether the caller may see
// unredacted data for a given student. Centralised so handlers in different
// packages (students, active, …) can share one definition of "full access".
//
// The rules are:
//   - Admin permissions (`admin:*` or `*:*`)            → full access
//   - Tenant scope `gdpr.student_data_scope = all_staff` → full access for verified staff
//   - Otherwise                                          → full access only when the
//     caller supervises the student's education group
package common

import (
	"log/slog"
	"net/http"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/users"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/usercontext"
)

// StudentAccessContext caches the access decisions for a single request so
// per-student checks during list iteration stay O(1).
type StudentAccessContext struct {
	IsAdmin       bool
	AllStaffScope bool
	MyGroupIDs    map[int64]struct{}
}

// DetermineStudentAccess resolves the access context for the caller of r.
// Admin status comes from JWT permissions; the all_staff scope and supervised
// group set are looked up only for non-admin staff (other roles never receive
// unredacted access regardless of scope).
func DetermineStudentAccess(
	r *http.Request,
	userContextSvc usercontext.UserContextService,
	settingsSvc configService.SettingsService,
	logger *slog.Logger,
) *StudentAccessContext {
	ctx := &StudentAccessContext{
		IsAdmin: hasAdminPermission(jwt.PermissionsFromCtx(r.Context())),
	}

	if ctx.IsAdmin {
		return ctx
	}

	staff, err := userContextSvc.GetCurrentStaff(r.Context())
	if err != nil || staff == nil {
		return ctx
	}

	scope := configService.ResolveStringOrDefault(
		r.Context(),
		settingsSvc,
		configModel.KeyStudentDataScope,
		configModel.StudentDataScopeGroupSupervisorsOnly,
		logger,
	)
	ogsGroupsEnabled := configService.ResolveOGSGroupsEnabled(r.Context(), settingsSvc, logger)
	ctx.AllStaffScope = !ogsGroupsEnabled || scope == configModel.StudentDataScopeAllStaff

	if educationGroups, err := userContextSvc.GetMyGroups(r.Context()); err == nil {
		ctx.MyGroupIDs = make(map[int64]struct{}, len(educationGroups))
		for _, g := range educationGroups {
			ctx.MyGroupIDs[g.ID] = struct{}{}
		}
	}

	return ctx
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

// hasAdminPermission is intentionally inlined here so the common package does
// not depend on api/students. The wildcard tokens are stable across the app.
func hasAdminPermission(permissions []string) bool {
	for _, perm := range permissions {
		if perm == "admin:*" || perm == "*:*" {
			return true
		}
	}
	return false
}
