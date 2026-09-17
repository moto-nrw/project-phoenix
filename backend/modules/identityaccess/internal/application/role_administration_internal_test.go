package application

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Behaviour of the role and permission administration (#3314) over in-memory
// ports. The cases carry over the retained auth service's role management
// tests and pin the rules the move must keep byte-identical: system roles
// are immutable, account mutations lock in one order, the Lehrkraft guards
// hold and every assignment change revokes the account's sessions.

const raTenant int64 = 9

var (
	errRAPolicyNotAssignable = errors.New("policy: not assignable")
	errRAPolicyForeign       = errors.New("policy: foreign tenant")
	errRAPolicyGuardian      = errors.New("policy: guardian")
)

// raStore is the in-memory role store. calls records every lock-relevant
// store call in order.
type raStore struct {
	mu           sync.Mutex
	roles        map[int64]domain.ManagedRole
	permissions  map[int64]domain.ManagedPermission
	rolePerms    map[int64][]int64
	accountRoles map[int64][]int64
	manageable   map[int64]bool
	memberships  map[int64]bool
	calls        []string
	nextID       int64

	findRoleErr          error
	listRolesErr         error
	listAccountRolesErr  error
	roleNamesErr         error
	avatarsErr           error
	emailsErr            error
	findPermissionErr    error
	deleteAssignmentsErr error
	deleteRolePermsErr   error
	deleteRoleErr        error
	deleteAccountRoleErr error
	createdAssignments   []raAssignment
	grants               []raAssignment
}

type raAssignment struct{ accountID, otherID, tenantID int64 }

func newRAStore() *raStore {
	return &raStore{
		roles: map[int64]domain.ManagedRole{}, permissions: map[int64]domain.ManagedPermission{},
		rolePerms: map[int64][]int64{}, accountRoles: map[int64][]int64{},
		manageable: map[int64]bool{}, memberships: map[int64]bool{}, nextID: 500,
	}
}

func (s *raStore) record(call string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, call)
}

func (s *raStore) addRole(role domain.ManagedRole) domain.ManagedRole {
	s.nextID++
	role.ID = s.nextID
	s.roles[role.ID] = role
	return role
}

func (s *raStore) addPermission(name string) domain.ManagedPermission {
	s.nextID++
	permission := domain.ManagedPermission{ID: s.nextID, Name: name}
	s.permissions[permission.ID] = permission
	return permission
}

func (s *raStore) CreateRole(_ context.Context, role domain.ManagedRole) (domain.ManagedRole, error) {
	return s.addRole(role), nil
}

func (s *raStore) FindRole(_ context.Context, id int64) (domain.ManagedRole, bool, error) {
	if s.findRoleErr != nil {
		return domain.ManagedRole{}, false, s.findRoleErr
	}
	role, found := s.roles[id]
	return role, found, nil
}

func (s *raStore) FindRoleIgnoringTenant(ctx context.Context, id int64) (domain.ManagedRole, bool, error) {
	s.record("find role ignoring tenant")
	return s.FindRole(ctx, id)
}

func (s *raStore) FindRoleForUpdate(ctx context.Context, id int64) (domain.ManagedRole, bool, error) {
	s.record("lock role")
	return s.FindRole(ctx, id)
}

func (s *raStore) UpdateRole(_ context.Context, role domain.ManagedRole) error {
	s.record("update role")
	s.roles[role.ID] = role
	return nil
}

func (s *raStore) DeleteRole(_ context.Context, id int64) error {
	s.record("delete role")
	if s.deleteRoleErr != nil {
		return s.deleteRoleErr
	}
	delete(s.roles, id)
	return nil
}

func (s *raStore) ListRoles(context.Context, domain.RoleFilter) ([]domain.ManagedRole, error) {
	if s.listRolesErr != nil {
		return nil, s.listRolesErr
	}
	result := make([]domain.ManagedRole, 0, len(s.roles))
	for _, role := range s.roles {
		result = append(result, role)
	}
	return result, nil
}

func (s *raStore) ListAccountRoles(_ context.Context, accountID int64) ([]domain.ManagedRole, error) {
	if s.listAccountRolesErr != nil {
		return nil, s.listAccountRolesErr
	}
	result := []domain.ManagedRole{}
	for _, id := range s.accountRoles[accountID] {
		result = append(result, s.roles[id])
	}
	return result, nil
}

func (s *raStore) ListAccountRoleNames(context.Context, []int64) (map[int64]string, error) {
	if s.roleNamesErr != nil {
		return nil, s.roleNamesErr
	}
	return map[int64]string{}, nil
}

func (s *raStore) AccountHoldsRole(_ context.Context, accountID, roleID int64) (bool, error) {
	return slices.Contains(s.accountRoles[accountID], roleID), nil
}

func (s *raStore) CreateAccountRole(_ context.Context, accountID, roleID, tenantID int64) error {
	s.record("create account role")
	s.accountRoles[accountID] = append(s.accountRoles[accountID], roleID)
	s.createdAssignments = append(s.createdAssignments, raAssignment{accountID, roleID, tenantID})
	return nil
}

func (s *raStore) DeleteAccountRole(_ context.Context, accountID, roleID int64) error {
	s.record("delete account role")
	if s.deleteAccountRoleErr != nil {
		return s.deleteAccountRoleErr
	}
	s.accountRoles[accountID] = slices.DeleteFunc(s.accountRoles[accountID], func(id int64) bool { return id == roleID })
	return nil
}

func (s *raStore) DeleteRoleAssignments(context.Context, int64) error {
	s.record("delete role assignments")
	return s.deleteAssignmentsErr
}

func (s *raStore) DeleteRolePermissions(context.Context, int64) error {
	s.record("delete role permissions")
	return s.deleteRolePermsErr
}

func (s *raStore) CreatePermission(_ context.Context, permission domain.ManagedPermission) (domain.ManagedPermission, error) {
	return permission, nil
}

func (s *raStore) FindPermission(_ context.Context, id int64) (domain.ManagedPermission, bool, error) {
	if s.findPermissionErr != nil {
		return domain.ManagedPermission{}, false, s.findPermissionErr
	}
	permission, found := s.permissions[id]
	return permission, found, nil
}

func (s *raStore) FindPermissionByName(_ context.Context, name string) (domain.ManagedPermission, error) {
	for _, permission := range s.permissions {
		if permission.Name == name {
			return permission, nil
		}
	}
	return domain.ManagedPermission{}, errors.New("no rows")
}

func (s *raStore) UpdatePermission(context.Context, domain.ManagedPermission) error { return nil }
func (s *raStore) DeletePermission(context.Context, int64) error                    { return nil }

func (s *raStore) ListPermissions(context.Context, domain.PermissionFilter) ([]domain.ManagedPermission, error) {
	return nil, nil
}

func (s *raStore) ListRolePermissions(_ context.Context, roleID int64) ([]domain.ManagedPermission, error) {
	result := []domain.ManagedPermission{}
	for _, id := range s.rolePerms[roleID] {
		result = append(result, s.permissions[id])
	}
	return result, nil
}

func (s *raStore) ListAccountPermissions(context.Context, int64) ([]domain.ManagedPermission, error) {
	return nil, nil
}

func (s *raStore) ListAccountDirectPermissions(context.Context, int64) ([]domain.ManagedPermission, error) {
	return nil, nil
}

func (s *raStore) AssignRolePermission(_ context.Context, roleID, permissionID int64) error {
	s.record("assign role permission")
	s.rolePerms[roleID] = append(s.rolePerms[roleID], permissionID)
	return nil
}

func (s *raStore) RemoveRolePermission(_ context.Context, roleID, permissionID int64) error {
	s.record("remove role permission")
	s.rolePerms[roleID] = slices.DeleteFunc(s.rolePerms[roleID], func(id int64) bool { return id == permissionID })
	return nil
}

func (s *raStore) DeletePermissionAssignments(context.Context, int64) error { return nil }
func (s *raStore) DeletePermissionGrants(context.Context, int64) error      { return nil }

func (s *raStore) GrantAccountPermission(_ context.Context, accountID, permissionID int64) error {
	s.record("grant account permission")
	s.grants = append(s.grants, raAssignment{accountID: accountID, otherID: permissionID})
	return nil
}

func (s *raStore) DenyAccountPermission(context.Context, int64, int64) error   { return nil }
func (s *raStore) RemoveAccountPermission(context.Context, int64, int64) error { return nil }

func (s *raStore) FindManageableAccount(_ context.Context, accountID int64) (bool, error) {
	s.record("find manageable account")
	return s.manageable[accountID], nil
}

func (s *raStore) LockAccount(_ context.Context, accountID int64) (bool, error) {
	s.record("lock account")
	return s.manageable[accountID], nil
}

func (s *raStore) HasTenantMembership(_ context.Context, accountID, _ int64, share bool) (bool, error) {
	if share {
		s.record("membership for share")
	} else {
		s.record("membership")
	}
	return s.memberships[accountID], nil
}

func (s *raStore) ListAccountEmails(context.Context, []int64) (map[int64]string, error) {
	if s.emailsErr != nil {
		return nil, s.emailsErr
	}
	return map[int64]string{}, nil
}

func (s *raStore) ListAccountAvatars(context.Context, []int64) (map[int64]string, error) {
	if s.avatarsErr != nil {
		return nil, s.avatarsErr
	}
	return map[int64]string{}, nil
}

// raProfiles answers the caregiver-profile fact per account.
type raProfiles struct{ live map[int64]bool }

func (p raProfiles) HasLiveCaregiverProfile(_ context.Context, accountID int64) (bool, error) {
	return p.live[accountID], nil
}

// raPolicy mirrors the public school-role policy over the domain facts.
type raPolicy struct{}

func (raPolicy) IsLehrkraftSystemRole(role *domain.RoleFacts) bool {
	return role != nil && role.IsSystem && strings.EqualFold(strings.TrimSpace(role.Name), "lehrkraft")
}

func (raPolicy) IsGuardianTierRole(role *domain.RoleFacts) bool {
	return role != nil && roleTier(role) == "guardian"
}

func (p raPolicy) ValidateAssignableSchoolRole(role *domain.RoleFacts, tenantID int64) error {
	switch {
	case role == nil:
		return errRAPolicyNotAssignable
	case role.TenantID != nil && *role.TenantID != tenantID:
		return errRAPolicyForeign
	case p.IsGuardianTierRole(role):
		return errRAPolicyGuardian
	}
	return nil
}

type raIdentity struct{ inputs []domain.SchoolIdentityInput }

func (i *raIdentity) EnsureSchoolIdentity(_ context.Context, input domain.SchoolIdentityInput) (*domain.SchoolIdentity, error) {
	i.inputs = append(i.inputs, input)
	return nil, nil
}

type raSessions struct {
	deleted   []int64
	queued    []int64
	deleteErr error
}

func (s *raSessions) DeleteAccountSessionsWithAudit(_ context.Context, accountID int64, reason, _, _ string) ([]domain.AccountSession, error) {
	if s.deleteErr != nil {
		return nil, s.deleteErr
	}
	s.deleted = append(s.deleted, accountID)
	return []domain.AccountSession{{AccountID: accountID, FamilyID: reason}}, nil
}

func (s *raSessions) QueuePushCleanup(_ context.Context, accountID int64, sessions []domain.AccountSession, _ string) {
	s.queued = append(s.queued, accountID)
	for _, session := range sessions {
		if session.AccountID != accountID {
			panic("queued a foreign session")
		}
	}
}

type raFixture struct {
	store    *raStore
	profiles raProfiles
	identity *raIdentity
	sessions *raSessions
	admin    *RoleAdministration
}

func newRAFixture(t *testing.T) *raFixture {
	t.Helper()
	f := &raFixture{
		store: newRAStore(), profiles: raProfiles{live: map[int64]bool{}},
		identity: &raIdentity{}, sessions: &raSessions{},
	}
	var err error
	f.admin, err = NewRoleAdministration(RoleAdministrationDependencies{
		Store: f.store, Profiles: f.profiles, Policy: raPolicy{}, Roles: lifecycleRoles{},
		Identity: f.identity, Sessions: f.sessions, Runtime: &fakeRuntime{},
	})
	require.NoError(t, err)
	return f
}

func raContext() context.Context {
	return (&fakeRuntime{}).WithTenantID(context.Background(), raTenant)
}

func (f *raFixture) account() int64 {
	f.store.nextID++
	f.store.manageable[f.store.nextID] = true
	return f.store.nextID
}

func customRoleOf(name, tier string) domain.ManagedRole {
	tenantID := raTenant
	return domain.ManagedRole{Name: name, TenantID: &tenantID, BaseRole: &tier}
}

func TestNewRoleAdministrationRequiresEveryPort(t *testing.T) {
	t.Parallel()

	_, err := NewRoleAdministration(RoleAdministrationDependencies{})
	require.Error(t, err)
}

func TestRoleAdministration_BatchLookupsWrapStoreFailures(t *testing.T) {
	t.Parallel()

	f := newRAFixture(t)
	f.store.roleNamesErr = errors.New("role names lookup failed")
	f.store.avatarsErr = errors.New("avatar lookup failed")
	f.store.emailsErr = errors.New("email lookup failed")

	names, err := f.admin.GetAccountRoleNames(raContext(), []int64{f.account()})
	require.ErrorIs(t, err, f.store.roleNamesErr)
	assert.Nil(t, names)
	assert.EqualError(t, err, "auth error during get account role names: role names lookup failed")

	avatars, err := f.admin.GetAccountAvatars(raContext(), []int64{f.account()})
	require.ErrorIs(t, err, f.store.avatarsErr)
	assert.Nil(t, avatars)
	assert.EqualError(t, err, "auth error during get account avatars by IDs: avatar lookup failed")

	emails, err := f.admin.GetAccountEmails(raContext(), []int64{f.account()})
	require.ErrorIs(t, err, f.store.emailsErr)
	assert.Nil(t, emails)
	assert.EqualError(t, err, "auth error during get account emails by IDs: email lookup failed")
}

func TestRoleAdministration_UpdateRoleReturnsNotFoundOnLookupFailure(t *testing.T) {
	t.Parallel()

	f := newRAFixture(t)
	f.store.findRoleErr = errors.New("sql: no rows in result set")
	err := f.admin.UpdateRole(raContext(), domain.ManagedRole{ID: f.account()})
	require.ErrorIs(t, err, domain.ErrRoleNotFound)
	assert.NotErrorIs(t, err, f.store.findRoleErr)

	missing := newRAFixture(t)
	err = missing.admin.UpdateRole(raContext(), domain.ManagedRole{ID: missing.account()})
	require.ErrorIs(t, err, domain.ErrRoleNotFound)
}

func TestRoleAdministration_SystemRolesAreImmutable(t *testing.T) {
	t.Parallel()

	f := newRAFixture(t)
	system := f.store.addRole(domain.ManagedRole{Name: "admin", IsSystem: true})
	permission := f.store.addPermission("groups:read")
	ctx := raContext()

	// The stored row decides, never the caller's flag.
	err := f.admin.UpdateRole(ctx, domain.ManagedRole{ID: system.ID, Name: "renamed", IsSystem: false})
	require.ErrorIs(t, err, domain.ErrSystemRoleImmutable)
	assert.EqualError(t, err, "auth error during update role: system roles cannot be modified")

	require.ErrorIs(t, f.admin.DeleteRole(ctx, system.ID), domain.ErrSystemRoleImmutable)
	require.ErrorIs(t, f.admin.AssignPermissionToRole(ctx, system.ID, permission.ID), domain.ErrSystemRoleImmutable)
	require.ErrorIs(t, f.admin.ReplaceRolePermissions(ctx, system.ID, []int64{permission.ID}), domain.ErrSystemRoleImmutable)
	require.ErrorIs(t, f.admin.RemovePermissionFromRole(ctx, system.ID, permission.ID), domain.ErrSystemRoleImmutable)

	assert.Equal(t, "admin", f.store.roles[system.ID].Name)
	assert.Empty(t, f.store.rolePerms[system.ID])
	assert.NotContains(t, f.store.calls, "delete role assignments")
	assert.NotContains(t, f.store.calls, "update role")
}

func TestRoleAdministration_MissingRoleIsNotFoundForMutations(t *testing.T) {
	t.Parallel()

	f := newRAFixture(t)
	ctx := raContext()
	missing := f.account()

	require.ErrorIs(t, f.admin.DeleteRole(ctx, missing), domain.ErrRoleNotFound)
	require.ErrorIs(t, f.admin.AssignPermissionToRole(ctx, missing, missing), domain.ErrRoleNotFound)
	require.ErrorIs(t, f.admin.RemovePermissionFromRole(ctx, missing, missing), domain.ErrRoleNotFound)

	// An infrastructure failure is not a missing role.
	f.store.findRoleErr = errors.New("database unavailable")
	err := f.admin.DeleteRole(ctx, missing)
	require.ErrorIs(t, err, f.store.findRoleErr)
	assert.NotErrorIs(t, err, domain.ErrRoleNotFound)
}

func TestRoleAdministration_DeleteRolePropagatesIntermediateFailures(t *testing.T) {
	t.Parallel()

	cases := map[string]func(*raStore) error{
		"account-role mappings": func(s *raStore) error {
			s.deleteAssignmentsErr = errors.New("account-role delete failed")
			return s.deleteAssignmentsErr
		},
		"role-permission mappings": func(s *raStore) error {
			s.deleteRolePermsErr = errors.New("role-permission delete failed")
			return s.deleteRolePermsErr
		},
		"the role itself": func(s *raStore) error {
			s.deleteRoleErr = errors.New("role delete failed")
			return s.deleteRoleErr
		},
	}
	for name, arrange := range cases {
		t.Run(name, func(t *testing.T) {
			f := newRAFixture(t)
			role := f.store.addRole(customRoleOf("ogs", "user"))
			expected := arrange(f.store)

			err := f.admin.DeleteRole(raContext(), role.ID)
			require.ErrorIs(t, err, expected)
		})
	}

	f := newRAFixture(t)
	role := f.store.addRole(customRoleOf("ogs", "user"))
	require.NoError(t, f.admin.DeleteRole(raContext(), role.ID))
	// The role lock comes first, so a delete cannot deadlock against a
	// concurrent permission replacement.
	assert.Equal(t, []string{"lock role", "delete role assignments", "delete role permissions", "delete role"}, f.store.calls)
}

func TestRoleAdministration_ListAndGetAccountRolesErrorPaths(t *testing.T) {
	t.Parallel()

	f := newRAFixture(t)
	f.store.listRolesErr = errors.New("list failed")
	roles, err := f.admin.ListRoles(raContext(), domain.RoleFilter{Name: "admin"})
	require.ErrorIs(t, err, f.store.listRolesErr)
	assert.Nil(t, roles)

	account := f.account()
	f.store.listAccountRolesErr = errors.New("account roles failed")
	roles, err = f.admin.GetAccountRoles(raContext(), account)
	require.ErrorIs(t, err, f.store.listAccountRolesErr)
	assert.Nil(t, roles)

	unmanaged := f.store.nextID + 1
	_, err = f.admin.GetAccountRoles(raContext(), unmanaged)
	require.ErrorIs(t, err, domain.ErrAccountNotFound)
	assert.EqualError(t, err, "auth error during get account roles: account not found")
}

func TestRoleAdministration_AccountLockOrder(t *testing.T) {
	t.Parallel()

	t.Run("tenant scope takes no membership lock", func(t *testing.T) {
		f := newRAFixture(t)
		account := f.account()
		role := f.store.addRole(customRoleOf("leitung", "admin"))

		require.NoError(t, f.admin.AssignRoleToAccount(raContext(), account, role.ID))
		assert.Equal(t, []string{
			"find manageable account", "lock account", "find manageable account",
			"find role ignoring tenant", "create account role",
		}, f.store.calls)
	})

	t.Run("organisation scope checks and shares the membership", func(t *testing.T) {
		f := newRAFixture(t)
		account := f.account()
		f.store.memberships[account] = true
		permission := f.store.addPermission("groups:read")
		ctx := withScope(raContext(), domain.ScopeOrg, 3)

		require.NoError(t, f.admin.GrantPermissionToAccount(ctx, account, permission.ID))
		assert.Equal(t, []string{
			"find manageable account", "membership", "lock account",
			"find manageable account", "membership for share", "grant account permission",
		}, f.store.calls)
	})

	t.Run("organisation scope refuses an account of another school", func(t *testing.T) {
		f := newRAFixture(t)
		account := f.account()
		permission := f.store.addPermission("groups:read")
		ctx := withScope(raContext(), domain.ScopeOrg, 3)

		err := f.admin.GrantPermissionToAccount(ctx, account, permission.ID)
		require.ErrorIs(t, err, domain.ErrAccountNotFound)
		assert.NotContains(t, f.store.calls, "lock account")
		assert.Empty(t, f.store.grants)

		_, err = f.admin.GetAccountRoles(ctx, account)
		require.ErrorIs(t, err, domain.ErrAccountNotFound)
	})

	t.Run("organisation scope without a tenant is not found", func(t *testing.T) {
		f := newRAFixture(t)
		account := f.account()
		ctx := withScope(context.Background(), domain.ScopeOrg, 3)

		_, err := f.admin.GetAccountPermissions(ctx, account)
		require.ErrorIs(t, err, domain.ErrAccountNotFound)
		assert.NotContains(t, f.store.calls, "membership")
	})

	t.Run("an unmanageable account is not found before any lock", func(t *testing.T) {
		f := newRAFixture(t)
		unmanaged := f.store.nextID + 1
		role := f.store.addRole(customRoleOf("leitung", "admin"))

		err := f.admin.AssignRoleToAccount(raContext(), unmanaged, role.ID)
		require.ErrorIs(t, err, domain.ErrAccountNotFound)
		assert.EqualError(t, err, "auth error during assign role: account not found")
		assert.Equal(t, []string{"find manageable account"}, f.store.calls)
	})
}

func TestRoleAdministration_AssignAndRemoveRoleRevokeSessions(t *testing.T) {
	t.Parallel()

	t.Run("assignment writes the tenant and revokes after commit", func(t *testing.T) {
		f := newRAFixture(t)
		account := f.account()
		role := f.store.addRole(customRoleOf("leitung", "admin"))

		require.NoError(t, f.admin.AssignRoleToAccount(raContext(), account, role.ID))
		require.Equal(t, []raAssignment{{account, role.ID, raTenant}}, f.store.createdAssignments)
		assert.Equal(t, []int64{account}, f.sessions.deleted)
		assert.Equal(t, []int64{account}, f.sessions.queued)
	})

	t.Run("repeated assignment keeps the sessions but repairs the identity", func(t *testing.T) {
		f := newRAFixture(t)
		account := f.account()
		role := f.store.addRole(customRoleOf("leitung", "admin"))
		f.store.accountRoles[account] = []int64{role.ID}

		require.NoError(t, f.admin.AssignRoleToAccount(raContext(), account, role.ID))
		assert.Empty(t, f.store.createdAssignments)
		assert.Empty(t, f.sessions.deleted)
		require.Len(t, f.identity.inputs, 1)
	})

	t.Run("a revocation failure fails the assignment", func(t *testing.T) {
		f := newRAFixture(t)
		account := f.account()
		role := f.store.addRole(customRoleOf("leitung", "admin"))
		f.sessions.deleteErr = errors.New("revocation failed")

		err := f.admin.AssignRoleToAccount(raContext(), account, role.ID)
		require.ErrorIs(t, err, f.sessions.deleteErr)
		assert.Contains(t, err.Error(), "auth error during revoke tokens after role assignment")
		assert.Empty(t, f.sessions.queued)
	})

	t.Run("a role of another school is not found", func(t *testing.T) {
		f := newRAFixture(t)
		account := f.account()
		other := raTenant + 1
		tier := "user"
		role := f.store.addRole(domain.ManagedRole{Name: "fremd", TenantID: &other, BaseRole: &tier})

		err := f.admin.AssignRoleToAccount(raContext(), account, role.ID)
		assert.EqualError(t, err, "auth error during assign role: role not found")
		assert.Empty(t, f.store.createdAssignments)
	})

	t.Run("removal failure skips the session cleanup", func(t *testing.T) {
		f := newRAFixture(t)
		account := f.account()
		role := f.store.addRole(customRoleOf("leitung", "admin"))
		f.store.accountRoles[account] = []int64{role.ID}
		f.store.deleteAccountRoleErr = errors.New("assignment delete failed")

		err := f.admin.RemoveRoleFromAccount(raContext(), account, role.ID)
		require.ErrorIs(t, err, f.store.deleteAccountRoleErr)
		assert.Empty(t, f.sessions.deleted)
		assert.Empty(t, f.sessions.queued)
	})

	t.Run("removing an unheld role is a no-op", func(t *testing.T) {
		f := newRAFixture(t)
		account := f.account()
		role := f.store.addRole(customRoleOf("leitung", "admin"))

		require.NoError(t, f.admin.RemoveRoleFromAccount(raContext(), account, role.ID))
		assert.NotContains(t, f.store.calls, "delete account role")
		assert.Empty(t, f.sessions.deleted)
	})

	t.Run("removal revokes the account's sessions", func(t *testing.T) {
		f := newRAFixture(t)
		account := f.account()
		role := f.store.addRole(customRoleOf("leitung", "admin"))
		f.store.accountRoles[account] = []int64{role.ID}

		require.NoError(t, f.admin.RemoveRoleFromAccount(raContext(), account, role.ID))
		assert.Empty(t, f.store.accountRoles[account])
		assert.Equal(t, []int64{account}, f.sessions.deleted)
		assert.Equal(t, []int64{account}, f.sessions.queued)
	})
}

// Handing a staff-tier role to an account owes the same identity an invitation
// does (#2222), but the assignment carries no identity fields and never
// invents a person.
func TestRoleAdministration_AssignRoleProvisionsSchoolIdentity(t *testing.T) {
	t.Parallel()

	f := newRAFixture(t)
	account := f.account()
	role := f.store.addRole(customRoleOf("OGS-Kraft", "user"))

	require.NoError(t, f.admin.AssignRoleToAccount(raContext(), account, role.ID))
	require.Len(t, f.identity.inputs, 1)
	input := f.identity.inputs[0]
	assert.Equal(t, account, input.AccountID)
	assert.Equal(t, raTenant, input.TenantID)
	assert.False(t, input.CreatePerson, "a role assignment may not conjure an identity")
	require.NotNil(t, input.Role)
	assert.Equal(t, role.ID, input.Role.ID)
	assert.Equal(t, "user", *input.Role.BaseRole)
}

func TestRoleAdministration_LehrkraftGuards(t *testing.T) {
	t.Parallel()

	lehrkraft := func(f *raFixture) domain.ManagedRole {
		return f.store.addRole(domain.ManagedRole{Name: "lehrkraft", IsSystem: true})
	}

	t.Run("a Lehrkraft account keeps its role", func(t *testing.T) {
		f := newRAFixture(t)
		account := f.account()
		f.store.accountRoles[account] = []int64{lehrkraft(f).ID}
		admin := f.store.addRole(customRoleOf("leitung", "admin"))

		err := f.admin.AssignRoleToAccount(raContext(), account, admin.ID)
		require.ErrorIs(t, err, domain.ErrLehrkraftRoleImmutable)
		err = f.admin.ReplaceAccountRole(raContext(), account, admin.ID)
		require.ErrorIs(t, err, domain.ErrLehrkraftRoleImmutable)
		assert.Empty(t, f.store.createdAssignments)
	})

	t.Run("Lehrkraft is refused over a live caregiver profile", func(t *testing.T) {
		f := newRAFixture(t)
		account := f.account()
		f.profiles.live[account] = true

		err := f.admin.AssignRoleToAccount(raContext(), account, lehrkraft(f).ID)
		require.ErrorIs(t, err, domain.ErrRoleLehrkraftCaregiverProfile)
		assert.Empty(t, f.store.createdAssignments)
	})

	t.Run("Lehrkraft is assigned without a caregiver profile", func(t *testing.T) {
		f := newRAFixture(t)
		account := f.account()
		role := lehrkraft(f)

		require.NoError(t, f.admin.AssignRoleToAccount(raContext(), account, role.ID))
		assert.Equal(t, []int64{role.ID}, f.store.accountRoles[account])
	})

	t.Run("a Lehrkraft account gets no caregiver role", func(t *testing.T) {
		f := newRAFixture(t)
		account := f.account()
		role := lehrkraft(f)
		f.store.accountRoles[account] = []int64{role.ID}
		caregiver := f.store.addRole(domain.ManagedRole{Name: "user", IsSystem: true})

		// Re-assigning Lehrkraft itself passes the immutability guard; any
		// other role, a caregiver role included, is refused by it.
		require.NoError(t, f.admin.AssignRoleToAccount(raContext(), account, role.ID))
		err := f.admin.AssignRoleToAccount(raContext(), account, caregiver.ID)
		require.ErrorIs(t, err, domain.ErrLehrkraftRoleImmutable)
	})

	t.Run("caregiver roles need the profile only on Lehrkraft accounts", func(t *testing.T) {
		f := newRAFixture(t)
		plain := f.account()
		caregiver := f.store.addRole(domain.ManagedRole{Name: "user", IsSystem: true})

		require.NoError(t, f.admin.AssignRoleToAccount(raContext(), plain, caregiver.ID))
	})
}

// ErrRoleCaregiverNeedsProfile is reachable only for a Lehrkraft account the
// immutability guard lets through: a Lehrkraft-classified caregiver role.
func TestRoleAdministration_CaregiverRoleNeedsProfileOnLehrkraftAccount(t *testing.T) {
	t.Parallel()

	newEnv := func(t *testing.T, liveProfile bool) (*raFixture, int64, domain.ManagedRole) {
		f := newRAFixture(t)
		account := f.account()
		lehrkraft := f.store.addRole(domain.ManagedRole{Name: "lehrkraft", IsSystem: true})
		f.store.accountRoles[account] = []int64{lehrkraft.ID}
		f.profiles.live[account] = liveProfile
		return f, account, lehrkraft
	}

	f, account, _ := newEnv(t, false)
	admin := &RoleAdministration{
		store: f.store, profiles: f.profiles, policy: raPolicy{}, roles: caregiverLehrkraftRoles{},
		identity: f.identity, sessions: f.sessions, runtime: &fakeRuntime{}, logger: f.admin.logger,
	}
	role := f.store.addRole(domain.ManagedRole{Name: "lehrkraft", IsSystem: true})
	err := admin.AssignRoleToAccount(raContext(), account, role.ID)
	require.ErrorIs(t, err, domain.ErrRoleCaregiverNeedsProfile)

	g, account, _ := newEnv(t, true)
	admin.store, admin.profiles = g.store, g.profiles
	role = g.store.addRole(domain.ManagedRole{Name: "lehrkraft", IsSystem: true})
	err = admin.AssignRoleToAccount(raContext(), account, role.ID)
	require.ErrorIs(t, err, domain.ErrRoleLehrkraftCaregiverProfile, "the Lehrkraft profile guard runs first")
}

// caregiverLehrkraftRoles classifies every role as caregiver tier so the
// reverse Lehrkraft guard can be exercised in isolation.
type caregiverLehrkraftRoles struct{ lifecycleRoles }

func (caregiverLehrkraftRoles) RoleNeedsCaregiverProfile(*domain.RoleFacts) bool { return true }

func TestRoleAdministration_ReplaceAccountRoleKeepsGuardianAndTarget(t *testing.T) {
	t.Parallel()

	f := newRAFixture(t)
	account := f.account()
	old := f.store.addRole(customRoleOf("betreuung", "user"))
	stale := f.store.addRole(customRoleOf("assistenz", "user"))
	guardian := f.store.addRole(customRoleOf("eltern", "guardian"))
	target := f.store.addRole(customRoleOf("leitung", "admin"))
	f.store.accountRoles[account] = []int64{old.ID, guardian.ID, stale.ID}

	require.NoError(t, f.admin.ReplaceAccountRole(raContext(), account, target.ID))
	held := f.store.accountRoles[account]
	assert.ElementsMatch(t, []int64{guardian.ID, target.ID}, held)
	// The target is assigned before anything is removed.
	assert.Less(t, slices.Index(f.store.calls, "create account role"), slices.Index(f.store.calls, "delete account role"))
	assert.Equal(t, "find manageable account", f.store.calls[0])
}

func TestRoleAdministration_ResolveAssignableSchoolRole(t *testing.T) {
	t.Parallel()

	f := newRAFixture(t)
	role := f.store.addRole(customRoleOf("ogs", "user"))
	permission := f.store.addPermission("groups:read")
	f.store.rolePerms[role.ID] = []int64{permission.ID}

	resolved, names, err := f.admin.ResolveAssignableSchoolRole(raContext(), role.ID, raTenant)
	require.NoError(t, err)
	assert.Equal(t, role.ID, resolved.ID)
	assert.Equal(t, []string{"groups:read"}, names)

	_, _, err = f.admin.ResolveAssignableSchoolRole(raContext(), 0, raTenant)
	require.ErrorIs(t, err, errRAPolicyNotAssignable)
	_, _, err = f.admin.ResolveAssignableSchoolRole(raContext(), permission.ID+100, raTenant)
	require.ErrorIs(t, err, errRAPolicyNotAssignable)
	_, _, err = f.admin.ResolveAssignableSchoolRole(raContext(), role.ID, raTenant+1)
	require.ErrorIs(t, err, errRAPolicyForeign)

	f.store.findRoleErr = errors.New("database unavailable")
	_, _, err = f.admin.ResolveAssignableSchoolRole(raContext(), role.ID, raTenant)
	require.ErrorIs(t, err, f.store.findRoleErr)
	assert.NotErrorIs(t, err, errRAPolicyNotAssignable)
}

func TestRoleAdministration_ReplaceRolePermissions(t *testing.T) {
	t.Parallel()

	t.Run("propagates a permission lookup failure", func(t *testing.T) {
		f := newRAFixture(t)
		role := f.store.addRole(customRoleOf("ogs", "user"))
		f.store.findPermissionErr = errors.New("permission database unavailable")

		err := f.admin.ReplaceRolePermissions(raContext(), role.ID, []int64{role.ID + 1})
		require.ErrorIs(t, err, f.store.findPermissionErr)
		assert.NotErrorIs(t, err, domain.ErrPermissionNotFound)
	})

	t.Run("an invalid request leaves the selection untouched", func(t *testing.T) {
		f := newRAFixture(t)
		role := f.store.addRole(customRoleOf("ogs", "user"))
		kept := f.store.addPermission("groups:read")
		f.store.rolePerms[role.ID] = []int64{kept.ID}

		require.ErrorIs(t, f.admin.ReplaceRolePermissions(raContext(), role.ID, []int64{kept.ID, 0}), domain.ErrPermissionNotFound)
		require.ErrorIs(t, f.admin.ReplaceRolePermissions(raContext(), role.ID, []int64{kept.ID + 100}), domain.ErrPermissionNotFound)
		assert.Equal(t, []int64{kept.ID}, f.store.rolePerms[role.ID])
	})

	t.Run("applies the complete selection after the role lock", func(t *testing.T) {
		f := newRAFixture(t)
		role := f.store.addRole(customRoleOf("ogs", "user"))
		dropped := f.store.addPermission("rooms:read")
		kept := f.store.addPermission("groups:read")
		added := f.store.addPermission("students:read")
		f.store.rolePerms[role.ID] = []int64{dropped.ID, kept.ID}

		require.NoError(t, f.admin.ReplaceRolePermissions(raContext(), role.ID, []int64{kept.ID, added.ID}))
		assert.ElementsMatch(t, []int64{kept.ID, added.ID}, f.store.rolePerms[role.ID])
		assert.Equal(t, []string{"lock role", "remove role permission", "assign role permission"}, f.store.calls)
	})

	t.Run("single mutations need an existing permission", func(t *testing.T) {
		f := newRAFixture(t)
		role := f.store.addRole(customRoleOf("ogs", "user"))
		account := f.account()

		require.ErrorIs(t, f.admin.AssignPermissionToRole(raContext(), role.ID, role.ID+100), domain.ErrPermissionNotFound)
		require.ErrorIs(t, f.admin.GrantPermissionToAccount(raContext(), account, role.ID+100), domain.ErrPermissionNotFound)
		require.ErrorIs(t, f.admin.DenyPermissionToAccount(raContext(), account, role.ID+100), domain.ErrPermissionNotFound)
	})
}

func TestRoleAdministration_CreateRoleRequiresBaseRole(t *testing.T) {
	t.Parallel()

	f := newRAFixture(t)
	_, err := f.admin.CreateRole(raContext(), "ogs", "", nil)
	assert.EqualError(t, err, "auth error during create role: base_role is required for custom roles")
	empty := ""
	_, err = f.admin.CreateRole(raContext(), "ogs", "", &empty)
	require.ErrorIs(t, err, domain.ErrBaseRoleRequired)

	tier := "user"
	role, err := f.admin.CreateRole(raContext(), "ogs", "Betreuung", &tier)
	require.NoError(t, err)
	assert.NotZero(t, role.ID)
}

func TestRoleAdministration_GrantStaffDefaultPermission(t *testing.T) {
	t.Parallel()

	t.Run("grants the permission to a staff account", func(t *testing.T) {
		f := newRAFixture(t)
		account := f.account()
		permission := f.store.addPermission("groups:read")

		f.admin.GrantStaffDefaultPermission(raContext(), account, false, "groups:read")
		assert.Equal(t, []raAssignment{{accountID: account, otherID: permission.ID}}, f.store.grants)
	})

	t.Run("skips a Lehrkraft account", func(t *testing.T) {
		f := newRAFixture(t)
		account := f.account()
		role := f.store.addRole(domain.ManagedRole{Name: "lehrkraft", IsSystem: true})
		f.store.accountRoles[account] = []int64{role.ID}
		f.store.addPermission("groups:read")

		f.admin.GrantStaffDefaultPermission(raContext(), account, true, "groups:read")
		assert.Empty(t, f.store.grants)
	})

	t.Run("fails closed when the roles cannot be read", func(t *testing.T) {
		f := newRAFixture(t)
		account := f.account()
		f.store.addPermission("groups:read")
		f.store.listAccountRolesErr = errors.New("roles unavailable")

		f.admin.GrantStaffDefaultPermission(raContext(), account, false, "groups:read")
		assert.Empty(t, f.store.grants)
	})

	t.Run("an unknown permission grants nothing", func(t *testing.T) {
		f := newRAFixture(t)
		account := f.account()

		f.admin.GrantStaffDefaultPermission(raContext(), account, false, "groups:read")
		assert.Empty(t, f.store.grants)
	})
}
