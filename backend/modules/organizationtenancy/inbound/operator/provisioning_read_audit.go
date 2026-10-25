package operator

import (
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

// AuditReadAccess records one bounded access attempt before invoking the reader.
// Only route IDs and the fixed view survive; query strings and payloads do not.
func (rs *ProvisioningResource) AuditReadAccess(view organizationtenancy.OperatorReadView, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := sessiontoken.ClaimsFromCtx(r.Context())
		if claims.ID <= 0 || !claims.IsPlatformScope() {
			common.RenderError(w, r, common.OperatorUnauthorized())
			return
		}
		access := organizationtenancy.OperatorReadAccess{View: view, OperatorID: int64(claims.ID), ClientIP: common.ParseClientIP(r)}
		if !operatorReadTarget(w, r, &access) {
			return
		}
		if err := rs.service.RecordOperatorRead(r.Context(), access); err != nil {
			common.RenderError(w, r, ProvisioningErrorRenderer(err))
			return
		}
		next(w, r)
	}
}

func operatorReadTarget(w http.ResponseWriter, r *http.Request, access *organizationtenancy.OperatorReadAccess) bool {
	var ok bool
	switch access.View {
	case organizationtenancy.ReadAnnouncementViews, organizationtenancy.ReadAnnouncement:
		access.AnnouncementID, ok = common.ParseInt64IDWithError(w, r, "id", "invalid announcement ID")
		if !ok {
			return false
		}
	case organizationtenancy.ReadSchoolAccounts, organizationtenancy.ReadSchoolPersons,
		organizationtenancy.ReadSchoolBookingAuthorityImpact,
		organizationtenancy.ReadSchool, organizationtenancy.ReadSchoolAdminInvitation,
		organizationtenancy.ReadSchoolAccountCreation, organizationtenancy.ReadSchoolSettingsSchema,
		organizationtenancy.ReadSchoolSettingReveal,
		organizationtenancy.ReadSchoolAccountMFA, organizationtenancy.ReadSchoolAccountCaregiver:
		access.SchoolID, ok = common.ParseInt64IDWithError(w, r, "id", "invalid school ID")
		if !ok {
			return false
		}
	case organizationtenancy.ReadOrganizationAccounts, organizationtenancy.ReadOrganizationPersons,
		organizationtenancy.ReadOrganizationSchools, organizationtenancy.ReadTraegerMembers:
		access.OrganizationID, ok = common.ParseInt64IDWithError(w, r, "id", "invalid organization ID")
		if !ok {
			return false
		}
	}
	switch access.View {
	case organizationtenancy.ReadSchoolAccountCaregiver, organizationtenancy.ReadSchoolAccountMFA,
		organizationtenancy.ReadAccountMFA, organizationtenancy.ReadAccountTenants:
		access.AccountID, ok = common.ParseInt64IDWithError(w, r, "accountId", "invalid account ID")
		return ok
	default:
		return true
	}
}
