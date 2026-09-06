// Package requestfeedhttp exposes the authenticated subscription controls and
// the token-protected RSS document.
package requestfeedhttp

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/modules/careplan/requestfeed"
)

type Middleware = func(http.Handler) http.Handler

type Runtime struct {
	Protected        func(chi.Router, func(chi.Router, Middleware))
	CurrentTenantID  func(*http.Request) int64
	CurrentAccountID func(*http.Request) int64
	Logger           *slog.Logger
}

type Resource struct {
	module  *requestfeed.Module
	runtime Runtime
}

func NewResource(module *requestfeed.Module, runtime Runtime) *Resource {
	if module == nil || runtime.Protected == nil || runtime.CurrentTenantID == nil || runtime.CurrentAccountID == nil || runtime.Logger == nil {
		panic("request feed HTTP: all dependencies are required")
	}
	return &Resource{module: module, runtime: runtime}
}

func (rs *Resource) TenantRouter() chi.Router {
	router := chi.NewRouter()
	rs.runtime.Protected(router, func(protected chi.Router, withTx Middleware) {
		protected.With(withTx).Get("/", rs.status)
		protected.With(withTx).Post("/", rs.create)
		protected.With(withTx).Post("/rotate", rs.rotate)
	})
	return router
}

func (rs *Resource) PublicRouter() chi.Router {
	router := chi.NewRouter()
	router.Get("/{token}", rs.feed)
	return router
}

func (rs *Resource) status(w http.ResponseWriter, r *http.Request) {
	status, err := rs.module.Status(r.Context(), rs.runtime.CurrentTenantID(r), rs.runtime.CurrentAccountID(r))
	if err != nil {
		rs.failure(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"active": status.Active})
}

func (rs *Resource) create(w http.ResponseWriter, r *http.Request) {
	created, err := rs.module.Provision(r.Context(), rs.runtime.CurrentTenantID(r), rs.runtime.CurrentAccountID(r))
	if err != nil {
		rs.failure(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"url": created.URL})
}

func (rs *Resource) rotate(w http.ResponseWriter, r *http.Request) {
	created, err := rs.module.Rotate(r.Context(), rs.runtime.CurrentTenantID(r), rs.runtime.CurrentAccountID(r))
	if err != nil {
		rs.failure(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"url": created.URL})
}

func (rs *Resource) feed(w http.ResponseWriter, r *http.Request) {
	feed, err := rs.module.ByToken(r.Context(), chi.URLParam(r, "token"))
	if err != nil {
		rs.failure(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(feed.XML))
}

func (rs *Resource) failure(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, requestfeed.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, requestfeed.ErrAlreadyActive):
		status = http.StatusConflict
	default:
		rs.runtime.Logger.Error("request feed failed", "error", err)
	}
	http.Error(w, http.StatusText(status), status)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
