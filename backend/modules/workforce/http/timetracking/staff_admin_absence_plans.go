package timetracking

import (
	"net/http"

	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/modules/workforce"
)

// absencePlanSyncResponse tells whether the absence took its plan effect;
// false means it is no longer in force.
type absencePlanSyncResponse struct {
	Applied bool `json:"applied"`
}

// listAbsencePlanGaps handles GET /api/staff/absences/plan-gaps: the
// absences in force whose shifts and blocks from today on still lack their
// effect (#7021).
func (rs *StaffAdminResource) listAbsencePlanGaps(w http.ResponseWriter, r *http.Request) {
	gaps, err := rs.AbsenceWrites.AbsencePlanGaps(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	if gaps == nil {
		gaps = []workforce.AbsencePlanGap{}
	}
	common.Respond(w, r, http.StatusOK, gaps, "Absence plan gaps retrieved")
}

// syncAbsencePlan handles POST /api/staff/absences/{absenceId}/plan-sync:
// the absence takes its effect on the plans (#7021). Shifts an admin
// cancelled, covers and rows already absent stay as planned.
func (rs *StaffAdminResource) syncAbsencePlan(w http.ResponseWriter, r *http.Request) {
	absenceID, err := parseInt64Param(r, "absenceId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	actorStaffID, err := rs.resolveEditorStaffID(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	accountID := rs.identity(r.Context()).AccountID
	applied, err := rs.AbsenceWrites.SyncAbsencePlan(r.Context(), absenceID, actorStaffID, &accountID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, absencePlanSyncResponse{Applied: applied}, "Absence plan synchronized")
}
