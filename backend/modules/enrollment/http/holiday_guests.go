package enrollmenthttp

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// The holiday guests of the hosting school (#4666, ADR 0120): the school
// picks the group of a hosted week at PUT /api/holiday-weeks/{weekId}/guest-group,
// which copies the week's confirmed guests into it, and the staff read the
// sheet of a guest (Betreuung, Notfall, Abholung) at
// GET /api/holiday-guests/{studentId}. The school and the person come from
// the session.

// NewHolidayHostGuestResource adds to the participant routes the group
// choice of a hosted week; requireAssign is the school permission that
// allows the choice.
func NewHolidayHostGuestResource(participants *HolidayHostResource, guests capability.HolidayGuests, requireAssign common.Middleware) *HolidayHostResource {
	if participants == nil || guests == nil || requireAssign == nil {
		panic("enrollment holiday host resource: the participant routes, the holiday guests and the permission are required")
	}
	return &HolidayHostResource{
		participants: participants.participants, schoolNames: participants.schoolNames, requireRead: participants.requireRead,
		guests: guests, requireAssign: requireAssign,
	}
}

type holidayGuestGroupRequest struct {
	GroupID int64 `json:"group_id,string"`
}

func (req *holidayGuestGroupRequest) Bind(*http.Request) error {
	if req.GroupID <= 0 {
		return errors.New("group_id is required")
	}
	return nil
}

func (rs *HolidayHostResource) assignGuestGroup(w http.ResponseWriter, r *http.Request) {
	host, weekID, ok := hostedWeekKey(w, r)
	if !ok {
		return
	}
	request := &holidayGuestGroupRequest{}
	if err := render.Bind(r, request); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	transfer, err := rs.guests.AssignHolidayGuestGroup(r.Context(), host, weekID, request.GroupID)
	if err != nil {
		renderHolidayGuestError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, transfer, "Holiday guests transferred successfully")
}

// HolidayGuestResource serves the sheet of a holiday guest.
type HolidayGuestResource struct {
	guests      capability.HolidayGuests
	requireRead common.Middleware
}

// NewHolidayGuestResource builds the route. requireRead is the school
// permission that opens the contacts of a child.
func NewHolidayGuestResource(guests capability.HolidayGuests, requireRead common.Middleware) *HolidayGuestResource {
	if guests == nil || requireRead == nil {
		panic("enrollment holiday guest resource: the holiday guests and the permission are required")
	}
	return &HolidayGuestResource{guests: guests, requireRead: requireRead}
}

// Router mounts the route behind the school session.
func (rs *HolidayGuestResource) Router(protectedTenantRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTenantRoutes(r, func(r chi.Router, withTx common.Middleware) {
		r.With(rs.requireRead, withTx).Get("/{studentId}", rs.getSheet)
	})
	return r
}

func (rs *HolidayGuestResource) getSheet(w http.ResponseWriter, r *http.Request) {
	host, ok := hostOf(w, r)
	if !ok {
		return
	}
	studentID, err := common.ParseIDParam(r, "studentId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	sheet, err := rs.guests.HolidayGuestSheet(r.Context(), host, studentID)
	if err != nil {
		renderHolidayGuestError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, sheet, "Holiday guest retrieved successfully")
}

// renderHolidayGuestError answers a refused group choice or sheet read; the
// transaction rolls back with it.
func renderHolidayGuestError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, capability.ErrHolidayGuestNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeEnrollmentHolidayGuestNotFound))
	case errors.Is(err, capability.ErrHolidayGuestGroupInvalid):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeEnrollmentHolidayGuestGroupInvalid))
	default:
		renderHolidayHostError(w, r, err)
	}
}
