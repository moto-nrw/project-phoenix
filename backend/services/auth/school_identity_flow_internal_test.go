package auth

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	authModel "github.com/moto-nrw/project-phoenix/models/auth"
	baseModel "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

// The regression test for #2222 at the flow level: accepting an invitation
// that carries a school's own role must leave a usable staff member behind,
// not a person with nothing attached. The chain itself is provisioned through
// the Identity & Access port (#3225); its own cases run in the module.
func TestAcceptInvitation_CustomSchoolRoleCreatesStaff(t *testing.T) {
	t.Parallel()

	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())
	t.Cleanup(func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
		require.NoError(t, sqlDB.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	})

	tenantID := int64(42)
	adminBase := authModel.BaseRoleAdmin
	invitations := newStubInvitationTokenRepository()
	roles := newStubRoleRepository(
		&authModel.Role{
			Model:    baseModel.Model{ID: 71},
			Name:     "ogs-leitung",
			TenantID: &tenantID,
			BaseRole: &adminBase,
		},
	)
	persons := newStubPersonRepository()
	staff, staffAll := newStubStaffRepository()
	teachers := newStubTeacherRepository()

	service := newTestInvitationService(t, InvitationServiceConfig{
		InvitationRepo:    invitations,
		AccountRepo:       newStubAccountRepository(),
		AccountTenantRepo: newStubAccountTenantRepository(),
		RoleRepo:          roles,
		AccountRoleRepo:   newStubAccountRoleRepository(),
		SchoolIdentity:    identityStub(persons, staff, teachers),
		SchoolRepo:        newStubSchoolRepository(nil),
		FrontendURL:       "http://localhost:3000",
		DefaultFrom:       newDefaultFromEmail(),
		InvitationExpiry:  48 * time.Hour,
		DB:                bunDB,
	})

	token := &authModel.InvitationToken{
		Email:     "schulleitung@example.com",
		Token:     "custom-role-token",
		RoleID:    71,
		CreatedBy: nullableCreatedBy(31),
		ExpiresAt: time.Now().Add(10 * time.Hour),
	}
	token.SetTenantID(tenantID)
	require.NoError(t, invitations.Create(context.Background(), token))

	expectAdminTx(mock)
	account, err := service.AcceptInvitation(context.Background(), token.Token, UserRegistrationData{
		FirstName:       "Ada",
		LastName:        "Lovelace",
		Password:        testStrongCredential,
		ConfirmPassword: testStrongCredential,
	})
	require.NoError(t, err)
	require.NotNil(t, account)

	require.Len(t, persons.people, 1)
	createdStaff := staffAll()
	require.Len(t, createdStaff, 1, "a school's own role must produce a staff record")
	require.Equal(t, tenantID, createdStaff[0].TenantID)
	require.Empty(t, teachers.All(), "an admin-tier role runs without a caregiver profile")
}
