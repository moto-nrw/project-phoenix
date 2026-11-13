package operator

import (
	"net/http"

	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy"
)

// RecordTraegerSettingChange records the operator audit entry of a change of
// key of the Träger in the route parameter id (#5748); reset marks the return
// to the default. The settings change calls it after its checks and before
// it writes. It answers false after writing the error response, an unknown
// Träger included.
func (rs *ProvisioningResource) RecordTraegerSettingChange(w http.ResponseWriter, r *http.Request, key string, reset bool) bool {
	claims := sessiontoken.ClaimsFromCtx(r.Context())
	if claims.ID <= 0 || !claims.IsPlatformScope() {
		common.RenderError(w, r, common.OperatorUnauthorized())
		return false
	}
	organizationID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid organization ID")
	if !ok {
		return false
	}
	err := rs.service.RecordTraegerSettingChange(r.Context(), organizationtenancy.TraegerSettingChange{
		OperatorID: int64(claims.ID), OrganizationID: organizationID, SettingKey: key,
		Reset: reset, ClientIP: common.ParseClientIP(r),
	})
	if err != nil {
		common.RenderError(w, r, ProvisioningErrorRenderer(err))
		return false
	}
	return true
}
