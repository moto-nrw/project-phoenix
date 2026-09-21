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
// newest first) on the identity read's own tenant transaction.
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

// resolve runs the identity read through the People Directory stand-in.
func resolve(ctx context.Context, module *schoolmembership.Module, accountID int64) (schoolmembership.StaffIdentity, error) {
	return module.ResolveStaffIdentityByAccount(ctx, accountID, accountPersons{})
}

// createLinkedStaff gives a fresh account a person and a staff row, and a
// teacher profile when asked to.
func createLinkedStaff(t *testing.T, ctx context.Context, db *bun.DB, module *schoolmembership.Module, email string, teacher bool) (accountID int64, staff schoolmembership.Staff, teacherID int64) {
	t.Helper()
	account := testpkg.CreateTestAccount(t, db, email)
	person := testpkg.CreateTestPersonWithAccountID(t, db, "Sam", "Stamm", account.ID)
	staff, err := module.CreateStaff(ctx, schoolmembership.CreateStaff{StaffFields: schoolmembership.StaffFields{PersonID: person.ID}})
	require.NoError(t, err)
	if teacher {
		created, err := module.CreateTeacher(ctx, schoolmembership.CreateTeacher{TeacherFields: schoolmembership.TeacherFields{StaffID: staff.ID}})
		require.NoError(t, err)
		teacherID = created.ID
	}
	return account.ID, staff, teacherID
}

func TestStaffIdentityReportsAnAccountWithoutPerson(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	account := testpkg.CreateTestAccount(t, db, "identity-no-person")

	identity, err := resolve(testpkg.Ctx(t), module, account.ID)

	require.NoError(t, err)
	assert.Equal(t, schoolmembership.StaffIdentity{Link: schoolmembership.StaffLinkNoPerson}, identity)
	assert.False(t, identity.IsTeacher())
}

func TestStaffIdentityReportsAPersonWithoutStaff(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	account := testpkg.CreateTestAccount(t, db, "identity-no-staff")
	person := testpkg.CreateTestPersonWithAccountID(t, db, "Nora", "Nichtpersonal", account.ID)

	identity, err := resolve(testpkg.Ctx(t), module, account.ID)

	require.NoError(t, err)
	assert.Equal(t, schoolmembership.StaffIdentity{
		Link:     schoolmembership.StaffLinkNotStaff,
		PersonID: person.ID,
	}, identity)
	assert.False(t, identity.IsTeacher())
}

func TestStaffIdentityReportsStaffWithoutTeacherRole(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	accountID, staff, _ := createLinkedStaff(t, ctx, db, module, "identity-staff", false)

	identity, err := resolve(ctx, module, accountID)

	require.NoError(t, err)
	assert.Equal(t, schoolmembership.StaffIdentity{
		Link:     schoolmembership.StaffLinkStaff,
		PersonID: staff.PersonID,
		StaffID:  staff.ID,
	}, identity)
	assert.False(t, identity.IsTeacher())
}

func TestStaffIdentityReportsATeacher(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	accountID, staff, teacherID := createLinkedStaff(t, ctx, db, module, "identity-teacher", true)

	identity, err := resolve(ctx, module, accountID)

	require.NoError(t, err)
	assert.Equal(t, schoolmembership.StaffIdentity{
		Link:      schoolmembership.StaffLinkTeacher,
		PersonID:  staff.PersonID,
		StaffID:   staff.ID,
		TeacherID: teacherID,
	}, identity)
	assert.True(t, identity.IsTeacher())
}

func TestStaffIdentityIgnoresSoftDeletedMemberships(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	accountID, staff, teacherID := createLinkedStaff(t, ctx, db, module, "identity-offboarded", true)

	require.NoError(t, module.DeleteTeacher(ctx, teacherID))
	identity, err := resolve(ctx, module, accountID)
	require.NoError(t, err)
	assert.Equal(t, schoolmembership.StaffLinkStaff, identity.Link)
	assert.Zero(t, identity.TeacherID)

	require.NoError(t, module.DeleteStaff(ctx, staff.ID))
	identity, err = resolve(ctx, module, accountID)
	require.NoError(t, err)
	assert.Equal(t, schoolmembership.StaffIdentity{Link: schoolmembership.StaffLinkNotStaff, PersonID: staff.PersonID}, identity)
}

func TestStaffIdentityWrongTenantLookupIsNotFound(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	accountID, _, _ := createLinkedStaff(t, ctx, db, module, "identity-wrong-tenant", true)
	otherCtx, _ := otherTenantContext(t, db)

	identity, err := resolve(otherCtx, module, accountID)

	require.NoError(t, err)
	assert.Equal(t, schoolmembership.StaffIdentity{Link: schoolmembership.StaffLinkNoPerson}, identity)
}

func TestStaffIdentityRefusesAContextWithoutTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	accountID, _, _ := createLinkedStaff(t, testpkg.Ctx(t), db, module, "identity-no-tenant", true)

	// The module's other reads fall back to an admin transaction without a
	// tenant; the identity read must not, or it would answer across schools.
	identity, err := resolve(testpkg.WithPackageTenantRuntime(context.Background()), module, accountID)

	require.ErrorIs(t, err, tenant.ErrTenantRequired)
	assert.Equal(t, schoolmembership.StaffIdentity{}, identity)
}

func TestStaffIdentityKeepsUnexpectedErrorsVisible(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	account := testpkg.CreateTestAccount(t, db, "identity-directory-down")

	directoryDown := errors.New("people directory unavailable")
	identity, err := module.ResolveStaffIdentityByAccount(ctx, account.ID, failingAccountPersons{err: directoryDown})
	require.ErrorIs(t, err, directoryDown)
	assert.Equal(t, schoolmembership.StaffIdentity{}, identity)

	_, err = resolve(ctx, module, 0)
	require.ErrorIs(t, err, schoolmembership.ErrInvalidMembership)
	_, err = module.ResolveStaffIdentityByAccount(ctx, account.ID, nil)
	require.ErrorIs(t, err, schoolmembership.ErrInvalidMembership)
}
