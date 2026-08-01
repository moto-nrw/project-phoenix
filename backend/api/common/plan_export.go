package common

import (
	"context"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
)

// CanExportInternalPlan reports whether a caller may receive plan details
// that are deliberately absent from the wall-sheet variant, such as absence
// and cancellation reasons.
func CanExportInternalPlan(ctx context.Context) bool {
	return authorize.HasPermission(permissions.SchedulesManage, jwt.PermissionsFromCtx(ctx))
}
