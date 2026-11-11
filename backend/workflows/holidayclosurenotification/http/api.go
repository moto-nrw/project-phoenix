// Package http serves the holiday closure notices (ADR 0196): the school's
// list and click at /holiday-closure-notices and the Träger-Büro's list
// "Eltern noch nicht informiert" and click at /traeger/holiday-closures/pending.
// No route opens a transaction around the handler: the workflow opens the
// school's and the Träger's units one after the other, never nested.
package http

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	notification "github.com/moto-nrw/project-phoenix/workflows/holidayclosurenotification"
)

// Resource serves both portals' routes.
type Resource struct {
	notices       notification.Notifications
	schoolRead    common.Middleware
	schoolNotify  common.Middleware
	traegerManage common.Middleware
}

// NewResource builds the routes. schoolRead and schoolNotify are the
// school's rights to see and change closing days, traegerManage the Büro's
// right "Einstellungen verwalten", which plans closing days.
func NewResource(notices notification.Notifications, schoolRead, schoolNotify, traegerManage common.Middleware) *Resource {
	if notices == nil || schoolRead == nil || schoolNotify == nil || traegerManage == nil {
		panic("holiday closure notification http: the workflow and the right checks are required")
	}
	return &Resource{notices: notices, schoolRead: schoolRead, schoolNotify: schoolNotify, traegerManage: traegerManage}
}

// SchoolRouter returns the routes mounted at /holiday-closure-notices.
func (rs *Resource) SchoolRouter(protectedTenantRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTenantRoutes(r, func(r chi.Router, _ common.Middleware) {
		r.With(rs.schoolRead).Get("/", rs.schoolList)
		r.With(rs.schoolNotify).Post("/{id}/notify", rs.schoolNotifyOne)
	})
	return r
}

// TraegerRouter returns the routes mounted at /traeger/holiday-closures/pending.
func (rs *Resource) TraegerRouter(protectedTraegerRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTraegerRoutes(r, func(r chi.Router, _ common.Middleware) {
		manage := r.With(rs.traegerManage)
		manage.Get("/", rs.traegerList)
		manage.Post("/notify", rs.traegerNotifyAll)
		manage.Post("/{id}/notify", rs.traegerNotifyOne)
	})
	return r
}

// NotifyRequest confirms one notice at the revision the caller saw.
type NotifyRequest struct {
	Token    string `json:"token"`
	Revision int64  `json:"revision"`
}

// Bind accepts the body.
func (req *NotifyRequest) Bind(*http.Request) error { return nil }

// NotifyAllRequest confirms the list the Büro saw.
type NotifyAllRequest struct {
	Token string `json:"token"`
}

// Bind accepts the body.
func (req *NotifyAllRequest) Bind(*http.Request) error { return nil }

func (rs *Resource) schoolList(w http.ResponseWriter, r *http.Request) {
	principal, err := common.CurrentPrincipal(r.Context())
	if err != nil || principal.TenantID() <= 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return
	}
	notices, err := rs.notices.SchoolNotices(r.Context(), principal.TenantID())
	if err != nil {
		renderError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, notices, "Holiday closure notices retrieved successfully")
}

func (rs *Resource) schoolNotifyOne(w http.ResponseWriter, r *http.Request) {
	principal, err := common.CurrentPrincipal(r.Context())
	if err != nil || principal.TenantID() <= 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return
	}
	id, req, ok := notifyInput(w, r)
	if !ok {
		return
	}
	notice, err := rs.notices.NotifyFromSchool(r.Context(), principal.TenantID(), principal.AccountID(), id, req.Revision, req.Token)
	if err != nil {
		renderError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, notice, "Parents' notice confirmed")
}

func (rs *Resource) traegerList(w http.ResponseWriter, r *http.Request) {
	principal, err := common.CurrentPrincipal(r.Context())
	if err != nil || principal.OrganizationID() <= 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return
	}
	pending, err := rs.notices.PendingHolidayClosures(r.Context(), principal.OrganizationID())
	if err != nil {
		renderError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, pending, "Pending holiday closure notices retrieved successfully")
}

func (rs *Resource) traegerNotifyOne(w http.ResponseWriter, r *http.Request) {
	principal, err := common.CurrentPrincipal(r.Context())
	if err != nil || principal.OrganizationID() <= 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return
	}
	id, req, ok := notifyInput(w, r)
	if !ok {
		return
	}
	notice, err := rs.notices.NotifyFromTraeger(r.Context(), principal.OrganizationID(), principal.AccountID(), id, req.Revision, req.Token)
	if err != nil {
		renderError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, notice, "Parents' notice confirmed")
}

func (rs *Resource) traegerNotifyAll(w http.ResponseWriter, r *http.Request) {
	principal, err := common.CurrentPrincipal(r.Context())
	if err != nil || principal.OrganizationID() <= 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return
	}
	req := &NotifyAllRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	result, err := rs.notices.NotifyPendingHolidayClosures(r.Context(), principal.OrganizationID(), principal.AccountID(), req.Token)
	if err != nil {
		renderError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, result, "Parents' notices confirmed")
}

func notifyInput(w http.ResponseWriter, r *http.Request) (int64, *NotifyRequest, bool) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFoundWithCode(notification.ErrNoticeNotFound, common.CodeTimetableClosingDayNoticeNotFound))
		return 0, nil, false
	}
	req := &NotifyRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return 0, nil, false
	}
	return id, req, true
}

func renderError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, notification.ErrNoticeNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeTimetableClosingDayNoticeNotFound))
	case errors.Is(err, notification.ErrNoticeChanged):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeTimetableClosingDayNoticeChanged))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}
