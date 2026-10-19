// Package catererportalhttp serves the caterer portal workflow (ADR 0074):
// the school's caterer routes under the tenant API, and the caterer's own
// routes under /caterer with their own session scope.
package catererportalhttp

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/workflows/catererportal"
)

// Resource serves the caterer portal. The root binds the caller's account
// and the password reset.
type Resource struct {
	portal           catererportal.Portal
	currentAccountID func(context.Context) int64
	resets           PasswordResetRuntime
}

// NewResource builds the resource; the portal and the account are required,
// a zero reset runtime answers its routes as unavailable.
func NewResource(portal catererportal.Portal, currentAccountID func(context.Context) int64, resets PasswordResetRuntime) *Resource {
	if portal == nil || currentAccountID == nil {
		panic("caterer portal HTTP: portal and account are required")
	}
	return &Resource{portal: portal, currentAccountID: currentAccountID, resets: resets}
}

// SchoolRouter serves the school's caterer under the tenant API. Every route
// needs meals:manage and runs in the school's transaction.
func (rs *Resource) SchoolRouter(protected func(chi.Router, func(chi.Router, common.Middleware)), manage common.Middleware) chi.Router {
	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	protected(router, func(r chi.Router, withTx common.Middleware) {
		r.With(manage, withTx).Get("/", rs.getSchoolCaterer)
		r.With(manage, withTx).Post("/invitations", rs.inviteCaterer)
		r.With(manage, withTx).Delete("/invitations/{invitationId}", rs.revokeInvitation)
		r.With(manage, withTx).Delete("/supply", rs.endSupply)
	})
	return router
}

// CatererRouter serves the caterer under /caterer: the rate-limited sign-in
// and invitation routes, and the routes of a caterer session.
func (rs *Resource) CatererRouter(authRateLimiter func(http.Handler) http.Handler, configureSession func(chi.Router)) chi.Router {
	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Group(func(r chi.Router) {
		if authRateLimiter != nil {
			r.Use(authRateLimiter)
		}
		r.Post("/auth/login", rs.login)
		r.Post("/auth/password-reset", rs.initiatePasswordReset)
		r.Post("/auth/password-reset/confirm", rs.resetPassword)
		r.Get("/invitations/{schoolId}/{token}", rs.previewInvitation)
		r.Post("/invitations/{schoolId}/{token}/accept", rs.acceptInvitation)
	})
	router.Group(func(r chi.Router) {
		configureSession(r)
		r.Get("/schools", rs.getSchools)
		r.Get("/schools/{schoolId}/orders", rs.getSchoolOrders)
		r.Post("/schools/{schoolId}/orders/{orderId}/confirm", rs.confirmOrder)
	})
	return router
}

func pathID(r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, name), 10, 64)
	return id, err == nil && id > 0
}

func renderFailure(w http.ResponseWriter, r *http.Request, err error) {
	common.RenderError(w, r, common.RenderWithRules(err, errorRules, common.ErrorInternalServerRenderer("caterer portal request failed")))
}

var errInvalidID = errors.New("invalid id")
