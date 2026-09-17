package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	authjwt "github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	deliveryModels "github.com/moto-nrw/project-phoenix/models/delivery"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/uptrace/bun"
)

// Identity & Access owns tenant, parent and school login, refresh, tenant
// and school switching, logout, session validation, session cleanup and
// session revocation (#3251). This file binds the seams those flows need to
// the retained owners the root still composes (schools, persons, the MFA
// service, the settings lock, the audit ledger, push subscriptions, the JWT
// signer) and serves the retained auth service's consumer-owned port over
// the public module.

// accountAuthenticationWiring is the retained material the session seams
// are bound to. mfa is read at call time so the auth service's
// SetMFAService keeps its meaning; tenantRuntime attaches the unit of work
// the auth service was composed with.
type accountAuthenticationWiring struct {
	repos         sessionRepositories
	tokenAuth     *authjwt.TokenAuth
	settings      config.SettingsService
	audit         auditModels.Command
	logger        *slog.Logger
	observe       IdentityAccessObserver
	tenantRuntime func(context.Context) context.Context
	// mfa composes the second factor and the passkey ceremonies (#3331);
	// nil composes the module without them.
	mfa *mfaWiring
	// operators composes the operator flows (#3252); nil leaves them
	// unavailable, as the cleanup roots compose the module.
	operators *identityaccessCompose.OperatorDependencies
	// lifecycle binds the account lifecycle seams (#3225); nil composes the
	// module without them (cleanup roots, repository fixtures).
	lifecycle *lifecycleWiring
	// resets configures the password reset flows (#2722); nil composes the
	// module without them and every reset reports it as unavailable.
	resets *passwordResetWiring
	// invitations configures the school invitation flows (#2722); nil
	// composes the module with the invitation maintenance only.
	invitations *invitationWiring
}

// sessionRepositories are the retained repositories the session seams read:
// schools and persons for claims material, the auth event ledger for audit
// evidence, push subscriptions for revocation follow-ups. lifecycle carries
// the repositories the lifecycle seams bind (#3225), extracted here so the
// factory is read in one place.
type sessionRepositories struct {
	schools           schoolDirectory
	persons           userModels.PersonRepository
	authEvents        auditModels.AuthEventRepository
	pushSubscriptions deliveryModels.PushSubscriptionRepository
	lifecycle         lifecycleRepositories
}

func sessionRepositoriesOf(repos *repositories.Factory, organizations organizationtenancy.Query) sessionRepositories {
	if repos == nil {
		return sessionRepositories{}
	}
	lifecycle := lifecycleRepositories{
		persons: repos.Person, staff: repos.Staff, teachers: repos.Teacher, students: repos.Student,
		guardianProfiles: repos.GuardianProfile, studentGuardians: repos.StudentGuardian, authEvents: repos.AuthEvent,
	}
	lifecycle.roles, lifecycle.rolesErr = repositories.NewIdentityRoleDirectory(repositories.IdentityRoleRepositories{
		Roles: repos.Role, Permissions: repos.Permission, RolePermissions: repos.RolePermission,
		AccountRoles: repos.AccountRole, AccountPermissions: repos.AccountPermission,
		Accounts: repos.Account, AccountTenants: repos.AccountTenant,
	})
	return sessionRepositories{
		schools: newSchoolDirectory(organizations, repos),
		persons: repos.Person, authEvents: repos.AuthEvent, pushSubscriptions: repos.PushSubscription,
		lifecycle: lifecycle,
	}
}

// newIdentityAccessWithSessions composes the Identity & Access module with
// the account-authentication flows bound.
func newIdentityAccessWithSessions(db *bun.DB, wiring accountAuthenticationWiring) (*identityaccess.Module, error) {
	if wiring.repos.schools.schools == nil || wiring.repos.persons == nil || wiring.repos.authEvents == nil || wiring.repos.pushSubscriptions == nil || wiring.tokenAuth == nil || wiring.audit == nil {
		return nil, errors.New("identity access composition: repositories, token auth and audit command are required")
	}
	observe := func(identityaccessCompose.Observation) {}
	if wiring.observe != nil {
		observe = func(observation identityaccessCompose.Observation) {
			wiring.observe(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, identityaccess.ErrorCode(observation.Err), observation.Err)
		}
	}
	var lifecycleBinding *lifecycleWiring
	if wiring.lifecycle != nil {
		bound := *wiring.lifecycle
		bound.repos = wiring.repos.lifecycle
		lifecycleBinding = &bound
	}
	lifecycle, err := lifecycleDependencies(lifecycleBinding, wiring.logger)
	if err != nil {
		return nil, err
	}
	// The reset delivery records its outcome through the module it is
	// composed into, so it reads the module back at call time.
	var module *identityaccess.Module
	resets := passwordResetDependencies(wiring.resets, func() identityaccess.PasswordResets { return module }, wiring.logger)
	invitations := invitationDependencies(wiring.invitations, func() identityaccess.SchoolInvitations { return module }, wiring.logger)
	module, err = identityaccessCompose.New(identityaccessCompose.Dependencies{
		Lifecycle:   lifecycle,
		Resets:      resets,
		Invitations: invitations,
		MFA:         mfaDependencies(wiring.mfa),
		DB:          db,
		Observe:     observe,
		Sessions: &identityaccessCompose.SessionDependencies{
			Schools:       wiring.repos.schools,
			Persons:       personDirectory{persons: wiring.repos.persons},
			Passwords:     passwordVerifier{},
			Codec:         sessionTokenCodec{tokenAuth: wiring.tokenAuth},
			MFALock:       mfaPolicyLock{settings: wiring.settings},
			Audit:         authAudit{command: wiring.audit, events: wiring.repos.authEvents},
			Push:          pushSubscriptionCleanup{subscriptions: wiring.repos.pushSubscriptions},
			TenantRuntime: wiring.tenantRuntime,
			Logger:        wiring.logger,
		},
		Operators: wiring.operators,
	})
	if err != nil {
		return nil, err
	}
	return module, nil
}

// AccountAuthentication returns the Identity & Access module the retained
// auth service delegates its session work to, so the HTTP composition can
// hand it to the routes that call the public contract directly. It is nil
// when the auth service was composed without the port.
func (f *Factory) AccountAuthentication() *identityaccess.Module {
	return identityAccessOf(f.Auth)
}

// identityAccessOf returns the module behind the retained auth service's
// session port, or nil when the service was composed without it.
func identityAccessOf(service auth.AuthService) *identityaccess.Module {
	provider, ok := service.(interface{ AccountSessions() auth.AccountSessions })
	if !ok {
		return nil
	}
	sessions, ok := provider.AccountSessions().(*accountSessions)
	if !ok {
		return nil
	}
	return sessions.module
}

// --- retained owner seams -------------------------------------------------

// newSchoolDirectory binds the session school seam to the Organisation &
// Tenancy capability and to the retained account-tenant memberships.
func newSchoolDirectory(organizations organizationtenancy.Query, repos *repositories.Factory) schoolDirectory {
	directory := schoolDirectory{schools: organizations}
	if repos == nil || repos.AccountTenant == nil {
		return directory
	}
	memberships := repos.AccountTenant
	directory.activeTenantIDs = func(ctx context.Context, accountID int64) ([]int64, error) {
		rows, err := memberships.FindActiveByAccountID(ctx, accountID)
		if err != nil {
			return nil, err
		}
		ids := make([]int64, 0, len(rows))
		for _, row := range rows {
			ids = append(ids, row.TenantID)
		}
		return ids, nil
	}
	return directory
}

type schoolDirectory struct {
	schools organizationtenancy.Query
	// activeTenantIDs lists the schools the account holds an active
	// membership in.
	activeTenantIDs func(ctx context.Context, accountID int64) ([]int64, error)
}

func schoolFact(school organizationtenancy.School) identityaccess.School {
	return identityaccess.School{
		ID: school.ID, OrganizationID: school.OrganizationID, Name: school.Name, Slug: school.Slug,
		Subdomain: school.Subdomain, Active: school.Active, Deleted: school.IsDeleted(),
		LogoURL: schoolLogoURLFromSettings(school.Settings),
	}
}

func (d schoolDirectory) FindSchool(ctx context.Context, id int64) (identityaccess.School, bool, error) {
	if d.schools == nil {
		return identityaccess.School{}, false, errors.New("school directory is not composed")
	}
	school, found, err := findSchool(ctx, d.schools, id)
	if err != nil || !found {
		return identityaccess.School{}, false, err
	}
	return schoolFact(school), true, nil
}

func (d schoolDirectory) FindSchoolBySubdomain(ctx context.Context, subdomain string) (identityaccess.School, bool, error) {
	if d.schools == nil {
		return identityaccess.School{}, false, errors.New("school directory is not composed")
	}
	school, err := d.schools.FindSchoolBySubdomain(ctx, subdomain)
	if errors.Is(err, organizationtenancy.ErrSchoolNotFound) {
		return identityaccess.School{}, false, nil
	}
	if err != nil {
		return identityaccess.School{}, false, err
	}
	return schoolFact(school), true, nil
}

func (d schoolDirectory) LockSchoolShared(ctx context.Context, id int64) (identityaccess.School, bool, error) {
	if d.schools == nil {
		return identityaccess.School{}, false, errors.New("school directory is not composed")
	}
	school, err := d.schools.FindSchoolForShare(ctx, id)
	if errors.Is(err, organizationtenancy.ErrSchoolNotFound) {
		return identityaccess.School{}, false, nil
	}
	if err != nil {
		return identityaccess.School{}, false, err
	}
	return schoolFact(school), true, nil
}

func (d schoolDirectory) ListActiveSchoolsOfAccount(ctx context.Context, accountID int64) ([]identityaccess.School, error) {
	if d.schools == nil || d.activeTenantIDs == nil {
		return nil, errors.New("school directory is not composed")
	}
	ids, err := d.activeTenantIDs(ctx, accountID)
	if err != nil || len(ids) == 0 {
		return []identityaccess.School{}, err
	}
	schools, err := d.schools.ListSchoolsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]identityaccess.School, 0, len(schools))
	for _, school := range schools {
		if school.Active && !school.IsDeleted() {
			result = append(result, schoolFact(school))
		}
	}
	return result, nil
}

type personDirectory struct{ persons userModels.PersonRepository }

func (d personDirectory) FindPersonName(ctx context.Context, accountID int64) (string, string, bool, error) {
	if d.persons == nil {
		return "", "", false, errors.New("person repository is not composed")
	}
	person, err := d.persons.FindByAccountID(ctx, accountID)
	if err != nil {
		return "", "", false, err
	}
	if person == nil {
		return "", "", false, nil
	}
	return person.FirstName, person.LastName, true, nil
}

type passwordVerifier struct{}

func (passwordVerifier) VerifyPassword(password, hash string) (bool, error) {
	return auth.VerifyPassword(password, hash)
}

type sessionTokenCodec struct{ tokenAuth *authjwt.TokenAuth }

func appClaims(claims identityaccess.SessionClaims) authjwt.AppClaims {
	return authjwt.AppClaims{
		ID: int(claims.AccountID), Sub: claims.Email, Username: claims.Username, FirstName: claims.FirstName, LastName: claims.LastName,
		Roles: claims.Roles, Permissions: claims.Permissions, IsAdmin: claims.IsAdmin, Scope: claims.Scope,
		TenantID: claims.TenantID, OrgID: claims.OrgID, FamilyID: claims.FamilyID,
		ReadOnly: claims.ReadOnly, ActingAdminID: claims.ActingAdminID, PreviewID: claims.PreviewID,
		CommonClaims: authjwt.CommonClaims{ExpiresAt: claims.ExpiresAt, IssuedAt: claims.IssuedAt},
	}
}

func sessionClaims(claims *authjwt.AppClaims) identityaccess.SessionClaims {
	return identityaccess.SessionClaims{
		AccountID: int64(claims.ID), Email: claims.Sub, Username: claims.Username, FirstName: claims.FirstName, LastName: claims.LastName,
		Roles: claims.Roles, Permissions: claims.Permissions, IsAdmin: claims.IsAdmin, Scope: claims.Scope,
		TenantID: claims.TenantID, OrgID: claims.OrgID, FamilyID: claims.FamilyID,
		ReadOnly: claims.ReadOnly, ActingAdminID: claims.ActingAdminID, PreviewID: claims.PreviewID,
		ExpiresAt: claims.ExpiresAt, IssuedAt: claims.IssuedAt,
	}
}

func (c sessionTokenCodec) IssueTokenPair(access identityaccess.SessionClaims, refresh identityaccess.RefreshClaims) (string, string, error) {
	return c.tokenAuth.GenTokenPair(appClaims(access), authjwt.RefreshClaims{
		ID: int(refresh.AccountID), Token: refresh.Token, TenantID: refresh.TenantID, Scope: refresh.Scope,
		CommonClaims: authjwt.CommonClaims{ExpiresAt: refresh.ExpiresAt},
	})
}

func (c sessionTokenCodec) IssueMFAEnrollmentToken(accountID, tenantID int64, scope string, ttl time.Duration) (string, error) {
	enrollmentScope := authjwt.MFAEnrollmentScopeTenant
	switch scope {
	case "school":
		enrollmentScope = authjwt.MFAEnrollmentScopeSchool
	case "platform":
		enrollmentScope = authjwt.MFAEnrollmentScopePlatform
	}
	return c.tokenAuth.CreateMFAEnrollmentJWT(authjwt.MFAEnrollmentClaims{AccountID: accountID, Scope: enrollmentScope, TenantID: tenantID}, ttl)
}

func (c sessionTokenCodec) ParseAccessToken(token string) (identityaccess.SessionClaims, error) {
	claims, err := c.tokenAuth.ParseAccessJWT(token)
	if err != nil {
		return identityaccess.SessionClaims{}, err
	}
	return sessionClaims(claims), nil
}

func (c sessionTokenCodec) ParseRefreshToken(token string) (identityaccess.RefreshClaims, error) {
	decoded, err := c.tokenAuth.JwtAuth.Decode(token)
	if err != nil {
		return identityaccess.RefreshClaims{}, err
	}
	raw := make(map[string]any)
	for _, key := range decoded.Keys() {
		var value any
		if decoded.Get(key, &value) == nil {
			raw[key] = value
		}
	}
	var claims authjwt.RefreshClaims
	if err := claims.ParseClaims(raw); err != nil {
		return identityaccess.RefreshClaims{}, err
	}
	if expiry, ok := decoded.Expiration(); ok {
		claims.ExpiresAt = expiry.Unix()
	}
	return identityaccess.RefreshClaims{AccountID: int64(claims.ID), Token: claims.Token, TenantID: claims.TenantID, Scope: claims.Scope, ExpiresAt: claims.ExpiresAt}, nil
}

func (c sessionTokenCodec) RefreshExpiry() time.Duration { return c.tokenAuth.JwtRefreshExpiry }

type mfaPolicyLock struct{ settings config.SettingsService }

// LockMFAPolicySharedForTenant is a no-op without a settings service, as
// the retained mint guard treated it.
func (l mfaPolicyLock) LockMFAPolicySharedForTenant(ctx context.Context, tenantID int64) error {
	if l.settings == nil {
		return nil
	}
	return l.settings.LockMFAPolicySharedForTenant(ctx, tenantID)
}

type authAudit struct {
	command auditModels.Command
	events  auditModels.AuthEventRepository
}

func (a authAudit) RecordAuthEvent(ctx context.Context, event identityaccess.AuthEvent) error {
	if a.command == nil {
		return fmt.Errorf("audit auth event: command is not configured")
	}
	row := auditModels.NewAuthEvent(event.AccountID, event.Type, event.Success, event.IPAddress)
	if event.TenantID > 0 {
		row.SetTenantID(event.TenantID)
	}
	row.UserAgent = event.UserAgent
	if event.ErrorMessage != "" {
		row.ErrorMessage = event.ErrorMessage
	}
	if evidence := event.TenantAccess; evidence != nil {
		row.SetMetadata("school_id", evidence.SchoolID)
		row.SetMetadata("school_name", evidence.SchoolName)
		row.SetMetadata("operator_id", evidence.OperatorID)
		switch event.Type {
		case auditModels.EventTypeTenantAccessGranted:
			row.SetMetadata("role", evidence.Role)
		case auditModels.EventTypeTenantRoleChanged:
			row.SetMetadata("role", evidence.Role)
			row.SetMetadata("removed_roles", evidence.RemovedRoles)
		case auditModels.EventTypeTenantAccessRevoked:
			row.SetMetadata("account_deactivated", evidence.AccountDeactivated)
		}
	}
	if evidence := event.RevokedSessions; evidence != nil {
		row.SetMetadata("portal_scope", evidence.PortalScope)
		row.SetMetadata("family_fingerprint", evidence.FamilyFingerprint)
		row.SetMetadata("reason", evidence.Reason)
		row.SetMetadata("revoked_token_count", evidence.Count)
	}
	if evidence := event.PendingWipe; evidence != nil {
		row.SetMetadata("reason", evidence.Reason)
		row.SetMetadata("pending_account_wide_wipe", true)
	}
	if evidence := event.CompletedWipe; evidence != nil {
		row.SetMetadata("pending_event_id", evidence.PendingEventID)
	}
	return a.command.Append(ctx, row)
}

func pendingWipes(values []auditModels.PendingAccountWideWipe) []identityaccess.PendingAccountWideWipe {
	result := make([]identityaccess.PendingAccountWideWipe, 0, len(values))
	for _, value := range values {
		result = append(result, identityaccess.PendingAccountWideWipe{
			EventID: value.EventID, TenantID: value.TenantID, AccountID: value.AccountID, Reason: value.Reason, CreatedAt: value.CreatedAt,
		})
	}
	return result
}

// ListPendingAccountWideWipes and ClaimPendingAccountWideWipes answer empty
// without an auth-event repository, as the retained reconciliation did.
func (a authAudit) ListPendingAccountWideWipes(ctx context.Context) ([]identityaccess.PendingAccountWideWipe, error) {
	if a.events == nil {
		return nil, nil
	}
	pending, err := a.events.ListPendingAccountWideWipes(ctx, time.Time{})
	if err != nil {
		return nil, err
	}
	return pendingWipes(pending), nil
}

func (a authAudit) ClaimPendingAccountWideWipes(ctx context.Context, accountID int64) ([]identityaccess.PendingAccountWideWipe, error) {
	if a.events == nil {
		return nil, nil
	}
	claimed, err := a.events.ClaimPendingAccountWideWipes(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return pendingWipes(claimed), nil
}

type pushSubscriptionCleanup struct {
	subscriptions deliveryModels.PushSubscriptionRepository
}

func (p pushSubscriptionCleanup) DeleteStaffByAccount(ctx context.Context, accountID int64) error {
	if p.subscriptions == nil {
		return nil
	}
	return p.subscriptions.DeleteStaffByAccountID(ctx, accountID)
}

func (p pushSubscriptionCleanup) DeleteSchoolByAccount(ctx context.Context, accountID int64) error {
	if p.subscriptions == nil {
		return nil
	}
	return p.subscriptions.DeleteSchoolByAccountID(ctx, accountID)
}

func (p pushSubscriptionCleanup) DeleteParentByAccount(ctx context.Context, accountID int64) error {
	if p.subscriptions == nil {
		return nil
	}
	return p.subscriptions.DeleteParentByAccountID(ctx, accountID)
}

func (p pushSubscriptionCleanup) DeleteByTokenFamily(ctx context.Context, accountID int64, familyID string) error {
	if p.subscriptions == nil {
		return nil
	}
	return p.subscriptions.DeleteByTokenFamilyID(ctx, accountID, familyID)
}

func (p pushSubscriptionCleanup) DeleteUnboundByAccount(ctx context.Context, accountID, tenantID int64, portal string) error {
	if p.subscriptions == nil {
		return nil
	}
	switch portal {
	case deliveryModels.PushPortalParent:
		return p.subscriptions.DeleteParentUnboundByAccount(ctx, accountID, tenantID)
	case deliveryModels.PushPortalSchool:
		return p.subscriptions.DeleteSchoolUnboundByAccount(ctx, accountID, tenantID)
	default:
		return p.subscriptions.DeleteStaffUnboundByAccount(ctx, accountID, tenantID)
	}
}

func (p pushSubscriptionCleanup) DeleteOrphaned(ctx context.Context) error {
	if p.subscriptions == nil {
		return nil
	}
	return p.subscriptions.DeleteOrphanedSubscriptions(ctx)
}

// --- the retained auth service's consumer-owned port -----------------------

// accountSessions serves auth.AccountSessions over the public module and
// translates the public contract back into the retained error envelope.
type accountSessions struct{ module *identityaccess.Module }

func newAccountSessions(module *identityaccess.Module) *accountSessions {
	return &accountSessions{module: module}
}

func (a *accountSessions) LoginWithAudit(ctx context.Context, email, password, ipAddress, userAgent, tenantSlug string) (string, string, error) {
	access, refresh, err := a.module.LoginWithAudit(ctx, email, password, ipAddress, userAgent, tenantSlug)
	return access, refresh, authServiceError(err)
}

func (a *accountSessions) LoginWithMFAGate(ctx context.Context, email, password, ipAddress, userAgent, tenantSlug, trustedDeviceCookie string) (*auth.LoginResult, error) {
	result, err := a.module.LoginWithMFAGate(ctx, email, password, ipAddress, userAgent, tenantSlug, trustedDeviceCookie)
	return retainedLoginResult(result), authServiceError(err)
}

func (a *accountSessions) LoginParentWithAudit(ctx context.Context, email, password, ipAddress, userAgent string) (string, string, error) {
	access, refresh, err := a.module.LoginParentWithAudit(ctx, email, password, ipAddress, userAgent)
	return access, refresh, authServiceError(err)
}

func (a *accountSessions) LoginSchoolWithMFAGate(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie string) (*auth.LoginResult, error) {
	result, err := a.module.LoginSchoolWithMFAGate(ctx, email, password, ipAddress, userAgent, trustedDeviceCookie)
	return retainedLoginResult(result), authServiceError(err)
}

func (a *accountSessions) LoginSchoolAtTenantWithMFAGate(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie, tenantSlug string) (*auth.LoginResult, error) {
	result, err := a.module.LoginSchoolAtTenantWithMFAGate(ctx, email, password, ipAddress, userAgent, trustedDeviceCookie, tenantSlug)
	return retainedLoginResult(result), authServiceError(err)
}

func (a *accountSessions) IssueTokensForAuthenticatedAccount(ctx context.Context, accountID, tenantID int64, ipAddress, userAgent string) (string, string, error) {
	access, refresh, err := a.module.IssueTokensForAuthenticatedAccount(ctx, accountID, tenantID, ipAddress, userAgent)
	return access, refresh, authServiceError(err)
}

func (a *accountSessions) IssueSchoolTokensForAuthenticatedAccount(ctx context.Context, accountID, tenantID int64, ipAddress, userAgent string) (string, string, error) {
	access, refresh, err := a.module.IssueSchoolTokensForAuthenticatedAccount(ctx, accountID, tenantID, ipAddress, userAgent)
	return access, refresh, authServiceError(err)
}

func (a *accountSessions) RefreshTokenWithAudit(ctx context.Context, refreshToken, ipAddress, userAgent string) (string, string, error) {
	access, refresh, err := a.module.RefreshTokenWithAudit(ctx, refreshToken, ipAddress, userAgent)
	return access, refresh, authServiceError(err)
}

func (a *accountSessions) LogoutWithAudit(ctx context.Context, refreshToken, ipAddress, userAgent string) error {
	return authServiceError(a.module.LogoutWithAudit(ctx, refreshToken, ipAddress, userAgent))
}

func (a *accountSessions) SwitchTenant(ctx context.Context, accountID int64, tenantSlug, presentedFamilyID string) (string, string, error) {
	access, refresh, err := a.module.SwitchTenant(ctx, accountID, tenantSlug, presentedFamilyID)
	return access, refresh, authServiceError(err)
}

func (a *accountSessions) SwitchSchool(ctx context.Context, accountID int64, tenantSlug, ipAddress, userAgent string) (string, string, error) {
	access, refresh, err := a.module.SwitchSchool(ctx, accountID, tenantSlug, ipAddress, userAgent)
	return access, refresh, authServiceError(err)
}

func (a *accountSessions) HasSchoolPortalAccess(ctx context.Context, accountID, tenantID int64) (bool, error) {
	allowed, err := a.module.HasSchoolPortalAccess(ctx, accountID, tenantID)
	return allowed, authServiceError(err)
}

func (a *accountSessions) ValidateSessionTokens(ctx context.Context, accessToken, refreshToken, portal string) (*authjwt.AppClaims, error) {
	claims, err := a.module.ValidateSessionTokens(ctx, accessToken, refreshToken, portal)
	if err != nil {
		return nil, authServiceError(err)
	}
	result := appClaims(claims)
	return &result, nil
}

func (a *accountSessions) VerifyAccountTenantMembership(ctx context.Context, accountID, tenantID int64) (bool, error) {
	ok, err := a.module.VerifyAccountTenantMembership(ctx, accountID, tenantID)
	return ok, authServiceError(err)
}

func (a *accountSessions) CountExpiredTokens(ctx context.Context) (int, error) {
	count, err := a.module.CountExpiredTokens(ctx)
	return count, authServiceError(err)
}

func (a *accountSessions) CleanupExpiredTokens(ctx context.Context) (int, error) {
	count, err := a.module.CleanupExpiredTokens(ctx)
	return count, authServiceError(err)
}

func (a *accountSessions) ListActiveSessions(ctx context.Context, accountID int64) ([]auth.ActiveSession, error) {
	sessions, err := a.module.ListActiveSessions(ctx, accountID)
	if err != nil {
		return nil, authServiceError(err)
	}
	result := make([]auth.ActiveSession, 0, len(sessions))
	for _, session := range sessions {
		active := auth.ActiveSession{ID: session.ID, Token: session.Token, Expiry: session.Expiry, Mobile: session.Mobile, CreatedAt: session.CreatedAt}
		if session.Identifier != nil {
			active.Identifier = *session.Identifier
		}
		result = append(result, active)
	}
	return result, nil
}

func (a *accountSessions) ListSessionIDs(ctx context.Context, accountID int64) ([]int64, error) {
	ids, err := a.module.ListSessionIDs(ctx, accountID)
	return ids, authServiceError(err)
}

func (a *accountSessions) RevokeAllTokensWithReason(ctx context.Context, accountID int64, reason string) error {
	return authServiceError(a.module.RevokeAllTokensWithReason(ctx, accountID, reason))
}

func (a *accountSessions) RevokeTokensByTenantID(ctx context.Context, tenantID int64) (int, error) {
	count, err := a.module.RevokeTokensByTenantID(ctx, tenantID)
	return count, authServiceError(err)
}

func revokedSessions(sessions []identityaccess.AccountSession) []auth.RevokedSession {
	result := make([]auth.RevokedSession, 0, len(sessions))
	for _, session := range sessions {
		result = append(result, auth.RevokedSession{ID: session.ID, AccountID: session.AccountID, TenantID: session.TenantID, FamilyID: session.FamilyID, PortalScope: session.PortalScope})
	}
	return result
}

func (a *accountSessions) DeleteAccountSessionsWithAudit(ctx context.Context, accountID int64, reason, ipAddress, userAgent string) ([]auth.RevokedSession, error) {
	sessions, err := a.module.DeleteAccountSessionsWithAudit(ctx, accountID, reason, ipAddress, userAgent)
	if err != nil {
		return nil, authServiceError(err)
	}
	return revokedSessions(sessions), nil
}

func (a *accountSessions) QueuePushCleanup(ctx context.Context, accountID int64, revoked []auth.RevokedSession, reason string) {
	sessions := make([]identityaccess.AccountSession, 0, len(revoked))
	for _, session := range revoked {
		sessions = append(sessions, identityaccess.AccountSession{ID: session.ID, AccountID: session.AccountID, TenantID: session.TenantID, FamilyID: session.FamilyID, PortalScope: session.PortalScope})
	}
	a.module.QueuePushCleanup(ctx, accountID, sessions, reason)
}

func (a *accountSessions) ScheduleAccountWideRevoke(ctx context.Context, accountID int64, reason, ipAddress, userAgent string) error {
	return authServiceError(a.module.ScheduleAccountWideRevoke(ctx, accountID, reason, ipAddress, userAgent))
}

func (a *accountSessions) MarkAccountWideWipeCompleted(ctx context.Context, accountID int64) error {
	return authServiceError(a.module.MarkAccountWideWipeCompleted(ctx, accountID))
}

func (a *accountSessions) LoadAccountClaims(ctx context.Context, accountID, tenantID int64) (*auth.AccountClaims, error) {
	claims, err := a.module.LoadAccountClaims(ctx, accountID, tenantID)
	if err != nil {
		return nil, authServiceError(err)
	}
	roles := make([]auth.AccountRoleClaim, 0, len(claims.Roles))
	for _, role := range claims.Roles {
		roles = append(roles, auth.AccountRoleClaim{ID: role.ID, Name: role.Name, IsSystem: role.IsSystem, TenantID: role.TenantID})
	}
	return &auth.AccountClaims{
		RoleNames: claims.RoleNames, Roles: roles, Permissions: claims.Permissions, Username: claims.Username,
		FirstName: claims.FirstName, LastName: claims.LastName, IsAdmin: claims.IsAdmin,
		TenantID: claims.TenantID, OrgID: claims.OrgID, Scope: claims.Scope,
	}, nil
}

func (a *accountSessions) FindGuardianTenant(ctx context.Context, accountID int64) (bool, int64, error) {
	found, tenantID, err := a.module.FindGuardianTenant(ctx, accountID)
	return found, tenantID, authServiceError(err)
}

func (a *accountSessions) FindSchoolPortalTenant(ctx context.Context, accountID int64) (bool, int64, error) {
	found, tenantID, err := a.module.FindSchoolPortalTenant(ctx, accountID)
	return found, tenantID, authServiceError(err)
}

func retainedLoginResult(result *identityaccess.LoginResult) *auth.LoginResult {
	if result == nil {
		return nil
	}
	return &auth.LoginResult{
		Status: auth.LoginStatus(result.Status), AccessToken: result.AccessToken, RefreshToken: result.RefreshToken,
		ChallengeToken: result.ChallengeToken, MaskedEmail: result.MaskedEmail, MFAEnrollmentRequired: result.MFAEnrollmentRequired,
		TrustedDeviceEnabled: result.TrustedDeviceEnabled, TrustedDeviceDays: result.TrustedDeviceDays,
	}
}

type retainedSentinel struct {
	public   error
	retained error
}

var retainedSentinels = []retainedSentinel{
	{identityaccess.ErrInvalidCredentials, auth.ErrInvalidCredentials},
	{identityaccess.ErrAccountNotFound, auth.ErrAccountNotFound},
	{identityaccess.ErrAccountInactive, auth.ErrAccountInactive},
	{identityaccess.ErrTenantNotFound, auth.ErrTenantNotFound},
	{identityaccess.ErrTenantAccessDenied, auth.ErrTenantAccessDenied},
	{identityaccess.ErrParentMustUseParentPortal, auth.ErrParentMustUseParentPortal},
	{identityaccess.ErrAccountNoGuardianRole, auth.ErrAccountNoGuardianRole},
	{identityaccess.ErrAccountNoSchoolPortalRole, auth.ErrAccountNoSchoolPortalRole},
	{identityaccess.ErrMustUseSchoolPortal, auth.ErrMustUseSchoolPortal},
	{identityaccess.ErrInvalidToken, auth.ErrInvalidToken},
	{identityaccess.ErrTokenExpired, auth.ErrTokenExpired},
	{identityaccess.ErrTokenNotFound, auth.ErrTokenNotFound},
	{identityaccess.ErrAccountAuthenticationUnavailable, auth.ErrAccountSessionsUnavailable},
	// A session that vanished or was rotated underneath a consumer reads as
	// the retained token sentinels the refresh flow already reports.
	{identityaccess.ErrAccountSessionNotFound, auth.ErrTokenNotFound},
	{identityaccess.ErrAccountSessionRotated, auth.ErrInvalidToken},
}

// authServiceError translates the public contract into the retained
// envelope: an AuthenticationError becomes an AuthError with the same
// operation, and a cause that carries a public sentinel gains the retained
// one while keeping its text.
func authServiceError(err error) error {
	if err == nil {
		return nil
	}
	var operation *identityaccess.AuthenticationError
	if errors.As(err, &operation) && operation == err {
		return &auth.AuthError{Op: operation.Op, Err: authServiceError(operation.Err)}
	}
	for _, sentinels := range [][]retainedSentinel{retainedSentinels, lifecycleRetainedSentinels, roleRetainedSentinels, mfaRetainedSentinels} {
		for _, sentinel := range sentinels {
			if !errors.Is(err, sentinel.public) {
				continue
			}
			if err == sentinel.public {
				return sentinel.retained
			}
			return &retainedError{text: err.Error(), sentinel: sentinel.retained, cause: err}
		}
	}
	return err
}

// retainedError keeps the public cause's text while exposing the retained
// sentinel to errors.Is.
type retainedError struct {
	text     string
	sentinel error
	cause    error
}

func (e *retainedError) Error() string { return e.text }

func (e *retainedError) Unwrap() []error { return []error{e.sentinel, e.cause} }
