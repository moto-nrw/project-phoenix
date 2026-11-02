package api

import (
	"fmt"
	"log/slog"
	"net/http"

	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"

	"github.com/go-chi/chi/v5"
	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
	carePlanModule "github.com/moto-nrw/project-phoenix/modules/careplan"
	communicationModule "github.com/moto-nrw/project-phoenix/modules/communication"
	platformAnnouncementsHTTP "github.com/moto-nrw/project-phoenix/modules/communication/http/platformannouncements"
	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	deliveryRuntime "github.com/moto-nrw/project-phoenix/modules/delivery/runtime"
	devicefleetCompose "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose"
	tagScanOperatorAPI "github.com/moto-nrw/project-phoenix/modules/devicefleet/inbound/operator"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	authAPI "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/account"
	identityOperatorAPI "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/operator"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	provisioningOperatorAPI "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/inbound/operator"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	calendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal/compose"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	settingsOperatorAPI "github.com/moto-nrw/project-phoenix/modules/settings/inbound/operator"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	transactionFailures "github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/realtime"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	parentportal "github.com/moto-nrw/project-phoenix/workflows/parentportal"
	parentAPI "github.com/moto-nrw/project-phoenix/workflows/parentportal/http"
	"github.com/spf13/viper"
)

// mountPortalRoutes mounts the root-level session portals: tenant auth,
// operator dashboard (separate from the tenant API) and the cross-tenant
// guardian portal. The auth rate limiter guards each login.
func mountPortalRoutes(root chi.Router, limiters authRateLimiters, account *authAPI.Resource, operator chi.Router, parent *parentAPI.Resource) {
	// RouterWithAuthRateLimiter applies the limiter only to the public login,
	// password-reset, MFA and passkey-login routes.
	root.Mount("/auth", account.RouterWithAuthRateLimiter(limiters.authMiddleware(), TenantSessionChain))
	root.Mount("/operator", operator)
	root.Mount("/parent", parent.RouterWithAuthRateLimiter(limiters.authMiddleware(), ParentSessionChain))
}

// caregiverCapabilityViews binds People Directory's caregiver capability for
// the account and operator routes that toggle it (#2736); db opens the
// administrative transaction of the operator routes.
func caregiverCapabilityViews(db transactionruntime.Database, caregivers peoplePorts.CaregiverCapabilityService) peopleCompose.CaregiverCapabilityViews {
	return peopleCompose.NewCaregiverCapabilityViews(caregivers, db.Pool())
}

// newAccountResource composes the tenant account routes. Their caregiver
// capability is the one the operator routes provision with.
func newAccountResource(db transactionruntime.Database, identity identityaccess.Owner, organizations organizationtenancy.Query, caregivers peoplePorts.CaregiverCapabilityService, settings settingsCompose.SettingsService) *authAPI.Resource {
	authSchools := authSchoolDirectory{schools: organizations, memberships: identity}
	resource := authAPI.NewResource(identity, identity, authSchools, identity, identityaccessCompose.TenantUnitOfWork{}, transactionFailures.IsConstraintViolation, settings)
	resource.CaregiverCapabilityService = caregiverCapabilityViews(db, caregivers)
	resource.RoleGrants = owners.RoleGrantPolicy{}
	resource.MFAService = identity
	resource.PasskeyService = identity
	return resource
}

// newOperatorIdentity binds the operator sign-in, session and account routes
// to Identity & Access.
func newOperatorIdentity(sessionAuth *sessiontoken.TokenAuth, identity identityaccess.Owner) operatorIdentity {
	var caterers identityOperatorAPI.CatererMFA
	if identity != nil {
		if access := identity.Caterers(); access != nil {
			caterers = access
		}
	}
	return operatorIdentity{
		caterers:    caterers,
		sessions:    identityOperatorAPI.NewSessions(sessionAuth, identity),
		account:     identityOperatorAPI.NewResource(identity, operatorIdentityResponses()),
		access:      identity,
		passkeys:    identity,
		mfa:         identity,
		invitations: identity,
		accountMFA:  identity,
	}
}

// newOperatorRouter composes the operator dashboard. The Device Fleet review
// of unregistered RFID scans answers in the operator surface's format (#3232).
func newOperatorRouter(
	db transactionruntime.Database, logger *slog.Logger, sessionAuth *sessiontoken.TokenAuth, limiters authRateLimiters,
	iot devicefleetCompose.IoTService, organizations organizationtenancy.Query, settings settingsCompose.SettingsService,
	hub *realtime.Hub, active studentpresence.Presence, settingsWrites SettingsWrites, identity identityaccess.Owner,
	provisioning organizationtenancy.Provisioning, caregivers peoplePorts.CaregiverCapabilityService,
	lifecycle carePlanModule.CareLifecycle, communication communicationModule.Capability,
	students importModels.RowImporter[importModels.StudentImportRow],
) (chi.Router, error) {
	tagScanReview := tagScanOperatorAPI.NewResource(tagScanOperatorAPI.Config{
		Scans:     iot.Fleet(),
		Directory: tagScanSchoolDirectory{schools: organizations},
		Surface: tagScanOperatorAPI.Surface{
			InvalidRequest: apiCommon.OperatorInvalidRequest,
			Internal:       apiCommon.OperatorInternal,
			RenderError:    apiCommon.RenderError,
			Respond:        apiCommon.Respond,
			OperatorID:     operatorIDFromSession,
		},
	})
	billing, err := owners.NewOperatorBilling(logger)
	if err != nil {
		return nil, fmt.Errorf("compose operator billing: %w", err)
	}
	schoolSettings := settingsCompose.NewOperatorSchoolSettings(settingsCompose.OperatorDependencies{
		Settings:       settings,
		DB:             db.Pool(),
		Notify:         owners.SettingsChangedNotifier(hub),
		OpenAttendance: OpenAttendanceChecker{Service: active},
		// Mirror the tenant-side OnValueSet hook so operator writes also
		// trigger side effects (e.g. auto-creating the Schulhof/WC rooms when
		// the corresponding checkout toggle flips on).
		OnValueSet: settingsWrites.SideEffects.Dispatch,
	})
	provisioningConfig := provisioningOperatorAPI.ProvisioningConfig{
		Service:             provisioning,
		CaregiverCapability: caregiverCapabilityViews(db, caregivers),
		AppEnv:              viper.GetString("app_env"),
	}
	isLocalSeedRequest := func(r *http.Request) bool {
		return apiCommon.IsLocalSeedRequest(r, viper.GetString("app_env"))
	}
	mergeRoutes, err := newStaffPersonMergeRoutes(db)
	if err != nil {
		return nil, err
	}
	return operatorRouter(
		newOperatorIdentity(sessionAuth, identity),
		provisioningConfig,
		provisioningOperatorAPI.NewBillingResource(billing, nil, isLocalSeedRequest),
		&operatorSchoolSettings{
			values: settingsOperatorAPI.NewSettingsResource(settingsOperatorAPI.SettingsConfig{
				Settings:   schoolSettings,
				Schools:    operatorSettingsSchools{schools: organizations},
				OperatorID: operatorIDFromSession,
			}),
			bookingAuthorityImpact: newBookingAuthorityImpact(lifecycle),
		},
		communication,
		tagScanReview.Router(),
		limiters,
		mergeRoutes,
		newSampleChildrenRoute(provisioning, students),
		newDemoPresenceHistoryRoute(db, viper.GetString("app_env")),
	), nil
}

// newParentResource composes the cross-tenant guardian portal: public
// /parent/auth/login and the protected /parent/* routes.
func newParentResource(
	logger *slog.Logger, identity identityaccess.Owner, parent *parentportal.Portal, calendarApp calendarCompose.Application, flows owners.EnrollmentFlows,
	guardianProfiles *peopleCompose.GuardianProfileLoader, organizations organizationtenancy.Query,
	devices owners.DeliveryDevices, notifications owners.DeliveryNotifications, delivery deliveryRuntime.Capability,
	healthData enrollmentOwner.HolidayHealthCipher,
) *parentAPI.Resource {
	return parentAPI.NewResource(parentAPI.ResourceConfig{
		Auth:        parentPortalLogin(identity),
		Resets:      parentPasswordResets(identity),
		Parent:      parent,
		Calendar:    calendarApp,
		Conferences: calendarApp,
		Enrollment: parentEnrollmentForms(
			enrollmentAPI.NewRequestService(flows.Intake),
			enrollmentUploadUnits{intake: flows.Intake, records: owners.NewEnrollmentBookings(), reads: flows.Uploads},
			NewEnrollmentUploadStorage(logger.With("handler", "parent-enrollment-uploads")),
		),
		GuardianProfileLoader: newParentGuardianProfiles(guardianProfiles),
		Schools:               parentSchoolDirectory{schools: organizations},
		HolidayLinks:          newParentHolidayRegistrationLinks(organizations, delivery, healthData, logger),
		Push:                  bindParentPushSubscriptions(devices.PushSubscriptions),
		Preferences:           bindParentPreferences(notifications.Preferences),
		PWAUsage:              devices.PWAUsage,
		Reports:               newDeclarationReports(),
		SupportedLocale:       bindParentLocale(notifications.Preferences),
	})
}

func newPlatformRuntime() platformAnnouncementsHTTP.Runtime {
	return platformAnnouncementsHTTP.Runtime{
		// TenantMiddleware supplies the scope guard: parent- and school-scope
		// tokens are rejected with 401 (#2207) — before it, this group was the
		// only /api mount without a scope check, so a portal-bound token could
		// read tenant announcements here. Tenant, org, and platform tokens pass
		// unchanged.
		Protected: func(router chi.Router, register func(chi.Router)) {
			router.Group(func(r chi.Router) {
				r.Use(sessiontoken.Authenticator)
				r.Use(ReadOnlyPreviewMiddleware)
				r.Use(TenantScopeMiddleware)
				r.Use(SecurityPrincipalMiddleware)
				register(r)
			})
		},
		Viewer: func(r *http.Request) platformAnnouncementsHTTP.Viewer {
			claims := sessiontoken.ClaimsFromCtx(r.Context())
			return platformAnnouncementsHTTP.Viewer{AccountID: int64(claims.ID), Roles: claims.Roles, TenantID: claims.TenantID, OrgID: claims.OrgID}
		},
		IDParam: apiCommon.ParseInt64IDWithError,
		Success: apiCommon.Respond,
		Failure: func(w http.ResponseWriter, r *http.Request, failure platformAnnouncementsHTTP.Failure) {
			apiCommon.RenderError(w, r, apiCommon.ErrorInternalServerWrap(failure.Message, failure.Err))
		},
	}
}

// newSessionTokenAuth hands the signing configuration to Identity's
// composition, which builds the HTTP session runtime (ADR 0053 point 6).
func newSessionTokenAuth() (*sessiontoken.TokenAuth, error) {
	return NewConfiguredSessionAuth(
		viper.GetString("auth_jwt_secret"),
		viper.GetDuration("auth_jwt_expiry"),
		viper.GetDuration("auth_jwt_refresh_expiry"),
	)
}
