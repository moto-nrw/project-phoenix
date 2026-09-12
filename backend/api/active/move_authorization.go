package active

import (
	"net/http"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/tenant"
)

func (rs *Resource) bulkStudentMoveAuthorization(w http.ResponseWriter, r *http.Request) (*activeSvc.StudentMoveAuthorization, bool) {
	if canBypassBulkMoveResourceChecks(r) {
		return &activeSvc.StudentMoveAuthorization{BypassResourceChecks: true}, true
	}

	staff, err := rs.extractStaffFromRequest(w, r)
	if err != nil {
		return nil, false
	}

	claims := jwt.ClaimsFromCtx(r.Context())
	ogsScope := claims.Scope == "" || claims.Scope == "tenant" || claims.Scope == "org"
	eligible := ogsScope && claims.ID > 0 && claims.TenantID > 0 &&
		claims.TenantID == tenant.FromContext(r.Context()) && staff.TenantID == claims.TenantID &&
		authorize.HasPermission(permissions.VisitsUpdate, jwt.PermissionsFromCtx(r.Context()))
	return &activeSvc.StudentMoveAuthorization{
		StaffID: staff.ID, SchoolWideAttendanceEligible: eligible,
	}, true
}

func canBypassBulkMoveResourceChecks(r *http.Request) bool {
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.IsAdmin {
		return true
	}
	return authorize.HasAdminWildcard(jwt.PermissionsFromCtx(r.Context()))
}
