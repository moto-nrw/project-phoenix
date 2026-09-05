// Package reminders exposes the staff reminder query over HTTP.
package reminders

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	reminder "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery"
)

type Middleware = func(http.Handler) http.Handler

type Runtime struct {
	Protected      func(chi.Router, func(chi.Router, Middleware))
	ReadPermission Middleware
	EffectiveAdmin func(context.Context) bool
	Success        func(http.ResponseWriter, *http.Request, int, any, string)
	Failure        func(http.ResponseWriter, *http.Request, int, error)
}

type Resource struct {
	query   reminder.CallerQuery
	runtime Runtime
}

func NewResource(query reminder.CallerQuery, runtime Runtime) *Resource {
	if runtime.Protected == nil || runtime.ReadPermission == nil || runtime.EffectiveAdmin == nil || runtime.Success == nil || runtime.Failure == nil {
		panic("reminders HTTP: runtime dependencies are required")
	}
	return &Resource{query: query, runtime: runtime}
}

func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	rs.runtime.Protected(r, func(r chi.Router, withTx Middleware) {
		r.With(rs.runtime.ReadPermission, withTx).Get("/", rs.listReminders)
	})
	return r
}

func (rs *Resource) listReminders(w http.ResponseWriter, r *http.Request) {
	if rs.query == nil {
		rs.runtime.Failure(w, r, http.StatusInternalServerError, errors.New("reminders service is not configured"))
		return
	}
	result, err := rs.query.ComputeForCaller(r.Context(), rs.runtime.EffectiveAdmin(r.Context()))
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, reminder.ErrNotLinkedToStaff) {
			status = http.StatusForbidden
		}
		rs.runtime.Failure(w, r, status, err)
		return
	}
	rs.runtime.Success(w, r, http.StatusOK, result, "Reminders retrieved successfully")
}
