package timetracking

import (
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
)

// getOwnAbsenceAllowances handles GET /api/time-tracking/absence-allowances
// (#4971): the staff member's own Kontingente in the wire form of the
// Leitung's view. The staff member comes from the session, never from the
// request, so nobody reads another person's accounts here.
func (rs *Resource) getOwnAbsenceAllowances(w http.ResponseWriter, r *http.Request) {
	staffID, err := rs.getStaffIDFromClaims(r.Context(), rs.identity(r.Context()))
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	year, err := parseYearParam(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	accounts, err := rs.allowances.AllowanceAccounts(r.Context(), staffID, year)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, rs.allowanceResponses(accounts), "Absence allowances retrieved")
}
