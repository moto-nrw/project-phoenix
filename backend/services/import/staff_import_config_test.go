package importpkg

import (
	"context"
	"errors"
	"testing"

	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	staffImportTestTenantID  = int64(42)
	staffImportTestRoleID    = int64(43)
	staffImportTestAccountID = int64(44)
	staffImportTestActorID   = int64(46)
)

type stubStaffRoleRepo struct {
	roles      []*importModels.SchoolRole
	findByName func(context.Context, string) (*importModels.SchoolRole, error)
	listErr    error
}

func (r stubStaffRoleRepo) ListSchoolRoles(context.Context) ([]*importModels.SchoolRole, error) {
	return r.roles, r.listErr
}
func (r stubStaffRoleRepo) FindSchoolRoleByName(ctx context.Context, name string) (*importModels.SchoolRole, error) {
	if r.findByName != nil {
		return r.findByName(ctx, name)
	}
	return nil, importModels.ErrRoleNotFound
}

type staffInvitationMock struct {
	InviteStaffFn func(context.Context, importModels.StaffInvitation) error
}

func (m *staffInvitationMock) InviteStaff(ctx context.Context, invitation importModels.StaffInvitation) error {
	return m.InviteStaffFn(ctx, invitation)
}

// newStaffInvitationServiceMock records the command payload and can fail it.
func newStaffInvitationServiceMock(err error) (*staffInvitationMock, *importModels.StaffInvitation) {
	captured := &importModels.StaffInvitation{}
	return &staffInvitationMock{InviteStaffFn: func(_ context.Context, req importModels.StaffInvitation) error {
		*captured = req
		return err
	}}, captured
}

func TestStaffImportConfig_PreloadReferenceData_LoadsRoleDisplayNamesAndSchool(t *testing.T) {
	t.Parallel()

	ctx := testpkg.ContextForTenant(context.Background(), staffImportTestTenantID)
	config := NewStaffImportConfig(StaffImportDeps{RolePolicy: newTestRolePolicy(), Authorization: newTestAuthorization(), Transactions: newTestTransactions(),
		Roles: stubStaffRoleRepo{roles: []*importModels.SchoolRole{
			{ID: staffImportTestRoleID, Name: "user"},
			{ID: staffImportTestRoleID + 10, Name: "koordination"},
		}},
		SchoolName: func(context.Context) (string, error) { return "OGS Phoenix", nil },
	})

	require.NoError(t, config.PreloadReferenceData(ctx))

	assert.Equal(t, []string{"Betreuer", "koordination"}, config.roleDisplayNames)
	assert.Equal(t, "OGS Phoenix", config.schoolName)
}

func TestStaffImportConfig_Validate_ResolvesGermanDisplayRole(t *testing.T) {
	t.Parallel()

	config := NewStaffImportConfig(StaffImportDeps{RolePolicy: newTestRolePolicy(), Authorization: newTestAuthorization(), Transactions: newTestTransactions(),
		Roles: stubStaffRoleRepo{
			findByName: func(_ context.Context, name string) (*importModels.SchoolRole, error) {
				assert.Equal(t, "user", name)
				return &importModels.SchoolRole{ID: staffImportTestRoleID, Name: name, IsSystem: true}, nil
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

	config := NewStaffImportConfig(StaffImportDeps{RolePolicy: newTestRolePolicy(), Authorization: newTestAuthorization(), Transactions: newTestTransactions(),
		Roles: stubStaffRoleRepo{
			findByName: func(_ context.Context, name string) (*importModels.SchoolRole, error) {
				return &importModels.SchoolRole{ID: staffImportTestRoleID, Name: name, IsSystem: true}, nil
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

	baseRole := "user"
	tenantID := staffImportTestTenantID
	config := NewStaffImportConfig(StaffImportDeps{RolePolicy: newTestRolePolicy(), Authorization: newTestAuthorization(), Transactions: newTestTransactions(),
		Roles: stubStaffRoleRepo{
			findByName: func(_ context.Context, name string) (*importModels.SchoolRole, error) {
				return &importModels.SchoolRole{
					ID:          staffImportTestRoleID,
					TenantID:    &tenantID,
					Name:        name,
					BaseRole:    &baseRole,
					Permissions: []string{"users:manage"},
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

	ctx := testpkg.ContextForTenant(context.Background(), staffImportTestTenantID)
	errs := config.Validate(ctx, row)

	require.Len(t, errs, 1)
	assert.Equal(t, "role_grant_not_permitted", errs[0].Code)

	ctx = importModels.ContextWithImporterPermissions(ctx, []string{"users:manage"})
	assert.Empty(t, config.Validate(ctx, row))
}

func TestStaffImportConfig_Validate_RejectsRolesReservedForOtherFlows(t *testing.T) {
	t.Parallel()

	guardian := "guardian"
	tenantID := staffImportTestTenantID
	role := &importModels.SchoolRole{
		ID:       staffImportTestRoleID,
		TenantID: &tenantID,
		Name:     "guardian-custom",
		BaseRole: &guardian,
	}
	config := NewStaffImportConfig(StaffImportDeps{RolePolicy: newTestRolePolicy(), Authorization: newTestAuthorization(), Transactions: newTestTransactions(),
		Roles: stubStaffRoleRepo{
			findByName: func(context.Context, string) (*importModels.SchoolRole, error) {
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

	errs := config.Validate(testpkg.ContextForTenant(context.Background(), staffImportTestTenantID), row)

	require.Len(t, errs, 1)
	assert.Equal(t, "role_not_assignable", errs[0].Code)
}

func TestStaffImportConfig_ValidateBatch_DetectsDuplicateEmailsAfterNormalization(t *testing.T) {
	t.Parallel()

	config := NewStaffImportConfig(StaffImportDeps{RolePolicy: newTestRolePolicy(), Authorization: newTestAuthorization(), Transactions: newTestTransactions()})
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

	config := NewStaffImportConfig(StaffImportDeps{RolePolicy: newTestRolePolicy(), Authorization: newTestAuthorization(), Transactions: newTestTransactions(),
		Roles: stubStaffRoleRepo{
			findByName: func(context.Context, string) (*importModels.SchoolRole, error) {
				return nil, importModels.ErrRoleNotFound
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
	config := NewStaffImportConfig(StaffImportDeps{RolePolicy: newTestRolePolicy(), Authorization: newTestAuthorization(), Transactions: newTestTransactions(),
		Roles: stubStaffRoleRepo{
			findByName: func(context.Context, string) (*importModels.SchoolRole, error) {
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
		config := NewStaffImportConfig(StaffImportDeps{RolePolicy: newTestRolePolicy(), Authorization: newTestAuthorization(), Transactions: newTestTransactions()})

		id, err := config.FindExisting(context.Background(), importModels.StaffImportRow{Email: "  "})

		require.NoError(t, err)
		assert.Nil(t, id)
	})

	t.Run("missing account returns nil", func(t *testing.T) {
		config := NewStaffImportConfig(StaffImportDeps{RolePolicy: newTestRolePolicy(), Authorization: newTestAuthorization(), Transactions: newTestTransactions(),
			FindSchoolAccount: func(context.Context, string) (int64, bool, error) { return 0, false, nil },
		})

		id, err := config.FindExisting(context.Background(), importModels.StaffImportRow{Email: "anna@example.com"})

		require.NoError(t, err)
		assert.Nil(t, id)
	})

	t.Run("existing account in other tenant returns nil", func(t *testing.T) {
		config := NewStaffImportConfig(StaffImportDeps{RolePolicy: newTestRolePolicy(), Authorization: newTestAuthorization(), Transactions: newTestTransactions(),
			FindSchoolAccount: func(context.Context, string) (int64, bool, error) { return staffImportTestAccountID, false, nil },
		})
		ctx := testpkg.ContextForTenant(context.Background(), staffImportTestTenantID)

		id, err := config.FindExisting(ctx, importModels.StaffImportRow{Email: " anna@example.com "})

		require.NoError(t, err)
		assert.Nil(t, id)
	})

	t.Run("existing account in current tenant returns id", func(t *testing.T) {
		config := NewStaffImportConfig(StaffImportDeps{RolePolicy: newTestRolePolicy(), Authorization: newTestAuthorization(), Transactions: newTestTransactions(),
			FindSchoolAccount: func(context.Context, string) (int64, bool, error) { return staffImportTestAccountID, true, nil },
		})
		ctx := testpkg.ContextForTenant(context.Background(), staffImportTestTenantID)

		id, err := config.FindExisting(ctx, importModels.StaffImportRow{Email: " anna@example.com "})

		require.NoError(t, err)
		require.NotNil(t, id)
		assert.Equal(t, staffImportTestAccountID, *id)
	})
}

func TestStaffImportConfig_FindExisting_PersonnelNumberDoesNotFallBackToNameWithoutNumber(t *testing.T) {
	t.Parallel()

	staff := &indexedStaff{ID: 99, FirstName: "Anna", LastName: "Lehmann"}
	config := NewStaffImportConfig(StaffImportDeps{RolePolicy: newTestRolePolicy(), Authorization: newTestAuthorization(), Transactions: newTestTransactions()})
	config.staffByName = map[string][]*indexedStaff{
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
	config := NewStaffImportConfig(StaffImportDeps{RolePolicy: newTestRolePolicy(), Authorization: newTestAuthorization(), Transactions: newTestTransactions(), Invitations: invitations})

	_, err := config.Create(context.Background(), importModels.StaffImportRow{FirstName: "Anna", LastName: "Lehmann", Email: "anna@example.com"})

	require.Error(t, err)
	assert.Empty(t, req.Email, "no invitation may go out when the Stammdatensatz cannot be written")
}

func TestStaffImportConfig_PreloadReferenceData_ReturnsRoleListError(t *testing.T) {
	t.Parallel()

	config := NewStaffImportConfig(StaffImportDeps{RolePolicy: newTestRolePolicy(), Authorization: newTestAuthorization(), Transactions: newTestTransactions(),
		Roles: stubStaffRoleRepo{listErr: errors.New("list failed")},
	})

	err := config.PreloadReferenceData(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "preload roles")
}

func TestStaffImportConfig_InvitationServiceCompileGuard(t *testing.T) {
	t.Parallel()

	var _ importModels.StaffInviter = (*staffInvitationMock)(nil)
	var _ importModels.SchoolRoleQuery = stubStaffRoleRepo{}
}

// TestStaffImportConfig_AuthorizeImportMode pins the permission boundary of
// the update-capable import modes (#2906): users:create alone files new
// records, changing existing ones needs both personnel permissions.
func TestStaffImportConfig_AuthorizeImportMode(t *testing.T) {
	t.Parallel()

	config := NewStaffImportConfig(StaffImportDeps{RolePolicy: newTestRolePolicy(), Authorization: newTestAuthorization(), Transactions: newTestTransactions()})
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
			ctx := importModels.ContextWithImporterPermissions(context.Background(), tc.permissions)
			err := config.AuthorizeImportMode(ctx, tc.mode)
			if tc.forbidden {
				require.ErrorIs(t, err, importModels.ErrImportModeForbidden)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestStaffImportConfig_EntityNameAndUpdate(t *testing.T) {
	t.Parallel()

	config := NewStaffImportConfig(StaffImportDeps{RolePolicy: newTestRolePolicy(), Authorization: newTestAuthorization(), Transactions: newTestTransactions()})

	assert.Equal(t, "Mitarbeiter", config.EntityName())
	// Update writes the Stammdatensatz (#2600); without the repositories it
	// must fail loudly instead of silently skipping the row.
	require.Error(t, config.Update(context.Background(), 0, importModels.StaffImportRow{}))
}
