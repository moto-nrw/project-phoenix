package importpkg

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
	importModels "github.com/moto-nrw/project-phoenix/models/import"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	authsvc "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/services/auth/authtest"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	staffImportTestTenantID  = int64(42)
	staffImportTestRoleID    = int64(43)
	staffImportTestAccountID = int64(44)
	staffImportTestInviteID  = int64(45)
	staffImportTestActorID   = int64(46)
)

type stubStaffRoleRepo struct {
	roles      []*authModels.Role
	findByName func(context.Context, string) (*authModels.Role, error)
	listErr    error
}

func (r stubStaffRoleRepo) Create(context.Context, *authModels.Role) error { panic("not implemented") }
func (r stubStaffRoleRepo) FindByID(context.Context, interface{}) (*authModels.Role, error) {
	panic("not implemented")
}
func (r stubStaffRoleRepo) Update(context.Context, *authModels.Role) error { panic("not implemented") }
func (r stubStaffRoleRepo) Delete(context.Context, interface{}) error      { panic("not implemented") }
func (r stubStaffRoleRepo) List(context.Context, map[string]interface{}) ([]*authModels.Role, error) {
	return r.roles, r.listErr
}
func (r stubStaffRoleRepo) FindByName(ctx context.Context, name string) (*authModels.Role, error) {
	if r.findByName != nil {
		return r.findByName(ctx, name)
	}
	return nil, sql.ErrNoRows
}
func (r stubStaffRoleRepo) FindByAccountID(context.Context, int64) ([]*authModels.Role, error) {
	panic("not implemented")
}
func (r stubStaffRoleRepo) FindRoleNamesByAccountIDs(context.Context, []int64) (map[int64]string, error) {
	panic("not implemented")
}
func (r stubStaffRoleRepo) AssignRoleToAccount(context.Context, int64, int64) error {
	panic("not implemented")
}
func (r stubStaffRoleRepo) RemoveRoleFromAccount(context.Context, int64, int64) error {
	panic("not implemented")
}
func (r stubStaffRoleRepo) GetRoleWithPermissions(context.Context, int64) (*authModels.Role, error) {
	panic("not implemented")
}

type stubStaffAccountRepo struct {
	account *authModels.Account
	err     error
}

func (r stubStaffAccountRepo) Create(context.Context, *authModels.Account) error {
	panic("not implemented")
}
func (r stubStaffAccountRepo) FindByID(context.Context, interface{}) (*authModels.Account, error) {
	panic("not implemented")
}
func (r stubStaffAccountRepo) FindManageableByID(context.Context, int64) (*authModels.Account, error) {
	panic("not implemented")
}
func (r stubStaffAccountRepo) ListManageable(context.Context, map[string]interface{}) ([]*authModels.Account, error) {
	panic("not implemented")
}
func (r stubStaffAccountRepo) UpdateManageable(context.Context, *authModels.Account) error {
	panic("not implemented")
}
func (r stubStaffAccountRepo) FindByIDForUpdate(context.Context, int64) (*authModels.Account, error) {
	panic("not implemented")
}
func (r stubStaffAccountRepo) FindByEmail(context.Context, string) (*authModels.Account, error) {
	return r.account, r.err
}
func (r stubStaffAccountRepo) FindByUsername(context.Context, string) (*authModels.Account, error) {
	panic("not implemented")
}

func (r stubStaffAccountRepo) FindByCalendarFeedToken(context.Context, string) (*authModels.Account, error) {
	panic("not implemented")
}

func (r stubStaffAccountRepo) EnsureCalendarFeedToken(context.Context, int64, string) (string, error) {
	panic("not implemented")
}

func (r stubStaffAccountRepo) SetCalendarFeedToken(context.Context, int64, string) error {
	panic("not implemented")
}
func (r stubStaffAccountRepo) Update(context.Context, *authModels.Account) error {
	panic("not implemented")
}
func (r stubStaffAccountRepo) Delete(context.Context, interface{}) error { panic("not implemented") }
func (r stubStaffAccountRepo) List(context.Context, map[string]interface{}) ([]*authModels.Account, error) {
	panic("not implemented")
}
func (r stubStaffAccountRepo) UpdateLastLogin(context.Context, int64) error { panic("not implemented") }
func (r stubStaffAccountRepo) UpdatePassword(context.Context, int64, string) error {
	panic("not implemented")
}
func (r stubStaffAccountRepo) UpdateAvatar(context.Context, int64, string) error {
	panic("not implemented")
}
func (r stubStaffAccountRepo) SetActive(context.Context, int64, bool) error { panic("not implemented") }
func (r stubStaffAccountRepo) FindByRole(context.Context, string) ([]*authModels.Account, error) {
	panic("not implemented")
}
func (r stubStaffAccountRepo) ListEffectiveAdminAccountIDs(context.Context) ([]int64, error) {
	panic("not implemented")
}
func (r stubStaffAccountRepo) FindAccountsWithRolesAndPermissions(context.Context, map[string]interface{}) ([]*authModels.Account, error) {
	panic("not implemented")
}
func (r stubStaffAccountRepo) FindEmailsByAccountIDs(context.Context, []int64) (map[int64]string, error) {
	panic("not implemented")
}
func (r stubStaffAccountRepo) FindAvatarsByAccountIDs(context.Context, []int64) (map[int64]string, error) {
	panic("not implemented")
}
func (r stubStaffAccountRepo) IncrementMFAAttempts(context.Context, int64, int, time.Duration) (authModels.MFAAttemptResult, error) {
	panic("not implemented")
}
func (r stubStaffAccountRepo) ResetMFAAttempts(context.Context, int64) error {
	panic("not implemented")
}
func (r stubStaffAccountRepo) IncrementPINAttempts(context.Context, int64, int, time.Duration) (authModels.PINAttemptResult, error) {
	panic("not implemented")
}
func (r stubStaffAccountRepo) ResetPINAttempts(context.Context, int64) error {
	panic("not implemented")
}
func (r stubStaffAccountRepo) ClearPIN(context.Context, int64) error {
	panic("not implemented")
}

type stubStaffAccountTenantRepo struct {
	exists bool
	err    error
}

func (r stubStaffAccountTenantRepo) Create(context.Context, *authModels.AccountTenant) error {
	panic("not implemented")
}
func (r stubStaffAccountTenantRepo) EnsureActive(context.Context, *authModels.AccountTenant) error {
	panic("not implemented")
}
func (r stubStaffAccountTenantRepo) Deactivate(context.Context, int64, int64) error {
	panic("not implemented")
}
func (r stubStaffAccountTenantRepo) FindActiveByAccountID(context.Context, int64) ([]authModels.AccountTenant, error) {
	panic("not implemented")
}
func (r stubStaffAccountTenantRepo) FindActiveGuardianByAccountID(context.Context, int64) ([]authModels.AccountTenant, error) {
	panic("not implemented")
}
func (r stubStaffAccountTenantRepo) ExistsByAccountAndTenant(context.Context, int64, int64) (bool, error) {
	return r.exists, r.err
}
func (r stubStaffAccountTenantRepo) ExistsActiveByAccountAndTenantForShare(context.Context, int64, int64) (bool, error) {
	return r.exists, r.err
}
func (r stubStaffAccountTenantRepo) ListAccountsByTenantID(context.Context, int64) ([]authModels.TenantAccountInfo, error) {
	panic("not implemented")
}
func (r stubStaffAccountTenantRepo) ListAccountsByOrganizationID(context.Context, int64) ([]authModels.OrgAccountInfo, error) {
	panic("not implemented")
}
func (r stubStaffAccountTenantRepo) ListAllAccounts(context.Context) ([]authModels.OrgAccountInfo, error) {
	panic("not implemented")
}
func (r stubStaffAccountTenantRepo) ListTenantAccessByAccountID(context.Context, int64) ([]authModels.AccountTenantAccessInfo, error) {
	panic("not implemented")
}

// newStaffInvitationServiceMock wires an authtest.InvitationServiceMock to
// reproduce stubStaffInvitationService's exact behavior: CreateInvitation
// captures the request (readable via the returned *authsvc.InvitationRequest)
// and returns err when set, else a token with staffImportTestInviteID. All
// other methods keep the mock's zero-value defaults.
func newStaffInvitationServiceMock(err error) (*authtest.InvitationServiceMock, *authsvc.InvitationRequest) {
	captured := &authsvc.InvitationRequest{}
	m := &authtest.InvitationServiceMock{
		CreateInvitationFn: func(_ context.Context, req authsvc.InvitationRequest) (*authModels.InvitationToken, error) {
			*captured = req
			if err != nil {
				return nil, err
			}
			return &authModels.InvitationToken{Model: base.Model{ID: staffImportTestInviteID}}, nil
		},
	}
	return m, captured
}

func TestStaffImportConfig_PreloadReferenceData_LoadsRoleDisplayNamesAndSchool(t *testing.T) {
	t.Parallel()

	ctx := tenant.WithTenantID(context.Background(), staffImportTestTenantID)
	config := NewStaffImportConfig(StaffImportDeps{
		RoleRepo: stubStaffRoleRepo{roles: []*authModels.Role{
			{Model: base.Model{ID: staffImportTestRoleID}, Name: "user"},
			{Model: base.Model{ID: staffImportTestRoleID + 10}, Name: "koordination"},
		}},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(context.Context, int64) (*platformModels.School, error) {
				return &platformModels.School{
					Model: base.Model{ID: staffImportTestTenantID},
					Name:  "OGS Phoenix",
				}, nil
			},
		},
	})

	require.NoError(t, config.PreloadReferenceData(ctx))

	assert.Equal(t, []string{"Betreuer", "koordination"}, config.roleDisplayNames)
	assert.Equal(t, "OGS Phoenix", config.schoolName)
}

func TestStaffImportConfig_Validate_ResolvesGermanDisplayRole(t *testing.T) {
	t.Parallel()

	config := NewStaffImportConfig(StaffImportDeps{
		RoleRepo: stubStaffRoleRepo{
			findByName: func(_ context.Context, name string) (*authModels.Role, error) {
				assert.Equal(t, "user", name)
				return &authModels.Role{Model: base.Model{ID: staffImportTestRoleID}, Name: name, IsSystem: true}, nil
			},
		},
	})
	row := &importModels.StaffImportRow{
		FirstName: " Anna ",
		LastName:  " Lehmann ",
		Email:     " anna@example.com ",
		RoleName:  " Betreuer ",
		Position:  " Leitung ",
	}

	errs := config.Validate(context.Background(), row)

	require.Empty(t, errs)
	assert.Equal(t, "Anna", row.FirstName)
	assert.Equal(t, "Lehmann", row.LastName)
	assert.Equal(t, "anna@example.com", row.Email)
	assert.Equal(t, "Betreuer", row.RoleName)
	assert.Equal(t, "Leitung", row.Position)
	assert.Equal(t, staffImportTestRoleID, row.RoleID)
}

func TestStaffImportConfig_Validate_NormalizesDisplayNameEmail(t *testing.T) {
	t.Parallel()

	config := NewStaffImportConfig(StaffImportDeps{
		RoleRepo: stubStaffRoleRepo{
			findByName: func(_ context.Context, name string) (*authModels.Role, error) {
				return &authModels.Role{Model: base.Model{ID: staffImportTestRoleID}, Name: name, IsSystem: true}, nil
			},
		},
	})
	row := &importModels.StaffImportRow{
		FirstName: "Max",
		LastName:  "Mustermann",
		Email:     " Max Mustermann <Max.Mustermann@Example.COM> ",
		RoleName:  "Betreuer",
	}

	errs := config.Validate(context.Background(), row)

	require.Empty(t, errs)
	assert.Equal(t, "max.mustermann@example.com", row.Email)
}

func TestStaffImportConfig_Validate_RequiresManagePermissionForTenantRole(t *testing.T) {
	t.Parallel()

	baseRole := authModels.BaseRoleUser
	tenantID := staffImportTestTenantID
	config := NewStaffImportConfig(StaffImportDeps{
		RoleRepo: stubStaffRoleRepo{
			findByName: func(_ context.Context, name string) (*authModels.Role, error) {
				return &authModels.Role{
					Model:    base.Model{ID: staffImportTestRoleID},
					TenantID: &tenantID,
					Name:     name,
					BaseRole: &baseRole,
					Permissions: []*authModels.Permission{
						{Name: "users:manage"},
					},
				}, nil
			},
		},
	})

	row := &importModels.StaffImportRow{
		FirstName: "Max",
		LastName:  "Mustermann",
		Email:     "max@example.com",
		RoleName:  "Sekretariat",
	}

	ctx := tenant.WithTenantID(context.Background(), staffImportTestTenantID)
	errs := config.Validate(ctx, row)

	require.Len(t, errs, 1)
	assert.Equal(t, "role_grant_not_permitted", errs[0].Code)

	ctx = ContextWithImporterPermissions(ctx, []string{"users:manage"})
	assert.Empty(t, config.Validate(ctx, row))
}

func TestStaffImportConfig_Validate_RejectsRolesReservedForOtherFlows(t *testing.T) {
	t.Parallel()

	guardian := authModels.BaseRoleGuardian
	tenantID := staffImportTestTenantID
	role := &authModels.Role{
		Model:    base.Model{ID: staffImportTestRoleID},
		TenantID: &tenantID,
		Name:     "guardian-custom",
		BaseRole: &guardian,
	}
	config := NewStaffImportConfig(StaffImportDeps{
		RoleRepo: stubStaffRoleRepo{
			findByName: func(context.Context, string) (*authModels.Role, error) {
				return role, nil
			},
		},
	})
	row := &importModels.StaffImportRow{
		FirstName: "Max",
		LastName:  "Mustermann",
		Email:     "max@example.com",
		RoleName:  "guardian-custom",
	}

	errs := config.Validate(tenant.WithTenantID(context.Background(), staffImportTestTenantID), row)

	require.Len(t, errs, 1)
	assert.Equal(t, "role_not_assignable", errs[0].Code)
}

func TestStaffImportConfig_ValidateBatch_DetectsDuplicateEmailsAfterNormalization(t *testing.T) {
	t.Parallel()

	config := NewStaffImportConfig(StaffImportDeps{})
	rows := []importModels.StaffImportRow{
		{Email: "Max Mustermann <max@example.com>"},
		{Email: " max@example.COM "},
		{Email: "invalid"},
	}

	errs := config.ValidateBatch(context.Background(), rows)

	require.Len(t, errs, 1)
	require.Len(t, errs[1], 1)
	assert.Equal(t, "duplicate_in_file", errs[1][0].Code)
	assert.Equal(t, "max@example.com", errs[1][0].ActualValue)
	assert.Contains(t, errs[1][0].Message, "erste Zeile: 2")
}

func TestStaffImportConfig_Validate_ReportsRequiredInvalidEmailAndRoleSuggestion(t *testing.T) {
	t.Parallel()

	config := NewStaffImportConfig(StaffImportDeps{
		RoleRepo: stubStaffRoleRepo{
			findByName: func(context.Context, string) (*authModels.Role, error) {
				return nil, sql.ErrNoRows
			},
		},
	})
	config.roleDisplayNames = []string{"Administrator", "Betreuer", "Gast"}
	row := &importModels.StaffImportRow{
		Email:    "keine-mail",
		RoleName: "Betrueer",
	}

	errs := config.Validate(context.Background(), row)

	require.Len(t, errs, 4)
	assert.Equal(t, "first_name", errs[0].Field)
	assert.Equal(t, "last_name", errs[1].Field)
	assert.Equal(t, "email", errs[2].Field)
	assert.Equal(t, "invalid_email", errs[2].Code)
	assert.Equal(t, "role", errs[3].Field)
	assert.Equal(t, "role_not_found", errs[3].Code)
	assert.Contains(t, errs[3].Suggestions, "Betreuer")
	require.NotNil(t, errs[3].AutoFix)
	assert.Equal(t, "Betreuer", errs[3].AutoFix.Replacement)
}

func TestStaffImportConfig_Validate_RoleLookupError(t *testing.T) {
	t.Parallel()

	lookupErr := errors.New("repo unavailable")
	config := NewStaffImportConfig(StaffImportDeps{
		RoleRepo: stubStaffRoleRepo{
			findByName: func(context.Context, string) (*authModels.Role, error) {
				return nil, lookupErr
			},
		},
	})
	row := &importModels.StaffImportRow{
		FirstName: "Anna",
		LastName:  "Lehmann",
		Email:     "anna@example.com",
		RoleName:  "custom",
	}

	errs := config.Validate(context.Background(), row)

	require.Len(t, errs, 1)
	assert.Equal(t, "role_lookup_failed", errs[0].Code)
	assert.Contains(t, errs[0].Message, lookupErr.Error())
}

func TestStaffImportConfig_FindExisting(t *testing.T) {
	t.Parallel()

	t.Run("blank email skips lookup", func(t *testing.T) {
		config := NewStaffImportConfig(StaffImportDeps{})

		id, err := config.FindExisting(context.Background(), importModels.StaffImportRow{Email: "  "})

		require.NoError(t, err)
		assert.Nil(t, id)
	})

	t.Run("missing account returns nil", func(t *testing.T) {
		config := NewStaffImportConfig(StaffImportDeps{
			AccountRepo: stubStaffAccountRepo{err: sql.ErrNoRows},
		})

		id, err := config.FindExisting(context.Background(), importModels.StaffImportRow{Email: "anna@example.com"})

		require.NoError(t, err)
		assert.Nil(t, id)
	})

	t.Run("existing account in other tenant returns nil", func(t *testing.T) {
		config := NewStaffImportConfig(StaffImportDeps{
			AccountRepo: stubStaffAccountRepo{
				account: &authModels.Account{Model: base.Model{ID: staffImportTestAccountID}},
			},
			AccountTenantRepo: stubStaffAccountTenantRepo{exists: false},
		})
		ctx := tenant.WithTenantID(context.Background(), staffImportTestTenantID)

		id, err := config.FindExisting(ctx, importModels.StaffImportRow{Email: " anna@example.com "})

		require.NoError(t, err)
		assert.Nil(t, id)
	})

	t.Run("existing account in current tenant returns id", func(t *testing.T) {
		config := NewStaffImportConfig(StaffImportDeps{
			AccountRepo: stubStaffAccountRepo{
				account: &authModels.Account{Model: base.Model{ID: staffImportTestAccountID}},
			},
			AccountTenantRepo: stubStaffAccountTenantRepo{exists: true},
		})
		ctx := tenant.WithTenantID(context.Background(), staffImportTestTenantID)

		id, err := config.FindExisting(ctx, importModels.StaffImportRow{Email: " anna@example.com "})

		require.NoError(t, err)
		require.NotNil(t, id)
		assert.Equal(t, staffImportTestAccountID, *id)
	})
}

func TestStaffImportConfig_FindExisting_PersonnelNumberDoesNotFallBackToNameWithoutNumber(t *testing.T) {
	t.Parallel()

	staff := &userModels.Staff{Person: &userModels.Person{FirstName: "Anna", LastName: "Lehmann"}}
	staff.ID = 99
	config := NewStaffImportConfig(StaffImportDeps{})
	config.staffByName = map[string][]*userModels.Staff{
		staffNameKey("Anna", "Lehmann"): {staff},
	}

	id, err := config.FindExisting(context.Background(), importModels.StaffImportRow{
		FirstName: "Anna", LastName: "Lehmann", PersonnelNumber: "P-2600",
	})

	require.NoError(t, err)
	assert.Nil(t, id, "a supplied personnel number must not match a namesake without one")
}

func TestStaffImportConfig_Create_WithoutRepositoriesFails(t *testing.T) {
	t.Parallel()

	invitations, req := newStaffInvitationServiceMock(nil)
	config := NewStaffImportConfig(StaffImportDeps{InvitationService: invitations})

	_, err := config.Create(context.Background(), importModels.StaffImportRow{FirstName: "Anna", LastName: "Lehmann", Email: "anna@example.com"})

	require.Error(t, err)
	assert.Empty(t, req.Email, "no invitation may go out when the Stammdatensatz cannot be written")
}

func TestStaffImportConfig_PreloadReferenceData_ReturnsRoleListError(t *testing.T) {
	t.Parallel()

	config := NewStaffImportConfig(StaffImportDeps{
		RoleRepo: stubStaffRoleRepo{listErr: errors.New("list failed")},
	})

	err := config.PreloadReferenceData(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "preload roles")
}

func TestStaffImportConfig_InvitationServiceCompileGuard(t *testing.T) {
	t.Parallel()

	var _ authsvc.InvitationService = (*authtest.InvitationServiceMock)(nil)
	var _ authModels.RoleRepository = stubStaffRoleRepo{}
	var _ authModels.AccountRepository = stubStaffAccountRepo{}
	var _ authModels.AccountTenantRepository = stubStaffAccountTenantRepo{}
	var _ platformModels.SchoolRepository = (*testpkg.SchoolRepoMock)(nil)
}

// Stub for the issue #585 refactor interface addition — unused here.
func (r stubStaffAccountRepo) AnonymizeForDeletion(context.Context, int64, string) error {
	return nil
}

// TestStaffImportConfig_AuthorizeImportMode pins the permission boundary of
// the update-capable import modes (#2906): users:create alone files new
// records, changing existing ones needs both personnel permissions.
func TestStaffImportConfig_AuthorizeImportMode(t *testing.T) {
	t.Parallel()

	config := NewStaffImportConfig(StaffImportDeps{})
	cases := []struct {
		name        string
		mode        importModels.ImportMode
		permissions []string
		forbidden   bool
	}{
		{name: "create needs no personnel permission", mode: importModels.ImportModeCreate, permissions: []string{"users:create"}},
		{name: "update refused for users:create alone", mode: importModels.ImportModeUpdate, permissions: []string{"users:create", "users:update"}, forbidden: true},
		{name: "upsert refused for users:create alone", mode: importModels.ImportModeUpsert, permissions: []string{"users:create"}, forbidden: true},
		{name: "update refused with staff:manage only", mode: importModels.ImportModeUpdate, permissions: []string{"users:create", "staff:manage"}, forbidden: true},
		{name: "update refused with staff:stammdaten only", mode: importModels.ImportModeUpdate, permissions: []string{"users:create", "staff:stammdaten"}, forbidden: true},
		{name: "update allowed with both personnel permissions", mode: importModels.ImportModeUpdate, permissions: []string{"users:create", "staff:manage", "staff:stammdaten"}},
		{name: "upsert allowed for admin wildcard", mode: importModels.ImportModeUpsert, permissions: []string{"admin:*"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := ContextWithImporterPermissions(context.Background(), tc.permissions)
			err := config.AuthorizeImportMode(ctx, tc.mode)
			if tc.forbidden {
				require.ErrorIs(t, err, ErrImportModeForbidden)
				return
			}
			require.NoError(t, err)
		})
	}
}
