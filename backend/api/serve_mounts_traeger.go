package api

import (
	"context"
	"errors"

	"github.com/go-chi/chi/v5"

	"github.com/moto-nrw/project-phoenix/api/common"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	settingsHTTP "github.com/moto-nrw/project-phoenix/modules/settings/http"
	"github.com/moto-nrw/project-phoenix/modules/traegerportal"
	"github.com/moto-nrw/project-phoenix/modules/traegerview"
	traegerViewCompose "github.com/moto-nrw/project-phoenix/modules/traegerview/compose"
	traegerViewHTTP "github.com/moto-nrw/project-phoenix/modules/traegerview/http"
)

// mountTraegerPortalRoutes serves the Träger-Portal ("moto träger", ADR 0084,
// #4649) at /traeger: the public sign-in and the Träger route group, which
// accepts only Träger sessions. Token refresh and logout go through the
// shared scope-preserving /auth/refresh and /auth/logout.
func mountTraegerPortalRoutes(root chi.Router, limiters authRateLimiters, identity TraegerPortalIdentity, organizations organizationtenancy.TraegerMembershipQuery, settings settingsHTTP.TraegerOperations) error {
	portal, err := NewTraegerPortalHTTP(identity, organizations, settings)
	if err != nil {
		return err
	}
	root.Mount("/traeger", portal.RouterWithAuthRateLimiter(limiters.authMiddleware(), ProtectedTraegerRoutes))
	return nil
}

// TraegerPortalIdentity is what the Träger-Portal serves of Identity &
// Access: the sign-in and the Träger-Büro account management.
type TraegerPortalIdentity interface {
	identityaccess.TraegerAccountLogin
	identityaccess.TraegerAccountManagement
}

// NewTraegerPortalHTTP binds the Träger sign-in, the account management, the
// Träger projection, the holiday periods of Enrollment and the Träger
// settings to the portal's route ports.
func NewTraegerPortalHTTP(identity TraegerPortalIdentity, organizations organizationtenancy.TraegerMembershipQuery, settings settingsHTTP.TraegerOperations) (*traegerportal.Resource, error) {
	if identity == nil || organizations == nil || settings == nil {
		return nil, errors.New("traeger portal: identity, organizations and settings are required")
	}
	overview, err := traegerViewCompose.New(traegerSchoolIDs{organizations: organizations})
	if err != nil {
		return nil, err
	}
	holidayPeriods := enrollmentAPI.NewHolidayPeriodResource(enrollmentCompose.NewHolidayPeriods(),
		common.RequiresPermission(identityaccess.TraegerRightEnrollmentManage))
	traegerSettings := settingsHTTP.NewTraegerSettingsResource(settings, identityaccess.TraegerRightOverviewRead,
		identityaccess.TraegerRightSettingsManage, identityaccess.TraegerRightStaffManage, identityaccess.TraegerRightBillingManage)
	return traegerportal.NewResource(TraegerPortalAuth(identity), traegerViewHTTP.NewResource(overview), holidayPeriods, traegerSettings, identity), nil
}

// traegerSchoolIDs binds the Träger projection's school set to Organisation &
// Tenancy and translates a deactivated, deleted or unknown Träger into the
// projection's contract (#4657).
type traegerSchoolIDs struct {
	organizations organizationtenancy.TraegerMembershipQuery
}

func (s traegerSchoolIDs) ListTraegerSchoolIDs(ctx context.Context, organizationID int64) ([]int64, error) {
	ids, err := s.organizations.ListTraegerSchoolIDs(ctx, organizationID)
	if errors.Is(err, organizationtenancy.ErrOrganizationInactive) || errors.Is(err, organizationtenancy.ErrOrganizationNotFound) {
		return nil, traegerview.ErrTraegerUnavailable
	}
	return ids, err
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
