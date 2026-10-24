package api

import (
	"context"
	"net/http"

	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
	carePlanModule "github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanHTTP "github.com/moto-nrw/project-phoenix/modules/careplan/http"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// newBookingAuthorityImpact serves Care Plan's review before a school
// switches to booking-led care in the operator surface's wire format,
// inside the tenant transaction of the school in the path (#4072).
func newBookingAuthorityImpact(authority carePlanModule.BookingAuthority) http.Handler {
	return carePlanHTTP.NewBookingAuthorityImpact(carePlanHTTP.BookingAuthorityImpactConfig{
		Authority:    authority,
		WithinSchool: withinSchool,
		Surface: carePlanHTTP.Surface{
			InvalidRequest: apiCommon.ErrorInvalidRequest,
			Internal:       apiCommon.ErrorInternalServerWrap,
			RenderError:    apiCommon.RenderError,
			Respond:        apiCommon.Respond,
		},
	})
}

// withinSchool runs fn in the tenant transaction of the school an operator
// route names.
func withinSchool(ctx context.Context, schoolID int64, fn func(context.Context) error) error {
	id, err := tenant.NewTenantID(schoolID)
	if err != nil {
		return err
	}
	return tenant.WithinTenant(ctx, id, fn)
}

// operatorIDFromSession returns the authenticated operator of an operator
// route, or zero.
func operatorIDFromSession(ctx context.Context) int64 {
	return int64(sessiontoken.ClaimsFromCtx(ctx).ID)
}
