package api

import (
	"context"
	"errors"

	"github.com/go-chi/chi/v5"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/traegerportal"
	traegerViewCompose "github.com/moto-nrw/project-phoenix/modules/traegerview/compose"
	traegerViewHTTP "github.com/moto-nrw/project-phoenix/modules/traegerview/http"
)

// mountTraegerPortalRoutes serves the Träger-Portal ("moto träger", ADR 0084,
// #4649) at /traeger: the public sign-in and the Träger route group, which
// accepts only Träger sessions. Token refresh and logout go through the
// shared scope-preserving /auth/refresh and /auth/logout.
func mountTraegerPortalRoutes(root chi.Router, limiters authRateLimiters, identity identityaccess.TraegerAccountLogin, organizations organizationtenancy.TraegerMembershipQuery) error {
	portal, err := NewTraegerPortalHTTP(identity, organizations)
	if err != nil {
		return err
	}
	root.Mount("/traeger", portal.RouterWithAuthRateLimiter(limiters.authMiddleware(), ProtectedTraegerRoutes))
	return nil
}

// NewTraegerPortalHTTP binds the Träger sign-in and the Träger projection to
// the portal's route ports.
func NewTraegerPortalHTTP(identity identityaccess.TraegerAccountLogin, organizations organizationtenancy.TraegerMembershipQuery) (*traegerportal.Resource, error) {
	if identity == nil || organizations == nil {
		return nil, errors.New("traeger portal: identity and organizations are required")
	}
	overview, err := traegerViewCompose.New(organizations)
	if err != nil {
		return nil, err
	}
	return traegerportal.NewResource(TraegerPortalAuth(identity), traegerViewHTTP.NewResource(overview)), nil
}

// TraegerPortalAuth binds the Träger-Portal's session port to Identity &
// Access.
func TraegerPortalAuth(sessions identityaccess.TraegerAccountLogin) traegerportal.AuthRuntime {
	if sessions == nil {
		return traegerportal.AuthRuntime{}
	}
	return traegerportal.AuthRuntime{
		Login: func(ctx context.Context, email, password, ipAddress, userAgent string, organizationID int64) (*traegerportal.LoginResult, error) {
			result, err := sessions.LoginTraeger(ctx, email, password, ipAddress, userAgent, organizationID)
			if err != nil || result == nil {
				return nil, err
			}
			organizations := make([]traegerportal.Organization, 0, len(result.Organizations))
			for _, organization := range result.Organizations {
				organizations = append(organizations, traegerportal.Organization(organization))
			}
			return &traegerportal.LoginResult{
				Status: traegerportal.LoginStatus(result.Status), AccessToken: result.AccessToken,
				RefreshToken: result.RefreshToken, Organizations: organizations,
			}, nil
		},
		InvalidCredentials: identityInvalidCredentials,
		AccountInactive:    identityIs(identityaccess.ErrAccountInactive),
		NoMembership:       identityIs(identityaccess.ErrNoTraegerMembership),
	}
}
