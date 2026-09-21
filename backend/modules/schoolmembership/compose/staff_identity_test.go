package compose

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// accountPersons stands in for the People Directory behind the consumer-owned
// port; the architecture policy keeps this package from importing the owner.
// It repeats the owner's FindByAccount predicate (live person of the tenant,
// newest first) on the reader's own tenant transaction.
type accountPersons struct{}

func (accountPersons) FindPersonIDByAccount(ctx context.Context, accountID int64) (int64, bool, error) {
	ambient, _ := tenant.TransactionFromContext(ctx)
	transaction, ok := ambient.(bun.IDB)
	if !ok {
		return 0, false, errors.New("account persons: transaction is required")
	}
	var personIDs []int64
	err := transaction.NewRaw(
		`SELECT id FROM users.persons
		 WHERE account_id = ? AND tenant_id = ? AND deleted_at IS NULL
		 ORDER BY updated_at DESC, id ASC LIMIT 1`,
		accountID, tenant.FromContext(ctx),
	).Scan(ctx, &personIDs)
	if err != nil || len(personIDs) == 0 {
		return 0, false, err
	}
	return personIDs[0], true, nil
}

type failingAccountPersons struct{ err error }

func (p failingAccountPersons) FindPersonIDByAccount(context.Context, int64) (int64, bool, error) {
	return 0, false, p.err
}

func buildStaffIdentityReader(t *testing.T, db *bun.DB) (*schoolmembership.StaffIdentityReader, *schoolmembership.Module) {
	t.Helper()
	module := buildModule(t, db)
	return schoolmembership.NewStaffIdentityReader(module, accountPersons{}), module
}

func TestStaffIdentityReportsAnAccountWithoutPerson(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	reader, _ := buildStaffIdentityReader(t, db)
	account := testpkg.CreateTestAccount(t, db, "identity-no-person")

	identity, err := reader.ResolveByAccount(testpkg.Ctx(t), account.ID)

	require.NoError(t, err)
	assert.Equal(t, schoolmembership.StaffIdentity{Link: schoolmembership.StaffLinkNoPerson}, identity)
	assert.False(t, identity.IsStaff())
	assert.False(t, identity.IsTeacher())
}

func TestStaffIdentityReportsAPersonWithoutStaff(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	reader, _ := buildStaffIdentityReader(t, db)
	account := testpkg.CreateTestAccount(t, db, "identity-no-staff")
	person := testpkg.CreateTestPersonWithAccountID(t, db, "Nora", "Nichtpersonal", account.ID)

	identity, err := reader.ResolveByAccount(testpkg.Ctx(t), account.ID)

	require.NoError(t, err)
	assert.Equal(t, schoolmembership.StaffIdentity{
		Link:     schoolmembership.StaffLinkNotStaff,
		PersonID: person.ID,
	}, identity)
	assert.False(t, identity.IsStaff())
}

func TestStaffIdentityReportsStaffWithoutTeacherRole(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	reader, module := buildStaffIdentityReader(t, db)
	ctx := testpkg.Ctx(t)
	account := testpkg.CreateTestAccount(t, db, "identity-staff")
	person := testpkg.CreateTestPersonWithAccountID(t, db, "Stefan", "Personal", account.ID)
	staff, err := module.CreateStaff(ctx, schoolmembership.CreateStaff{StaffFields: schoolmembership.StaffFields{PersonID: person.ID}})
	require.NoError(t, err)

	identity, err := reader.ResolveByAccount(ctx, account.ID)

	require.NoError(t, err)
	assert.Equal(t, schoolmembership.StaffIdentity{
		Link:     schoolmembership.StaffLinkStaff,
		PersonID: person.ID,
		StaffID:  staff.ID,
	}, identity)
	assert.True(t, identity.IsStaff())
	assert.False(t, identity.IsTeacher())
}

func TestStaffIdentityReportsATeacher(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	reader, module := buildStaffIdentityReader(t, db)
	ctx := testpkg.Ctx(t)
	account := testpkg.CreateTestAccount(t, db, "identity-teacher")
	person := testpkg.CreateTestPersonWithAccountID(t, db, "Tina", "Lehrkraft", account.ID)
	staff, err := module.CreateStaff(ctx, schoolmembership.CreateStaff{StaffFields: schoolmembership.StaffFields{PersonID: person.ID}})
	require.NoError(t, err)
	teacher, err := module.CreateTeacher(ctx, schoolmembership.CreateTeacher{TeacherFields: schoolmembership.TeacherFields{StaffID: staff.ID}})
	require.NoError(t, err)

	want := schoolmembership.StaffIdentity{
		Link:      schoolmembership.StaffLinkTeacher,
		PersonID:  person.ID,
		StaffID:   staff.ID,
		TeacherID: teacher.ID,
	}

	byAccount, err := reader.ResolveByAccount(ctx, account.ID)
	require.NoError(t, err)
	assert.Equal(t, want, byAccount)
	assert.True(t, byAccount.IsStaff())
	assert.True(t, byAccount.IsTeacher())

	byPerson, err := reader.ResolveByPerson(ctx, person.ID)
	require.NoError(t, err)
	assert.Equal(t, want, byPerson)
}

func TestStaffIdentityIgnoresSoftDeletedMemberships(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	reader, module := buildStaffIdentityReader(t, db)
	ctx := testpkg.Ctx(t)
	person := testpkg.CreateTestPerson(t, db, "Theo", "Ehemalig")
	staff, err := module.CreateStaff(ctx, schoolmembership.CreateStaff{StaffFields: schoolmembership.StaffFields{PersonID: person.ID}})
	require.NoError(t, err)
	teacher, err := module.CreateTeacher(ctx, schoolmembership.CreateTeacher{TeacherFields: schoolmembership.TeacherFields{StaffID: staff.ID}})
	require.NoError(t, err)

	require.NoError(t, module.DeleteTeacher(ctx, teacher.ID))
	identity, err := reader.ResolveByPerson(ctx, person.ID)
	require.NoError(t, err)
	assert.Equal(t, schoolmembership.StaffLinkStaff, identity.Link)
	assert.Zero(t, identity.TeacherID)

	require.NoError(t, module.DeleteStaff(ctx, staff.ID))
	identity, err = reader.ResolveByPerson(ctx, person.ID)
	require.NoError(t, err)
	assert.Equal(t, schoolmembership.StaffIdentity{Link: schoolmembership.StaffLinkNotStaff, PersonID: person.ID}, identity)
}

func TestStaffIdentityWrongTenantLookupIsNotFound(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	reader, module := buildStaffIdentityReader(t, db)
	ctx := testpkg.Ctx(t)
	account := testpkg.CreateTestAccount(t, db, "identity-wrong-tenant")
	person := testpkg.CreateTestPersonWithAccountID(t, db, "Wanda", "Woanders", account.ID)
	staff, err := module.CreateStaff(ctx, schoolmembership.CreateStaff{StaffFields: schoolmembership.StaffFields{PersonID: person.ID}})
	require.NoError(t, err)
	_, err = module.CreateTeacher(ctx, schoolmembership.CreateTeacher{TeacherFields: schoolmembership.TeacherFields{StaffID: staff.ID}})
	require.NoError(t, err)
	otherCtx, _ := otherTenantContext(t, db)

	byAccount, err := reader.ResolveByAccount(otherCtx, account.ID)
	require.NoError(t, err)
	assert.Equal(t, schoolmembership.StaffIdentity{Link: schoolmembership.StaffLinkNoPerson}, byAccount)

	// Even a caller that already holds the foreign person ID learns nothing
	// about the other school's staff or teacher rows.
	byPerson, err := reader.ResolveByPerson(otherCtx, person.ID)
	require.NoError(t, err)
	assert.Equal(t, schoolmembership.StaffIdentity{Link: schoolmembership.StaffLinkNotStaff, PersonID: person.ID}, byPerson)
}

func TestStaffIdentityRefusesAContextWithoutTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	reader, module := buildStaffIdentityReader(t, db)
	ctx := testpkg.Ctx(t)
	account := testpkg.CreateTestAccount(t, db, "identity-no-tenant")
	person := testpkg.CreateTestPersonWithAccountID(t, db, "Nele", "Niemandsland", account.ID)
	_, err := module.CreateStaff(ctx, schoolmembership.CreateStaff{StaffFields: schoolmembership.StaffFields{PersonID: person.ID}})
	require.NoError(t, err)

	// The module's other reads fall back to an admin transaction without a
	// tenant; the identity read must not, or it would answer across schools.
	noTenant := testpkg.WithPackageTenantRuntime(context.Background())

	byPerson, err := reader.ResolveByPerson(noTenant, person.ID)
	require.ErrorIs(t, err, tenant.ErrTenantRequired)
	assert.Equal(t, schoolmembership.StaffIdentity{}, byPerson)

	byAccount, err := reader.ResolveByAccount(noTenant, account.ID)
	require.ErrorIs(t, err, tenant.ErrTenantRequired)
	assert.Equal(t, schoolmembership.StaffIdentity{}, byAccount)
}

func TestStaffIdentityKeepsUnexpectedErrorsVisible(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	account := testpkg.CreateTestAccount(t, db, "identity-directory-down")

	directoryDown := errors.New("people directory unavailable")
	reader := schoolmembership.NewStaffIdentityReader(module, failingAccountPersons{err: directoryDown})
	identity, err := reader.ResolveByAccount(ctx, account.ID)
	require.ErrorIs(t, err, directoryDown)
	assert.Equal(t, schoolmembership.StaffIdentity{}, identity)

	_, err = reader.ResolveByAccount(ctx, 0)
	require.ErrorIs(t, err, schoolmembership.ErrInvalidMembership)
	_, err = reader.ResolveByPerson(ctx, 0)
	require.ErrorIs(t, err, schoolmembership.ErrInvalidMembership)
}
