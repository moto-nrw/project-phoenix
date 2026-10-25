// Package traegerportal is the HTTP layer of the Träger-Portal ("moto
// träger", ADR 0084, #4649): its sign-in, the Träger-Büro account routes
// (#4650) and the mount of the Träger routes.
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
// The sign-in always asks for the second factor (ADR 0084, #4651): an e-mailed
// code under the org challenge scope, set up first where the account has
// none. Refresh and logout run through the shared, scope-preserving
// /auth/refresh and /auth/logout like the school portal's.
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
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
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
	// LoginStatusMFARequired carries the challenge of the e-mailed code.
	LoginStatusMFARequired LoginStatus = "mfa_required"
	// LoginStatusMFAEnrollmentRequired carries an enrollment token: the
	// account sets up its second factor first.
	LoginStatusMFAEnrollmentRequired LoginStatus = "mfa_enrollment_required"
)

// Organization is a Träger the person may choose at sign-in.
type Organization struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// LoginResult is the Träger login answer the portal renders. On the
// enrollment branch AccessToken is the enrollment token; OrganizationID names
// the Träger the login resolved on both second-factor branches.
type LoginResult struct {
	Status         LoginStatus
	AccessToken    string
	RefreshToken   string
	ChallengeToken string
	MaskedEmail    string
	OrganizationID int64
	Organizations  []Organization
}

// AuthRuntime is the Träger-Portal's session port. The Träger sign-in is
// owned by Identity & Access; the composition root binds it, so this package
// does not name the owner's contract.
type AuthRuntime struct {
	Login func(ctx context.Context, email, password, ipAddress, userAgent string, organizationID int64) (*LoginResult, error)
	// IssueTokens mints the Träger session after a proven second factor;
	// the membership is checked again.
	IssueTokens func(ctx context.Context, proof VerifiedChallenge, organizationID int64, ipAddress, userAgent string) (*LoginResult, error)
	// InvalidCredentials reports an unknown address or a wrong password;
	// the portal masks both as one answer.
	InvalidCredentials func(error) bool
	// AccountInactive reports a deactivated account.
	AccountInactive func(error) bool
	// NoMembership reports a correct password without Träger-Büro access.
	NoMembership func(error) bool
	// MFABlocked reports a locked-out or rate-limited second factor.
	MFABlocked func(error) bool
	// MFAUnavailable reports the fail-closed second-factor gate.
	MFAUnavailable func(error) bool
	// InvalidSession reports a proof a reset revoked before the mint.
	InvalidSession func(error) bool
}

func (rt *AuthRuntime) complete() bool {
	return rt != nil && rt.Login != nil && rt.IssueTokens != nil && rt.InvalidCredentials != nil &&
		rt.AccountInactive != nil && rt.NoMembership != nil && rt.MFABlocked != nil &&
		rt.MFAUnavailable != nil && rt.InvalidSession != nil
}

// TraegerRouter is a Träger route resource the root supplies; the portal
// does not import the HTTP adapters it mounts. protectedTraegerRoutes
// registers routes behind the Träger session and hands them the middleware
// that runs a route in the Träger transaction (ADR 0105).
type TraegerRouter interface {
	TraegerRouter(protectedTraegerRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router
}

// Resource bundles the Träger-Portal handlers.
type Resource struct {
	auth           AuthRuntime
	mfa            MFARuntime
	schools        TraegerRouter
	holidayPeriods TraegerRouter
	accounts       identityaccess.TraegerAccountManagement
}

// NewResource builds the Träger-Portal resource. accounts serves the
// Träger-Büro account routes (#4650); without it they answer 500.
func NewResource(auth AuthRuntime, mfa MFARuntime, schools, holidayPeriods TraegerRouter, accounts identityaccess.TraegerAccountManagement) *Resource {
	if schools == nil || holidayPeriods == nil {
		panic("traeger portal: the school list and holiday period routes are required")
	}
	return &Resource{auth: auth, mfa: mfa, schools: schools, holidayPeriods: holidayPeriods, accounts: accounts}
}

// The sentences the Träger-Portal renders for a refused login.
var (
	errLoginUnavailable   = errors.New("the Träger sessions are not composed")
	errInvalidCredentials = errors.New("invalid username or password")
	errAccountInactive    = errors.New("account is inactive")
	errNoMembership       = errors.New("account has no Träger-Büro access")
	errMFABlocked         = errors.New("the second factor is locked or rate limited")
	errMFAUnavailable     = errors.New("the second factor is unavailable")
	errMFACodeInvalid     = errors.New("the code is invalid or expired")
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
		// The Träger second factor: redeems only org-scope challenges.
		r.Post("/mfa/verify", rs.mfaVerify)
		r.Post("/mfa/resend", rs.mfaResend)
		// Setting up the factor takes the narrow enrollment token the
		// login mints, never a Träger session; the handlers pin the org
		// enrollment scope on top.
		r.Group(func(r chi.Router) {
			r.Use(sessiontoken.MFAEnrollmentAuthenticator)
			r.Post("/mfa/enroll/start", rs.mfaEnrollStart)
			r.Post("/mfa/enroll/confirm", rs.mfaEnrollConfirm)
		})
	})
	r.Mount("/invitations", rs.invitationRouter(authRateLimiter))
	r.Mount("/schools", rs.schools.TraegerRouter(protectedTraegerRoutes))
	r.Mount("/holiday-periods", rs.holidayPeriods.TraegerRouter(protectedTraegerRoutes))
	r.Mount("/accounts", rs.accountRouter(protectedTraegerRoutes))
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
	Status                string         `json:"status"`
	AccessToken           string         `json:"access_token,omitempty"`
	RefreshToken          string         `json:"refresh_token,omitempty"`
	ChallengeToken        string         `json:"challenge_token,omitempty"`
	MaskedEmail           string         `json:"masked_email,omitempty"`
	MFAEnrollmentRequired bool           `json:"mfa_enrollment_required,omitempty"`
	OrganizationID        int64          `json:"organization_id,omitempty"`
	Organizations         []Organization `json:"organizations,omitempty"`
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
	render.JSON(w, r, loginResponse(result))
}

func loginResponse(result *LoginResult) LoginResponse {
	return LoginResponse{
		Status: string(result.Status), AccessToken: result.AccessToken, RefreshToken: result.RefreshToken,
		ChallengeToken: result.ChallengeToken, MaskedEmail: result.MaskedEmail,
		MFAEnrollmentRequired: result.Status == LoginStatusMFAEnrollmentRequired,
		OrganizationID:        result.OrganizationID, Organizations: result.Organizations,
	}
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
	case rs.auth.MFABlocked(err):
		common.RenderError(w, r, common.ErrorTooManyRequestsWithCode(errMFABlocked, common.CodeIdentityMfaBlocked))
	case rs.auth.MFAUnavailable(err):
		common.RenderError(w, r, common.ErrorServiceUnavailableWithCode(errMFAUnavailable, common.CodeIdentityMfaStatusUnavailable))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}
