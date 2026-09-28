package students

import (
	"context"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	reviewidentity "github.com/moto-nrw/project-phoenix/modules/identityaccess/requestreview"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
)

// RequestReviewPrincipal resolves effective permissions at the HTTP identity
// boundary. Native owners consume facts without importing JWT machinery.
func RequestReviewPrincipal(ctx context.Context) reviewidentity.Principal {
	grants := jwt.PermissionsFromCtx(ctx)
	return reviewidentity.Principal{
		Admin:        jwt.ClaimsFromCtx(ctx).IsAdmin || securityruntime.HasAdminWildcard(grants),
		UsersUpdate:  securityruntime.HasPermission(permissions.UsersUpdate, grants),
		UsersRead:    securityruntime.HasPermission(permissions.UsersRead, grants),
		UsersAbsence: securityruntime.HasPermission(permissions.UsersAbsence, grants),
	}
}

func RequestReviewCorrectionAccess(ctx context.Context, currentStaff func(context.Context) (bool, error)) bool {
	return reviewidentity.CorrectionsAllowed(ctx, securityruntime.HasAdminWildcard(jwt.PermissionsFromCtx(ctx)), currentStaff)
}
