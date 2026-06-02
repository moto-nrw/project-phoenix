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
	authsvc "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
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
func (r stubStaffAccountRepo) FindByEmail(context.Context, string) (*authModels.Account, error) {
	return r.account, r.err
}
func (r stubStaffAccountRepo) FindByUsername(context.Context, string) (*authModels.Account, error) {
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
func (r stubStaffAccountTenantRepo) FindActiveByAccountID(context.Context, int64) ([]authModels.AccountTenant, error) {
	panic("not implemented")
}
func (r stubStaffAccountTenantRepo) ExistsByAccountAndTenant(context.Context, int64, int64) (bool, error) {
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

type stubStaffSchoolRepo struct {
	school *platformModels.School
	err    error
}

func (r stubStaffSchoolRepo) Create(context.Context, *platformModels.School) error {
	panic("not implemented")
}
func (r stubStaffSchoolRepo) FindByID(context.Context, int64) (*platformModels.School, error) {
	return r.school, r.err
}
func (r stubStaffSchoolRepo) FindByIDForShare(context.Context, int64) (*platformModels.School, error) {
	panic("not implemented")
}
func (r stubStaffSchoolRepo) FindByIDForUpdate(context.Context, int64) (*platformModels.School, error) {
	panic("not implemented")
}
func (r stubStaffSchoolRepo) FindBySlug(context.Context, string) (*platformModels.School, error) {
	panic("not implemented")
}
func (r stubStaffSchoolRepo) FindByOrganizationAndSlug(context.Context, int64, string) (*platformModels.School, error) {
	panic("not implemented")
}
func (r stubStaffSchoolRepo) FindBySubdomain(context.Context, string) (*platformModels.School, error) {
	panic("not implemented")
}
func (r stubStaffSchoolRepo) List(context.Context) ([]*platformModels.School, error) {
	panic("not implemented")
}
func (r stubStaffSchoolRepo) ListActive(context.Context) ([]platformModels.School, error) {
	panic("not implemented")
}
func (r stubStaffSchoolRepo) ListPublic(context.Context) ([]platformModels.School, error) {
	panic("not implemented")
}
func (r stubStaffSchoolRepo) FindActiveByAccountID(context.Context, int64) ([]platformModels.School, error) {
	panic("not implemented")
}
func (r stubStaffSchoolRepo) Update(context.Context, *platformModels.School) error {
	panic("not implemented")
}
func (r stubStaffSchoolRepo) SoftDelete(context.Context, int64) error { panic("not implemented") }
func (r stubStaffSchoolRepo) Restore(context.Context, int64) error    { panic("not implemented") }
func (r stubStaffSchoolRepo) CountByIDs(context.Context, []int64) (int, error) {
	panic("not implemented")
}
func (r stubStaffSchoolRepo) CountNonDeletedByOrganizationID(context.Context, int64) (int, error) {
	panic("not implemented")
}

type stubStaffInvitationService struct {
	req authsvc.InvitationRequest
	err error
}

func (s *stubStaffInvitationService) WithTx(bun.Tx) interface{} { return s }
func (s *stubStaffInvitationService) CreateInvitation(_ context.Context, req authsvc.InvitationRequest) (*authModels.InvitationToken, error) {
	s.req = req
	if s.err != nil {
		return nil, s.err
	}
	return &authModels.InvitationToken{Model: base.Model{ID: staffImportTestInviteID}}, nil
}
func (s *stubStaffInvitationService) ValidateInvitation(context.Context, string) (*authsvc.InvitationValidationResult, error) {
	panic("not implemented")
}
func (s *stubStaffInvitationService) AcceptInvitation(context.Context, string, authsvc.UserRegistrationData) (*authModels.Account, error) {
	panic("not implemented")
}
func (s *stubStaffInvitationService) ResendInvitation(context.Context, int64, int64) error {
	panic("not implemented")
}
func (s *stubStaffInvitationService) ListPendingInvitations(context.Context) ([]*authModels.InvitationToken, error) {
	panic("not implemented")
}
func (s *stubStaffInvitationService) RevokeInvitation(context.Context, int64, int64) error {
	panic("not implemented")
}
func (s *stubStaffInvitationService) InvalidatePendingInvitationsByTenantID(context.Context, int64) (int, error) {
	panic("not implemented")
}
func (s *stubStaffInvitationService) CleanupExpiredInvitations(context.Context) (int, error) {
	panic("not implemented")
}

func TestStaffImportConfig_PreloadReferenceData_LoadsRoleDisplayNamesAndSchool(t *testing.T) {
	ctx := tenant.WithTenantID(context.Background(), staffImportTestTenantID)
	config := NewStaffImportConfig(StaffImportDeps{
		RoleRepo: stubStaffRoleRepo{roles: []*authModels.Role{
			{Model: base.Model{ID: staffImportTestRoleID}, Name: "user"},
			{Model: base.Model{ID: staffImportTestRoleID + 10}, Name: "koordination"},
		}},
		SchoolRepo: stubStaffSchoolRepo{school: &platformModels.School{
			Model: base.Model{ID: staffImportTestTenantID},
			Name:  "OGS Phoenix",
		}},
	})

	require.NoError(t, config.PreloadReferenceData(ctx))

	assert.Equal(t, []string{"Betreuer", "koordination"}, config.roleDisplayNames)
	assert.Equal(t, "OGS Phoenix", config.schoolName)
}

func TestStaffImportConfig_Validate_ResolvesGermanDisplayRole(t *testing.T) {
	config := NewStaffImportConfig(StaffImportDeps{
		RoleRepo: stubStaffRoleRepo{
			findByName: func(_ context.Context, name string) (*authModels.Role, error) {
				assert.Equal(t, "user", name)
				return &authModels.Role{Model: base.Model{ID: staffImportTestRoleID}, Name: name}, nil
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
	config := NewStaffImportConfig(StaffImportDeps{
		RoleRepo: stubStaffRoleRepo{
			findByName: func(_ context.Context, name string) (*authModels.Role, error) {
				return &authModels.Role{Model: base.Model{ID: staffImportTestRoleID}, Name: name}, nil
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

func TestStaffImportConfig_ValidateBatch_DetectsDuplicateEmailsAfterNormalization(t *testing.T) {
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

func TestStaffImportConfig_Create_PassesInvitationRequest(t *testing.T) {
	invitations := &stubStaffInvitationService{}
	config := NewStaffImportConfig(StaffImportDeps{InvitationService: invitations})
	config.schoolName = "OGS Phoenix"
	ctx := tenant.WithTenantID(context.Background(), staffImportTestTenantID)
	ctx = ContextWithImporterID(ctx, staffImportTestActorID)

	id, err := config.Create(ctx, importModels.StaffImportRow{
		FirstName: "Anna",
		LastName:  "Lehmann",
		Email:     "Anna Lehmann <Anna@Example.COM>",
		RoleID:    staffImportTestRoleID,
		Position:  "Leitung",
	})

	require.NoError(t, err)
	assert.Equal(t, staffImportTestInviteID, id)
	assert.Equal(t, "anna@example.com", invitations.req.Email)
	assert.Equal(t, staffImportTestRoleID, invitations.req.RoleID)
	assert.Equal(t, staffImportTestTenantID, invitations.req.TenantID)
	assert.Equal(t, staffImportTestActorID, invitations.req.CreatedBy)
	assert.Equal(t, "OGS Phoenix", invitations.req.SchoolName)
	require.NotNil(t, invitations.req.FirstName)
	require.NotNil(t, invitations.req.LastName)
	require.NotNil(t, invitations.req.Position)
	assert.Equal(t, "Anna", *invitations.req.FirstName)
	assert.Equal(t, "Lehmann", *invitations.req.LastName)
	assert.Equal(t, "Leitung", *invitations.req.Position)
}

func TestStaffImportConfig_Create_ReturnsInvitationError(t *testing.T) {
	inviteErr := errors.New("invite failed")
	config := NewStaffImportConfig(StaffImportDeps{
		InvitationService: &stubStaffInvitationService{err: inviteErr},
	})

	_, err := config.Create(context.Background(), importModels.StaffImportRow{})

	require.ErrorIs(t, err, inviteErr)
}

func TestStaffImportConfig_PreloadReferenceData_ReturnsRoleListError(t *testing.T) {
	config := NewStaffImportConfig(StaffImportDeps{
		RoleRepo: stubStaffRoleRepo{listErr: errors.New("list failed")},
	})

	err := config.PreloadReferenceData(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "preload roles")
}

func TestStaffImportConfig_InvitationServiceCompileGuard(t *testing.T) {
	var _ authsvc.InvitationService = (*stubStaffInvitationService)(nil)
	var _ authModels.RoleRepository = stubStaffRoleRepo{}
	var _ authModels.AccountRepository = stubStaffAccountRepo{}
	var _ authModels.AccountTenantRepository = stubStaffAccountTenantRepo{}
	var _ platformModels.SchoolRepository = stubStaffSchoolRepo{}
}
