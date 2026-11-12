package timetracking

import (
	"errors"
	"net/http"

	"github.com/moto-ogs/project-phoenix/modules/workforce"

	"github.com/moto-ogs/project-phoenix/api/common"
)

// requireReadableStaffHistory accepts live and retained employments. The
// session write handlers require a live membership through their own gate.
func (rs *StaffAdminResource) requireReadableStaffHistory(w http.ResponseWriter, r *http.Request, staffID int64) bool {
	if _, err := rs.PersonService.StaffByID(r.Context(), staffID, workforce.StaffMembershipOnly); err == nil {
		return true
	} else if !common.IsNotFound(err) {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return false
	}
	if _, err := rs.PersonService.StaffStammdaten(r.Context(), staffID); err != nil {
		if common.IsNotFound(err) {
			common.RenderError(w, r, common.ErrorNotFound(errors.New("staff not found")))
		} else {
			common.RenderError(w, r, common.ErrorInternalServer(err))
		}
		return false
	}
	return true
}
