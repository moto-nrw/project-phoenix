package authorize

import (
	"context"
	"log/slog"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
)

// ResolveStudentReadScope is the list-level counterpart to CanReadStudent: it
// reports which students the caller may read across the tenant.
//
//   - allStudents == true: the caller may read every student in the tenant
//     (admin permissions, or gdpr.student_data_scope == "all_staff" and the
//     caller is a verified staff member). groupIDs is nil and irrelevant.
//   - allStudents == false: read access is limited to the returned education
//     group IDs (the groups the caller supervises). An empty slice means the
//     caller may read nothing.
//
// Read-only — like CanReadStudent it must not gate writes.
func ResolveStudentReadScope(
	ctx context.Context,
	userPermissions []string,
	userCtx StudentReadUserContext,
	settings StudentReadSettings,
	logger *slog.Logger,
) (allStudents bool, groupIDs []int64) {
	if HasAdminWildcard(userPermissions) {
		return true, nil
	}

	scope := resolveStudentDataScope(ctx, settings, logger)
	if scope == configModel.StudentDataScopeAllStaff && userCtx != nil {
		if staff, err := userCtx.GetCurrentStaff(ctx); err == nil && staff != nil {
			return true, nil
		}
	}

	if userCtx == nil {
		return false, nil
	}
	groups, err := userCtx.GetMyGroups(ctx)
	if err != nil {
		return false, nil
	}
	ids := make([]int64, 0, len(groups))
	for _, g := range groups {
		ids = append(ids, g.ID)
	}
	return false, ids
}
