package enrollmenthttp

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// HolidayClosureResource serves the preview of a closing day the Träger-Büro
// plans, changes or deletes (#6434, #6457) at /traeger/holiday-closures. It
// runs in the Träger transaction of the session's Träger and needs the right
// that plans closing days. The notices of school closing days nobody told
// yet are the Holiday Closure Notification workflow's (ADR 0196).
type HolidayClosureResource struct {
	closures      capability.HolidayClosures
	requireManage common.Middleware
}

// NewHolidayClosureResource builds the resource; requireManage is the right
// "Einstellungen verwalten", which plans closing days.
func NewHolidayClosureResource(closures capability.HolidayClosures, requireManage common.Middleware) *HolidayClosureResource {
	if closures == nil || requireManage == nil {
		panic("enrollment http: the holiday closures and the right check are required")
	}
	return &HolidayClosureResource{closures: closures, requireManage: requireManage}
}

// TraegerRouter returns the routes mounted at /traeger/holiday-closures.
func (rs *HolidayClosureResource) TraegerRouter(protectedTraegerRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTraegerRoutes(r, func(r chi.Router, withTx common.Middleware) {
		manage := r.With(rs.requireManage, withTx)
		manage.Post("/preview", rs.preview)
	})
	return r
}

// HolidayClosurePreviewRequest is the body of POST /preview: the closing day
// as the Büro is about to save it, and for a change or a deletion the saved
// closing day with its stored version, whose days may open again (#6457).
type HolidayClosurePreviewRequest struct {
	StartDate    calendar.Date                  `json:"start_date"`
	EndDate      calendar.Date                  `json:"end_date"`
	Reason       string                         `json:"reason"`
	SchoolIDs    []int64                        `json:"school_ids"`
	ClosingDayID int64                          `json:"closing_day_id"`
	Previous     *capability.HolidayClosureSpan `json:"previous"`
	Delete       bool                           `json:"delete"`
}

// Bind accepts the body; the dates are checked by the handler.
func (req *HolidayClosurePreviewRequest) Bind(*http.Request) error { return nil }

func (rs *HolidayClosureResource) preview(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := traegerOf(w, r)
	if !ok {
		return
	}
	req := &HolidayClosurePreviewRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeTraegerClosingDayInvalid))
		return
	}
	if !req.Delete && (req.StartDate.IsZero() || req.EndDate.IsZero() || req.EndDate.Before(req.StartDate)) {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(errors.New("start and end date are required, the end not before the start"), common.CodeTraegerClosingDayInvalid))
		return
	}
	impact, err := rs.closures.HolidayClosureImpact(r.Context(), organizationID, capability.HolidayClosure{
		SchoolIDs: req.SchoolIDs, StartsOn: req.StartDate, EndsOn: req.EndDate, Reason: req.Reason,
		TraegerClosingDayID: req.ClosingDayID, Previous: req.Previous, Deleted: req.Delete,
	})
	if err != nil {
		renderHolidayClosureError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, impact, "Holiday bookings of the closing day retrieved successfully")
}

// renderHolidayClosureError names a refusal; every refusal comes before a
// write.
func renderHolidayClosureError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, capability.ErrHolidayHostSchoolOutsideTraeger):
		common.RenderError(w, r, common.ErrorInvalidOnField(err, common.CodeTraegerClosingDaySchoolOutside, "school_ids"))
	case errors.Is(err, capability.ErrHolidayClosureChanged):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeTraegerClosingDayBookingsChanged))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}
