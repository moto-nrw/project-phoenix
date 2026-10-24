package notifications

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Middleware = func(http.Handler) http.Handler

// Caller contains exactly the session facts the HTTP adapter consumes.
type Caller struct {
	ID       int
	FamilyID string
}

// Runtime contains the middleware, request facts and native response functions
// bound by Root. Delivery never imports another owner's HTTP adapter.
type Runtime struct {
	ProtectedTenant func(chi.Router, func(chi.Router, Middleware))
	ProtectedSchool func(chi.Router, func(chi.Router, Middleware))
	TenantOperation Middleware
	TenantID        func(context.Context) int64
	ActorID         func(context.Context) *int64
	Caller          func(context.Context) Caller
	Permissions     func(context.Context) []string
	Respond         func(http.ResponseWriter, *http.Request, int, any, string)
	Internal        func(http.ResponseWriter, *http.Request, error)
	Forbidden       func(http.ResponseWriter, *http.Request, error)
	Unauthorized    func(http.ResponseWriter, *http.Request)
	Invalid         func(http.ResponseWriter, *http.Request, error)
	Disabled        func(http.ResponseWriter, *http.Request)
	Conflict        func(http.ResponseWriter, *http.Request, error)
	NotFound        func(http.ResponseWriter, *http.Request, error)
	Wrapped         func(http.ResponseWriter, *http.Request, string, error)
}
