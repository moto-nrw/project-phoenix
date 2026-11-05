package parentbillinghttp

import (
	"net/http"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	capability "github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The sibling groups of a period (#5628) at
// /traeger/holiday-contributions/{periodId}/sibling-groups: the list, a
// merge and a split. They sit behind the right "Anmeldung verwalten" like
// the confirmation.

// HolidaySiblingMergeRequest is the body of POST
// /traeger/holiday-contributions/{periodId}/sibling-groups: the
// registrations to merge as siblings.
type HolidaySiblingMergeRequest struct {
	RegistrationIDs []int64 `json:"registration_ids"`
}

// Bind accepts the body; the Owner checks the registrations.
func (req *HolidaySiblingMergeRequest) Bind(*http.Request) error { return nil }

// periodKey reads the session and the period ID.
func periodKey(w http.ResponseWriter, r *http.Request) (reviewer capability.HolidayReviewer, periodID int64, ok bool) {
	reviewer, ok = reviewerOf(w, r)
	if !ok {
		return reviewer, 0, false
	}
	periodID, err := common.ParseIDParam(r, "periodId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return reviewer, 0, false
	}
	return reviewer, periodID, true
}

func (rs *HolidayResource) listSiblingGroups(w http.ResponseWriter, r *http.Request) {
	reviewer, periodID, ok := periodKey(w, r)
	if !ok {
		return
	}
	groups, err := rs.contributions.ListHolidaySiblingGroups(r.Context(), reviewer, periodID)
	if err != nil {
		renderHolidayError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, groups, "Holiday sibling groups retrieved successfully")
}

// mergeSiblings merges the registrations as siblings. Any refusal rolls the
// Träger transaction back, so no group stands without its contributions.
func (rs *HolidayResource) mergeSiblings(w http.ResponseWriter, r *http.Request) {
	reviewer, periodID, ok := periodKey(w, r)
	if !ok {
		return
	}
	req := &HolidaySiblingMergeRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	group, err := rs.contributions.MergeHolidaySiblings(r.Context(), reviewer, periodID, req.RegistrationIDs)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderHolidayError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, group, "Holiday siblings merged successfully")
}

// splitSibling takes the registration out of its sibling group. Any refusal
// rolls the Träger transaction back.
func (rs *HolidayResource) splitSibling(w http.ResponseWriter, r *http.Request) {
	reviewer, periodID, registrationID, ok := registrationKey(w, r)
	if !ok {
		return
	}
	if err := rs.contributions.SplitHolidaySibling(r.Context(), reviewer, periodID, registrationID); err != nil {
		tenant.MarkRollback(r.Context())
		renderHolidayError(w, r, err)
		return
	}
	common.RespondNoContent(w, r)
}
