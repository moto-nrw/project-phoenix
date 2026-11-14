// Package traegerportal is the HTTP layer of the Träger-Portal ("moto
// träger", ADR 0084, #4649): its sign-in, the Träger-Büro account routes
// (#4650) and the mount of the Träger routes.
//
// Every route below /traeger shares two properties:
//
//  1. Authenticated routes require a Träger token (scope org) enforced by
//     TraegerScopeMiddleware. Tenant, school, parent and platform
//     tokens are rejected with 401, the symmetric guard to TenantScopeMiddleware's
//     org rejection.
//  2. A Träger token names its Träger and no school, so no route opens a
//     tenant transaction. Reads across the schools of the Träger go through
//     the Träger projection (modules/traegerview) only. Tables of the Träger
//     itself, such as its holiday periods, its settings and its master
//     data, are read and written in the Träger transaction (ADR 0086
//     section 5, ADR 0105).
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

	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/inbound/portalauth"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/tokenprimitives"
)

// Organization is a Träger the person may choose at sign-in.
type Organization struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// AuthRuntime is the Träger-Portal's session port; the composition root binds
// it to Identity & Access.
type AuthRuntime struct {
	// Login takes the browser's remember-device cookie: a device remembered
	// at the chosen Träger skips the code (#6955).
	Login func(ctx context.Context, email, password, ipAddress, userAgent string, organizationID int64, trustedDeviceCookie string) (*identityaccess.TraegerLoginResult, error)
	// IssueTokens mints the Träger session after a proven second factor;
	// the membership is checked again.
	IssueTokens func(ctx context.Context, proof VerifiedChallenge, organizationID int64, ipAddress, userAgent string) (*identityaccess.TraegerLoginResult, error)
}

func (rt *AuthRuntime) complete() bool {
	return rt != nil && rt.Login != nil && rt.IssueTokens != nil
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
	mfa            MFARuntime
	resets         common.PasswordResetRoutes
	schools        TraegerRouter
	holidayPeriods TraegerRouter
	accessLog      TraegerRouter
	settings       TraegerRouter
	masterData     TraegerRouter
	accounts       identityaccess.TraegerAccountManagement
	profiles       identityaccess.TraegerMemberProfiles
	organization   TraegerOrganizationRouter
	schoolSessions identityaccess.TraegerSchoolSessions
}

// NewResource builds the Träger-Portal resource: the school list, the
// holiday periods, the access log (#4655), the Träger settings of the
// Settings Platform (ADR 0086), the master data of Organisation & Tenancy
// (#4653) and the Träger of the session (#4658).
// accounts serves the Träger-Büro account routes (#4650), profiles the
// names of its people (#4693) and
// schoolSessions the Absprung into a school (#4654), resets the password
// reset (#5149); without them those routes answer 500.
func NewResource(auth AuthRuntime, mfa MFARuntime, resets common.PasswordResetRoutes, schools, holidayPeriods, accessLog, settings, masterData TraegerRouter, accounts identityaccess.TraegerAccountManagement, profiles identityaccess.TraegerMemberProfiles, organization TraegerOrganizationRouter, schoolSessions identityaccess.TraegerSchoolSessions) *Resource {
	if schools == nil || holidayPeriods == nil || accessLog == nil || settings == nil || masterData == nil || organization == nil {
		panic("traeger portal: the school list, holiday period, access log, settings, master data and Träger routes are required")
	}
	return &Resource{auth: auth, mfa: mfa, resets: resets, schools: schools, holidayPeriods: holidayPeriods, accessLog: accessLog, settings: settings, masterData: masterData, accounts: accounts, profiles: profiles, organization: organization, schoolSessions: schoolSessions}
}

var errLoginUnavailable = errors.New("the Träger sessions are not composed")

// RouterWithAuthRateLimiter returns the router mounted at /traeger. The
// limiter guards the public sign-in like the other portals' logins; nil
// mounts it unthrottled.
func (rs *Resource) RouterWithAuthRateLimiter(authRateLimiter func(http.Handler) http.Handler, protectedTraegerRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Route("/auth", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			if authRateLimiter != nil {
				r.Use(authRateLimiter)
			}
			r.Post("/login", rs.login)
			// The reset link reaches Träger-Büro members only and lands on the
			// Träger host (#5149).
			r.Post("/password-reset", rs.resets.ServeInitiate)
			r.Post("/password-reset/confirm", rs.resets.ServeConfirm)
			// The Träger second factor: redeems only org-scope challenges.
			r.Post("/mfa/verify", rs.mfaVerify)
			r.Post("/mfa/resend", rs.mfaResend)
			// Setting up the factor takes the Träger enrollment token the login
			// mints, never a Träger session or another portal's token.
			r.Group(func(r chi.Router) {
				r.Use(sessiontoken.EnrollmentAuthenticatorFor(tokenprimitives.OrgClaims))
				r.Post("/mfa/enroll/start", rs.mfaEnrollStart)
				r.Post("/mfa/enroll/confirm", rs.mfaEnrollConfirm)
			})
		})
		// The devices remembered at the Träger of the session (#6955). They
		// are the account's own, so no Träger right guards them, and they
		// leave the Träger transaction alone: Identity & Access reads them.
		protectedTraegerRoutes(r, func(r chi.Router, _ common.Middleware) {
			r.Get("/mfa/trusted-devices", rs.mfaTrustedDevices)
			r.Delete("/mfa/trusted-devices/{deviceId}", rs.mfaRevokeTrustedDevice)
		})
	})
	r.Mount("/invitations", rs.invitationRouter(authRateLimiter))
	r.Mount("/schools", rs.schools.TraegerRouter(protectedTraegerRoutes))
	r.Mount("/holiday-periods", rs.holidayPeriods.TraegerRouter(protectedTraegerRoutes))
	r.Mount("/settings", rs.settings.TraegerRouter(protectedTraegerRoutes))
	r.Mount("/master-data", rs.masterData.TraegerRouter(protectedTraegerRoutes))
	r.Mount("/accounts", rs.accountRouter(protectedTraegerRoutes))
	r.Mount("/profile", rs.profileRouter(protectedTraegerRoutes))
	r.Mount("/access-log", rs.accessLog.TraegerRouter(protectedTraegerRoutes))
	r.Mount("/organization", rs.organization.TraegerOrganizationRouter(protectedTraegerRoutes))
	r.Mount("/school-sessions", rs.schoolSessionRouter(protectedTraegerRoutes))
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

// LoginResponse is the login answer of the other portals with the Träger
// the sign-in resolved or offers.
type LoginResponse struct {
	Status                string         `json:"status"`
	AccessToken           string         `json:"access_token,omitempty"`
	RefreshToken          string         `json:"refresh_token,omitempty"`
	ChallengeToken        string         `json:"challenge_token,omitempty"`
	MaskedEmail           string         `json:"masked_email,omitempty"`
	MFAEnrollmentRequired bool           `json:"mfa_enrollment_required,omitempty"`
	OrganizationID        int64          `json:"organization_id,omitempty"`
	Organizations         []Organization `json:"organizations,omitempty"`
	// TrustedDeviceEnabled and TrustedDeviceDays accompany a code step, like
	// the other portals' (#6955).
	TrustedDeviceEnabled bool `json:"trusted_device_enabled,omitempty"`
	TrustedDeviceDays    int  `json:"trusted_device_days,omitempty"`
}

// login signs a person in to the Träger-Portal.
func (rs *Resource) login(w http.ResponseWriter, r *http.Request) {
	req := &LoginRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if !common.RequireDependency(w, r, rs.auth.complete(), errLoginUnavailable) {
		return
	}
	client := portalauth.ClientOf(r)
	result, err := rs.auth.Login(r.Context(), req.Email, req.Password, client.IP, client.UserAgent, req.OrganizationID, common.TrustedDevice(r))
	if err != nil {
		portalauth.Render(w, r, err, portalauth.LoginRules)
		return
	}
	render.JSON(w, r, loginResponse(result))
}

func loginResponse(result *identityaccess.TraegerLoginResult) LoginResponse {
	organizations := make([]Organization, 0, len(result.Organizations))
	for _, organization := range result.Organizations {
		organizations = append(organizations, Organization(organization))
	}
	return LoginResponse{
		Status: string(result.Status), AccessToken: result.AccessToken, RefreshToken: result.RefreshToken,
		ChallengeToken: result.ChallengeToken, MaskedEmail: result.MaskedEmail,
		MFAEnrollmentRequired: result.Status == identityaccess.TraegerLoginMFAEnrollmentRequired,
		OrganizationID:        result.OrganizationID, Organizations: organizations,
		TrustedDeviceEnabled: result.TrustedDeviceEnabled, TrustedDeviceDays: result.TrustedDeviceDays,
	}
}
