package students

import (
	"context"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	reviewidentity "github.com/moto-nrw/project-phoenix/modules/identityaccess/requestreview"
)

// RequestReviewPrincipal resolves effective permissions at the HTTP identity
// boundary. Native owners consume facts without importing JWT machinery.
func RequestReviewPrincipal(ctx context.Context) reviewidentity.Principal {
	grants := jwt.PermissionsFromCtx(ctx)
	return reviewidentity.Principal{
		Admin:        jwt.ClaimsFromCtx(ctx).IsAdmin || authorize.HasAdminWildcard(grants),
		UsersUpdate:  authorize.HasPermission(permissions.UsersUpdate, grants),
		UsersRead:    authorize.HasPermission(permissions.UsersRead, grants),
		UsersAbsence: authorize.HasPermission(permissions.UsersAbsence, grants),
	}
}

func RequestReviewCorrectionAccess(ctx context.Context, currentStaff func(context.Context) (bool, error)) bool {
	return reviewidentity.CorrectionsAllowed(ctx, authorize.HasAdminWildcard(jwt.PermissionsFromCtx(ctx)), currentStaff)
}
