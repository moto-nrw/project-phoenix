// Package traegerportal is the HTTP layer of the Träger-Portal ("moto
// träger", ADR 0084, #4649): its sign-in and the mount of the Träger routes.
//
// Every route below /traeger shares two properties:
//
//  1. Authenticated routes require a Träger token (scope org) enforced by
//     sessiontoken.TraegerMiddleware. Tenant, school, parent and platform
//     tokens are rejected with 401, the symmetric guard to TenantMiddleware's
//     org rejection.
//  2. A Träger token names its Träger and no school, so no route opens a
//     tenant transaction. Reads across the schools of the Träger go through
//     the Träger projection (modules/traegerview) only. Tables of the Träger
//     itself, such as its holiday periods, are read and written in the
//     Träger transaction (ADR 0086 section 5, ADR 0105).
//
// Refresh and logout run through the shared, scope-preserving /auth/refresh
// and /auth/logout like the school portal's.
package traegerportal

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/go-ozzo/ozzo-validation/is"

	"github.com/moto-nrw/project-phoenix/api/common"
)

const headerUserAgent = "User-Agent"

// LoginStatus discriminates the Träger login answers.
type LoginStatus string

const (
	// LoginStatusAuthenticated carries a Träger token pair.
	LoginStatusAuthenticated LoginStatus = "authenticated"
	// LoginStatusOrganizationSelectionRequired lists the Träger of a member
	// of several; the client repeats the login with organization_id.
	LoginStatusOrganizationSelectionRequired LoginStatus = "organization_selection_required"
)

// Organization is a Träger the person may choose at sign-in.
type Organization struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// LoginResult is the Träger login answer the portal renders.
type LoginResult struct {
	Status        LoginStatus
	AccessToken   string
	RefreshToken  string
	Organizations []Organization
}

// AuthRuntime is the Träger-Portal's session port. The Träger sign-in is
// owned by Identity & Access; the composition root binds it, so this package
// does not name the owner's contract.
type AuthRuntime struct {
	Login func(ctx context.Context, email, password, ipAddress, userAgent string, organizationID int64) (*LoginResult, error)
	// InvalidCredentials reports an unknown address or a wrong password;
	// the portal masks both as one answer.
	InvalidCredentials func(error) bool
	// AccountInactive reports a deactivated account.
	AccountInactive func(error) bool
	// NoMembership reports a correct password without Träger-Büro access.
	NoMembership func(error) bool
}

func (rt *AuthRuntime) complete() bool {
	return rt != nil && rt.Login != nil && rt.InvalidCredentials != nil && rt.AccountInactive != nil && rt.NoMembership != nil
}

// TraegerRouter is a Träger route resource the root supplies; the portal
// does not import the HTTP adapters it mounts. protectedTraegerRoutes
// registers routes behind the Träger session and hands them the middleware
// that runs a route in the Träger transaction (ADR 0105).
type TraegerRouter interface {
	TraegerRouter(protectedTraegerRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router
}

// TraegerOrganizationRouter serves the Träger of the session, which the
// portal frame names and marks as a test system (#4658).
type TraegerOrganizationRouter interface {
	TraegerOrganizationRouter(protectedTraegerRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router
}

// Resource bundles the Träger-Portal handlers.
type Resource struct {
	auth           AuthRuntime
	schools        TraegerRouter
	holidayPeriods TraegerRouter
	organization   TraegerOrganizationRouter
}

// NewResource builds the Träger-Portal resource.
func NewResource(auth AuthRuntime, schools, holidayPeriods TraegerRouter, organization TraegerOrganizationRouter) *Resource {
	if schools == nil || holidayPeriods == nil || organization == nil {
		panic("traeger portal: the school list, holiday period and Träger routes are required")
	}
	return &Resource{auth: auth, schools: schools, holidayPeriods: holidayPeriods, organization: organization}
}

// The sentences the Träger-Portal renders for a refused login.
var (
	errLoginUnavailable   = errors.New("the Träger sessions are not composed")
	errInvalidCredentials = errors.New("invalid username or password")
	errAccountInactive    = errors.New("account is inactive")
	errNoMembership       = errors.New("account has no Träger-Büro access")
)

// RouterWithAuthRateLimiter returns the router mounted at /traeger. The
// limiter guards the public sign-in like the other portals' logins; nil
// mounts it unthrottled.
func (rs *Resource) RouterWithAuthRateLimiter(authRateLimiter func(http.Handler) http.Handler, protectedTraegerRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Route("/auth", func(r chi.Router) {
		if authRateLimiter != nil {
			r.Use(authRateLimiter)
		}
		r.Post("/login", rs.login)
	})
	r.Mount("/schools", rs.schools.TraegerRouter(protectedTraegerRoutes))
	r.Mount("/holiday-periods", rs.holidayPeriods.TraegerRouter(protectedTraegerRoutes))
	r.Mount("/organization", rs.organization.TraegerOrganizationRouter(protectedTraegerRoutes))
	return r
}

// LoginRequest is the body of POST /traeger/auth/login. OrganizationID is the
// Träger a member of several chose; zero lets the backend decide or ask.
type LoginRequest struct {
	Email          string `json:"email"`
	Password       string `json:"password"`
	OrganizationID int64  `json:"organization_id,omitempty"`
}

// Bind normalizes and validates the request.
func (req *LoginRequest) Bind(_ *http.Request) error {
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	return validation.ValidateStruct(req,
		validation.Field(&req.Email, validation.Required, is.Email),
		validation.Field(&req.Password, validation.Required),
		validation.Field(&req.OrganizationID, validation.Min(int64(0))),
	)
}

// LoginResponse mirrors the other portals' login answer.
type LoginResponse struct {
	Status        string         `json:"status"`
	AccessToken   string         `json:"access_token,omitempty"`
	RefreshToken  string         `json:"refresh_token,omitempty"`
	Organizations []Organization `json:"organizations,omitempty"`
}

// login signs a person in to the Träger-Portal.
func (rs *Resource) login(w http.ResponseWriter, r *http.Request) {
	req := &LoginRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if !rs.auth.complete() {
		common.RenderError(w, r, common.ErrorInternalServer(errLoginUnavailable))
		return
	}
	result, err := rs.auth.Login(r.Context(), req.Email, req.Password, common.GetClientIPString(r), r.Header.Get(headerUserAgent), req.OrganizationID)
	if err != nil {
		rs.renderLoginError(w, r, err)
		return
	}
	render.JSON(w, r, LoginResponse{
		Status: string(result.Status), AccessToken: result.AccessToken, RefreshToken: result.RefreshToken,
		Organizations: result.Organizations,
	})
}

// renderLoginError masks unknown addresses and wrong passwords like the other
// portals. The refusals behind a correct password leak nothing about foreign
// accounts and tell the owner why the portal refuses them.
func (rs *Resource) renderLoginError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case rs.auth.InvalidCredentials(err):
		common.RenderError(w, r, common.ErrorUnauthorizedWithCode(errInvalidCredentials, common.CodeIdentityInvalidCredentials))
	case rs.auth.AccountInactive(err):
		common.RenderError(w, r, common.ErrorUnauthorizedWithCode(errAccountInactive, common.CodeIdentitySessionAccountInactive))
	case rs.auth.NoMembership(err):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(errNoMembership, common.CodeTraegerNoMembership))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}
