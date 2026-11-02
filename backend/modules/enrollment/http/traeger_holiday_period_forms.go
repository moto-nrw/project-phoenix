package enrollmenthttp

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The form of a holiday period in the Träger-Portal (#5198): the Träger-Büro
// reads the current version with the standard template and saves the next
// version. The routes hang at /traeger/holiday-periods/{id}/form behind the
// right to change the period and in its Träger transaction (ADR 0105).

// HolidayPeriodFormRequest is the body of PUT
// /traeger/holiday-periods/{id}/form: every question of the new version in
// its order.
type HolidayPeriodFormRequest struct {
	Fields      []capability.HolidayFormField `json:"fields"`
	LegalBlocks []capability.FormLegalBlock   `json:"legal_blocks"`
}

// Bind accepts the body; the Owner validates the form as a whole.
func (req *HolidayPeriodFormRequest) Bind(*http.Request) error { return nil }

func (rs *HolidayPeriodResource) getForm(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := traegerOf(w, r)
	if !ok {
		return
	}
	periodID, err := common.ParseIDParam(r, "id")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	form, err := rs.periods.HolidayPeriodForm(r.Context(), organizationID, periodID)
	if err != nil {
		renderHolidayFormError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, form, "Holiday period form retrieved successfully")
}

func (rs *HolidayPeriodResource) saveForm(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	periodID, err := common.ParseIDParam(r, "id")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	req := &HolidayPeriodFormRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	form, err := rs.periods.SaveHolidayPeriodForm(r.Context(), reviewer.OrganizationID, periodID, reviewer.AccountID, req.Fields, req.LegalBlocks)
	if err != nil {
		renderHolidayFormError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, form, "Holiday period form saved successfully")
}

// renderHolidayFormError answers a refused form operation. A rejected field
// names its path, such as "fields.2.type", and the code of its check.
func renderHolidayFormError(w http.ResponseWriter, r *http.Request, err error) {
	tenant.MarkRollback(r.Context())
	var rejected *capability.InvalidInputError
	switch {
	case errors.As(err, &rejected):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	case errors.Is(err, capability.ErrHolidayPeriodNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeEnrollmentHolidayPeriodNotFound))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}
