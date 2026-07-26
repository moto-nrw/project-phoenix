package staff

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
)

// Personnel number maintenance (#1417 Tranche 2b). Gated on
// time_tracking:manage like every other payroll-relevant per-person write of
// #1417 — deliberately NOT users:update: users:read/update is held by people
// who maintain the staff directory, the Personalnummer feeds payroll.

// PayrollNumberRequest sets or clears the Personalnummer. Note is optional
// context for the audit trail.
type PayrollNumberRequest struct {
	PersonnelNumber json.RawMessage `json:"personnel_number"`
	Note            string          `json:"note"`

	personnelNumberValue *string
}

func (p *PayrollNumberRequest) Bind(_ *http.Request) error {
	if len(p.PersonnelNumber) == 0 {
		return errors.New("personnel_number is required")
	}
	if err := json.Unmarshal(p.PersonnelNumber, &p.personnelNumberValue); err != nil {
		return errors.New("personnel_number must be a string or null")
	}
	return nil
}

// PayrollNumberResponse mirrors the stored state after a read or write.
type PayrollNumberResponse struct {
	StaffID         int64   `json:"staff_id"`
	PersonnelNumber *string `json:"personnel_number"`
}

// getPayrollNumber returns the staff member's Personalnummer. A separate
// sub-resource instead of a field on GET /{id}: that endpoint is users:read,
// and the payroll identifier stays behind time_tracking:manage.
func (rs *Resource) getPayrollNumber(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", common.MsgInvalidStaffID)
	if !ok {
		return
	}
	staff, err := rs.PersonService.GetStaffByID(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(err))
		return
	}
	common.Respond(w, r, http.StatusOK, &PayrollNumberResponse{StaffID: staff.ID, PersonnelNumber: staff.PersonnelNumber}, "Personnel number retrieved successfully")
}

// updatePayrollNumber sets or clears the Personalnummer. The service writes
// the audit row in the same tenant transaction and maps duplicates to a
// stable conflict code.
func (rs *Resource) updatePayrollNumber(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", common.MsgInvalidStaffID)
	if !ok {
		return
	}
	req := &PayrollNumberRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	changedBy, err := rs.resolveEditorStaffID(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	staff, err := rs.PersonService.UpdatePersonnelNumber(r.Context(), id, req.personnelNumberValue, changedBy, req.Note)
	if err != nil {
		switch {
		case errors.Is(err, usersSvc.ErrPersonnelNumberInvalid):
			common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, "personnel_number_invalid"))
		case errors.Is(err, usersSvc.ErrPersonnelNumberTaken):
			common.RenderError(w, r, common.ErrorConflictWithCode(err, "personnel_number_taken"))
		case modelBase.IsNoRows(err):
			common.RenderError(w, r, common.ErrorNotFound(err))
		default:
			common.RenderError(w, r, common.ErrorInternalServer(err))
		}
		return
	}
	common.Respond(w, r, http.StatusOK, &PayrollNumberResponse{StaffID: staff.ID, PersonnelNumber: staff.PersonnelNumber}, "Personnel number updated successfully")
}
