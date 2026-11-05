package operator

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

// AuditTraegerSettingChange records the operator audit entry of a change of
// a Träger setting (#5748) before invoking the change. The Träger comes from
// the route parameter id, the key from key; reset marks the return to the
// default. Without the entry nothing changes.
func (rs *ProvisioningResource) AuditTraegerSettingChange(reset bool, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := sessiontoken.ClaimsFromCtx(r.Context())
		if claims.ID <= 0 || !claims.IsPlatformScope() {
			common.RenderError(w, r, common.OperatorUnauthorized())
			return
		}
		organizationID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid organization ID")
		if !ok {
			return
		}
		err := rs.service.RecordTraegerSettingChange(r.Context(), organizationtenancy.TraegerSettingChange{
			OperatorID: int64(claims.ID), OrganizationID: organizationID, SettingKey: chi.URLParam(r, "key"),
			Reset: reset, ClientIP: common.ParseClientIP(r),
		})
		if err != nil {
			common.RenderError(w, r, ProvisioningErrorRenderer(err))
			return
		}
		next(w, r)
	}
}
