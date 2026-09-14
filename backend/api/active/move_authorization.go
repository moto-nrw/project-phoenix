package active

import (
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
)

func (rs *Resource) bulkStudentMoveAuthorization(w http.ResponseWriter, r *http.Request) (*activeSvc.StudentMoveAuthorization, bool) {
	if canBypassBulkMoveResourceChecks(r) {
		return &activeSvc.StudentMoveAuthorization{BypassResourceChecks: true}, true
	}

	staff, err := rs.extractStaffFromRequest(w, r)
	if err != nil {
		return nil, false
	}

	eligible := common.CanUseSchoolWideAttendance(r.Context(), staff.TenantID, rs.runtime.TenantID(r.Context()))
	return &activeSvc.StudentMoveAuthorization{
		StaffID: staff.ID, SchoolWideAttendanceEligible: eligible,
	}, true
}

func canBypassBulkMoveResourceChecks(r *http.Request) bool {
	return common.HasEffectiveAdminScope(r.Context())
}
