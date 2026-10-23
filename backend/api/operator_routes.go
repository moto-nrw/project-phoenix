package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	customMiddleware "github.com/moto-nrw/project-phoenix/middleware"
	communicationModule "github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/communication/http/operatorannouncements"
	identityOperatorAPI "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/operator"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	provisioningOperatorAPI "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/inbound/operator"
	settingsOperatorAPI "github.com/moto-nrw/project-phoenix/modules/settings/inbound/operator"
)

// The operator router mounts the operator routes of Identity & Access,
// Organisation & Tenancy, Settings Platform, Communication and Device Fleet
// behind the operator session chains and rate limiters, so the operator
// surface keeps one wire format (#2736). The handlers, the session middleware
// and every service live with their owners; the HTTP host only mounts them
// (ADR 0056). No struct here holds values of two owners (ADR 0059): each
// owner's routes arrive as their own values.

// operatorIdentity is what the operator router mounts from Identity &
// Access: the operator session chains and the services of its operator
// routes.
type operatorIdentity struct {
	// sessions are the operator session chains. Required.
	sessions identityOperatorAPI.Sessions
	// account serves operator login, refresh, the profile and password
	// changes and the school access of accounts (#3252). Without it those
	// routes are not mounted.
	account     *identityOperatorAPI.Resource
	access      identityOperatorAPI.OperatorAccess
	passkeys    identityOperatorAPI.OperatorPasskeys
	mfa         identityOperatorAPI.OperatorMFA
	invitations identityOperatorAPI.OperatorAccess
	// accountMFA is the tenant-side MFA service. The operator dashboard
	// reuses it to read and write the MFA state of school staff accounts;
	// mfa is the operator's own MFA service (operator login flow).
	accountMFA identityOperatorAPI.AccountMFA
}

// operatorIdentityRoutes are the Identity & Access route resources the
// operator router mounts.
type operatorIdentityRoutes struct {
	sessions    identityOperatorAPI.Sessions
	account     *identityOperatorAPI.Resource
	passkeys    *identityOperatorAPI.PasskeyResource
	mfa         *identityOperatorAPI.MFAResource
	mfaAdmin    *identityOperatorAPI.SchoolAccountMFAResource
	profile     *identityOperatorAPI.ProfileResource
	invitations *identityOperatorAPI.InvitationsResource
}

func (in operatorIdentity) routes() operatorIdentityRoutes {
	return operatorIdentityRoutes{
		sessions:    in.sessions,
		account:     in.account,
		passkeys:    identityOperatorAPI.NewPasskeyResource(in.passkeys),
		mfa:         identityOperatorAPI.NewMFAResource(in.access, in.mfa, in.sessions.TokenAuth()),
		mfaAdmin:    &identityOperatorAPI.SchoolAccountMFAResource{TenantMFAService: in.accountMFA},
		profile:     identityOperatorAPI.NewProfileResource(in.access),
		invitations: identityOperatorAPI.NewInvitationsResource(in.invitations),
	}
}

// operatorIdentityResponses hands the operator surface's error bodies to the
// Identity & Access operator routes. The school access routes fall back to
// the provisioning error mapping.
func operatorIdentityResponses() identityOperatorAPI.Responses {
	return identityOperatorAPI.OperatorResponses(provisioningOperatorAPI.ProvisioningErrorRenderer)
}

// operatorSchoolSettings are the per-school settings routes: Settings
// Platform's values and Care Plan's review before a school switches to
// booking-led care (#4072). Both are required.
type operatorSchoolSettings struct {
	values                 *settingsOperatorAPI.SettingsResource
	bookingAuthorityImpact http.Handler
}

// operatorRouter builds the operator routes. billing (#2791) and
// schoolSettings (#2736) are optional: without them their routes are not
// mounted. unregisteredTagScans is the optional Device Fleet review of
// unregistered RFID scans (#3232). The auth limiter guards operator login
// against brute force; e-mail confirmation and invitations have their own
// limiters and fall back to the auth limiter.
func operatorRouter(
	identity operatorIdentity,
	provisioning provisioningOperatorAPI.ProvisioningConfig,
	billing *provisioningOperatorAPI.BillingResource,
	schoolSettings *operatorSchoolSettings,
	announcements communicationModule.Capability,
	unregisteredTagScans http.Handler,
	limiters authRateLimiters,
	staffPersonMerge http.Handler,
) chi.Router {
	// The root provides the session chains; without the signer the verifier
	// mounts below would fail with a nil dereference instead of naming the
	// missing configuration.
	if !identity.sessions.Configured() {
		panic("operator api: the operator session chains are required to mount the operator routes")
	}
	if schoolSettings != nil && (schoolSettings.values == nil || schoolSettings.bookingAuthorityImpact == nil) {
		panic("operator api: the school settings routes need Settings Platform's values and Care Plan's booking authority review")
	}
	if staffPersonMerge == nil {
		panic("operator api: staff person merge routes are required")
	}
	identityRoutes := identity.routes()
	provisioningRoutes := provisioningOperatorAPI.NewProvisioningResource(provisioning)
	announcementRoutes := operatorannouncements.NewAnnouncementsResource(announcements)

	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	identityRoutes.mountPublicAuthRoutes(r, limiters)
	identityRoutes.mountRefreshRoute(r)
	identityRoutes.mountMFAEnrollmentRoutes(r)
	r.Group(func(r chi.Router) {
		r.Use(identityRoutes.sessions.Operator(ReadOnlyPreviewMiddleware, SecurityPrincipalMiddleware)...)

		identityRoutes.mountPasskeyRoutes(r)
		mountOperatorProvisioningRoutes(r, provisioningRoutes)
		mountOperatorSchoolRoutes(r, provisioningRoutes, identityRoutes.mfaAdmin, schoolSettings)
		mountOperatorPersonRoutes(r, provisioningRoutes)
		r.Mount("/staff-person-merges", staffPersonMerge)
		mountOperatorTagScanRoutes(r, unregisteredTagScans)
		identityRoutes.mountAccountMFARoutes(r, provisioningRoutes)
		identityRoutes.mountAccountTenantAccessRoutes(r, provisioningRoutes)
		identityRoutes.mountProfileRoutes(r)
		identityRoutes.mountTrustedDeviceRoutes(r)
		identityRoutes.mountInvitationRoutes(r)
		mountOperatorAnnouncementRoutes(r, announcementRoutes)
		mountOperatorBillingRoutes(r, billing)
	})

	return r
}

// rateLimitMiddleware is the limiter's middleware, nil while rate limiting
// is disabled.
func rateLimitMiddleware(limiter *customMiddleware.RateLimiter) func(http.Handler) http.Handler {
	if limiter == nil {
		return nil
	}
	return limiter.Middleware()
}

// useRateLimiter applies primary (falling back to fallback) when non-nil.
func useRateLimiter(r chi.Router, primary, fallback func(http.Handler) http.Handler) {
	limiter := primary
	if limiter == nil {
		limiter = fallback
	}
	if limiter != nil {
		r.Use(limiter)
	}
}

// mountPublicAuthRoutes registers the unauthenticated /auth routes.
// Login and email-confirm use separate rate limiter instances so that
// flooding one endpoint cannot exhaust the budget for the other.
func (rs operatorIdentityRoutes) mountPublicAuthRoutes(r chi.Router, limiters authRateLimiters) {
	authRateLimiter := limiters.authMiddleware()
	r.Route("/auth", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			if authRateLimiter != nil {
				r.Use(authRateLimiter)
			}
			if rs.account != nil {
				r.Post("/login", rs.account.Login)
			}

			// MFA challenge → token-pair exchange (issue #1308). Mirror of
			// the tenant-side endpoints — they take the short-lived
			// challenge JWT in the request body, NOT in the Authorization
			// header, because the operator is mid-login and has no access
			// token yet.
			r.Post("/mfa/verify", rs.mfa.Verify)
			r.Post("/mfa/resend", rs.mfa.Resend)
			r.Post("/passkeys/login/options", rs.passkeys.PasskeyLoginOptions)
			r.Post("/passkeys/login/verify", rs.passkeys.PasskeyLoginVerify)
		})
		r.Group(func(r chi.Router) {
			useRateLimiter(r, rateLimitMiddleware(limiters.emailConfirm), authRateLimiter)
			r.Post("/email-confirm", rs.profile.ConfirmEmailChange)
		})
		r.Group(func(r chi.Router) {
			useRateLimiter(r, rateLimitMiddleware(limiters.invitation), authRateLimiter)
			r.Post("/invitations/validate", rs.invitations.ValidateInvitation)
			r.Post("/invitations/accept", rs.invitations.AcceptInvitation)
		})
	})
}

// mountRefreshRoute registers the refresh token route (requires valid refresh
// JWT, no scope check).
func (rs operatorIdentityRoutes) mountRefreshRoute(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(rs.sessions.Refresh()...)
		if rs.account != nil {
			r.Post("/auth/refresh", rs.account.RefreshToken)
		}
	})
}

// mountMFAEnrollmentRoutes registers the enrollment-only routes (issue #1308):
// operator-side mirror of the tenant /auth/mfa/enroll/* group. Accepts the
// narrow enrollment JWT that operator login mints when no MFA credential is on
// file. The enrollment authenticator guarantees mfa_enrollment_pending=true so
// these routes are reachable only from a pre-enrollment session, never from a
// full operator access token.
func (rs operatorIdentityRoutes) mountMFAEnrollmentRoutes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(rs.sessions.MFAEnrollment()...)
		r.Post("/auth/mfa/enroll/start", rs.mfa.EnrollStart)
		r.Post("/auth/mfa/enroll/confirm", rs.mfa.EnrollConfirm)
	})
}

// mountOperatorBillingRoutes registers the optional billing report (#2791).
func mountOperatorBillingRoutes(r chi.Router, billing *provisioningOperatorAPI.BillingResource) {
	if billing == nil {
		return
	}
	r.Route("/billing", func(r chi.Router) {
		r.Get("/key-day", billing.GetKeyDay)
		r.Put("/key-day", billing.UpdateKeyDay)
		r.Get("/key-date-counts", billing.ListKeyDateCounts)
		r.Get("/key-date-counts/export", billing.ExportKeyDateCounts)
		r.Post("/key-date-counts/seed", billing.SeedKeyDateCounts)
	})
}

// mountPasskeyRoutes registers passkey management for the currently-authenticated
// operator.
//
// Keep these as individual leaves instead of r.Route("/auth/passkeys", ...)
// because public passkey login also lives under /auth/passkeys/login/*.
// A protected subtree on /auth/passkeys shadows those public login routes in
// chi and makes anonymous passkey login fail with 401.
//
// Register the list endpoint under BOTH the no-slash and trailing-slash forms:
// chi treats "/auth/passkeys" and "/auth/passkeys/" as distinct patterns (no
// RedirectSlashes on this router), and the old r.Route("/auth/passkeys").Get("/")
// form answered both. The proxy sends the slash form, but direct authenticated
// operator clients hit the no-slash form, so dropping it 404s them.
func (rs operatorIdentityRoutes) mountPasskeyRoutes(r chi.Router) {
	r.Get("/auth/passkeys", rs.passkeys.PasskeyList)
	r.Get("/auth/passkeys/", rs.passkeys.PasskeyList)
	r.Post("/auth/passkeys/enrollment/challenge", rs.passkeys.PasskeyEnrollmentChallenge)
	r.Post("/auth/passkeys/register/options", rs.passkeys.PasskeyRegisterOptions)
	r.Post("/auth/passkeys/register/verify", rs.passkeys.PasskeyRegisterVerify)
	r.Delete("/auth/passkeys/{passkeyId}", rs.passkeys.PasskeyRevoke)
}

// mountOperatorProvisioningRoutes registers account/role/stat/device/organization
// provisioning endpoints.
func mountOperatorProvisioningRoutes(r chi.Router, provisioning *provisioningOperatorAPI.ProvisioningResource) {
	r.Get("/accounts", provisioning.AuditReadAccess(organizationtenancy.ReadAllAccounts, provisioning.ListAllAccounts))
	r.Get("/roles", provisioning.ListSystemRoles)
	r.Get("/stats", provisioning.GetProvisioningStats)
	r.Route("/devices", func(r chi.Router) {
		r.Get("/", provisioning.ListAllDevices)
		r.Post("/", provisioning.CreateDevice)
		r.Post("/{id}/set-api-key", provisioning.SetDeviceAPIKey)
		r.Get("/{id}/transfer-status", provisioning.GetDeviceTransferStatus)
		r.Post("/{id}/transfer", provisioning.TransferDevice)
		r.Delete("/{id}", provisioning.DeleteDevice)
	})

	r.Route("/organizations", func(r chi.Router) {
		r.Get("/", provisioning.ListOrganizations)
		r.Get("/summaries", provisioning.ListOrganizationSummaries)
		r.Post("/", provisioning.CreateOrganization)
		r.Put("/{id}", provisioning.UpdateOrganization)
		r.Delete("/{id}", provisioning.SoftDeleteOrganization)
		r.Post("/{id}/restore", provisioning.RestoreOrganization)
		r.Get("/{id}/accounts", provisioning.AuditReadAccess(organizationtenancy.ReadOrganizationAccounts, provisioning.ListOrganizationAccounts))
		r.Get("/{id}/devices", provisioning.ListOrganizationDevices)
		r.Get("/{id}/schools", provisioning.ListOrganizationSchoolSummaries)
		r.Get("/{id}/persons", provisioning.AuditReadAccess(organizationtenancy.ReadOrganizationPersons, provisioning.ListOrganizationPersons))
	})
}

// mountOperatorSchoolRoutes registers the /schools subtree, including the
// optional per-school settings endpoints.
func mountOperatorSchoolRoutes(
	r chi.Router,
	provisioning *provisioningOperatorAPI.ProvisioningResource,
	mfaAdmin *identityOperatorAPI.SchoolAccountMFAResource,
	schoolSettings *operatorSchoolSettings,
) {
	r.Route("/schools", func(r chi.Router) {
		r.Get("/", provisioning.ListSchools)
		r.Get("/summaries", provisioning.ListSchoolSummaries)
		r.Post("/", provisioning.CreateSchool)
		r.Put("/{id}", provisioning.UpdateSchool)
		r.Delete("/{id}", provisioning.SoftDeleteSchool)
		r.Post("/{id}/restore", provisioning.RestoreSchool)
		r.Post("/{id}/invite-admin", provisioning.InviteSchoolAdmin)
		r.Post("/{id}/create-account", provisioning.CreateSchoolAccount)
		r.Get("/{id}/accounts", provisioning.AuditReadAccess(organizationtenancy.ReadSchoolAccounts, provisioning.ListSchoolAccounts))
		r.Route("/{id}/accounts/{accountId}/caregiver-capability", func(r chi.Router) {
			r.Get("/", provisioning.AuditReadAccess(organizationtenancy.ReadSchoolAccountCaregiver, provisioning.GetSchoolAccountCaregiverCapability))
			r.Post("/", provisioning.EnableSchoolAccountCaregiverCapability)
			r.Delete("/", provisioning.DisableSchoolAccountCaregiverCapability)
		})
		// MFA admin actions for school staff. Operator-side mirror of the
		// tenant-admin MFA endpoints — same write semantics, separate
		// audit metadata (actor_type=operator).
		r.Route("/{id}/accounts/{accountId}/mfa", func(r chi.Router) {
			r.Get("/", provisioning.AuditReadAccess(organizationtenancy.ReadSchoolAccountMFA, mfaAdmin.GetSchoolAccountMFAState))
			r.Delete("/", mfaAdmin.ResetSchoolAccountMFA)
			r.Put("/override", mfaAdmin.SetSchoolAccountMFAOverride)
		})
		r.Get("/{id}/devices", provisioning.ListSchoolDevices)
		r.Get("/{id}/persons", provisioning.AuditReadAccess(organizationtenancy.ReadSchoolPersons, provisioning.ListSchoolPersons))
		r.Get("/{id}/pwa-usage", provisioning.GetSchoolPWAUsage)
		if schoolSettings != nil {
			values := schoolSettings.values
			r.Route("/{id}/settings", func(r chi.Router) {
				r.Get("/schema", values.GetSchoolSettingsSchema)
				r.Method(http.MethodGet, "/booking-authority-impact", schoolSettings.bookingAuthorityImpact)
				r.Get("/values/{key}/reveal", values.RevealSchoolSettingValue)
				r.Put("/values/{key}", values.SetSchoolSettingValue)
				r.Delete("/values/{key}", values.ResetSchoolSettingValue)
			})
		}
	})
}

// mountOperatorPersonRoutes registers platform-level person management.
func mountOperatorPersonRoutes(r chi.Router, provisioning *provisioningOperatorAPI.ProvisioningResource) {
	r.Route("/persons", func(r chi.Router) {
		r.Delete("/{id}", provisioning.SoftDeletePerson)
	})
}

// mountOperatorTagScanRoutes mounts the optional Device Fleet review of
// unregistered RFID scans (GET / and POST /{id}/resolve).
func mountOperatorTagScanRoutes(r chi.Router, unregisteredTagScans http.Handler) {
	if unregisteredTagScans == nil {
		return
	}
	r.Mount("/unregistered-tag-scans", unregisteredTagScans)
}

// mountAccountMFARoutes registers the account-wide MFA override surface
// ("mailbox lockout emergency switch"). Deliberately decoupled from
// /schools/{id}/accounts/{} because the platform-wide override row applies
// regardless of which school an account belongs to — see #1430 review round 2.
func (rs operatorIdentityRoutes) mountAccountMFARoutes(r chi.Router, provisioning *provisioningOperatorAPI.ProvisioningResource) {
	r.Route("/accounts/{accountId}/mfa", func(r chi.Router) {
		r.Get("/global-override", provisioning.AuditReadAccess(organizationtenancy.ReadAccountMFA, rs.mfaAdmin.GetAccountMFAGlobalOverride))
		r.Put("/global-override", rs.mfaAdmin.SetAccountMFAGlobalOverride)
	})
}

// mountAccountTenantAccessRoutes registers cross-school access management for a
// single account (issue #1021). Keyed by account, not by school, because the
// whole point is to see and change every school one account can reach.
func (rs operatorIdentityRoutes) mountAccountTenantAccessRoutes(r chi.Router, provisioning *provisioningOperatorAPI.ProvisioningResource) {
	if rs.account == nil {
		return
	}
	// Chi treats collection paths with and without a trailing slash as distinct
	// routes. The operator client deliberately uses the canonical no-slash form.
	r.Get("/accounts/{accountId}/tenants", provisioning.AuditReadAccess(organizationtenancy.ReadAccountTenants, rs.account.ListAccountTenantAccess))
	r.Get("/accounts/{accountId}/tenants/", provisioning.AuditReadAccess(organizationtenancy.ReadAccountTenants, rs.account.ListAccountTenantAccess))
	r.Post("/accounts/{accountId}/tenants", rs.account.GrantAccountTenantAccess)
	r.Post("/accounts/{accountId}/tenants/", rs.account.GrantAccountTenantAccess)
	r.Get("/accounts/{accountId}/tenants/{tenantId}/roles", rs.account.ListAssignableSchoolRoles)
	r.Put("/accounts/{accountId}/tenants/{tenantId}", rs.account.UpdateAccountTenantRole)
	r.Delete("/accounts/{accountId}/tenants/{tenantId}", rs.account.RevokeAccountTenantAccess)
}

// mountProfileRoutes registers operator profile management.
func (rs operatorIdentityRoutes) mountProfileRoutes(r chi.Router) {
	r.Route("/profile", func(r chi.Router) {
		r.Get("/", rs.profile.GetProfile)
		if rs.account != nil {
			r.Put("/", rs.account.UpdateProfile)
			r.Post("/password", rs.account.ChangePassword)
		}
		r.Post("/email-change", rs.profile.InitiateEmailChange)
	})
}

// mountTrustedDeviceRoutes registers self-service trusted-device management.
// MFA enrollment lives in its own group (uses the dedicated
// MFAEnrollmentAuthenticator); ownership here is enforced in the service.
func (rs operatorIdentityRoutes) mountTrustedDeviceRoutes(r chi.Router) {
	r.Get("/auth/mfa/trusted-devices", rs.mfa.ListTrustedDevices)
	r.Delete("/auth/mfa/trusted-devices/{deviceId}", rs.mfa.RevokeTrustedDevice)
}

// mountInvitationRoutes registers operator invitation management.
func (rs operatorIdentityRoutes) mountInvitationRoutes(r chi.Router) {
	r.Route("/invitations", func(r chi.Router) {
		r.Post("/", rs.invitations.CreateInvitation)
		r.Get("/", rs.invitations.ListInvitations)
		r.Post("/{id}/resend", rs.invitations.ResendInvitation)
		r.Delete("/{id}", rs.invitations.RevokeInvitation)
	})
	r.Post("/operators/{id}/deactivate", rs.invitations.DeactivateOperator)
	r.Post("/operators/{id}/reactivate", rs.invitations.ReactivateOperator)
}

// mountOperatorAnnouncementRoutes registers announcements management.
func mountOperatorAnnouncementRoutes(r chi.Router, announcements *operatorannouncements.AnnouncementsResource) {
	r.Route("/announcements", func(r chi.Router) {
		r.Get("/", announcements.ListAnnouncements)
		r.Post("/", announcements.CreateAnnouncement)
		r.Get("/{id}", announcements.GetAnnouncement)
		r.Put("/{id}", announcements.UpdateAnnouncement)
		r.Delete("/{id}", announcements.DeleteAnnouncement)
		r.Post("/{id}/publish", announcements.PublishAnnouncement)
		r.Get("/{id}/stats", announcements.GetStats)
		r.Get("/{id}/views", announcements.GetViewDetails)
	})
}
