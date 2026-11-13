package presence

import (
	"net/http"

	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/modules/studentpresence"
)

func (rs *Resource) BulkStudentMoveAuthorization(w http.ResponseWriter, r *http.Request) (*studentpresence.StudentMoveAuthorization, bool) {
	if canBypassBulkMoveResourceChecks(r) {
		return &studentpresence.StudentMoveAuthorization{BypassResourceChecks: true}, true
	}

	staff, err := rs.extractStaffFromRequest(w, r)
	if err != nil {
		return nil, false
	}
	return rs.staffMoveAuthorization(r, staff), true
}

// staffMoveAuthorization is the move right of a staff member who is not an
// administrator: their supervisions, widened by the school-wide attendance
// scope when the school enables it.
func (rs *Resource) staffMoveAuthorization(r *http.Request, staff *StaffIdentity) *studentpresence.StudentMoveAuthorization {
	eligible := common.CanUseSchoolWideAttendance(r.Context(), staff.TenantID, rs.runtime.TenantID(r.Context()))
	return &studentpresence.StudentMoveAuthorization{StaffID: staff.ID, SchoolWideAttendanceEligible: eligible}
}

func canBypassBulkMoveResourceChecks(r *http.Request) bool {
	return common.HasEffectiveAdminScope(r.Context())
}

// CheckInMoveActor resolves the staff member a check-in into a session
// (#6953) is attributed to, and the move rights the placement revalidates.
// A check-in needs a staff record even for administrators: the attendance row
// names who checked the child in.
func (rs *Resource) CheckInMoveActor(w http.ResponseWriter, r *http.Request) (int64, *studentpresence.StudentMoveAuthorization, bool) {
	staff, err := rs.extractStaffFromRequest(w, r)
	if err != nil {
		return 0, nil, false
	}
	if canBypassBulkMoveResourceChecks(r) {
		return staff.ID, &studentpresence.StudentMoveAuthorization{StaffID: staff.ID, BypassResourceChecks: true}, true
	}
	return staff.ID, rs.staffMoveAuthorization(r, staff), true
}
