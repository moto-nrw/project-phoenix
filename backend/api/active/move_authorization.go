package active

import (
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

func (rs *Resource) bulkStudentMoveAuthorization(w http.ResponseWriter, r *http.Request) (*studentpresence.StudentMoveAuthorization, bool) {
	if canBypassBulkMoveResourceChecks(r) {
		return &studentpresence.StudentMoveAuthorization{BypassResourceChecks: true}, true
	}

	staff, err := rs.extractStaffFromRequest(w, r)
	if err != nil {
		return nil, false
	}

	eligible := common.CanUseSchoolWideAttendance(r.Context(), staff.TenantID, rs.runtime.TenantID(r.Context()))
	return &studentpresence.StudentMoveAuthorization{
		StaffID: staff.ID, SchoolWideAttendanceEligible: eligible,
	}, true
}

func canBypassBulkMoveResourceChecks(r *http.Request) bool {
	return common.HasEffectiveAdminScope(r.Context())
}
