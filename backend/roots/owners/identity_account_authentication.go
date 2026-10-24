package owners

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	identityBindingPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	organizationCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	configModel "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// sessionRepositories are the retained repositories the session seams read:
// schools and persons for claims material, the auth event ledger for audit
// evidence, push subscriptions for revocation follow-ups. lifecycle carries
// the repositories the lifecycle seams bind (#3225), extracted here so the
// factory is read in one place.
type sessionRepositories struct {
	schools           SchoolDirectory
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
	lifecycle.AuthEvents = authEvents
	return sessionRepositories{
		schools: SchoolDirectory{Schools: organizations},
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
func NewIdentityAccessWithSessions(base identityaccessCompose.Dependencies, repos sessionRepositories, codec identityaccessCompose.SignedIdentityTokens, settings settingsCompose.SettingsService, audit auditModels.Command, logger *slog.Logger) (identityaccess.Owner, error) {
	if repos.schools.Schools == nil || repos.persons == nil || repos.authEvents == nil || repos.pushSubscriptions == nil || codec == nil || audit == nil {
		return nil, errors.New("identity access composition: repositories, token codec and audit command are required")
	}
	base.Sessions = &identityBindingPorts.SessionDependencies{
		Schools:             repos.schools,
		Persons:             PersonDirectory{Persons: repos.persons},
		Passwords:           passwordVerifier{},
		PasswordLoginPolicy: passwordLoginPolicy,
		Codec:               codec,
		MFALock:             mfaPolicyLock{settings: settings},
		Audit:               AuthAudit{Command: audit, Events: repos.authEvents},
		Push:                PushSubscriptionCleanup{Subscriptions: repos.pushSubscriptions},
		Logger:              logger,
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

func DemoDependencies(wiring *DemoAccessWiring, standingSchool string, schools organizationtenancy.Query) (*identityBindingPorts.DemoDependencies, error) {
	if wiring == nil {
		return nil, nil
	}
	if wiring.MaxActiveSchools < 1 {
		return nil, errors.New("the public demo requires serve --demo-max-active-schools above zero")
	}
	return &identityBindingPorts.DemoDependencies{
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

type SchoolDirectory struct {
	Schools organizationtenancy.Query
}

func schoolFact(school organizationtenancy.School) identityaccess.School {
	return identityaccess.School{
		ID: school.ID, OrganizationID: school.OrganizationID, Name: school.Name, Slug: school.Slug,
		Subdomain: school.Subdomain, Active: school.Active, Deleted: school.IsDeleted(),
		LogoURL: schoolLogoURLFromSettings(school.Settings),
	}
}

func (d SchoolDirectory) FindSchool(ctx context.Context, id int64) (identityaccess.School, bool, error) {
	if d.Schools == nil {
		return identityaccess.School{}, false, errors.New("school directory is not composed")
	}
	school, found, err := LookupSchool(ctx, d.Schools, id)
	if err != nil || !found {
		return identityaccess.School{}, false, err
	}
	return schoolFact(school), true, nil
}

func (d SchoolDirectory) FindSchoolBySubdomain(ctx context.Context, subdomain string) (identityaccess.School, bool, error) {
	if d.Schools == nil {
		return identityaccess.School{}, false, errors.New("school directory is not composed")
	}
	school, err := d.Schools.FindSchoolBySubdomain(ctx, subdomain)
	if errors.Is(err, organizationtenancy.ErrSchoolNotFound) {
		return identityaccess.School{}, false, nil
	}
	if err != nil {
		return identityaccess.School{}, false, err
	}
	return schoolFact(school), true, nil
}

func (d SchoolDirectory) LockSchoolShared(ctx context.Context, id int64) (identityaccess.School, bool, error) {
	if d.Schools == nil {
		return identityaccess.School{}, false, errors.New("school directory is not composed")
	}
	school, err := d.Schools.FindSchoolForShare(ctx, id)
	if errors.Is(err, organizationtenancy.ErrSchoolNotFound) {
		return identityaccess.School{}, false, nil
	}
	if err != nil {
		return identityaccess.School{}, false, err
	}
	return schoolFact(school), true, nil
}

func (d SchoolDirectory) ListActiveSchoolsByID(ctx context.Context, ids []int64) ([]identityaccess.School, error) {
	if d.Schools == nil {
		return nil, errors.New("school directory is not composed")
	}
	if len(ids) == 0 {
		return []identityaccess.School{}, nil
	}
	schools, err := d.Schools.ListSchoolsByID(ctx, ids)
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
func (d SchoolDirectory) ListManageableSchoolIDs(ctx context.Context, organizationID int64) ([]int64, error) {
	if d.Schools == nil {
		return nil, errors.New("school directory is not composed")
	}
	schools, err := d.Schools.ListSchoolsByOrganization(ctx, organizationID)
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

type PersonDirectory struct{ Persons personRows }

func (d PersonDirectory) FindPersonName(ctx context.Context, accountID int64) (string, string, bool, error) {
	if d.Persons == nil {
		return "", "", false, errors.New("person repository is not composed")
	}
	person, err := d.Persons.FindByAccountID(ctx, accountID)
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
	if hash == "" {
		var err error
		hash, err = dummyPasswordHash()
		if err != nil {
			return false, err
		}
	}
	return securityruntime.VerifyPassword(password, hash)
}

// This is not an account credential. Equal-cost verification prevents a missing
// identity or password from skipping Argon2id work; nothing is persisted/logged.
var dummyPasswordHash = sync.OnceValues(func() (string, error) {
	value, err := securityruntime.RandomLowerAlphanumeric(32)
	if err != nil {
		return "", err
	}
	return securityruntime.HashPassword(value)
})

// A platform identity can span schools and portals. A school's override must
// not weaken protection by choosing a different login host or portal.
func passwordLoginPolicy() (int, time.Duration, error) {
	threshold := configModel.GetDefinition(configModel.KeyAccountLockoutThreshold)
	duration := configModel.GetDefinition(configModel.KeyAccountLockoutDurationMinutes)
	if threshold == nil || duration == nil {
		return 0, 0, errors.New("password login policy is not registered")
	}
	attempts, okAttempts := threshold.Default.(int)
	minutes, okMinutes := duration.Default.(int)
	if !okAttempts || !okMinutes || attempts <= 0 || minutes <= 0 {
		return 0, 0, errors.New("invalid password login policy")
	}
	return attempts, time.Duration(minutes) * time.Minute, nil
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

type AuthAudit struct {
	Command auditModels.Command
	Events  authEventLedger
}

func (a AuthAudit) RecordAuthEvent(ctx context.Context, event identityaccess.AuthEvent) error {
	if a.Command == nil {
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
	return a.Command.Append(ctx, row)
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
func (a AuthAudit) ListPendingAccountWideWipes(ctx context.Context) ([]identityaccess.PendingAccountWideWipe, error) {
	if a.Events == nil {
		return nil, nil
	}
	pending, err := a.Events.ListPendingAccountWideWipes(ctx, time.Time{})
	if err != nil {
		return nil, err
	}
	return pendingWipes(pending), nil
}

func (a AuthAudit) ClaimPendingAccountWideWipes(ctx context.Context, accountID int64) ([]identityaccess.PendingAccountWideWipe, error) {
	if a.Events == nil {
		return nil, nil
	}
	claimed, err := a.Events.ClaimPendingAccountWideWipes(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return pendingWipes(claimed), nil
}

type PushSubscriptionCleanup struct {
	Subscriptions pushSubscriptionRows
}

func (p PushSubscriptionCleanup) DeleteStaffByAccount(ctx context.Context, accountID int64) error {
	if p.Subscriptions == nil {
		return nil
	}
	return p.Subscriptions.DeleteStaffByAccountID(ctx, accountID)
}

func (p PushSubscriptionCleanup) DeleteSchoolByAccount(ctx context.Context, accountID int64) error {
	if p.Subscriptions == nil {
		return nil
	}
	return p.Subscriptions.DeleteSchoolByAccountID(ctx, accountID)
}

func (p PushSubscriptionCleanup) DeleteParentByAccount(ctx context.Context, accountID int64) error {
	if p.Subscriptions == nil {
		return nil
	}
	return p.Subscriptions.DeleteParentByAccountID(ctx, accountID)
}

func (p PushSubscriptionCleanup) DeleteByTokenFamily(ctx context.Context, accountID int64, familyID string) error {
	if p.Subscriptions == nil {
		return nil
	}
	return p.Subscriptions.DeleteByTokenFamilyID(ctx, accountID, familyID)
}

func (p PushSubscriptionCleanup) DeleteUnboundByAccount(ctx context.Context, accountID, tenantID int64, portal string) error {
	if p.Subscriptions == nil {
		return nil
	}
	return p.Subscriptions.DeleteUnboundByAccountForPortal(ctx, accountID, tenantID, portal)
}

func (p PushSubscriptionCleanup) DeleteOrphaned(ctx context.Context) error {
	if p.Subscriptions == nil {
		return nil
	}
	return p.Subscriptions.DeleteOrphanedSubscriptions(ctx)
}
