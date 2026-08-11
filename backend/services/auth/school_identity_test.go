package auth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/DATA-DOG/go-sqlmock"
	authModel "github.com/moto-nrw/project-phoenix/models/auth"
	baseModel "github.com/moto-nrw/project-phoenix/models/base"
	userModel "github.com/moto-nrw/project-phoenix/models/users"
)

const identityTenantID int64 = 77

func identityRepos() (SchoolIdentityRepos, *stubPersonRepository, func() []*userModel.Staff, *stubTeacherRepository) {
	repos, persons, staffAll, teachers, _ := identityReposWithStudents()
	return repos, persons, staffAll, teachers
}

// identityReposWithStudents additionally exposes the student repository, for
// the cases that care whether the person is a child's record.
func identityReposWithStudents() (SchoolIdentityRepos, *stubPersonRepository, func() []*userModel.Staff, *stubTeacherRepository, *stubStudentRepository) {
	persons := newStubPersonRepository()
	staff, staffAll := newStubStaffRepository()
	teachers := newStubTeacherRepository()
	students := newStubStudentRepository()
	return SchoolIdentityRepos{
		Persons:  persons,
		Staff:    staff,
		Teachers: teachers,
		Students: students,
	}, persons, staffAll, teachers, students
}

func customRole(name, base string) *authModel.Role {
	tenantID := identityTenantID
	role := &authModel.Role{Model: baseModel.Model{ID: 501}, Name: name, TenantID: &tenantID}
	if base != "" {
		role.BaseRole = &base
	}
	return role
}

func identityInput(role *authModel.Role) SchoolIdentityInput {
	return SchoolIdentityInput{
		AccountID:    9001,
		TenantID:     identityTenantID,
		Role:         role,
		FirstName:    "Ada",
		LastName:     "Lovelace",
		CreatePerson: true,
	}
}

// The bug of #2222: an account invited with a school's own role received a
// person and no staff record, which breaks every screen behind GetCurrentStaff.
func TestEnsureSchoolIdentity_CustomAdminRoleGetsStaffWithoutCaregiverProfile(t *testing.T) {
	repos, persons, staffAll, teachers := identityRepos()

	identity, err := EnsureSchoolIdentity(context.Background(), repos, identityInput(customRole("OGS-Leitung", authModel.BaseRoleAdmin)))
	require.NoError(t, err)
	require.NotNil(t, identity)

	require.NotNil(t, identity.Person)
	require.Equal(t, identityTenantID, identity.Person.TenantID)
	require.NotNil(t, identity.Person.AccountID)
	require.Equal(t, int64(9001), *identity.Person.AccountID)
	require.Len(t, persons.people, 1)

	require.Len(t, staffAll(), 1)
	require.Equal(t, identityTenantID, staffAll()[0].TenantID)

	require.Nil(t, identity.Teacher)
	require.Empty(t, teachers.All(), "an admin-tier role runs without a caregiver profile")
}

// A school's own caregiver-tier role gets the same caregiver profile the
// platform user role gets. Without it the account shows up in the staff list
// and still finds its groups and supervisions empty.
func TestEnsureSchoolIdentity_CustomUserRoleGetsCaregiverProfile(t *testing.T) {
	repos, _, staffAll, teachers := identityRepos()

	in := identityInput(customRole("OGS-Kraft", authModel.BaseRoleUser))
	in.Position = "Gruppenleitung"

	identity, err := EnsureSchoolIdentity(context.Background(), repos, in)
	require.NoError(t, err)
	require.NotNil(t, identity)
	require.NotNil(t, identity.Teacher)

	require.Len(t, teachers.All(), 1)
	require.Equal(t, staffAll()[0].ID, teachers.All()[0].StaffID)
	require.Equal(t, "Gruppenleitung", teachers.All()[0].Role)
	require.Equal(t, identityTenantID, teachers.All()[0].TenantID)
}

// base_role is NULL on school roles created before migration 1.15.31. Unknown
// tier counts as personnel: a staff row grants nothing, withholding it is what
// breaks the account.
func TestEnsureSchoolIdentity_UnknownTierIsStaff(t *testing.T) {
	repos, _, staffAll, teachers := identityRepos()

	identity, err := EnsureSchoolIdentity(context.Background(), repos, identityInput(customRole("Alt-Rolle", "")))
	require.NoError(t, err)
	require.NotNil(t, identity)
	require.Len(t, staffAll(), 1)
	require.Empty(t, teachers.All())
}

func TestEnsureSchoolIdentity_GuardianTierProvisionsNothing(t *testing.T) {
	repos, persons, staffAll, _ := identityRepos()

	identity, err := EnsureSchoolIdentity(context.Background(), repos, identityInput(customRole("Sorgeberechtigt", authModel.BaseRoleGuardian)))
	require.NoError(t, err)
	require.Nil(t, identity)
	require.Empty(t, persons.people)
	require.Empty(t, staffAll())
}

// The Lehrkraft role is class_day-read-only by design (#1772) and keeps the
// staff record without a caregiver profile, even though its tier is 'user'.
func TestEnsureSchoolIdentity_LehrkraftGetsStaffWithoutCaregiverProfile(t *testing.T) {
	repos, _, staffAll, teachers := identityRepos()

	lehrkraft := &authModel.Role{Model: baseModel.Model{ID: 55}, Name: "lehrkraft", IsSystem: true}
	base := authModel.BaseRoleUser
	lehrkraft.BaseRole = &base

	identity, err := EnsureSchoolIdentity(context.Background(), repos, identityInput(lehrkraft))
	require.NoError(t, err)
	require.NotNil(t, identity)
	require.Len(t, staffAll(), 1)
	require.Empty(t, teachers.All())
}

// Re-granting access must reuse the existing chain. A second person for the
// same account at the same school is refused by the partial unique index on
// (tenant_id, account_id) anyway — that is how a re-invitation after
// offboarding used to fail.
func TestEnsureSchoolIdentity_IsIdempotent(t *testing.T) {
	repos, persons, staffAll, teachers := identityRepos()
	in := identityInput(customRole("OGS-Kraft", authModel.BaseRoleUser))

	first, err := EnsureSchoolIdentity(context.Background(), repos, in)
	require.NoError(t, err)

	second, err := EnsureSchoolIdentity(context.Background(), repos, in)
	require.NoError(t, err)

	require.Equal(t, first.Person.ID, second.Person.ID)
	require.Equal(t, first.Staff.ID, second.Staff.ID)
	require.Equal(t, first.Teacher.ID, second.Teacher.ID)
	require.Len(t, persons.people, 1)
	require.Len(t, staffAll(), 1)
	require.Len(t, teachers.All(), 1)
}

// An account with a person but no staff record is exactly the state the
// invitation flow produced. Provisioning must attach the staff record to that
// person instead of creating a second identity.
func TestEnsureSchoolIdentity_AdoptsExistingPerson(t *testing.T) {
	repos, persons, staffAll, _ := identityRepos()

	accountID := int64(9001)
	existing := &userModel.Person{FirstName: "Bestehend", LastName: "Person", AccountID: &accountID}
	existing.SetTenantID(identityTenantID)
	require.NoError(t, persons.Create(context.Background(), existing))

	identity, err := EnsureSchoolIdentity(context.Background(), repos, identityInput(customRole("OGS-Leitung", authModel.BaseRoleAdmin)))
	require.NoError(t, err)

	require.Equal(t, existing.ID, identity.Person.ID)
	require.Len(t, persons.people, 1, "the existing person must be reused")
	require.Len(t, staffAll(), 1)
	require.Equal(t, existing.ID, staffAll()[0].PersonID)
}

func TestEnsureSchoolIdentity_RefusesToInventAnIdentity(t *testing.T) {
	repos, _, staffAll, _ := identityRepos()

	in := identityInput(customRole("OGS-Leitung", authModel.BaseRoleAdmin))
	in.FirstName = ""

	_, err := EnsureSchoolIdentity(context.Background(), repos, in)
	require.ErrorIs(t, err, ErrSchoolIdentityNamesRequired)
	require.Empty(t, staffAll())
}

func TestEnsureSchoolIdentity_WithoutCreatePersonLeavesAccountAlone(t *testing.T) {
	repos, persons, staffAll, _ := identityRepos()

	in := identityInput(customRole("OGS-Leitung", authModel.BaseRoleAdmin))
	in.CreatePerson = false

	identity, err := EnsureSchoolIdentity(context.Background(), repos, in)
	require.NoError(t, err)
	require.Nil(t, identity)
	require.Empty(t, persons.people)
	require.Empty(t, staffAll())
}

// The regression test for the reported bug, at the flow level: accepting an
// invitation that carries a school's own role must leave a usable staff member
// behind, not a person with nothing attached.
func TestAcceptInvitation_CustomSchoolRoleCreatesStaff(t *testing.T) {
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

	service := NewInvitationService(InvitationServiceConfig{
		InvitationRepo:    invitations,
		AccountRepo:       newStubAccountRepository(),
		AccountTenantRepo: newStubAccountTenantRepository(),
		RoleRepo:          roles,
		AccountRoleRepo:   newStubAccountRoleRepository(),
		PersonRepo:        persons,
		StaffRepo:         staff,
		TeacherRepo:       teachers,
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

// The chain can only be built on the person that carries the account_id, and
// when that person is a child's record, building it would file the child as
// personnel. Refusing is the one case where leaving the account incomplete is
// the better outcome: the resulting rows are indistinguishable from legitimate
// ones, so nothing could repair it afterwards.
func TestEnsureSchoolIdentity_RefusesPersonThatIsAStudent(t *testing.T) {
	repos, persons, staffAll, teachers, students := identityReposWithStudents()

	accountID := int64(9001)
	child := &userModel.Person{FirstName: "Kind", LastName: "Datensatz", AccountID: &accountID}
	child.SetTenantID(identityTenantID)
	require.NoError(t, persons.Create(context.Background(), child))
	students.markStudent(child.ID)

	_, err := EnsureSchoolIdentity(context.Background(), repos, identityInput(customRole("OGS-Leitung", authModel.BaseRoleAdmin)))
	require.ErrorIs(t, err, ErrSchoolIdentityPersonIsStudent)

	require.Empty(t, staffAll(), "no staff record may be built on a child's person")
	require.Empty(t, teachers.All())
	require.Len(t, persons.people, 1, "the child's record itself stays untouched")
}

// A person that is not a student is the ordinary case and must pass the same
// check unharmed — the repository reports the miss as sql.ErrNoRows, which is
// not an error here.
func TestEnsureSchoolIdentity_ExistingNonStudentPersonPasses(t *testing.T) {
	repos, persons, staffAll, _, _ := identityReposWithStudents()

	accountID := int64(9001)
	existing := &userModel.Person{FirstName: "Bestehend", LastName: "Person", AccountID: &accountID}
	existing.SetTenantID(identityTenantID)
	require.NoError(t, persons.Create(context.Background(), existing))

	identity, err := EnsureSchoolIdentity(context.Background(), repos, identityInput(customRole("OGS-Leitung", authModel.BaseRoleAdmin)))
	require.NoError(t, err)
	require.Equal(t, existing.ID, identity.Person.ID)
	require.Len(t, staffAll(), 1)
}
