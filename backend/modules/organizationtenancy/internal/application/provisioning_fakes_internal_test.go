package application

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
	"github.com/stretchr/testify/require"
)

// The provisioning tests drive the application over in-memory owners. The
// Organizations facade is the real organizationtenancy.Module over an
// in-memory engine, so its normalisation and validation stay in play.

var (
	fixedTime  = time.Date(2026, time.March, 2, 9, 0, 0, 0, time.UTC)
	operatorIP = net.IPv4(127, 0, 0, 1)
	errBoom    = errors.New("boom")
)

const testOperatorID int64 = 7

type adminCtxKey struct{}

func isAdmin(ctx context.Context) bool {
	admin, _ := ctx.Value(adminCtxKey{}).(bool)
	return admin
}

type tenantCtxKey struct{}

func tenantOf(ctx context.Context) (int64, bool) {
	tenantID, ok := ctx.Value(tenantCtxKey{}).(int64)
	return tenantID, ok
}

// adminGuard records port calls made outside a transaction, and audit calls
// made outside the administrative transaction.
type adminGuard struct {
	mu         sync.Mutex
	violations []string
}

func (g *adminGuard) check(ctx context.Context, call string) {
	if _, inTenant := tenantOf(ctx); isAdmin(ctx) || inTenant {
		return
	}
	g.record(call)
}

func (g *adminGuard) checkAdmin(ctx context.Context, call string) {
	if isAdmin(ctx) {
		return
	}
	g.record(call)
}

// checkTenant records a school write made outside that school's transaction.
func (g *adminGuard) checkTenant(ctx context.Context, call string, tenantID int64) {
	if active, inTenant := tenantOf(ctx); inTenant && active == tenantID {
		return
	}
	g.record(call)
}

func (g *adminGuard) record(call string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.violations = append(g.violations, call)
}

// fakeTx marks the context as administrative or school-scoped and records
// every run. Like the tenant runtime, it refuses to nest one scope in the
// other.
type fakeTx struct {
	adminRuns  int
	readRuns   int
	tenantRuns int
	lastErr    error
}

func (tx *fakeTx) RunAdmin(ctx context.Context, fn func(context.Context) error) error {
	if _, inTenant := tenantOf(ctx); inTenant {
		return errors.New("ambient transaction is not administrative")
	}
	tx.adminRuns++
	err := fn(context.WithValue(ctx, adminCtxKey{}, true))
	tx.lastErr = err
	return err
}

func (tx *fakeTx) RunTenant(ctx context.Context, tenantID int64, fn func(context.Context) error) error {
	if active, inTenant := tenantOf(ctx); isAdmin(ctx) || (inTenant && active != tenantID) {
		return errors.New("ambient transaction has no matching tenant")
	}
	tx.tenantRuns++
	err := fn(context.WithValue(ctx, tenantCtxKey{}, tenantID))
	tx.lastErr = err
	return err
}

func (tx *fakeTx) RunRead(ctx context.Context, fn func(context.Context) error) error {
	tx.readRuns++
	return fn(ctx)
}

// fakeEngine is the in-memory organisation and school store behind the real
// organizationtenancy.Module. fail injects an error per engine method name.
type fakeEngine struct {
	orgs    map[int64]*organizationtenancy.Organization
	schools map[int64]*organizationtenancy.School
	nextID  int64
	fail    map[string]error
	calls   []string
	updates []organizationtenancy.UpdateSchool
}

func newFakeEngine() *fakeEngine {
	return &fakeEngine{
		orgs:    map[int64]*organizationtenancy.Organization{},
		schools: map[int64]*organizationtenancy.School{},
		nextID:  1000,
		fail:    map[string]error{},
	}
}

func (e *fakeEngine) call(name string) error {
	e.calls = append(e.calls, name)
	return e.fail[name]
}

func (e *fakeEngine) called(name string) bool {
	for _, call := range e.calls {
		if call == name {
			return true
		}
	}
	return false
}

func (e *fakeEngine) newID() int64 {
	e.nextID++
	return e.nextID
}

func (e *fakeEngine) Create(_ context.Context, input organizationtenancy.CreateOrganization) (organizationtenancy.Organization, error) {
	if err := e.call("Create"); err != nil {
		return organizationtenancy.Organization{}, err
	}
	for _, org := range e.orgs {
		if org.Slug == input.Slug {
			return organizationtenancy.Organization{}, organizationtenancy.ErrOrganizationSlugConflict
		}
	}
	org := &organizationtenancy.Organization{
		ID: e.newID(), CreatedAt: fixedTime, UpdatedAt: fixedTime,
		Name: input.Name, Slug: input.Slug, Active: input.Active,
	}
	e.orgs[org.ID] = org
	return *org, nil
}

func (e *fakeEngine) Update(_ context.Context, input organizationtenancy.UpdateOrganization) (organizationtenancy.Organization, error) {
	if err := e.call("Update"); err != nil {
		return organizationtenancy.Organization{}, err
	}
	org, ok := e.orgs[input.ID]
	if !ok {
		return organizationtenancy.Organization{}, organizationtenancy.ErrOrganizationNotFound
	}
	for _, other := range e.orgs {
		if other.ID != input.ID && other.Slug == input.Slug {
			return organizationtenancy.Organization{}, organizationtenancy.ErrOrganizationSlugConflict
		}
	}
	org.Name, org.Slug, org.Active = input.Name, input.Slug, input.Active
	return *org, nil
}

func (e *fakeEngine) SoftDelete(_ context.Context, id int64) (organizationtenancy.Organization, error) {
	if err := e.call("SoftDelete"); err != nil {
		return organizationtenancy.Organization{}, err
	}
	org, ok := e.orgs[id]
	if !ok {
		return organizationtenancy.Organization{}, organizationtenancy.ErrOrganizationNotFound
	}
	if org.IsDeleted() {
		return organizationtenancy.Organization{}, organizationtenancy.ErrOrganizationAlreadyDeleted
	}
	count := 0
	for _, school := range e.schools {
		if school.OrganizationID == id && !school.IsDeleted() {
			count++
		}
	}
	if count > 0 {
		return organizationtenancy.Organization{}, &organizationtenancy.OrganizationHasSchoolsError{SchoolCount: count}
	}
	deletedAt := fixedTime
	org.DeletedAt = &deletedAt
	return *org, nil
}

func (e *fakeEngine) Restore(_ context.Context, id int64) (organizationtenancy.Organization, error) {
	if err := e.call("Restore"); err != nil {
		return organizationtenancy.Organization{}, err
	}
	org, ok := e.orgs[id]
	if !ok {
		return organizationtenancy.Organization{}, organizationtenancy.ErrOrganizationNotFound
	}
	if !org.IsDeleted() {
		return organizationtenancy.Organization{}, organizationtenancy.ErrOrganizationNotDeleted
	}
	org.DeletedAt = nil
	return *org, nil
}

func (e *fakeEngine) findOrg(name string, id int64) (organizationtenancy.Organization, error) {
	if err := e.call(name); err != nil {
		return organizationtenancy.Organization{}, err
	}
	org, ok := e.orgs[id]
	if !ok {
		return organizationtenancy.Organization{}, organizationtenancy.ErrOrganizationNotFound
	}
	return *org, nil
}

func (e *fakeEngine) FindByID(_ context.Context, id int64) (organizationtenancy.Organization, error) {
	return e.findOrg("FindByID", id)
}

func (e *fakeEngine) FindForMutation(_ context.Context, id int64) (organizationtenancy.Organization, error) {
	return e.findOrg("FindForMutation", id)
}

func (e *fakeEngine) FindForSchoolMutation(_ context.Context, id int64) (organizationtenancy.Organization, error) {
	return e.findOrg("FindForSchoolMutation", id)
}

func (e *fakeEngine) FindBySlug(_ context.Context, slug string) (organizationtenancy.Organization, error) {
	if err := e.call("FindBySlug"); err != nil {
		return organizationtenancy.Organization{}, err
	}
	for _, org := range e.orgs {
		if org.Slug == slug {
			return *org, nil
		}
	}
	return organizationtenancy.Organization{}, organizationtenancy.ErrOrganizationNotFound
}

func (e *fakeEngine) sortedOrgs(keep func(organizationtenancy.Organization) bool) []organizationtenancy.Organization {
	result := []organizationtenancy.Organization{}
	for _, org := range e.orgs {
		if keep(*org) {
			result = append(result, *org)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func (e *fakeEngine) List(context.Context) ([]organizationtenancy.Organization, error) {
	if err := e.call("List"); err != nil {
		return nil, err
	}
	return e.sortedOrgs(func(organizationtenancy.Organization) bool { return true }), nil
}

func (e *fakeEngine) ListByIDs(_ context.Context, ids []int64) ([]organizationtenancy.Organization, error) {
	if err := e.call("ListByIDs"); err != nil {
		return nil, err
	}
	wanted := idSet(ids)
	return e.sortedOrgs(func(org organizationtenancy.Organization) bool { return wanted[org.ID] }), nil
}

func (e *fakeEngine) CountByIDs(ctx context.Context, ids []int64) (int, error) {
	orgs, err := e.ListByIDs(ctx, ids)
	return len(orgs), err
}

func (e *fakeEngine) CreateSchool(_ context.Context, input organizationtenancy.CreateSchool) (organizationtenancy.School, error) {
	if err := e.call("CreateSchool"); err != nil {
		return organizationtenancy.School{}, err
	}
	org, ok := e.orgs[input.OrganizationID]
	if !ok {
		return organizationtenancy.School{}, organizationtenancy.ErrOrganizationNotFound
	}
	if org.IsDeleted() {
		return organizationtenancy.School{}, organizationtenancy.ErrOrganizationDeleted
	}
	for _, school := range e.schools {
		if school.Subdomain == input.Subdomain {
			return organizationtenancy.School{}, organizationtenancy.ErrSchoolDomainConflict
		}
		if school.OrganizationID == input.OrganizationID && school.Slug == input.Slug {
			return organizationtenancy.School{}, organizationtenancy.ErrSchoolSlugConflict
		}
	}
	school := &organizationtenancy.School{
		ID: e.newID(), CreatedAt: fixedTime, UpdatedAt: fixedTime,
		OrganizationID: input.OrganizationID, Name: input.Name, Slug: input.Slug, Subdomain: input.Subdomain,
		Active: input.Active, Hidden: input.Hidden, Settings: input.Settings,
		Address: input.Address, City: input.City, Zip: input.Zip, Phone: input.Phone, Email: input.Email,
		DevicePinHash: input.DevicePinHash,
	}
	e.schools[school.ID] = school
	return *school, nil
}

func (e *fakeEngine) UpdateSchool(_ context.Context, input organizationtenancy.UpdateSchool) (organizationtenancy.School, error) {
	e.updates = append(e.updates, input)
	if err := e.call("UpdateSchool"); err != nil {
		return organizationtenancy.School{}, err
	}
	school, ok := e.schools[input.ID]
	if !ok {
		return organizationtenancy.School{}, organizationtenancy.ErrSchoolNotFound
	}
	school.OrganizationID, school.Name, school.Slug, school.Subdomain = input.OrganizationID, input.Name, input.Slug, input.Subdomain
	school.Active, school.Hidden, school.Settings = input.Active, input.Hidden, input.Settings
	school.Address, school.City, school.Zip, school.Phone, school.Email = input.Address, input.City, input.Zip, input.Phone, input.Email
	school.DevicePinHash = input.DevicePinHash
	return *school, nil
}

func (e *fakeEngine) SoftDeleteSchool(_ context.Context, id int64) (organizationtenancy.School, error) {
	if err := e.call("SoftDeleteSchool"); err != nil {
		return organizationtenancy.School{}, err
	}
	school, ok := e.schools[id]
	if !ok {
		return organizationtenancy.School{}, organizationtenancy.ErrSchoolNotFound
	}
	if school.IsDeleted() {
		return organizationtenancy.School{}, organizationtenancy.ErrSchoolAlreadyDeleted
	}
	deletedAt := fixedTime
	school.DeletedAt = &deletedAt
	return *school, nil
}

func (e *fakeEngine) RestoreSchool(_ context.Context, id int64) (organizationtenancy.School, error) {
	if err := e.call("RestoreSchool"); err != nil {
		return organizationtenancy.School{}, err
	}
	school, ok := e.schools[id]
	if !ok {
		return organizationtenancy.School{}, organizationtenancy.ErrSchoolNotFound
	}
	if !school.IsDeleted() {
		return organizationtenancy.School{}, organizationtenancy.ErrSchoolNotDeleted
	}
	school.DeletedAt = nil
	return *school, nil
}

func (e *fakeEngine) FindSchoolByID(_ context.Context, id int64, _ string) (organizationtenancy.School, error) {
	if err := e.call("FindSchoolByID"); err != nil {
		return organizationtenancy.School{}, err
	}
	school, ok := e.schools[id]
	if !ok {
		return organizationtenancy.School{}, organizationtenancy.ErrSchoolNotFound
	}
	return *school, nil
}

func (e *fakeEngine) findSchoolWhere(name string, match func(organizationtenancy.School) bool) (organizationtenancy.School, error) {
	if err := e.call(name); err != nil {
		return organizationtenancy.School{}, err
	}
	for _, school := range e.sortedSchools(match) {
		return school, nil
	}
	return organizationtenancy.School{}, organizationtenancy.ErrSchoolNotFound
}

func (e *fakeEngine) FindSchoolBySlug(_ context.Context, slug string) (organizationtenancy.School, error) {
	return e.findSchoolWhere("FindSchoolBySlug", func(s organizationtenancy.School) bool { return s.Slug == slug })
}

func (e *fakeEngine) FindSchoolByOrganizationAndSlug(_ context.Context, organizationID int64, slug string) (organizationtenancy.School, error) {
	return e.findSchoolWhere("FindSchoolByOrganizationAndSlug", func(s organizationtenancy.School) bool {
		return s.OrganizationID == organizationID && s.Slug == slug
	})
}

func (e *fakeEngine) FindSchoolBySubdomain(_ context.Context, subdomain string) (organizationtenancy.School, error) {
	return e.findSchoolWhere("FindSchoolBySubdomain", func(s organizationtenancy.School) bool { return s.Subdomain == subdomain })
}

func (e *fakeEngine) sortedSchools(keep func(organizationtenancy.School) bool) []organizationtenancy.School {
	result := []organizationtenancy.School{}
	for _, school := range e.schools {
		if keep(*school) {
			result = append(result, *school)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func (e *fakeEngine) listSchools(name string, keep func(organizationtenancy.School) bool) ([]organizationtenancy.School, error) {
	if err := e.call(name); err != nil {
		return nil, err
	}
	return e.sortedSchools(keep), nil
}

func (e *fakeEngine) ListSchools(context.Context) ([]organizationtenancy.School, error) {
	return e.listSchools("ListSchools", func(organizationtenancy.School) bool { return true })
}

func (e *fakeEngine) ListSchoolsByID(_ context.Context, ids []int64) ([]organizationtenancy.School, error) {
	wanted := idSet(ids)
	return e.listSchools("ListSchoolsByID", func(s organizationtenancy.School) bool { return wanted[s.ID] })
}

func (e *fakeEngine) ListSchoolsByOrganization(_ context.Context, organizationID int64) ([]organizationtenancy.School, error) {
	return e.listSchools("ListSchoolsByOrganization", func(s organizationtenancy.School) bool { return s.OrganizationID == organizationID })
}

func (e *fakeEngine) ListNonDeletedSchools(context.Context) ([]organizationtenancy.School, error) {
	return e.listSchools("ListNonDeletedSchools", func(s organizationtenancy.School) bool { return !s.IsDeleted() })
}

func (e *fakeEngine) ListActiveSchools(context.Context) ([]organizationtenancy.School, error) {
	return e.listSchools("ListActiveSchools", func(s organizationtenancy.School) bool { return !s.IsDeleted() && s.Active })
}

func (e *fakeEngine) ListPublicSchools(context.Context) ([]organizationtenancy.School, error) {
	return e.listSchools("ListPublicSchools", func(s organizationtenancy.School) bool {
		return !s.IsDeleted() && s.Active && !s.Hidden
	})
}

func (e *fakeEngine) CountSchoolsByID(ctx context.Context, ids []int64) (int, error) {
	schools, err := e.ListSchoolsByID(ctx, ids)
	return len(schools), err
}

func idSet(ids []int64) map[int64]bool {
	set := make(map[int64]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}

// fakeDashboard derives the school and organisation rows from the engine so
// the listings see the same schools as the facade.
type fakeDashboard struct {
	guard        *adminGuard
	engine       *fakeEngine
	counts       domain.DashboardCounts
	accounts     map[int64]int
	pwaRows      []domain.PWAUsageRow
	pwaTenantID  int64
	pwaWindow    time.Duration
	schoolFilter []*int64
	fail         map[string]error
}

func (d *fakeDashboard) Counts(ctx context.Context) (domain.DashboardCounts, error) {
	d.guard.check(ctx, "Dashboard.Counts")
	if err := d.fail["Counts"]; err != nil {
		return domain.DashboardCounts{}, err
	}
	return d.counts, nil
}

func (d *fakeDashboard) OrganizationSummaries(ctx context.Context) ([]domain.OrganizationSummary, error) {
	d.guard.check(ctx, "Dashboard.OrganizationSummaries")
	if err := d.fail["OrganizationSummaries"]; err != nil {
		return nil, err
	}
	rows := []domain.OrganizationSummary{}
	for _, org := range d.engine.sortedOrgs(func(organizationtenancy.Organization) bool { return true }) {
		schools := 0
		for _, school := range d.engine.schools {
			if school.OrganizationID == org.ID && !school.IsDeleted() {
				schools++
			}
		}
		rows = append(rows, domain.OrganizationSummary{
			ID: org.ID, Name: org.Name, Slug: org.Slug, Active: org.Active,
			CreatedAt: org.CreatedAt, UpdatedAt: org.UpdatedAt, DeletedAt: org.DeletedAt, Settings: org.Settings,
			SchoolCount: schools, AccountCount: d.accounts[org.ID],
		})
	}
	return rows, nil
}

func (d *fakeDashboard) SchoolSummaries(ctx context.Context, organizationID *int64) ([]domain.SchoolSummary, error) {
	d.guard.check(ctx, "Dashboard.SchoolSummaries")
	d.schoolFilter = append(d.schoolFilter, organizationID)
	if err := d.fail["SchoolSummaries"]; err != nil {
		return nil, err
	}
	rows := []domain.SchoolSummary{}
	for _, school := range d.engine.sortedSchools(func(s organizationtenancy.School) bool {
		return organizationID == nil || s.OrganizationID == *organizationID
	}) {
		orgName := ""
		if org, ok := d.engine.orgs[school.OrganizationID]; ok {
			orgName = org.Name
		}
		rows = append(rows, domain.SchoolSummary{
			ID: school.ID, OrganizationID: school.OrganizationID, OrganizationName: orgName,
			Name: school.Name, Slug: school.Slug, Subdomain: school.Subdomain,
			Active: school.Active, Hidden: school.Hidden, CreatedAt: school.CreatedAt, UpdatedAt: school.UpdatedAt,
			DeletedAt: school.DeletedAt, Address: school.Address, City: school.City, Zip: school.Zip,
			Phone: school.Phone, Email: school.Email, Settings: school.Settings,
			AccountCount: d.accounts[school.ID],
		})
	}
	return rows, nil
}

func (d *fakeDashboard) PWAUsage(ctx context.Context, tenantID int64, window time.Duration) ([]domain.PWAUsageRow, error) {
	d.guard.check(ctx, "Dashboard.PWAUsage")
	d.pwaTenantID, d.pwaWindow = tenantID, window
	if err := d.fail["PWAUsage"]; err != nil {
		return nil, err
	}
	return d.pwaRows, nil
}

// fakeIdentity is the Identity & Access seam.
type fakeIdentity struct {
	guard         *adminGuard
	roles         []domain.Role
	invitations   []domain.SchoolAdminInvitationRequest
	registrations []domain.SchoolAccountRegistration
	identities    []domain.SchoolIdentityRequest
	assigned      [][3]int64
	schoolAccts   []domain.SchoolAccount
	orgAccts      []domain.OrganizationAccount
	orgAcctsFor   []int64
	revoked       []int64
	invalidated   []int64
	deactivated   []int64
	anonymized    map[int64]string
	calls         []string
	fail          map[string]error
}

func (f *fakeIdentity) call(ctx context.Context, name string) error {
	f.guard.check(ctx, "Identity."+name)
	f.calls = append(f.calls, name)
	return f.fail[name]
}

func (f *fakeIdentity) ListSystemRoles(ctx context.Context) ([]domain.Role, error) {
	if err := f.call(ctx, "ListSystemRoles"); err != nil {
		return nil, err
	}
	result := []domain.Role{}
	for _, role := range f.roles {
		if role.IsSystem {
			result = append(result, role)
		}
	}
	return result, nil
}

func (f *fakeIdentity) FindSystemRole(ctx context.Context, name string) (domain.Role, bool, error) {
	if err := f.call(ctx, "FindSystemRole"); err != nil {
		return domain.Role{}, false, err
	}
	for _, role := range f.roles {
		if role.IsSystem && role.TenantID == nil && role.Name == name {
			return role, true, nil
		}
	}
	return domain.Role{}, false, nil
}

func (f *fakeIdentity) FindRole(ctx context.Context, id int64) (domain.Role, bool, error) {
	if err := f.call(ctx, "FindRole"); err != nil {
		return domain.Role{}, false, err
	}
	for _, role := range f.roles {
		if role.ID == id {
			return role, true, nil
		}
	}
	return domain.Role{}, false, nil
}

func (f *fakeIdentity) roleName(id int64) string {
	for _, role := range f.roles {
		if role.ID == id {
			return role.Name
		}
	}
	return ""
}

func (f *fakeIdentity) InviteSchoolAdmin(ctx context.Context, request domain.SchoolAdminInvitationRequest) (domain.SchoolAdminInvitation, error) {
	if err := f.call(ctx, "InviteSchoolAdmin"); err != nil {
		return domain.SchoolAdminInvitation{}, err
	}
	f.invitations = append(f.invitations, request)
	return domain.SchoolAdminInvitation{
		ID: 900, Email: request.Email, RoleID: request.RoleID, RoleName: f.roleName(request.RoleID),
		Token: "invite-token", ExpiresAt: fixedTime.Add(48 * time.Hour),
		FirstName: request.FirstName, LastName: request.LastName, Position: request.Position,
		CaregiverEnabled: request.CaregiverEnabled,
	}, nil
}

func (f *fakeIdentity) RegisterSchoolAccount(ctx context.Context, registration domain.SchoolAccountRegistration) (domain.CreatedAccount, error) {
	f.guard.checkTenant(ctx, "Identity.RegisterSchoolAccount", registration.TenantID)
	if err := f.call(ctx, "RegisterSchoolAccount"); err != nil {
		return domain.CreatedAccount{}, err
	}
	f.registrations = append(f.registrations, registration)
	username := registration.Username
	return domain.CreatedAccount{
		ID: 100, CreatedAt: fixedTime, UpdatedAt: fixedTime,
		Email: registration.Email, Username: &username, Active: true,
	}, nil
}

func (f *fakeIdentity) EnsureSchoolIdentity(ctx context.Context, request domain.SchoolIdentityRequest) error {
	f.guard.checkTenant(ctx, "Identity.EnsureSchoolIdentity", request.TenantID)
	if err := f.call(ctx, "EnsureSchoolIdentity"); err != nil {
		return err
	}
	f.identities = append(f.identities, request)
	return nil
}

func (f *fakeIdentity) AssignRole(ctx context.Context, tenantID, accountID, roleID int64) error {
	f.guard.checkTenant(ctx, "Identity.AssignRole", tenantID)
	if err := f.call(ctx, "AssignRole"); err != nil {
		return err
	}
	f.assigned = append(f.assigned, [3]int64{tenantID, accountID, roleID})
	return nil
}

func (f *fakeIdentity) ListSchoolAccounts(ctx context.Context, _ int64) ([]domain.SchoolAccount, error) {
	if err := f.call(ctx, "ListSchoolAccounts"); err != nil {
		return nil, err
	}
	return f.schoolAccts, nil
}

func (f *fakeIdentity) ListOrganizationAccounts(ctx context.Context, organizationID int64) ([]domain.OrganizationAccount, error) {
	if err := f.call(ctx, "ListOrganizationAccounts"); err != nil {
		return nil, err
	}
	f.orgAcctsFor = append(f.orgAcctsFor, organizationID)
	return f.orgAccts, nil
}

func (f *fakeIdentity) ListAllAccounts(ctx context.Context) ([]domain.OrganizationAccount, error) {
	if err := f.call(ctx, "ListAllAccounts"); err != nil {
		return nil, err
	}
	return f.orgAccts, nil
}

func (f *fakeIdentity) RevokeSchoolSessions(ctx context.Context, tenantID int64) (int, error) {
	if err := f.call(ctx, "RevokeSchoolSessions"); err != nil {
		return 0, err
	}
	f.revoked = append(f.revoked, tenantID)
	return 4, nil
}

func (f *fakeIdentity) InvalidatePendingInvitations(ctx context.Context, tenantID int64) (int, error) {
	if err := f.call(ctx, "InvalidatePendingInvitations"); err != nil {
		return 0, err
	}
	f.invalidated = append(f.invalidated, tenantID)
	return 2, nil
}

func (f *fakeIdentity) DeactivateAccount(ctx context.Context, accountID int64) error {
	if err := f.call(ctx, "DeactivateAccount"); err != nil {
		return err
	}
	f.deactivated = append(f.deactivated, accountID)
	return nil
}

func (f *fakeIdentity) AnonymizeAccount(ctx context.Context, accountID int64, email string) error {
	if err := f.call(ctx, "AnonymizeAccount"); err != nil {
		return err
	}
	f.anonymized[accountID] = email
	return nil
}

// fakeDevices is the Device Fleet seam with the store's uniqueness rules.
// createErrs and updateErrs are consumed one per call before the store runs.
type fakeDevices struct {
	guard      *adminGuard
	devices    map[int64]*domain.Device
	nextID     int64
	created    []domain.NewDevice
	updated    []domain.Device
	deleted    []int64
	createErrs []error
	updateErrs []error
	fail       map[string]error
}

func (f *fakeDevices) add(device domain.Device) {
	stored := device
	f.devices[device.ID] = &stored
}

func (f *fakeDevices) keyTaken(key *string, exceptID int64) bool {
	if key == nil {
		return false
	}
	for _, device := range f.devices {
		if device.ID != exceptID && device.APIKey != nil && *device.APIKey == *key {
			return true
		}
	}
	return false
}

func popErr(queue *[]error) error {
	if len(*queue) == 0 {
		return nil
	}
	err := (*queue)[0]
	*queue = (*queue)[1:]
	return err
}

func (f *fakeDevices) CountDevicesByTenant(ctx context.Context) (map[int64]int, error) {
	f.guard.check(ctx, "Devices.CountDevicesByTenant")
	if err := f.fail["CountDevicesByTenant"]; err != nil {
		return nil, err
	}
	counts := map[int64]int{}
	for _, device := range f.devices {
		if device.ArchivedAt == nil {
			counts[device.TenantID]++
		}
	}
	return counts, nil
}

func (f *fakeDevices) ListDevicesByTenant(ctx context.Context, tenantIDs []int64) ([]domain.Device, error) {
	f.guard.check(ctx, "Devices.ListDevicesByTenant")
	if err := f.fail["ListDevicesByTenant"]; err != nil {
		return nil, err
	}
	wanted := idSet(tenantIDs)
	result := []domain.Device{}
	for _, device := range f.devices {
		if wanted[device.TenantID] && device.ArchivedAt == nil {
			result = append(result, *device)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

func (f *fakeDevices) FindDeviceForUpdate(ctx context.Context, id int64) (domain.Device, bool, error) {
	f.guard.check(ctx, "Devices.FindDeviceForUpdate")
	if err := f.fail["FindDeviceForUpdate"]; err != nil {
		return domain.Device{}, false, err
	}
	device, ok := f.devices[id]
	if !ok || device.ArchivedAt != nil {
		return domain.Device{}, false, nil
	}
	return *device, true, nil
}

func (f *fakeDevices) CreateDevice(ctx context.Context, input domain.NewDevice) (domain.Device, error) {
	f.guard.check(ctx, "Devices.CreateDevice")
	f.created = append(f.created, input)
	if err := popErr(&f.createErrs); err != nil {
		return domain.Device{}, err
	}
	for _, device := range f.devices {
		if device.ArchivedAt == nil && device.TenantID == input.TenantID && device.DeviceID == input.DeviceID {
			return domain.Device{}, domain.ErrDeviceIDTaken
		}
	}
	if f.keyTaken(input.APIKey, 0) {
		return domain.Device{}, domain.ErrDeviceAPIKeyTaken
	}
	f.nextID++
	device := domain.Device{
		ID: f.nextID, TenantID: input.TenantID, CreatedAt: fixedTime, UpdatedAt: fixedTime,
		DeviceID: input.DeviceID, DeviceType: input.DeviceType, Name: input.Name,
		Status: input.Status, APIKey: input.APIKey,
	}
	f.add(device)
	return device, nil
}

func (f *fakeDevices) UpdateDevice(ctx context.Context, device domain.Device) error {
	f.guard.check(ctx, "Devices.UpdateDevice")
	f.updated = append(f.updated, device)
	if err := popErr(&f.updateErrs); err != nil {
		return err
	}
	if f.keyTaken(device.APIKey, device.ID) {
		return domain.ErrDeviceAPIKeyTaken
	}
	f.add(device)
	return nil
}

func (f *fakeDevices) DeleteDevice(ctx context.Context, id int64) error {
	f.guard.check(ctx, "Devices.DeleteDevice")
	if err := f.fail["DeleteDevice"]; err != nil {
		return err
	}
	f.deleted = append(f.deleted, id)
	delete(f.devices, id)
	return nil
}

// fakePeople is the People Directory seam. calls keeps the write order.
type fakePeople struct {
	guard         *adminGuard
	persons       map[int64]domain.Person
	staff         map[int64]domain.StaffMember
	listings      []domain.PersonListing
	listedTenants [][]int64
	counts        map[int64]int
	calls         []string
	fail          map[string]error
}

func (f *fakePeople) call(ctx context.Context, name string) error {
	f.guard.check(ctx, "People."+name)
	f.calls = append(f.calls, name)
	return f.fail[name]
}

func (f *fakePeople) CountPersonsByTenant(ctx context.Context) (map[int64]int, error) {
	if err := f.call(ctx, "CountPersonsByTenant"); err != nil {
		return nil, err
	}
	return f.counts, nil
}

func (f *fakePeople) ListPersons(ctx context.Context, tenantIDs []int64) ([]domain.PersonListing, error) {
	if err := f.call(ctx, "ListPersons"); err != nil {
		return nil, err
	}
	f.listedTenants = append(f.listedTenants, tenantIDs)
	wanted := idSet(tenantIDs)
	result := []domain.PersonListing{}
	for _, person := range f.listings {
		if wanted[person.TenantID] {
			result = append(result, person)
		}
	}
	return result, nil
}

func (f *fakePeople) FindPerson(ctx context.Context, id int64) (domain.Person, bool, error) {
	if err := f.call(ctx, "FindPerson"); err != nil {
		return domain.Person{}, false, err
	}
	person, ok := f.persons[id]
	return person, ok, nil
}

func (f *fakePeople) FindStaff(ctx context.Context, personID int64) (domain.StaffMember, bool, error) {
	if err := f.call(ctx, "FindStaff"); err != nil {
		return domain.StaffMember{}, false, err
	}
	staff, ok := f.staff[personID]
	return staff, ok, nil
}

func (f *fakePeople) UnlinkRFIDCard(ctx context.Context, _ int64) error {
	return f.call(ctx, "UnlinkRFIDCard")
}

func (f *fakePeople) UnlinkAccount(ctx context.Context, _ int64) error {
	return f.call(ctx, "UnlinkAccount")
}

func (f *fakePeople) AnonymizeAndSoftDelete(ctx context.Context, personID int64) error {
	if err := f.call(ctx, "AnonymizeAndSoftDelete"); err != nil {
		return err
	}
	delete(f.persons, personID)
	return nil
}

// fakePresence is the Student Presence seam.
type fakePresence struct {
	guard            *adminGuard
	sessions         map[int64]*domain.DeviceSession
	sessionLookups   [][2]int64
	sessionErr       error
	supervisions     map[int64]int
	supervisionCalls [][2]int64
	supervisionErr   error
}

func (f *fakePresence) ActiveDeviceSession(ctx context.Context, tenantID, deviceID int64) (*domain.DeviceSession, error) {
	f.guard.check(ctx, "Presence.ActiveDeviceSession")
	f.sessionLookups = append(f.sessionLookups, [2]int64{tenantID, deviceID})
	if f.sessionErr != nil {
		return nil, f.sessionErr
	}
	return f.sessions[deviceID], nil
}

func (f *fakePresence) CountActiveSupervisions(ctx context.Context, tenantID, staffID int64) (int, error) {
	f.guard.check(ctx, "Presence.CountActiveSupervisions")
	f.supervisionCalls = append(f.supervisionCalls, [2]int64{tenantID, staffID})
	if f.supervisionErr != nil {
		return 0, f.supervisionErr
	}
	return f.supervisions[staffID], nil
}

type fakeCategories struct {
	guard  *adminGuard
	seeded map[int64][]domain.ActivityCategory
	err    error
}

func (f *fakeCategories) SeedCategories(ctx context.Context, tenantID int64, categories []domain.ActivityCategory) error {
	f.guard.check(ctx, "Categories.SeedCategories")
	if f.err != nil {
		return f.err
	}
	f.seeded[tenantID] = append(f.seeded[tenantID], categories...)
	return nil
}

// fakeSettings records whether it was asked inside the administrative
// transaction: the real resolver opens its own tenant transaction.
type fakeSettings struct {
	minutes     int
	err         error
	tenants     []int64
	adminCalled bool
}

func (f *fakeSettings) DeviceOnlineWindowMinutes(ctx context.Context, tenantID int64) (int, error) {
	f.tenants = append(f.tenants, tenantID)
	if isAdmin(ctx) {
		f.adminCalled = true
	}
	return f.minutes, f.err
}

type fakeAudit struct {
	guard   *adminGuard
	entries []domain.OperatorAuditEntry
	err     error
}

func (f *fakeAudit) RecordOperatorAction(ctx context.Context, entry domain.OperatorAuditEntry) error {
	f.guard.checkAdmin(ctx, "Audit.RecordOperatorAction")
	if f.err != nil {
		return f.err
	}
	f.entries = append(f.entries, entry)
	return nil
}

// fakeSecrets hands out keys in order and then generated ones.
type fakeSecrets struct {
	keys  []string
	calls int
	err   error
}

func (f *fakeSecrets) APIKey() (string, error) {
	f.calls++
	if f.err != nil {
		return "", f.err
	}
	if len(f.keys) == 0 {
		return "generated-key-" + string(rune('a'+f.calls)), nil
	}
	key := f.keys[0]
	f.keys = f.keys[1:]
	return key, nil
}

func (f *fakeSecrets) UsernameSuffix() string { return "abc123" }

type provisioningHarness struct {
	guard      *adminGuard
	engine     *fakeEngine
	tx         *fakeTx
	dashboard  *fakeDashboard
	identity   *fakeIdentity
	devices    *fakeDevices
	people     *fakePeople
	presence   *fakePresence
	categories *fakeCategories
	settings   *fakeSettings
	audit      *fakeAudit
	secrets    *fakeSecrets
	logs       *bytes.Buffer
	svc        *Provisioning
}

func newHarness(t *testing.T) *provisioningHarness {
	t.Helper()
	guard := &adminGuard{}
	engine := newFakeEngine()
	h := &provisioningHarness{
		guard:     guard,
		engine:    engine,
		tx:        &fakeTx{},
		dashboard: &fakeDashboard{guard: guard, engine: engine, accounts: map[int64]int{}, fail: map[string]error{}},
		identity: &fakeIdentity{
			guard: guard, anonymized: map[int64]string{}, fail: map[string]error{},
			roles: []domain.Role{
				{ID: 1, Name: "admin", IsSystem: true, CaregiverPermissions: true},
				{ID: 2, Name: "user", IsSystem: true, CaregiverPermissions: true},
				{ID: 3, Name: "guardian", IsSystem: true},
				{ID: 4, Name: "teacher", IsSystem: true},
				{ID: 5, Name: "lehrkraft", IsSystem: true, Lehrkraft: true},
				{ID: 6, Name: "sekretariat", IsSystem: true},
			},
		},
		devices:    &fakeDevices{guard: guard, devices: map[int64]*domain.Device{}, nextID: 500, fail: map[string]error{}},
		people:     &fakePeople{guard: guard, persons: map[int64]domain.Person{}, staff: map[int64]domain.StaffMember{}, counts: map[int64]int{}, fail: map[string]error{}},
		presence:   &fakePresence{guard: guard, sessions: map[int64]*domain.DeviceSession{}, supervisions: map[int64]int{}},
		categories: &fakeCategories{guard: guard, seeded: map[int64][]domain.ActivityCategory{}},
		settings:   &fakeSettings{},
		audit:      &fakeAudit{guard: guard},
		secrets:    &fakeSecrets{},
		logs:       &bytes.Buffer{},
	}
	svc, err := NewProvisioning(ProvisioningDependencies{
		Organizations: organizationtenancy.NewModule(engine),
		Transaction:   h.tx, Dashboard: h.dashboard, Identity: h.identity, Devices: h.devices,
		People: h.people, Presence: h.presence, Categories: h.categories, Settings: h.settings,
		Audit: h.audit, Secrets: h.secrets,
		Logger: slog.New(slog.NewTextHandler(h.logs, &slog.HandlerOptions{Level: slog.LevelDebug})),
	})
	require.NoError(t, err)
	h.svc = svc
	t.Cleanup(func() {
		require.Empty(t, guard.violations, "port calls outside their transaction")
	})
	return h
}

func (h *provisioningHarness) addOrg(id int64, name, slug string, deleted bool) *organizationtenancy.Organization {
	org := &organizationtenancy.Organization{ID: id, CreatedAt: fixedTime, UpdatedAt: fixedTime, Name: name, Slug: slug, Active: true}
	if deleted {
		deletedAt := fixedTime
		org.DeletedAt = &deletedAt
	}
	h.engine.orgs[id] = org
	return org
}

// addSchool stores an active, live school; adjust the returned pointer for
// other states.
func (h *provisioningHarness) addSchool(id, organizationID int64, name, slug string) *organizationtenancy.School {
	school := &organizationtenancy.School{
		ID: id, CreatedAt: fixedTime, UpdatedAt: fixedTime, OrganizationID: organizationID,
		Name: name, Slug: slug, Subdomain: slug, Active: true,
	}
	h.engine.schools[id] = school
	return school
}

func deletedNow() *time.Time {
	deletedAt := fixedTime
	return &deletedAt
}

// onlyAudit returns the single recorded audit entry and checks the fields
// every provisioning write fills.
func (h *provisioningHarness) onlyAudit(t *testing.T, action, resource string, resourceID int64) domain.OperatorAuditEntry {
	t.Helper()
	require.Len(t, h.audit.entries, 1)
	entry := h.audit.entries[0]
	require.Equal(t, testOperatorID, entry.OperatorID)
	require.Equal(t, action, entry.Action)
	require.Equal(t, resource, entry.ResourceType)
	require.NotNil(t, entry.ResourceID)
	require.Equal(t, resourceID, *entry.ResourceID)
	require.True(t, operatorIP.Equal(entry.ClientIP))
	return entry
}

func auditChanges(t *testing.T, entry domain.OperatorAuditEntry) map[string]any {
	t.Helper()
	require.NotNil(t, entry.Changes)
	changes := map[string]any{}
	require.NoError(t, json.Unmarshal(entry.Changes, &changes))
	return changes
}

func strPtr(value string) *string { return &value }

func int64Ptr(value int64) *int64 { return &value }
