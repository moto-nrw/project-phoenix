package repositories_test

// The staff, teacher and guest repositories are compositions over the School
// Membership owner (#2667). The tests below pin what the composition adds on
// top of the owner: the legacy not-found error shape callers classify on,
// tenant isolation, and the person/account/role compositions that used to be
// SQL joins.

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// missingID derives an id no fixture can have produced, so the test carries no
// literal row id.
func missingID(anchor int64) int64 { return anchor + 1_000_000 }

func assignSystemCaregiverRole(t *testing.T, db *testpkg.DB, accountID, tenantID int64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var roleID int64
	err := db.NewSelect().
		ColumnExpr("id").
		TableExpr("auth.roles").
		Where("LOWER(name) = ?", "user").
		Where("is_system = TRUE").
		Where("tenant_id IS NULL").
		Scan(ctx, &roleID)
	require.NoError(t, err, "the seeded user system role must exist")
	_, err = db.NewInsert().
		Model(&map[string]any{"account_id": accountID, "role_id": roleID, "tenant_id": tenantID}).
		TableExpr("auth.account_roles").
		On("CONFLICT DO NOTHING").
		Exec(ctx)
	require.NoError(t, err)
}

func TestStaffMembershipAdapter_NotFoundKeepsTheLegacyErrorShape(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	factory := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)

	staff := testpkg.CreateTestStaff(t, db, "Missing", "Contract")
	absent := missingID(staff.ID)

	t.Run("FindByID", func(t *testing.T) {
		_, err := factory.Staff.FindByID(ctx, absent)
		require.Error(t, err)
		assert.True(t, testpkg.IsNotFoundError(err), "callers classify staff lookups with sql.ErrNoRows")
	})

	t.Run("FindWithPerson", func(t *testing.T) {
		_, err := factory.Staff.FindWithPerson(ctx, absent)
		require.Error(t, err)
		assert.True(t, testpkg.IsNotFoundError(err))
	})

	t.Run("FindByPersonID", func(t *testing.T) {
		_, err := factory.Staff.FindByPersonID(ctx, absent)
		require.Error(t, err)
		assert.True(t, testpkg.IsNotFoundError(err))
	})

	t.Run("FindByIDForUpdate", func(t *testing.T) {
		_, err := factory.Staff.FindByIDForUpdate(ctx, absent)
		require.Error(t, err)
		assert.True(t, testpkg.IsNotFoundError(err))
	})

	t.Run("GetStaffContactInfo", func(t *testing.T) {
		_, err := factory.Staff.GetStaffContactInfo(ctx, absent)
		require.Error(t, err)
		assert.True(t, testpkg.IsNotFoundError(err))
	})

	t.Run("teacher and guest", func(t *testing.T) {
		_, err := factory.Teacher.FindByID(ctx, absent)
		require.Error(t, err)
		assert.True(t, testpkg.IsNotFoundError(err))

		_, err = factory.Guest.FindByID(ctx, absent)
		require.Error(t, err)
		assert.True(t, testpkg.IsNotFoundError(err))

		// A staff member who is not a teacher is not an error — every caller
		// branches on the nil.
		teacher, err := factory.Teacher.FindByStaffID(ctx, staff.ID)
		require.NoError(t, err)
		assert.Nil(t, teacher)
	})
}

func TestStaffMembershipAdapter_ListsAreTenantScoped(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	factory := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)

	own := testpkg.CreateTestStaff(t, db, "Own", "Tenant")
	foreignTenant, _ := testpkg.CreateTestTenant(t, db)
	foreign := testpkg.CreateTestStaffForTenant(t, db, foreignTenant, "Foreign", "Tenant")

	members, err := factory.Staff.ListAllWithPerson(ctx)
	require.NoError(t, err)
	ids := make(map[int64]bool, len(members))
	for _, member := range members {
		ids[member.ID] = true
	}
	assert.True(t, ids[own.ID], "the tenant's own staff must be listed")
	assert.False(t, ids[foreign.ID], "another school's staff must stay invisible")

	byID, err := factory.Staff.FindByIDs(ctx, []int64{own.ID, foreign.ID})
	require.NoError(t, err)
	assert.Contains(t, byID, own.ID)
	assert.NotContains(t, byID, foreign.ID)

	_, err = factory.Staff.FindByID(ctx, foreign.ID)
	require.Error(t, err)
	assert.True(t, testpkg.IsNotFoundError(err))
}

func TestStaffMembershipAdapter_AttachesPersonAndAccount(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	factory := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)

	staff, account := testpkg.CreateTestStaffWithAccount(t, db, "Person", "Attached")

	withPerson, err := factory.Staff.FindWithPerson(ctx, staff.ID)
	require.NoError(t, err)
	require.NotNil(t, withPerson.Person)
	assert.Equal(t, "Person", withPerson.Person.FirstName)

	byIDs, err := factory.Staff.FindWithPersonByIDs(ctx, []int64{staff.ID})
	require.NoError(t, err)
	require.Contains(t, byIDs, staff.ID)
	require.NotNil(t, byIDs[staff.ID].Person)
	assert.Equal(t, "Attached", byIDs[staff.ID].Person.LastName)

	info, err := factory.Staff.GetStaffContactInfo(ctx, staff.ID)
	require.NoError(t, err)
	assert.Equal(t, account.Email, info.Email, "the e-mail comes from the account owner")
	assert.Equal(t, "Person", info.FirstName)
}

// FindByIDs keeps the legacy soft-delete filter (its callers validate live
// supervisors and ledger staff); FindWithPersonByIDs keeps resolving
// offboarded staff for the historical projections.
func TestStaffMembershipAdapter_IDLookupsKeepLegacyDeletedSemantics(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	factory := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Historical", "Staff")
	require.NoError(t, factory.Staff.Delete(ctx, staff.ID))

	byID, err := factory.Staff.FindByIDs(ctx, []int64{staff.ID})
	require.NoError(t, err)
	require.NotContains(t, byID, staff.ID, "FindByIDs must keep excluding offboarded staff")

	withPerson, err := factory.Staff.FindWithPersonByIDs(ctx, []int64{staff.ID})
	require.NoError(t, err)
	require.Contains(t, withPerson, staff.ID)
}

func TestStaffMembershipAdapter_AccountIDsExcludeUnreachableStaff(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	factory := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)

	withAccount, account := testpkg.CreateTestCalendarStaff(t, db, "Reachable", "Colleague")
	withoutAccount := testpkg.CreateTestStaff(t, db, "Accountless", "Colleague")

	accounts, err := factory.Staff.ListAccountIDsByStaffIDs(ctx, []int64{withAccount.ID, withoutAccount.ID})
	require.NoError(t, err)
	assert.Equal(t, account.ID, accounts[withAccount.ID])
	assert.NotContains(t, accounts, withoutAccount.ID, "staff without an account must be absent, not mapped to zero")

	all, err := factory.Staff.ListAllStaffAccountIDs(ctx)
	require.NoError(t, err)
	assert.Contains(t, all, withAccount.ID)
	assert.NotContains(t, all, withoutAccount.ID)
}

func TestStaffMembershipAdapter_ExcludesSoftDeletedPersons(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	factory := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)
	staff, _ := testpkg.CreateTestStaffWithAccount(t, db, "Deleted", "Person")
	_, err := db.NewRaw(`UPDATE users.persons SET deleted_at = NOW() WHERE id = ?`, staff.PersonID).Exec(ctx)
	require.NoError(t, err)

	accounts, err := factory.Staff.ListAccountIDsByStaffIDs(ctx, []int64{staff.ID})
	require.NoError(t, err)
	assert.NotContains(t, accounts, staff.ID)

	all, err := factory.Staff.ListAllStaffAccountIDs(ctx)
	require.NoError(t, err)
	assert.NotContains(t, all, staff.ID)

	withPerson, err := factory.Staff.FindWithPersonByIDs(ctx, []int64{staff.ID})
	require.NoError(t, err)
	require.Contains(t, withPerson, staff.ID)
	assert.Nil(t, withPerson[staff.ID].Person)
}

func TestStaffMembershipAdapter_AccountActivityStaysTenantScoped(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	factory := repositories.NewFactory(db)
	primary, account := testpkg.CreateTestStaffWithAccount(t, db, "Primary", "Account")
	foreignTenant, _ := testpkg.CreateTestTenant(t, db)
	foreign, _ := testpkg.CreateTestStaffWithAccountForTenant(t, db, foreignTenant, "Foreign", "Account")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.NewRaw(`UPDATE users.persons SET account_id = ? WHERE id = ?`, account.ID, foreign.PersonID).Exec(ctx)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, foreignTenant)
	_, err = db.NewRaw(`UPDATE auth.account_tenants SET status = 'inactive' WHERE account_id = ? AND tenant_id = ?`, account.ID, foreignTenant).Exec(ctx)
	require.NoError(t, err)

	err = testpkg.WithinAdminContext(t, context.Background(), db, func(adminCtx context.Context) error {
		byID, err := factory.Staff.ListAccountIDsByStaffIDs(adminCtx, []int64{primary.ID, foreign.ID})
		if err != nil {
			return err
		}
		assert.Equal(t, account.ID, byID[primary.ID])
		assert.NotContains(t, byID, foreign.ID)

		all, err := factory.Staff.ListAllStaffAccountIDs(adminCtx)
		if err != nil {
			return err
		}
		assert.Equal(t, account.ID, all[primary.ID])
		assert.NotContains(t, all, foreign.ID)
		return nil
	})
	require.NoError(t, err)
}

func TestStaffMembershipAdapter_CalendarReachabilityUsesEffectivePermissions(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	factory := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)

	reachable, _ := testpkg.CreateTestCalendarStaff(t, db, "Calendar", "User")
	unreachable := testpkg.CreateTestStaff(t, db, "NoAccount", "User")

	result, err := factory.Staff.FindReachableCalendarStaffIDs(ctx, []int64{reachable.ID, unreachable.ID})
	require.NoError(t, err)
	assert.True(t, result[reachable.ID], "an active account with the user role can use the calendar")
	assert.False(t, result[unreachable.ID], "a staff member without a login account cannot")
}

func TestTeacherMembershipAdapter_ResolvesStaffAndPerson(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	factory := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)

	teacher, _ := testpkg.CreateTestTeacherWithAccount(t, db, "Composed", "Teacher")

	found, err := factory.Teacher.FindWithStaffAndPerson(ctx, teacher.ID)
	require.NoError(t, err)
	require.NotNil(t, found.Staff)
	require.NotNil(t, found.Staff.Person)
	assert.Equal(t, "Composed", found.Staff.Person.FirstName)

	byStaff, err := factory.Teacher.FindByStaffIDs(ctx, []int64{teacher.StaffID})
	require.NoError(t, err)
	require.Contains(t, byStaff, teacher.StaffID, "the map is keyed by staff id")
	assert.Equal(t, teacher.ID, byStaff[teacher.StaffID].ID)

	all, err := factory.Teacher.ListAllWithStaffAndPerson(ctx)
	require.NoError(t, err)
	var listed bool
	for _, entry := range all {
		if entry.ID == teacher.ID {
			listed = true
			require.NotNil(t, entry.Staff)
			require.NotNil(t, entry.Staff.Person)
		}
	}
	assert.True(t, listed)
}

func TestTeacherMembershipAdapter_ActiveCaregiversNeedAccountAndSystemRole(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	factory := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)

	// assignUserRole gives the account the seeded "user" system role at this
	// tenant, the way account provisioning does.
	assignUserRole := func(accountID int64) {
		assignSystemCaregiverRole(t, db, accountID, tenantID)
	}

	caregiver, caregiverAccount := testpkg.CreateTestTeacherWithAccount(t, db, "Active", "Caregiver")
	testpkg.EnsureAccountTenant(t, db, caregiverAccount.ID, tenantID)
	assignUserRole(caregiverAccount.ID)

	// A teacher without any system role is not an operational caregiver.
	roleless, rolelessAccount := testpkg.CreateTestTeacherWithAccount(t, db, "Roleless", "Teacher")
	testpkg.EnsureAccountTenant(t, db, rolelessAccount.ID, tenantID)

	caregivers, err := factory.Teacher.ListActiveCaregivers(ctx)
	require.NoError(t, err)
	listed := make(map[int64]bool, len(caregivers))
	for _, entry := range caregivers {
		listed[entry.TeacherID] = true
		if entry.TeacherID == caregiver.ID {
			assert.Equal(t, caregiverAccount.Email, entry.Email)
			assert.Equal(t, caregiver.StaffID, entry.StaffID)
		}
	}
	assert.True(t, listed[caregiver.ID])
	assert.False(t, listed[roleless.ID], "a teacher without a system caregiver role is not listed")

	found, err := factory.Teacher.FindActiveCaregiverByAccountID(ctx, caregiverAccount.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, caregiver.ID, found.TeacherID)

	missing, err := factory.Teacher.FindActiveCaregiverByAccountID(ctx, rolelessAccount.ID)
	require.NoError(t, err)
	assert.Nil(t, missing, "a non-caregiver resolves to nil, not an error")
}

func TestTeacherMembershipAdapter_ActiveCaregiversResolveInAdminContext(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	factory := repositories.NewFactory(db)
	foreignTenant, _ := testpkg.CreateTestTenant(t, db)
	staff, account := testpkg.CreateTestStaffWithAccountForTenant(t, db, foreignTenant, "Admin", "Caregiver")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var teacherID int64
	err := db.NewRaw(`INSERT INTO users.teachers (staff_id, tenant_id) VALUES (?, ?) RETURNING id`, staff.ID, foreignTenant).Scan(ctx, &teacherID)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, foreignTenant)
	assignSystemCaregiverRole(t, db, account.ID, foreignTenant)

	err = testpkg.WithinAdminContext(t, context.Background(), db, func(adminCtx context.Context) error {
		caregivers, err := factory.Teacher.ListActiveCaregivers(adminCtx)
		if err != nil {
			return err
		}
		listed := make(map[int64]bool, len(caregivers))
		for _, caregiver := range caregivers {
			listed[caregiver.TeacherID] = true
		}
		assert.True(t, listed[teacherID])

		found, err := factory.Teacher.FindActiveCaregiverByAccountID(adminCtx, account.ID)
		if err != nil {
			return err
		}
		require.NotNil(t, found)
		assert.Equal(t, teacherID, found.TeacherID)
		return nil
	})
	require.NoError(t, err)
}

func TestGuestMembershipAdapter_FindsByStaffAndActiveWindow(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	factory := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)

	guest := testpkg.CreateTestGuest(t, db, "Zirkus")

	found, err := factory.Guest.FindByStaffID(ctx, guest.StaffID)
	require.NoError(t, err)
	assert.Equal(t, guest.ID, found.ID)

	_, err = factory.Guest.FindByStaffID(ctx, missingID(guest.StaffID))
	require.Error(t, err)
	assert.True(t, testpkg.IsNotFoundError(err))

	active, err := factory.Guest.FindActive(ctx)
	require.NoError(t, err)
	var listed bool
	for _, entry := range active {
		if entry.ID == guest.ID {
			listed = true
		}
	}
	assert.True(t, listed, "a guest without a start or end date is active today")
}

func TestStaffMembershipAdapter_ListRejectsUnknownFilters(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	factory := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)

	staff := testpkg.CreateTestStaff(t, db, "Filter", "Staff")

	byPerson, err := factory.Staff.List(ctx, map[string]any{"person_id": staff.PersonID})
	require.NoError(t, err)
	require.Len(t, byPerson, 1)
	assert.Equal(t, staff.ID, byPerson[0].ID)

	_, err = factory.Staff.List(ctx, map[string]any{"staff_notes": "anything"})
	require.Error(t, err, "an unsupported filter must fail loudly instead of listing everything")
}
