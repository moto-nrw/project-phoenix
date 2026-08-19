package auth

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/email"
	authModel "github.com/moto-nrw/project-phoenix/models/auth"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// failingStaffRepo lets the identity provisioning get as far as the staff
// record and then fails, which is the step ordering the rollback has to cover:
// account, school mapping and role assignment are all written before it.
type failingStaffRepo struct {
	userModels.StaffRepository
	err error
}

func (r failingStaffRepo) Create(context.Context, *userModels.Staff) error {
	return r.err
}

// A failure while provisioning the school identity must take the whole
// acceptance with it. The account, the school mapping and the role assignment
// are written first, so anything left behind is precisely the state issue #2222
// describes: an account that holds a role at a school and is not staff.
//
// This runs against a real database on purpose. The in-memory stubs the rest of
// the invitation tests use have no transaction, so they cannot tell a rollback
// from a write that simply never happened.
func TestAcceptInvitationRollsBackAccountMappingAndRole(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	ctx := context.Background()
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	t.Cleanup(func() { testpkg.CleanupTenantTestData(t, db, tenantID) })

	repos := repositories.NewFactory(db)
	role := testpkg.CreateTestRoleForTenant(t, db, "rollback-kraft", tenantID)
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, db, "auth.roles", role.ID) })

	provisioningErr := errors.New("staff insert failed")
	service := NewInvitationService(InvitationServiceConfig{
		InvitationRepo:    repos.InvitationToken,
		AccountRepo:       repos.Account,
		AccountTenantRepo: repos.AccountTenant,
		RoleRepo:          repos.Role,
		AccountRoleRepo:   repos.AccountRole,
		PersonRepo:        repos.Person,
		StaffRepo:         failingStaffRepo{StaffRepository: repos.Staff, err: provisioningErr},
		TeacherRepo:       repos.Teacher,
		SchoolRepo:        repos.School,
		Mailer:            email.NewMockMailer(),
		FrontendURL:       "http://localhost:3000",
		InvitationExpiry:  time.Hour,
		DB:                db,
	})

	emailAddr := fmt.Sprintf("rollback-%d@test.local", time.Now().UnixNano())
	token := &authModel.InvitationToken{
		Email:     emailAddr,
		Token:     fmt.Sprintf("rollback-token-%d", time.Now().UnixNano()),
		RoleID:    role.ID,
		ExpiresAt: time.Now().Add(10 * time.Hour),
	}
	token.SetTenantID(tenantID)
	require.NoError(t, repos.InvitationToken.Create(testpkg.TenantContext(tenantID), token))
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM auth.invitation_tokens WHERE email = ?`, emailAddr)
	})

	_, err := service.AcceptInvitation(ctx, token.Token, UserRegistrationData{
		FirstName:       "Rolled",
		LastName:        "Back",
		Password:        testStrongCredential,
		ConfirmPassword: testStrongCredential,
	})
	require.Error(t, err)
	require.ErrorIs(t, err, provisioningErr)

	accountID := accountIDForEmail(t, db, emailAddr)
	assert.Zero(t, accountID, "the account must not survive a failed identity provisioning")

	assert.Zero(t, countRows(t, db,
		`SELECT COUNT(*) FROM auth.account_tenants at
		 JOIN auth.accounts a ON a.id = at.account_id
		 WHERE a.email = ? AND at.tenant_id = ?`, emailAddr, tenantID),
		"the school mapping must be rolled back with the account")

	assert.Zero(t, countRows(t, db,
		`SELECT COUNT(*) FROM auth.account_roles ar
		 JOIN auth.accounts a ON a.id = ar.account_id
		 WHERE a.email = ? AND ar.tenant_id = ?`, emailAddr, tenantID),
		"the role assignment must be rolled back with the account")

	assert.Zero(t, countRows(t, db,
		`SELECT COUNT(*) FROM users.persons WHERE tenant_id = ? AND first_name = 'Rolled' AND last_name = 'Back'`,
		tenantID),
		"the person must be rolled back with the account")

	stored, err := repos.InvitationToken.FindByToken(testpkg.TenantContext(tenantID), token.Token)
	require.NoError(t, err)
	require.NotNil(t, stored)
	assert.False(t, stored.IsUsed(), "the invitation must stay usable after a failed acceptance")
}

func accountIDForEmail(t *testing.T, db *bun.DB, emailAddr string) int64 {
	t.Helper()
	var ids []int64
	err := db.NewRaw(`SELECT id FROM auth.accounts WHERE email = ?`, emailAddr).Scan(context.Background(), &ids)
	require.NoError(t, err)
	if len(ids) == 0 {
		return 0
	}
	return ids[0]
}

func countRows(t *testing.T, db *bun.DB, query string, args ...any) int {
	t.Helper()
	var count int
	require.NoError(t, db.NewRaw(query, args...).Scan(context.Background(), &count))
	return count
}
