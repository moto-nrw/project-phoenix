//go:build testhooks

package api

import (
	"context"
	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	provisioningHTTP "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/inbound/operator"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	settingsHTTP "github.com/moto-nrw/project-phoenix/modules/settings/http"
	settingsOperatorHTTP "github.com/moto-nrw/project-phoenix/modules/settings/inbound/operator"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/spf13/viper"
)

// NewSettingsBudgetRouterForTest mounts the real Settings resources with the
// server's JWT, portal, permission and transaction chains. The operator read
// includes its mandatory provisioning audit and active-operator/session check.
// Outer Serve-host tenant/Träger session-liveness checks are not mounted.
// The existing typed test root supplies PayrollStatus's production Staff.List
// binding, not the lighter SettingsTestModule personnel callback.
func NewSettingsBudgetRouterForTest(ctx context.Context, db transactionruntime.Database, operatorID int64) (chi.Router, string, error) {
	_, _, _, _, _, _, identity, _, settings, _, _, _, _, _, _, _, _, _, provisioning, _, _, _, _, _, writes, _, _, err := composeTestRoot(db, slog.Default(), currentTestServiceConfig())
	if err != nil {
		return nil, "", err
	}
	auth, err := NewConfiguredSessionAuth(viper.GetString("auth_jwt_secret"), viper.GetDuration("auth_jwt_expiry"), viper.GetDuration("auth_jwt_refresh_expiry"))
	if err != nil {
		return nil, "", err
	}
	token, _, err := identity.IssueTokensForAuthenticatedOperator(ctx, operatorID, nil, "127.0.0.1", "settings-budget-test")
	if err != nil {
		return nil, "", err
	}
	home := settings.Service.(settingsHTTP.HomeLayoutOperations)
	r := chi.NewRouter()
	r.Use(auth.Verifier())
	r.Mount("/api/settings", newSettingsResource(db, writes.Tenant, home, owners.NewEnrollmentBookings().SchemaReferencesLegalDocument).SettingsRouter())
	traeger := settingsHTTP.NewTraegerSettingsResource(settings.Traeger, identityaccess.TraegerRightOverviewRead,
		identityaccess.TraegerRightSettingsManage, identityaccess.TraegerRightStaffManage, identityaccess.TraegerRightBillingManage)
	r.Mount("/traeger/settings", traeger.TraegerRouter(ProtectedTraegerRoutes))
	operator := newOperatorIdentity(auth, identity)
	resource := settingsOperatorHTTP.NewSettingsResource(settingsOperatorHTTP.SettingsConfig{
		Settings:   settingsCompose.NewOperatorSchoolSettings(settingsCompose.OperatorDependencies{Settings: settings.Service, DB: db.Pool()}),
		OperatorID: operatorIDFromSession,
	})
	provisioningRoutes := provisioningHTTP.NewProvisioningResource(provisioningHTTP.ProvisioningConfig{Service: provisioning})
	r.Group(func(r chi.Router) {
		r.Use(operator.sessions.Operator(ReadOnlyPreviewMiddleware, SecurityPrincipalMiddleware)...)
		r.Get("/operator/schools/{id}/settings/schema", provisioningRoutes.AuditReadAccess(organizationtenancy.ReadSchoolSettingsSchema, resource.GetSchoolSettingsSchema))
	})
	return r, token, nil
}
