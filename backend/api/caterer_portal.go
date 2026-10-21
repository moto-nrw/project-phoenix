package api

import (
	"context"
	"errors"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	mealplanModule "github.com/moto-nrw/project-phoenix/modules/mealplan"
	mealplanAPI "github.com/moto-nrw/project-phoenix/modules/mealplan/http"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/tenant"
	catererPortalCompose "github.com/moto-nrw/project-phoenix/workflows/catererportal/compose"
	catererPortalHTTP "github.com/moto-nrw/project-phoenix/workflows/catererportal/http"
)

// CatererScopeMiddleware accepts caterer-portal tokens only and binds no
// school (ADR 0077).
var CatererScopeMiddleware = sessiontoken.CatererMiddleware(tenant.ClaimScope{})

// CatererSessionChain authenticates the routes of a caterer session.
func CatererSessionChain(r chi.Router) {
	r.Use(sessiontoken.Authenticator, ReadOnlyPreviewMiddleware, CatererScopeMiddleware, SecurityPrincipalMiddleware)
}

// mountCatererPortal serves the caterer portal (ADR 0077): the school's
// caterer under /api/meal-caterer and the caterer's own routes under
// /caterer, where the auth rate limiter guards the sign-in and the
// invitation.
func mountCatererPortal(root, tenantRouter chi.Router, limiters authRateLimiters, identity identityaccess.Owner, mealPlan mealplanModule.Capability, schools organizationtenancy.Capability) error {
	caterers := identity.Caterers()
	if caterers == nil {
		return errors.New("caterer portal: identity access composed no caterer flows")
	}
	portal, err := catererPortalCompose.New(catererPortalCompose.Dependencies{
		Caterers:    caterers,
		Supplies:    mealPlan,
		SchoolNames: schoolNames(schools),
	})
	if err != nil {
		return err
	}
	renderer := newMealPlanExportRenderer()
	resource := catererPortalHTTP.NewResource(portal, catererPortalHTTP.Runtime{
		CurrentAccountID: func(ctx context.Context) int64 { return int64(sessiontoken.ClaimsFromCtx(ctx).ID) },
		RenderList: func(export mealplanModule.ListExport, format string) (catererPortalHTTP.ExportFile, error) {
			file, err := renderMealList(renderer, export, format)
			return catererPortalHTTP.ExportFile(file), err
		},
		Resets: catererPasswordResets(identity),
	})
	tenantRouter.Mount("/meal-caterer", resource.SchoolRouter(ProtectedTenantRoutes, requireMealPlanAccess(mealplanAPI.AccessWrite), requireMealPlanAccess(mealplanAPI.AccessRead)))
	root.Mount("/caterer", resource.CatererRouter(limiters.authMiddleware(), CatererSessionChain))
	return nil
}

// schoolNames names schools by ID in one read of Organisation & Tenancy.
func schoolNames(schools organizationtenancy.Capability) func(context.Context, []int64) (map[int64]string, error) {
	return func(ctx context.Context, ids []int64) (map[int64]string, error) {
		names := make(map[int64]string, len(ids))
		if len(ids) == 0 {
			return names, nil
		}
		found, err := schools.ListSchoolsByID(ctx, ids)
		if err != nil {
			return nil, err
		}
		for _, school := range found {
			names[school.ID] = school.Name
		}
		return names, nil
	}
}

// catererPasswordResets binds the caterer portal's reset: only caterer
// accounts are served, and the link lands on the caterer host.
func catererPasswordResets(resets identityaccess.PasswordResets) catererPortalHTTP.PasswordResetRuntime {
	return catererPortalHTTP.PasswordResetRuntime{
		Initiate:     initiatePasswordReset(resets, identityaccess.PasswordResetScopeCaterer),
		Reset:        resets.ResetPassword,
		LinkUnusable: passwordResetLinkUnusable,
		TooWeak:      passwordResetTooWeak,
		RetryAfter:   passwordResetRetryAfter,
	}
}
