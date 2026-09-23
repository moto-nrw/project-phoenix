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
	organizationCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
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
	// demoAccess composes the public demo flow only for APP_ENV=demo.
	demoAccess *demoAccessWiring
	// demoStandingSchool is the slug of the standing demo school every demo
	// access enters instead of a school of its own: the fallback of #3463.
	// Empty gives every access its own school.
	demoStandingSchool string
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
	demo, err := demoDependencies(wiring.demoAccess, wiring.demoStandingSchool, wiring.repos.schools.schools)
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
		Demo:      demo,
	})
	if err != nil {
		return nil, err
	}
	return module, nil
}

func demoDependencies(wiring *demoAccessWiring, standingSchool string, schools organizationtenancy.Query) (*identityaccessCompose.DemoDependencies, error) {
	if wiring == nil {
		return nil, nil
	}
	if wiring.maxActiveSchools < 1 {
		return nil, errors.New("the public demo requires serve --demo-max-active-schools above zero")
	}
	return &identityaccessCompose.DemoDependencies{
		Schools: demoSchoolDirectory{
			schools: schools, orders: organizationCompose.NewDemoSchoolOrders(SecureRandomSource(), wiring.maxActiveSchools), standing: standingSchool,
		},
		Mail:     newDemoAccessMail(wiring),
		NewToken: authjwt.NewOpaqueCapabilityToken, Fingerprint: authjwt.OpaqueCapabilityFingerprint,
		OperatorWithoutSecondFactor: wiring.operatorWithoutSecondFactor,
	}, nil
}

// StandingDemoSchoolSlug is the shared demo school `go run . demo` provisions.
// Every demo access enters it while the fallback is selected; accesses issued
// before #3463 carry it as well.
const StandingDemoSchoolSlug = "messe-demo"

func standingDemoSchool(selected bool) string {
	if selected {
		return StandingDemoSchoolSlug
	}
	return ""
}

type demoSchoolDirectory struct {
	schools  organizationtenancy.Query
	orders   *organizationtenancy.DemoSchoolOrders
	standing string
}

func (d demoSchoolDirectory) PrepareDemoSchool(ctx context.Context, schoolName, personName string) (string, error) {
	if d.standing != "" {
		return d.standing, nil
	}
	slug, err := d.orders.OrderDemoSchool(ctx, schoolName, personName)
	if errors.Is(err, organizationtenancy.ErrDemoCapacityReached) {
		return "", identityaccess.ErrDemoCapacityReached
	}
	return slug, err
}

// ReplaceDemoSchool hides the visitor's school and queues a fresh one with
// the same names (#3470). It hides first, so the restart needs no free place
// at the capacity; both happen in the caller's transaction, so a refused
// order leaves the old school as it was. The standing school is shared: a
// restart is refused.
func (d demoSchoolDirectory) ReplaceDemoSchool(ctx context.Context, slug, schoolName, personName string) (string, error) {
	if d.standing != "" || slug == StandingDemoSchoolSlug {
		return "", identityaccess.ErrDemoAccessInvalid
	}
	if err := d.orders.RetireDemoSchool(ctx, slug); err != nil {
		return "", err
	}
	return d.PrepareDemoSchool(ctx, schoolName, personName)
}

// MarkDemoSchoolUsed notes an entry. The standing school has no order and is
// always simulated, so there is nothing to note for it.
func (d demoSchoolDirectory) MarkDemoSchoolUsed(ctx context.Context, slug string, usedAt time.Time) error {
	if slug == d.standing {
		return nil
	}
	return d.orders.MarkDemoSchoolUsed(ctx, slug, usedAt)
}

func (d demoSchoolDirectory) DemoSchoolEntry(ctx context.Context, slug string) (identityaccess.DemoSchoolEntry, error) {
	preparing := identityaccess.DemoSchoolEntry{Status: identityaccess.DemoSchoolPreparing}
	progress, err := d.orders.DemoSchoolProgress(ctx, slug)
	if err != nil {
		return preparing, err
	}
	// The standing school is provisioned without an order; its school record
	// alone tells whether it can be entered.
	if progress == nil && slug != d.standing {
		return preparing, nil
	}
	if progress != nil && progress.Status != organizationtenancy.DemoSchoolReady {
		return identityaccess.DemoSchoolEntry{Status: progress.Status}, nil
	}
	var school organizationtenancy.School
	if progress != nil {
		// The order names the school it seeded; the slug is only its address.
		school, err = d.schools.FindSchool(ctx, progress.SchoolID)
	} else {
		school, err = d.schools.FindSchoolBySlug(ctx, slug)
	}
	if errors.Is(err, organizationtenancy.ErrSchoolNotFound) {
		return preparing, nil
	}
	if err != nil {
		return preparing, err
	}
	if !school.Active || school.IsDeleted() {
		return preparing, nil
	}
	entry := identityaccess.DemoSchoolEntry{Status: identityaccess.DemoSchoolReady, SchoolID: school.ID}
	if progress != nil {
		entry.AccountID, entry.ParentAccountID = progress.VisitorAccountID, progress.VisitorParentAccountID
	}
	return entry, nil
}

// AccountAuthentication returns the Identity & Access module the session,
// lifecycle, MFA and passkey routes consume, so the HTTP composition can
// hand it to the surfaces that call the public contract directly and bind
// the consumer-owned runtimes of those that may not (#3364).
func (f *Factory) AccountAuthentication() *identityaccess.Module {
	return f.Auth
}

// AccountRouteTenantRuntime is the tenant runtime the account routes of
// Identity & Access run in (#2736). The root may not name the module's
// composition, so it takes the runtime from here.
func AccountRouteTenantRuntime() identityaccessCompose.TenantUnitOfWork {
	return identityaccessCompose.TenantUnitOfWork{}
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
