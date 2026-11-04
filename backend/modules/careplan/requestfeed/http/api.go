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
	feeds   requestfeed.Capability
	runtime Runtime
}

func NewResource(feeds requestfeed.Capability, runtime Runtime) *Resource {
	if feeds == nil || runtime.Protected == nil || runtime.CurrentTenantID == nil || runtime.CurrentAccountID == nil || runtime.Logger == nil {
		panic("request feed HTTP: all dependencies are required")
	}
	return &Resource{feeds: feeds, runtime: runtime}
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
	status, err := rs.feeds.Status(r.Context(), rs.runtime.CurrentTenantID(r), rs.runtime.CurrentAccountID(r))
	if err != nil {
		rs.failure(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"active": status.Active})
}

func (rs *Resource) create(w http.ResponseWriter, r *http.Request) {
	created, err := rs.feeds.Provision(r.Context(), rs.runtime.CurrentTenantID(r), rs.runtime.CurrentAccountID(r))
	if err != nil {
		rs.failure(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"url": created.URL})
}

func (rs *Resource) rotate(w http.ResponseWriter, r *http.Request) {
	created, err := rs.feeds.Rotate(r.Context(), rs.runtime.CurrentTenantID(r), rs.runtime.CurrentAccountID(r))
	if err != nil {
		rs.failure(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"url": created.URL})
}

func (rs *Resource) feed(w http.ResponseWriter, r *http.Request) {
	feed, err := rs.feeds.ByToken(r.Context(), chi.URLParam(r, "token"))
	if err != nil {
		rs.failure(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(feed.XML))
}

// Registered class codes (error-registry/) of the feed's refusals. This
// package may not import api/common, so it names them once here.
const (
	codeGeneralInput             = "general.input"
	codeGeneralBusinessRejection = "general.business_rejection"
	codeGeneralServer            = "general.server"
)

// feedProblem is the shared error envelope (ADR 0006): the HTTP status
// number, the diagnostic text and the registered code.
// ProblemResponseMiddleware adds type, title and instance like for every
// error answer.
type feedProblem struct {
	Status int    `json:"status"`
	Detail string `json:"detail"`
	Code   string `json:"code"`
}

func (rs *Resource) failure(w http.ResponseWriter, err error) {
	status, code := http.StatusInternalServerError, codeGeneralServer
	switch {
	case errors.Is(err, requestfeed.ErrNotFound):
		status, code = http.StatusNotFound, codeGeneralInput
	case errors.Is(err, requestfeed.ErrAlreadyActive):
		status, code = http.StatusConflict, codeGeneralBusinessRejection
	default:
		rs.runtime.Logger.Error("request feed failed", "error", err)
	}
	writeJSON(w, status, feedProblem{Status: status, Detail: http.StatusText(status), Code: code})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
