package auth

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	authModel "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
	userModel "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roleManagementRoleRepo struct {
	noopRoleRepository

	findByIDFn           func(context.Context, interface{}) (*authModel.Role, error)
	updateFn             func(context.Context, *authModel.Role) error
	deleteFn             func(context.Context, interface{}) error
	listFn               func(context.Context, map[string]interface{}) ([]*authModel.Role, error)
	findByAccountIDFn    func(context.Context, int64) ([]*authModel.Role, error)
	findRoleNamesByIDsFn func(context.Context, []int64) (map[int64]string, error)
}

func (r roleManagementRoleRepo) FindByID(ctx context.Context, id interface{}) (*authModel.Role, error) {
	if r.findByIDFn != nil {
		return r.findByIDFn(ctx, id)
	}
	return nil, sql.ErrNoRows
}

func (r roleManagementRoleRepo) Update(ctx context.Context, role *authModel.Role) error {
	if r.updateFn != nil {
		return r.updateFn(ctx, role)
	}
	return nil
}

func (r roleManagementRoleRepo) Delete(ctx context.Context, id interface{}) error {
	if r.deleteFn != nil {
		return r.deleteFn(ctx, id)
	}
	return nil
}

func (r roleManagementRoleRepo) List(ctx context.Context, filters map[string]interface{}) ([]*authModel.Role, error) {
	if r.listFn != nil {
		return r.listFn(ctx, filters)
	}
	return nil, nil
}

func (r roleManagementRoleRepo) FindByAccountID(ctx context.Context, accountID int64) ([]*authModel.Role, error) {
	if r.findByAccountIDFn != nil {
		return r.findByAccountIDFn(ctx, accountID)
	}
	return nil, nil
}

func (r roleManagementRoleRepo) FindRoleNamesByAccountIDs(ctx context.Context, accountIDs []int64) (map[int64]string, error) {
	if r.findRoleNamesByIDsFn != nil {
		return r.findRoleNamesByIDsFn(ctx, accountIDs)
	}
	return map[int64]string{}, nil
}

type roleManagementAccountRepo struct {
	noopAccountRepository

	findByIDFn    func(context.Context, interface{}) (*authModel.Account, error)
	findEmailsFn  func(context.Context, []int64) (map[int64]string, error)
	findAvatarsFn func(context.Context, []int64) (map[int64]string, error)
}

func (r roleManagementAccountRepo) FindByID(ctx context.Context, id interface{}) (*authModel.Account, error) {
	if r.findByIDFn != nil {
		return r.findByIDFn(ctx, id)
	}
	return nil, sql.ErrNoRows
}

func (r roleManagementAccountRepo) FindByIDForUpdate(ctx context.Context, id int64) (*authModel.Account, error) {
	return r.FindByID(ctx, id)
}

func (r roleManagementAccountRepo) FindEmailsByAccountIDs(ctx context.Context, accountIDs []int64) (map[int64]string, error) {
	if r.findEmailsFn != nil {
		return r.findEmailsFn(ctx, accountIDs)
	}
	return map[int64]string{}, nil
}

func (r roleManagementAccountRepo) FindAvatarsByAccountIDs(ctx context.Context, accountIDs []int64) (map[int64]string, error) {
	if r.findAvatarsFn != nil {
		return r.findAvatarsFn(ctx, accountIDs)
	}
	return map[int64]string{}, nil
}

type roleManagementAccountRoleRepo struct {
	noopAccountRoleRepository

	createFn                 func(context.Context, *authModel.AccountRole) error
	findByAccountAndRoleFn   func(context.Context, int64, int64) (*authModel.AccountRole, error)
	deleteByAccountAndRoleFn func(context.Context, int64, int64) error
	deleteByRoleIDFn         func(context.Context, int64) error
}

func (r roleManagementAccountRoleRepo) Create(ctx context.Context, accountRole *authModel.AccountRole) error {
	if r.createFn != nil {
		return r.createFn(ctx, accountRole)
	}
	return nil
}

func (r roleManagementAccountRoleRepo) FindByAccountAndRole(ctx context.Context, accountID, roleID int64) (*authModel.AccountRole, error) {
	if r.findByAccountAndRoleFn != nil {
		return r.findByAccountAndRoleFn(ctx, accountID, roleID)
	}
	return nil, sql.ErrNoRows
}

func (r roleManagementAccountRoleRepo) DeleteByAccountRoleAndTenant(context.Context, int64, int64, int64) error {
	return nil
}

func (r roleManagementAccountRoleRepo) DeleteByAccountAndRole(ctx context.Context, accountID, roleID int64) error {
	if r.deleteByAccountAndRoleFn != nil {
		return r.deleteByAccountAndRoleFn(ctx, accountID, roleID)
	}
	return nil
}

func (r roleManagementAccountRoleRepo) DeleteByRoleID(ctx context.Context, roleID int64) error {
	if r.deleteByRoleIDFn != nil {
		return r.deleteByRoleIDFn(ctx, roleID)
	}
	return nil
}

type roleManagementRolePermissionRepo struct {
	deleteByRoleIDFn func(context.Context, int64) error
}

func (r roleManagementRolePermissionRepo) Create(context.Context, *authModel.RolePermission) error {
	panic("Create not implemented")
}

func (r roleManagementRolePermissionRepo) FindByID(context.Context, interface{}) (*authModel.RolePermission, error) {
	panic("FindByID not implemented")
}

func (r roleManagementRolePermissionRepo) Update(context.Context, *authModel.RolePermission) error {
	panic("Update not implemented")
}

func (r roleManagementRolePermissionRepo) Delete(context.Context, interface{}) error {
	panic("Delete not implemented")
}

func (r roleManagementRolePermissionRepo) List(context.Context, map[string]interface{}) ([]*authModel.RolePermission, error) {
	panic("List not implemented")
}

func (r roleManagementRolePermissionRepo) FindByRoleID(context.Context, int64) ([]*authModel.RolePermission, error) {
	panic("FindByRoleID not implemented")
}

func (r roleManagementRolePermissionRepo) FindByPermissionID(context.Context, int64) ([]*authModel.RolePermission, error) {
	panic("FindByPermissionID not implemented")
}

func (r roleManagementRolePermissionRepo) FindByRoleAndPermission(context.Context, int64, int64) (*authModel.RolePermission, error) {
	panic("FindByRoleAndPermission not implemented")
}

func (r roleManagementRolePermissionRepo) DeleteByRoleAndPermission(context.Context, int64, int64) error {
	panic("DeleteByRoleAndPermission not implemented")
}

func (r roleManagementRolePermissionRepo) DeleteByRoleID(ctx context.Context, roleID int64) error {
	if r.deleteByRoleIDFn != nil {
		return r.deleteByRoleIDFn(ctx, roleID)
	}
	return nil
}

func (r roleManagementRolePermissionRepo) DeleteByPermissionID(context.Context, int64) error {
	panic("DeleteByPermissionID not implemented")
}

func (r roleManagementRolePermissionRepo) FindRolePermissionsWithDetails(context.Context, map[string]interface{}) ([]*authModel.RolePermission, error) {
	panic("FindRolePermissionsWithDetails not implemented")
}

type roleManagementTokenRepo struct {
	noopTokenRepository

	deleteByAccountIDFn func(context.Context, int64) error
}

func (r roleManagementTokenRepo) DeleteByAccountIDReturning(ctx context.Context, accountID int64) ([]*authModel.Token, error) {
	if r.deleteByAccountIDFn != nil {
		return nil, r.deleteByAccountIDFn(ctx, accountID)
	}
	return nil, nil
}

func newRoleManagementService(
	roleRepo authModel.RoleRepository,
	accountRepo authModel.AccountRepository,
	accountRoleRepo authModel.AccountRoleRepository,
	rolePermissionRepo authModel.RolePermissionRepository,
	tokenRepo authModel.TokenRepository,
) *Service {
	svc, _, _, _ := newRoleManagementServiceWithIdentity(
		roleRepo, accountRepo, accountRoleRepo, rolePermissionRepo, tokenRepo)
	return svc
}

// newRoleManagementServiceWithIdentity additionally exposes the identity
// repositories. Assigning a staff-tier role provisions the school identity in
// the same transaction (#2222), so these are reached on every assign; with no
// person for the account the provisioning is a no-op, which is what keeps the
// cases above about role assignment alone.
func newRoleManagementServiceWithIdentity(
	roleRepo authModel.RoleRepository,
	accountRepo authModel.AccountRepository,
	accountRoleRepo authModel.AccountRoleRepository,
	rolePermissionRepo authModel.RolePermissionRepository,
	tokenRepo authModel.TokenRepository,
) (*Service, *stubPersonRepository, func() []*userModel.Staff, *stubTeacherRepository) {
	persons := newStubPersonRepository()
	staff, staffAll := newStubStaffRepository()
	teachers := newStubTeacherRepository()

	return &Service{
		repos: &repositories.Factory{
			Role:           roleRepo,
			Account:        accountRepo,
			AccountRole:    accountRoleRepo,
			RolePermission: rolePermissionRepo,
			Token:          tokenRepo,
			Person:         persons,
			Staff:          staff,
			Teacher:        teachers,
			Student:        newStubStudentRepository(),
		},
		logger: slog.Default(),
	}, persons, staffAll, teachers
}

func TestRoleManagement_GetAccountRoleNames(t *testing.T) {
	t.Parallel()

	t.Run("returns mapped role names", func(t *testing.T) {
		svc := newRoleManagementService(
			roleManagementRoleRepo{
				findRoleNamesByIDsFn: func(_ context.Context, accountIDs []int64) (map[int64]string, error) {
					assert.Equal(t, []int64{11, 22}, accountIDs)
					return map[int64]string{11: "admin", 22: "user"}, nil
				},
			},
			roleManagementAccountRepo{},
			roleManagementAccountRoleRepo{},
			roleManagementRolePermissionRepo{},
			roleManagementTokenRepo{},
		)

		roleNames, err := svc.GetAccountRoleNames(context.Background(), []int64{11, 22})
		require.NoError(t, err)
		assert.Equal(t, map[int64]string{11: "admin", 22: "user"}, roleNames)
	})

	t.Run("wraps repository failures", func(t *testing.T) {
		expectedErr := errors.New("role names lookup failed")
		svc := newRoleManagementService(
			roleManagementRoleRepo{
				findRoleNamesByIDsFn: func(context.Context, []int64) (map[int64]string, error) {
					return nil, expectedErr
				},
			},
			roleManagementAccountRepo{},
			roleManagementAccountRoleRepo{},
			roleManagementRolePermissionRepo{},
			roleManagementTokenRepo{},
		)

		roleNames, err := svc.GetAccountRoleNames(context.Background(), []int64{11})
		require.Error(t, err)
		assert.Nil(t, roleNames)
		assert.ErrorIs(t, err, expectedErr)
	})
}

func TestRoleManagement_GetAccountAvatarsByIDs(t *testing.T) {
	t.Parallel()

	t.Run("returns mapped avatar paths", func(t *testing.T) {
		svc := newRoleManagementService(
			roleManagementRoleRepo{},
			roleManagementAccountRepo{
				findAvatarsFn: func(_ context.Context, accountIDs []int64) (map[int64]string, error) {
					assert.Equal(t, []int64{31, 44}, accountIDs)
					return map[int64]string{31: "/avatars/31.png", 44: "/avatars/44.png"}, nil
				},
			},
			roleManagementAccountRoleRepo{},
			roleManagementRolePermissionRepo{},
			roleManagementTokenRepo{},
		)

		avatars, err := svc.GetAccountAvatarsByIDs(context.Background(), []int64{31, 44})
		require.NoError(t, err)
		assert.Equal(t, map[int64]string{31: "/avatars/31.png", 44: "/avatars/44.png"}, avatars)
	})

	t.Run("wraps repository failures", func(t *testing.T) {
		expectedErr := errors.New("avatar lookup failed")
		svc := newRoleManagementService(
			roleManagementRoleRepo{},
			roleManagementAccountRepo{
				findAvatarsFn: func(context.Context, []int64) (map[int64]string, error) {
					return nil, expectedErr
				},
			},
			roleManagementAccountRoleRepo{},
			roleManagementRolePermissionRepo{},
			roleManagementTokenRepo{},
		)

		avatars, err := svc.GetAccountAvatarsByIDs(context.Background(), []int64{99})
		require.Error(t, err)
		assert.Nil(t, avatars)
		assert.ErrorIs(t, err, expectedErr)
	})
}

func TestRoleManagement_UpdateRole_ReturnsNotFoundOnLookupFailure(t *testing.T) {
	t.Parallel()

	svc := newRoleManagementService(
		roleManagementRoleRepo{
			findByIDFn: func(context.Context, interface{}) (*authModel.Role, error) {
				return nil, sql.ErrNoRows
			},
		},
		roleManagementAccountRepo{},
		roleManagementAccountRoleRepo{},
		roleManagementRolePermissionRepo{},
		roleManagementTokenRepo{},
	)

	err := svc.UpdateRole(context.Background(), &authModel.Role{Model: base.Model{ID: 42}})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrRoleNotFound)
}

func TestRoleManagement_DeleteRole_PropagatesIntermediateFailures(t *testing.T) {
	t.Parallel()

	baseRoleRepo := roleManagementRoleRepo{
		findByIDFn: func(context.Context, interface{}) (*authModel.Role, error) {
			return &authModel.Role{Model: base.Model{ID: 77}}, nil
		},
		deleteFn: func(context.Context, interface{}) error {
			return nil
		},
	}

	t.Run("fails when deleting account-role mappings", func(t *testing.T) {
		expectedErr := errors.New("account-role delete failed")
		svc := newRoleManagementService(
			baseRoleRepo,
			roleManagementAccountRepo{},
			roleManagementAccountRoleRepo{
				deleteByRoleIDFn: func(context.Context, int64) error {
					return expectedErr
				},
			},
			roleManagementRolePermissionRepo{},
			roleManagementTokenRepo{},
		)

		err := svc.DeleteRole(context.Background(), 77)
		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("fails when deleting role-permission mappings", func(t *testing.T) {
		expectedErr := errors.New("role-permission delete failed")
		svc := newRoleManagementService(
			baseRoleRepo,
			roleManagementAccountRepo{},
			roleManagementAccountRoleRepo{},
			roleManagementRolePermissionRepo{
				deleteByRoleIDFn: func(context.Context, int64) error {
					return expectedErr
				},
			},
			roleManagementTokenRepo{},
		)

		err := svc.DeleteRole(context.Background(), 77)
		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("fails when deleting the role itself", func(t *testing.T) {
		expectedErr := errors.New("role delete failed")
		svc := newRoleManagementService(
			roleManagementRoleRepo{
				findByIDFn: func(context.Context, interface{}) (*authModel.Role, error) {
					return &authModel.Role{Model: base.Model{ID: 77}}, nil
				},
				deleteFn: func(context.Context, interface{}) error {
					return expectedErr
				},
			},
			roleManagementAccountRepo{},
			roleManagementAccountRoleRepo{},
			roleManagementRolePermissionRepo{},
			roleManagementTokenRepo{},
		)

		err := svc.DeleteRole(context.Background(), 77)
		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
	})
}

func TestRoleManagement_ListAndGetAccountRoles_ErrorPaths(t *testing.T) {
	t.Parallel()

	t.Run("ListRoles wraps repository failures", func(t *testing.T) {
		expectedErr := errors.New("list failed")
		svc := newRoleManagementService(
			roleManagementRoleRepo{
				listFn: func(context.Context, map[string]interface{}) ([]*authModel.Role, error) {
					return nil, expectedErr
				},
			},
			roleManagementAccountRepo{},
			roleManagementAccountRoleRepo{},
			roleManagementRolePermissionRepo{},
			roleManagementTokenRepo{},
		)

		roles, err := svc.ListRoles(context.Background(), map[string]interface{}{"active": true})
		require.Error(t, err)
		assert.Nil(t, roles)
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("GetAccountRoles wraps repository failures", func(t *testing.T) {
		expectedErr := errors.New("account roles failed")
		svc := newRoleManagementService(
			roleManagementRoleRepo{
				findByAccountIDFn: func(context.Context, int64) ([]*authModel.Role, error) {
					return nil, expectedErr
				},
			},
			roleManagementAccountRepo{},
			roleManagementAccountRoleRepo{},
			roleManagementRolePermissionRepo{},
			roleManagementTokenRepo{},
		)

		roles, err := svc.GetAccountRoles(context.Background(), 51)
		require.Error(t, err)
		assert.Nil(t, roles)
		assert.ErrorIs(t, err, expectedErr)
	})
}

func TestRoleManagement_AssignAndRemoveRole_RevokeTokens(t *testing.T) {
	t.Parallel()

	ctx := tenant.WithTenantID(context.Background(), 9)
	account := &authModel.Account{Model: base.Model{ID: 12}}
	role := &authModel.Role{Model: base.Model{ID: 34}}

	t.Run("AssignRoleToAccount revokes account tokens after creating assignment", func(t *testing.T) {
		var createdAssignment *authModel.AccountRole
		var revokedAccountID int64
		svc := newRoleManagementService(
			roleManagementRoleRepo{
				findByIDFn: func(_ context.Context, id interface{}) (*authModel.Role, error) {
					assert.Equal(t, int64(34), id)
					return role, nil
				},
			},
			roleManagementAccountRepo{
				findByIDFn: func(_ context.Context, id interface{}) (*authModel.Account, error) {
					assert.Equal(t, int64(12), id)
					return account, nil
				},
			},
			roleManagementAccountRoleRepo{
				findByAccountAndRoleFn: func(context.Context, int64, int64) (*authModel.AccountRole, error) {
					return nil, sql.ErrNoRows
				},
				createFn: func(_ context.Context, assignment *authModel.AccountRole) error {
					createdAssignment = assignment
					return nil
				},
			},
			roleManagementRolePermissionRepo{},
			roleManagementTokenRepo{
				deleteByAccountIDFn: func(_ context.Context, accountID int64) error {
					revokedAccountID = accountID
					return nil
				},
			},
		)

		err := svc.AssignRoleToAccount(ctx, 12, 34)
		require.NoError(t, err)
		require.NotNil(t, createdAssignment)
		assert.Equal(t, int64(9), createdAssignment.TenantID)
		assert.Equal(t, int64(12), revokedAccountID)
	})

	t.Run("RemoveRoleFromAccount returns delete failure without token cleanup", func(t *testing.T) {
		tokenCleanupCalled := false
		expectedErr := errors.New("assignment delete failed")
		svc := newRoleManagementService(
			roleManagementRoleRepo{},
			roleManagementAccountRepo{},
			roleManagementAccountRoleRepo{
				findByAccountAndRoleFn: func(_ context.Context, accountID, roleID int64) (*authModel.AccountRole, error) {
					assert.Equal(t, int64(12), accountID)
					assert.Equal(t, int64(34), roleID)
					return &authModel.AccountRole{AccountID: accountID, RoleID: roleID}, nil
				},
				deleteByAccountAndRoleFn: func(_ context.Context, accountID, roleID int64) error {
					assert.Equal(t, int64(12), accountID)
					assert.Equal(t, int64(34), roleID)
					return expectedErr
				},
			},
			roleManagementRolePermissionRepo{},
			roleManagementTokenRepo{
				deleteByAccountIDFn: func(_ context.Context, accountID int64) error {
					tokenCleanupCalled = true
					return nil
				},
			},
		)

		err := svc.RemoveRoleFromAccount(context.Background(), 12, 34)
		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
		assert.False(t, tokenCleanupCalled)
	})

	t.Run("RemoveRoleFromAccount no-op does not revoke account tokens", func(t *testing.T) {
		tokenCleanupCalled := false
		deleteCalled := false
		svc := newRoleManagementService(
			roleManagementRoleRepo{},
			roleManagementAccountRepo{},
			roleManagementAccountRoleRepo{
				findByAccountAndRoleFn: func(_ context.Context, accountID, roleID int64) (*authModel.AccountRole, error) {
					assert.Equal(t, int64(12), accountID)
					assert.Equal(t, int64(34), roleID)
					return nil, sql.ErrNoRows
				},
				deleteByAccountAndRoleFn: func(context.Context, int64, int64) error {
					deleteCalled = true
					return nil
				},
			},
			roleManagementRolePermissionRepo{},
			roleManagementTokenRepo{
				deleteByAccountIDFn: func(context.Context, int64) error {
					tokenCleanupCalled = true
					return nil
				},
			},
		)

		err := svc.RemoveRoleFromAccount(context.Background(), 12, 34)
		require.NoError(t, err)
		assert.False(t, deleteCalled)
		assert.False(t, tokenCleanupCalled)
	})

	t.Run("RemoveRoleFromAccount revokes account tokens after deleting assignment", func(t *testing.T) {
		var revokedAccountID int64
		svc := newRoleManagementService(
			roleManagementRoleRepo{},
			roleManagementAccountRepo{},
			roleManagementAccountRoleRepo{
				findByAccountAndRoleFn: func(_ context.Context, accountID, roleID int64) (*authModel.AccountRole, error) {
					assert.Equal(t, int64(12), accountID)
					assert.Equal(t, int64(34), roleID)
					return &authModel.AccountRole{AccountID: accountID, RoleID: roleID}, nil
				},
			},
			roleManagementRolePermissionRepo{},
			roleManagementTokenRepo{
				deleteByAccountIDFn: func(_ context.Context, accountID int64) error {
					revokedAccountID = accountID
					return nil
				},
			},
		)

		err := svc.RemoveRoleFromAccount(context.Background(), 12, 34)
		require.NoError(t, err)
		assert.Equal(t, int64(12), revokedAccountID)
	})
}

// Handing a staff-tier role to an account is the same act as inviting one, and
// owes the same identity (#2222). Without this the tenant RBAC endpoint is a
// further way to produce the broken state: an account that holds the role, logs
// in, and is not staff as far as the database is concerned.
func TestRoleManagement_AssignRoleProvisionsSchoolIdentity(t *testing.T) {
	t.Parallel()

	ctx := tenant.WithTenantID(context.Background(), 9)
	account := &authModel.Account{Model: base.Model{ID: 12}}
	tenantID := int64(9)

	newEnv := func(role *authModel.Role) (*Service, *stubPersonRepository, func() []*userModel.Staff, *stubTeacherRepository) {
		return newRoleManagementServiceWithIdentity(
			roleManagementRoleRepo{
				findByIDFn: func(context.Context, interface{}) (*authModel.Role, error) {
					return role, nil
				},
			},
			roleManagementAccountRepo{
				findByIDFn: func(context.Context, interface{}) (*authModel.Account, error) {
					return account, nil
				},
			},
			roleManagementAccountRoleRepo{
				findByAccountAndRoleFn: func(context.Context, int64, int64) (*authModel.AccountRole, error) {
					return nil, sql.ErrNoRows
				},
				createFn: func(context.Context, *authModel.AccountRole) error { return nil },
			},
			roleManagementRolePermissionRepo{},
			roleManagementTokenRepo{},
		)
	}

	seedPerson := func(t *testing.T, persons *stubPersonRepository) *userModel.Person {
		t.Helper()
		accountID := account.ID
		person := &userModel.Person{FirstName: "Vorhandene", LastName: "Person", AccountID: &accountID}
		person.SetTenantID(tenantID)
		require.NoError(t, persons.Create(ctx, person))
		return person
	}

	t.Run("custom caregiver-tier role gets staff record and caregiver profile", func(t *testing.T) {
		tier := authModel.BaseRoleUser
		role := &authModel.Role{Model: base.Model{ID: 34}, Name: "OGS-Kraft", BaseRole: &tier, TenantID: &tenantID}
		svc, persons, staffAll, teachers := newEnv(role)
		person := seedPerson(t, persons)

		require.NoError(t, svc.AssignRoleToAccount(ctx, 12, 34))

		require.Len(t, staffAll(), 1, "a staff-tier role owes a staff record")
		assert.Equal(t, person.ID, staffAll()[0].PersonID)
		require.Len(t, teachers.All(), 1, "caregiver tier reads through users.teachers")
	})

	t.Run("custom admin-tier role gets a staff record without a caregiver profile", func(t *testing.T) {
		tier := authModel.BaseRoleAdmin
		role := &authModel.Role{Model: base.Model{ID: 34}, Name: "OGS-Leitung", BaseRole: &tier, TenantID: &tenantID}
		svc, persons, staffAll, teachers := newEnv(role)
		seedPerson(t, persons)

		require.NoError(t, svc.AssignRoleToAccount(ctx, 12, 34))

		require.Len(t, staffAll(), 1)
		require.Empty(t, teachers.All())
	})

	t.Run("guardian-tier role provisions nothing", func(t *testing.T) {
		tier := authModel.BaseRoleGuardian
		role := &authModel.Role{Model: base.Model{ID: 34}, Name: "Sorgeberechtigt", BaseRole: &tier, TenantID: &tenantID}
		svc, persons, staffAll, teachers := newEnv(role)
		seedPerson(t, persons)

		require.NoError(t, svc.AssignRoleToAccount(ctx, 12, 34))

		require.Empty(t, staffAll(), "guardians are deliberately not personnel")
		require.Empty(t, teachers.All())
	})

	// The endpoint carries no identity fields, so it must not invent a person —
	// the same line the operator role change draws. The assignment still stands.
	t.Run("account without a person at this school is left alone", func(t *testing.T) {
		tier := authModel.BaseRoleAdmin
		role := &authModel.Role{Model: base.Model{ID: 34}, Name: "OGS-Leitung", BaseRole: &tier, TenantID: &tenantID}
		svc, persons, staffAll, _ := newEnv(role)

		require.NoError(t, svc.AssignRoleToAccount(ctx, 12, 34))

		require.Empty(t, persons.people, "a role assignment may not conjure an identity")
		require.Empty(t, staffAll())
	})
}
