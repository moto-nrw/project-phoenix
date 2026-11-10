package staffcalendarhttp

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/traegercalendar"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// TraegerClosingDayResource serves the closing days of a Träger in the
// Träger-Portal (#4702) at /traeger/closing-days. Every route sits behind the
// Träger session and runs in the Träger transaction of the session's Träger
// (ADR 0105); the Träger comes from the session, never from the request.
// Reading needs the right "Übersicht sehen"; planning needs the right
// "Einstellungen verwalten", because a closing day takes the Soll of every
// named school's staff.
type TraegerClosingDayResource struct {
	days          traegercalendar.ClosingDays
	requireRead   common.Middleware
	requireManage common.Middleware
}

// NewTraegerClosingDayResource builds the resource; the root binds the right
// checks to the rights Identity & Access bundles into the Träger roles.
func NewTraegerClosingDayResource(days traegercalendar.ClosingDays, requireRead, requireManage common.Middleware) *TraegerClosingDayResource {
	if days == nil || requireRead == nil || requireManage == nil {
		panic("school calendar http: the Träger closing days and the right checks are required")
	}
	return &TraegerClosingDayResource{days: days, requireRead: requireRead, requireManage: requireManage}
}

// TraegerRouter returns the routes mounted at /traeger/closing-days.
func (rs *TraegerClosingDayResource) TraegerRouter(protectedTraegerRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTraegerRoutes(r, func(r chi.Router, withTx common.Middleware) {
		r.With(rs.requireRead, withTx).Get("/", rs.calendar)
		manage := r.With(rs.requireManage, withTx)
		manage.Post("/", rs.create)
		manage.Put("/{id}", rs.update)
		manage.Delete("/{id}", rs.delete)
	})
	return r
}

// TraegerClosingDayRequest is the body of POST /traeger/closing-days and PUT
// /traeger/closing-days/{id}.
type TraegerClosingDayRequest struct {
	StartDate calendar.Date `json:"start_date"`
	EndDate   calendar.Date `json:"end_date"`
	Reason    string        `json:"reason"`
	SchoolIDs []int64       `json:"school_ids"`
}

// Bind accepts the body; the Owner validates the closing day.
func (req *TraegerClosingDayRequest) Bind(*http.Request) error { return nil }

func (req *TraegerClosingDayRequest) input() traegercalendar.ClosingDayInput {
	return traegercalendar.ClosingDayInput{StartDate: req.StartDate, EndDate: req.EndDate, Reason: req.Reason, SchoolIDs: req.SchoolIDs}
}

func traegerOf(w http.ResponseWriter, r *http.Request) (int64, bool) {
	principal, err := common.CurrentPrincipal(r.Context())
	if err != nil || principal.OrganizationID() <= 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return 0, false
	}
	return principal.OrganizationID(), true
}

// calendar answers GET /traeger/closing-days?from=YYYY-MM-DD&to=YYYY-MM-DD.
func (rs *TraegerClosingDayResource) calendar(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := traegerOf(w, r)
	if !ok {
		return
	}
	from, fromErr := calendar.ParseDate(r.URL.Query().Get("from"))
	to, toErr := calendar.ParseDate(r.URL.Query().Get("to"))
	if fromErr != nil || toErr != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(errors.Join(fromErr, toErr), common.CodeTraegerClosingDayInvalid))
		return
	}
	result, err := rs.days.Calendar(r.Context(), organizationID, from, to)
	if err != nil {
		renderTraegerClosingDayError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, result, "Träger calendar retrieved successfully")
}

func (rs *TraegerClosingDayResource) create(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := traegerOf(w, r)
	if !ok {
		return
	}
	req := &TraegerClosingDayRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeTraegerClosingDayInvalid))
		return
	}
	day, err := rs.days.CreateClosingDay(r.Context(), organizationID, req.input())
	if err != nil {
		renderTraegerClosingDayError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, day, "Träger closing day created successfully")
}

func (rs *TraegerClosingDayResource) update(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := traegerOf(w, r)
	if !ok {
		return
	}
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFoundWithCode(traegercalendar.ErrClosingDayNotFound, common.CodeTraegerClosingDayNotFound))
		return
	}
	req := &TraegerClosingDayRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeTraegerClosingDayInvalid))
		return
	}
	day, err := rs.days.UpdateClosingDay(r.Context(), organizationID, id, req.input())
	if err != nil {
		renderTraegerClosingDayError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, day, "Träger closing day updated successfully")
}

func (rs *TraegerClosingDayResource) delete(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := traegerOf(w, r)
	if !ok {
		return
	}
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFoundWithCode(traegercalendar.ErrClosingDayNotFound, common.CodeTraegerClosingDayNotFound))
		return
	}
	if err := rs.days.DeleteClosingDay(r.Context(), organizationID, id); err != nil {
		renderTraegerClosingDayError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, nil, "Träger closing day deleted successfully")
}

// renderTraegerClosingDayError names a refusal. The Owner refuses before it
// writes, so a refused request leaves the Träger transaction untouched.
func renderTraegerClosingDayError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, traegercalendar.ErrClosingDayNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeTraegerClosingDayNotFound))
	case errors.Is(err, traegercalendar.ErrSchoolOutside):
		common.RenderError(w, r, common.ErrorInvalidOnField(err, common.CodeTraegerClosingDaySchoolOutside, "school_ids"))
	case errors.Is(err, traegercalendar.ErrInvalidClosingDay):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeTraegerClosingDayInvalid))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}
