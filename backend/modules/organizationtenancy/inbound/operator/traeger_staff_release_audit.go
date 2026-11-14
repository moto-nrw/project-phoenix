package operator

import (
	"net/http"

	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy"
)

// RecordTraegerStaffReleaseChange records the operator audit entry of a
// grant or withdrawal of the release „Personal beim Träger“ of the Träger in
// the route parameter id (#4705, ADR 0089 section 2). The change calls it
// inside its administrative transaction, before it writes. It answers false
// after writing the error response, an unknown Träger included.
func (rs *ProvisioningResource) RecordTraegerStaffReleaseChange(w http.ResponseWriter, r *http.Request, kind, representation string) bool {
	claims := sessiontoken.ClaimsFromCtx(r.Context())
	if claims.ID <= 0 || !claims.IsPlatformScope() {
		common.RenderError(w, r, common.OperatorUnauthorized())
		return false
	}
	organizationID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid organization ID")
	if !ok {
		return false
	}
	if rs.traegerStaffReleaseAudit == nil {
		common.RenderError(w, r, common.OperatorInternal("the staff release audit is not configured"))
		return false
	}
	err := rs.traegerStaffReleaseAudit.RecordTraegerStaffReleaseChange(r.Context(), organizationtenancy.TraegerStaffReleaseChange{
		OperatorID: int64(claims.ID), OrganizationID: organizationID, Kind: kind, Representation: representation,
		ClientIP: common.ParseClientIP(r),
	})
	if err != nil {
		common.RenderError(w, r, ProvisioningErrorRenderer(err))
		return false
	}
	return true
}
