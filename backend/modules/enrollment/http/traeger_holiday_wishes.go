package enrollmenthttp

import (
	"net/http"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
)

// HolidayWishRequest is the body of PUT
// /traeger/holiday-periods/{id}/registrations/{registrationId}/children/{childId}/holiday-wish:
// the office's decision on the child's holiday care wish, "yes" or "no"
// (#4703).
type HolidayWishRequest struct {
	Wish string `json:"wish"`
}

// Bind accepts the body; the Owner checks the value.
func (req *HolidayWishRequest) Bind(*http.Request) error { return nil }

// setHolidayWish writes the office's decision and answers the changed
// registration.
func (rs *HolidayPeriodResource) setHolidayWish(w http.ResponseWriter, r *http.Request) {
	reviewer, periodID, registrationID, ok := registrationKey(w, r)
	if !ok {
		return
	}
	childID, err := common.ParseIDParam(r, "childId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	req := &HolidayWishRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	detail, err := rs.wishes.SetHolidayWish(r.Context(), reviewer, periodID, registrationID, childID, req.Wish)
	if err != nil {
		renderHolidayReviewError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, detail, "Holiday care wish saved")
}
