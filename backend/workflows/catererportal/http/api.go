// Package catererportalhttp serves the caterer portal workflow (ADR 0077):
// the school's caterer routes under the tenant API, and the caterer's own
// routes under /caterer with their own session scope.
package catererportalhttp

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/workflows/catererportal"
)

// ExportFile is a rendered list.
type ExportFile struct {
	Data        []byte
	ContentType string
	Filename    string
}

// Runtime is what the root binds: the caller's account, the list renderer
// (PDF and Excel), the password reset, the sign-in and the caterer's own
// second factor (#4497). A zero reset runtime answers its routes as
// unavailable.
type Runtime struct {
	CurrentAccountID func(context.Context) int64
	RenderList       func(mealplan.ListExport, string) (ExportFile, error)
	Resets           common.PasswordResetRoutes
	SignIn           SignIn
	SecondFactor     SecondFactor
}

// Resource serves the caterer portal.
type Resource struct {
	portal  catererportal.Portal
	runtime Runtime
}

// NewResource builds the resource; the portal, the account and the
// renderer are required.
func NewResource(portal catererportal.Portal, runtime Runtime) *Resource {
	if portal == nil || runtime.CurrentAccountID == nil || runtime.RenderList == nil || runtime.SignIn == nil || runtime.SecondFactor == nil {
		panic("caterer portal HTTP: portal, account, renderer, sign-in and second factor are required")
	}
	return &Resource{portal: portal, runtime: runtime}
}

func (rs *Resource) currentAccountID(ctx context.Context) int64 {
	return rs.runtime.CurrentAccountID(ctx)
}

// SchoolRouter serves the school's caterer under the tenant API, each route
// in the school's transaction. Seeing the caterer needs meals:manage; who
// supplies the school (inviting, revoking an invitation, ending the supply)
// is decided by admins only, because the invitation lets an external
// business read the school's orders. The delivery list needs meals:orders,
// like the other school exports.
func (rs *Resource) SchoolRouter(protected func(chi.Router, func(chi.Router, common.Middleware)), manage, admin, orders common.Middleware) chi.Router {
	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	protected(router, func(r chi.Router, withTx common.Middleware) {
		r.With(orders, withTx).Get("/orders/export", rs.exportSchoolDeliveries)
		r.With(manage, withTx).Get("/", rs.getSchoolCaterer)
		r.With(admin, withTx).Post("/invitations", rs.inviteCaterer)
		r.With(admin, withTx).Delete("/invitations/{invitationId}", rs.revokeInvitation)
		r.With(admin, withTx).Delete("/supply", rs.endSupply)
	})
	return router
}

// CatererRouter serves the caterer under /caterer: the rate-limited sign-in
// and invitation routes, and the routes of a caterer session, among them
// the rate-limited password change.
func (rs *Resource) CatererRouter(authRateLimiter func(http.Handler) http.Handler, configureSession func(chi.Router)) chi.Router {
	limitCredentialChecks := func(r chi.Router) {
		if authRateLimiter != nil {
			r.Use(authRateLimiter)
		}
	}
	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Group(func(r chi.Router) {
		limitCredentialChecks(r)
		r.Post("/auth/login", rs.login)
		// The second factor of the sign-in (#4497): the code, its resend and
		// the passkey. They check credentials, so they share the limit.
		r.Post("/auth/mfa/verify", rs.verifyMFA)
		r.Post("/auth/mfa/resend", rs.resendMFA)
		r.Post("/auth/passkeys/login/options", rs.passkeyLoginOptions)
		r.Post("/auth/passkeys/login/verify", rs.passkeyLoginVerify)
		r.Post("/auth/password-reset", rs.runtime.Resets.ServeInitiate)
		r.Post("/auth/password-reset/confirm", rs.runtime.Resets.ServeConfirm)
		r.Get("/invitations/{schoolId}/{token}", rs.previewInvitation)
		r.Post("/invitations/{schoolId}/{token}/accept", rs.acceptInvitation)
	})
	router.Group(func(r chi.Router) {
		configureSession(r)
		r.Get("/schools", rs.getSchools)
		r.Get("/orders/export", rs.exportDeliveries)
		r.Get("/schools/{schoolId}/orders", rs.getSchoolOrders)
		r.Post("/schools/{schoolId}/orders/{orderId}/confirm", rs.confirmOrder)
		// The caterer's own second factor (#4497). The session routes stay
		// leaves beside the public sign-in routes of the same prefix.
		r.Get("/account/mfa", rs.mfaStatus)
		r.Get("/auth/mfa/trusted-devices", rs.listTrustedDevices)
		r.Delete("/auth/mfa/trusted-devices/{deviceId}", rs.revokeTrustedDevice)
		r.Get("/auth/passkeys", rs.listPasskeys)
		r.Delete("/auth/passkeys/{passkeyId}", rs.revokePasskey)
		r.Post("/auth/passkeys/register/verify", rs.passkeyRegisterVerify)
		// The current password is a credential check: it shares the
		// sign-in's rate limit beside the account lock. Every mailed code
		// and every code check shares it too.
		r.Group(func(r chi.Router) {
			limitCredentialChecks(r)
			r.Post("/account/password", rs.changePassword)
			r.Post("/account/mfa/start", rs.startMFA)
			r.Post("/account/mfa/confirm", rs.confirmMFA)
			r.Post("/account/mfa/disable", rs.disableMFA)
			r.Post("/auth/passkeys/enrollment/challenge", rs.passkeyEnrollmentChallenge)
			r.Post("/auth/passkeys/register/options", rs.passkeyRegisterOptions)
		})
	})
	return router
}

func pathID(r *http.Request, name string) (int64, bool) {
	id, err := common.ParseIDParam(r, name)
	return id, err == nil && id > 0
}

func renderFailure(w http.ResponseWriter, r *http.Request, err error) {
	common.RenderError(w, r, common.RenderWithRules(err, errorRules, common.ErrorInternalServerRenderer("caterer portal request failed")))
}

var errInvalidID = errors.New("invalid id")
