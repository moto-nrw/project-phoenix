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
// GET /api/holiday-guests/{studentId}. GET /api/holiday-guests lists the hint
// „Gast aus …“ of the guests the school cares for today (#5630), and
// GET /api/holiday-guests/groups the holiday groups whose week has not begun
// (#6680). What a registration says about a child of the school itself
// (#6678) stands at GET /api/holiday-weeks/students/{studentId}/registration.
// The school and the person come from the session.

// NewHolidayHostGuestResource adds to the participant routes the group
// choice of a hosted week; requireAssign is the school permission that
// allows the choice.
func NewHolidayHostGuestResource(participants *HolidayHostResource, guests capability.HolidayGuests, requireAssign common.Middleware) *HolidayHostResource {
	if participants == nil || guests == nil || requireAssign == nil {
		panic("enrollment holiday host resource: the participant routes, the holiday guests and the permission are required")
	}
	resource := *participants
	resource.guests, resource.requireAssign = guests, requireAssign
	return &resource
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

// HolidayGuestResource serves the sheet of a holiday guest and the hints
// of the school's lists.
type HolidayGuestResource struct {
	guests      capability.HolidayGuests
	requireRead common.Middleware
	requireList common.Middleware
}

// NewHolidayGuestResource builds the routes. requireRead is the school
// permission that opens the contacts of a child, requireList the one that
// opens the child list, where the hints stand.
func NewHolidayGuestResource(guests capability.HolidayGuests, requireRead, requireList common.Middleware) *HolidayGuestResource {
	if guests == nil || requireRead == nil || requireList == nil {
		panic("enrollment holiday guest resource: the holiday guests and the permissions are required")
	}
	return &HolidayGuestResource{guests: guests, requireRead: requireRead, requireList: requireList}
}

// Router mounts the route behind the school session.
func (rs *HolidayGuestResource) Router(protectedTenantRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTenantRoutes(r, func(r chi.Router, withTx common.Middleware) {
		r.With(rs.requireList, withTx).Get("/", rs.listBadges)
		r.With(rs.requireList, withTx).Get("/groups", rs.listGroupHints)
		r.With(rs.requireRead, withTx).Get("/{studentId}", rs.getSheet)
	})
	return r
}

func (rs *HolidayGuestResource) listBadges(w http.ResponseWriter, r *http.Request) {
	host, ok := hostOf(w, r)
	if !ok {
		return
	}
	badges, err := rs.guests.HolidayGuestBadges(r.Context(), host.SchoolID)
	if err != nil {
		renderHolidayGuestError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, badges, "Holiday guest hints retrieved successfully")
}

// listGroupHints lists the holiday groups that wait for their week, with
// the number of guests that join them. It carries no personal data.
func (rs *HolidayGuestResource) listGroupHints(w http.ResponseWriter, r *http.Request) {
	host, ok := hostOf(w, r)
	if !ok {
		return
	}
	hints, err := rs.guests.HolidayGuestGroupHints(r.Context(), host.SchoolID)
	if err != nil {
		renderHolidayGuestError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, hints, "Holiday group hints retrieved successfully")
}

// getStudentRegistration returns what the confirmed registrations say about
// a child of the hosting school itself (#6678); the read is recorded.
func (rs *HolidayHostResource) getStudentRegistration(w http.ResponseWriter, r *http.Request) {
	host, ok := hostOf(w, r)
	if !ok {
		return
	}
	studentID, err := common.ParseIDParam(r, "studentId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	entries, err := rs.guests.HolidayStudentRegistrations(r.Context(), host, studentID)
	if err != nil {
		renderHolidayGuestError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, entries, "Holiday registration of the student retrieved successfully")
}

// listOwnChildAssignments says for each child of the school itself in a
// hosted week which student its registration was assigned to (#6678).
func (rs *HolidayHostResource) listOwnChildAssignments(w http.ResponseWriter, r *http.Request) {
	host, weekID, ok := hostedWeekKey(w, r)
	if !ok {
		return
	}
	assignments, err := rs.guests.HolidayOwnChildAssignments(r.Context(), host, weekID)
	if err != nil {
		renderHolidayGuestError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, assignments, "Own children of the holiday week retrieved successfully")
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
