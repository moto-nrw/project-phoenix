package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	deliveryModels "github.com/moto-nrw/project-phoenix/models/delivery"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	authjwt "github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
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
	codec         identityaccessCompose.SignedIdentityTokens
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
	// operatorLinks configures the operator invitation and e-mail change
	// flows (#3332); nil composes the module without them and every
	// operator link reports it as unavailable.
	operatorLinks *operatorLinkWiring
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
	return sessionRepositories{
		schools: schoolDirectory{schools: organizations},
		persons: repos.Person, authEvents: repos.AuthEvent, pushSubscriptions: repos.PushSubscription,
		lifecycle: lifecycle,
	}
}

// signedIdentityTokensOf adapts a root's resolved signer to the native claim
// codec. The signing primitive stays in the token adapter; Identity owns the
// claim schema and validation.
func signedIdentityTokensOf(signer *authjwt.TokenAuth) (identityaccessCompose.SignedIdentityTokens, error) {
	codec, err := identityaccessCompose.NewSessionTokenCodec(signer.JwtAuth, signer.JwtExpiry, signer.JwtRefreshExpiry)
	if err != nil {
		return nil, fmt.Errorf("identity token codec: %w", err)
	}
	return codec, nil
}

// newIdentityAccessWithSessions composes the Identity & Access module with
// the account-authentication flows bound.
func newIdentityAccessWithSessions(db *bun.DB, wiring accountAuthenticationWiring) (*identityaccess.Module, error) {
	if wiring.repos.schools.schools == nil || wiring.repos.persons == nil || wiring.repos.authEvents == nil || wiring.repos.pushSubscriptions == nil || wiring.codec == nil || wiring.audit == nil {
		return nil, errors.New("identity access composition: repositories, token codec and audit command are required")
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
	invitations := invitationDependencies(wiring.invitations, wiring.codec, func() identityaccess.SchoolInvitations { return module }, wiring.logger)
	operatorLinks := operatorProvisioningDependencies(wiring.operatorLinks, func() identityaccess.OperatorTokens { return module }, wiring.logger)
	module, err = identityaccessCompose.New(identityaccessCompose.Dependencies{
		Lifecycle:            lifecycle,
		Resets:               resets,
		Invitations:          invitations,
		MFA:                  mfaDependencies(wiring.mfa, wiring.codec),
		OperatorProvisioning: operatorLinks,
		DB:                   db,
		Observe:              observe,
		Sessions: &identityaccessCompose.SessionDependencies{
			Schools:       wiring.repos.schools,
			Persons:       personDirectory{persons: wiring.repos.persons},
			Passwords:     passwordVerifier{},
			Codec:         wiring.codec,
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

// AccountAuthentication returns the Identity & Access module the session,
// lifecycle, MFA and passkey routes consume, so the HTTP composition can
// hand it to the surfaces that call the public contract directly and bind
// the consumer-owned runtimes of those that may not (#3364).
func (f *Factory) AccountAuthentication() *identityaccess.Module {
	return f.Auth
}

// --- retained owner seams -------------------------------------------------

type schoolDirectory struct {
	schools organizationtenancy.Query
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

func (d schoolDirectory) ListActiveSchoolsByID(ctx context.Context, ids []int64) ([]identityaccess.School, error) {
	if d.schools == nil {
		return nil, errors.New("school directory is not composed")
	}
	if len(ids) == 0 {
		return []identityaccess.School{}, nil
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

// ListManageableSchoolIDs is the set an organisation-scoped administrator
// is bounded by: the organisation's live, active schools. An organisation
// without one leaves them with no account to administer, which is the
// refusal the account boundary applies.
func (d schoolDirectory) ListManageableSchoolIDs(ctx context.Context, organizationID int64) ([]int64, error) {
	if d.schools == nil {
		return nil, errors.New("school directory is not composed")
	}
	schools, err := d.schools.ListSchoolsByOrganization(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(schools))
	for _, school := range schools {
		if school.Active && !school.IsDeleted() {
			ids = append(ids, school.ID)
		}
	}
	return ids, nil
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
	return securityruntime.VerifyPassword(password, hash)
}

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
