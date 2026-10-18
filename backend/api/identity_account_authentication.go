package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	organizationCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
)

// Identity & Access owns tenant, parent and school login, refresh, tenant
// and school switching, logout, session validation, session cleanup and
// session revocation (#3251). This file binds the seams those flows need to
// the retained owners the root still composes (schools, persons, the MFA
// service, the settings lock, the audit ledger, push subscriptions, the JWT
// signer) and serves the retained auth service's consumer-owned port over
// the public module.

// sessionRepositories are the retained repositories the session seams read:
// schools and persons for claims material, the auth event ledger for audit
// evidence, push subscriptions for revocation follow-ups. lifecycle carries
// the repositories the lifecycle seams bind (#3225), extracted here so the
// factory is read in one place.
type sessionRepositories struct {
	schools           schoolDirectory
	persons           personRows
	authEvents        authEventLedger
	pushSubscriptions pushSubscriptionRows
	Lifecycle         LifecycleRepositories
}

// SessionRepositoriesOf binds the session seams to the identity chain rows
// in people (persons, staff, teachers, students, guardian profiles and
// relationships) and the given ledgers.
func SessionRepositoriesOf(people LifecycleRepositories, organizations organizationtenancy.Query, authEvents authEventLedger, pushSubscriptions pushSubscriptionRows) sessionRepositories {
	if people.Persons == nil {
		return sessionRepositories{}
	}
	lifecycle := people
	lifecycle.authEvents = authEvents
	return sessionRepositories{
		schools: schoolDirectory{schools: organizations},
		persons: people.Persons, authEvents: authEvents, pushSubscriptions: pushSubscriptions,
		Lifecycle: lifecycle,
	}
}

// NewIdentityAccessWithSessions composes the Identity & Access module with
// the account-authentication flows bound over repos, the session token
// codec, the settings that hold the MFA policy lock, the audit command and
// the logger. base carries the rest the caller builds: the database, the
// observation sink (requiredIdentityAccessObservation), the tenant runtime, the
// second factor, the account lifecycle (lifecycleDependencies over
// repos.Lifecycle), the operator flows, the password resets, the school
// invitations, the operator links and the demo access. A nil part of base
// composes the module without that flow, as the cleanup roots and the
// repository fixtures do.
func NewIdentityAccessWithSessions(base identityaccessCompose.Dependencies, repos sessionRepositories, codec identityaccessCompose.SignedIdentityTokens, settings settingsCompose.SettingsService, audit auditModels.Command, logger *slog.Logger) (*identityaccess.Module, error) {
	if repos.schools.schools == nil || repos.persons == nil || repos.authEvents == nil || repos.pushSubscriptions == nil || codec == nil || audit == nil {
		return nil, errors.New("identity access composition: repositories, token codec and audit command are required")
	}
	base.Sessions = &identityaccessCompose.SessionDependencies{
		Schools:   repos.schools,
		Persons:   personDirectory{persons: repos.persons},
		Passwords: passwordVerifier{},
		Codec:     codec,
		MFALock:   mfaPolicyLock{settings: settings},
		Audit:     authAudit{command: audit, events: repos.authEvents},
		Push:      pushSubscriptionCleanup{subscriptions: repos.pushSubscriptions},
		Logger:    logger,
	}
	return identityaccessCompose.New(base)
}

// requiredIdentityAccessObservation reports the module's observations to
// the root's observer. Without an observer it returns nil, which the module
// refuses to compose with.
func requiredIdentityAccessObservation(observe IdentityAccessObserver) func(identityaccessCompose.Observation) {
	if observe == nil {
		return nil
	}
	return identityAccessObservation(observe)
}

func demoDependencies(wiring *DemoAccessWiring, standingSchool string, schools organizationtenancy.Query) (*identityaccessCompose.DemoDependencies, error) {
	if wiring == nil {
		return nil, nil
	}
	if wiring.MaxActiveSchools < 1 {
		return nil, errors.New("the public demo requires serve --demo-max-active-schools above zero")
	}
	return &identityaccessCompose.DemoDependencies{
		Schools: demoSchoolDirectory{access: organizationtenancy.NewDemoSchoolAccess(
			schools, organizationCompose.NewDemoSchoolOrders(SecureRandomSource(), wiring.MaxActiveSchools), standingSchool,
		)},
		Mail:     newDemoAccessMail(wiring),
		NewToken: wiring.NewToken, Fingerprint: wiring.Fingerprint,
		OperatorWithoutSecondFactor: wiring.OperatorWithoutSecondFactor,
	}, nil
}

// demoSchoolDirectory serves Identity & Access's demo school port over
// Organisation & Tenancy's demo school access.
type demoSchoolDirectory struct {
	access organizationtenancy.DemoSchoolAccess
}

func (d demoSchoolDirectory) PrepareDemoSchool(ctx context.Context, schoolName, firstName, lastName string) (string, error) {
	return demoSchoolSlug(d.access.PrepareDemoSchool(ctx, schoolName, firstName, lastName))
}

func (d demoSchoolDirectory) ReplaceDemoSchool(ctx context.Context, slug, schoolName, firstName, lastName string) (string, error) {
	return demoSchoolSlug(d.access.ReplaceDemoSchool(ctx, slug, schoolName, firstName, lastName))
}

func (d demoSchoolDirectory) MarkDemoSchoolUsed(ctx context.Context, slug string, usedAt time.Time) error {
	return d.access.MarkDemoSchoolUsed(ctx, slug, usedAt)
}

func (d demoSchoolDirectory) DemoSchoolEntry(ctx context.Context, slug string) (identityaccess.DemoSchoolEntry, error) {
	entry, err := d.access.DemoSchoolEntry(ctx, slug)
	return identityaccess.DemoSchoolEntry(entry), err
}

// demoSchoolSlug reports Organisation & Tenancy's refusals as Identity &
// Access's demo errors.
func demoSchoolSlug(slug string, err error) (string, error) {
	switch {
	case errors.Is(err, organizationtenancy.ErrDemoCapacityReached):
		return "", identityaccess.ErrDemoCapacityReached
	case errors.Is(err, organizationtenancy.ErrDemoRestartRefused):
		return "", identityaccess.ErrDemoAccessInvalid
	}
	return slug, err
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
	school, found, err := lookupSchool(ctx, d.schools, id)
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
		if school.Operational() {
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
		if school.Operational() {
			ids = append(ids, school.ID)
		}
	}
	return ids, nil
}

type personDirectory struct{ persons personRows }

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

type mfaPolicyLock struct {
	settings settingsCompose.SettingsService
}

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
	events  authEventLedger
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
	for _, entry := range event.Metadata() {
		row.SetMetadata(entry.Key, entry.Value)
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
	subscriptions pushSubscriptionRows
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
	return p.subscriptions.DeleteUnboundByAccountForPortal(ctx, accountID, tenantID, portal)
}

func (p pushSubscriptionCleanup) DeleteOrphaned(ctx context.Context) error {
	if p.subscriptions == nil {
		return nil
	}
	return p.subscriptions.DeleteOrphanedSubscriptions(ctx)
}
