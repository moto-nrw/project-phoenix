package operator

import (
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

// RecordBelegRetentionChange records the operator audit entry of a change of
// the Beleg retention of the Träger in the route parameter id (#4720, ADR
// 0184). The change calls it inside its administrative transaction, before
// it writes. It answers false after writing the error response, an unknown
// Träger included.
func (rs *ProvisioningResource) RecordBelegRetentionChange(w http.ResponseWriter, r *http.Request, kind, belegType string) bool {
	claims := sessiontoken.ClaimsFromCtx(r.Context())
	if claims.ID <= 0 || !claims.IsPlatformScope() {
		common.RenderError(w, r, common.OperatorUnauthorized())
		return false
	}
	organizationID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid organization ID")
	if !ok {
		return false
	}
	if rs.belegRetentionAudit == nil {
		common.RenderError(w, r, common.OperatorInternal("the Beleg retention audit is not configured"))
		return false
	}
	err := rs.belegRetentionAudit.RecordBelegRetentionChange(r.Context(), organizationtenancy.BelegRetentionChange{
		OperatorID: int64(claims.ID), OrganizationID: organizationID, Kind: kind, BelegType: belegType,
		ClientIP: common.ParseClientIP(r),
	})
	if err != nil {
		common.RenderError(w, r, ProvisioningErrorRenderer(err))
		return false
	}
	return true
}
