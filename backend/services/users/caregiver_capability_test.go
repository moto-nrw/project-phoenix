package users_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func setupCaregiverFactory(t *testing.T) (*bun.DB, *services.Factory) {
	t.Helper()

	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	factory := setupServiceFactory(t, db)
	return db, factory
}

func setupServiceFactory(t *testing.T, db *bun.DB) *services.Factory {
	t.Helper()

	factory, err := services.NewFactory(repositories.NewFactory(db), db, slog.Default())
	require.NoError(t, err)
	return factory
}

func lookupSystemRoleID(t *testing.T, db *bun.DB, name string) int64 {
	t.Helper()

	var role authModels.Role
	err := db.NewSelect().
		Model(&role).
		ModelTableExpr(`auth.roles AS "role"`).
		Where(`LOWER("role".name) = LOWER(?)`, name).
		Where(`"role".is_system = TRUE`).
		Where(`"role".tenant_id IS NULL`).
		OrderExpr(`"role".id ASC`).
		Limit(1).
		Scan(context.Background())
	require.NoError(t, err)

	return role.ID
}

func assignSystemRoleToAccount(
	t *testing.T,
	db *bun.DB,
	accountID int64,
	tenantID int64,
	roleName string,
) {
	t.Helper()

	roleID := lookupSystemRoleID(t, db, roleName)
	_, err := db.ExecContext(
		context.Background(),
		`INSERT INTO auth.account_roles (account_id, role_id, tenant_id, created_at, updated_at)
		 VALUES (?, ?, ?, NOW(), NOW())
		 ON CONFLICT (account_id, role_id, tenant_id) DO NOTHING`,
		accountID,
		roleID,
		tenantID,
	)
	require.NoError(t, err)
}

func insertPlannedSupervisor(
	t *testing.T,
	db *bun.DB,
	tenantID int64,
	staffID int64,
	groupID int64,
) *activitiesModels.SupervisorPlanned {
	t.Helper()

	supervisor := &activitiesModels.SupervisorPlanned{
		StaffID:   staffID,
		GroupID:   groupID,
		IsPrimary: true,
	}
	supervisor.SetTenantID(tenantID)

	err := db.NewInsert().
		Model(supervisor).
		ModelTableExpr(`activities.supervisors`).
		Scan(context.Background())
	require.NoError(t, err)

	return supervisor
}

func TestCaregiverCapability_EnableCreatesOperationalProfile(t *testing.T) {
	db, factory := setupCaregiverFactory(t)
	ctx := testpkg.TenantContext(1)

	account := testpkg.CreateTestAccount(t, db, "caregiver-enable")
	t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, account.ID) })
	testpkg.EnsureAccountTenant(t, db, account.ID, 1)

	state, err := factory.CaregiverCapability.EnableCaregiverCapability(
		ctx,
		account.ID,
		userModels.EnableCaregiverCapabilityInput{
			FirstName: " Ada ",
			LastName:  " Lovelace ",
			Position:  " Betreuungskraft ",
		},
	)

	require.NoError(t, err)
	require.NotNil(t, state)
	assert.Equal(t, account.ID, state.AccountID)
	assert.Equal(t, "Ada", state.FirstName)
	assert.Equal(t, "Lovelace", state.LastName)
	assert.True(t, state.HasPerson)
	assert.True(t, state.HasStaff)
	assert.True(t, state.HasTeacher)
	assert.True(t, state.HasCaregiverProfile)
	assert.True(t, state.HasUserRole)
	assert.True(t, state.IsActiveCaregiver)

	person, err := factory.Users.FindByAccountID(ctx, account.ID)
	require.NoError(t, err)
	require.NotNil(t, person)
	t.Cleanup(func() { testpkg.CleanupActivityFixtures(t, db, person.ID) })
	assert.Equal(t, "Ada", person.FirstName)
	assert.Equal(t, "Lovelace", person.LastName)

	staff, err := factory.Users.StaffRepository().FindByPersonID(ctx, person.ID)
	require.NoError(t, err)
	require.NotNil(t, staff)
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, db, "users.staff", staff.ID) })

	teacher, err := factory.Users.TeacherRepository().FindByStaffID(ctx, staff.ID)
	require.NoError(t, err)
	require.NotNil(t, teacher)
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, db, "users.teachers", teacher.ID) })
	assert.Equal(t, "Betreuungskraft", teacher.Role)
}

func TestCaregiverCapability_DisableRemovesUserRoleWithoutDeletingProfile(t *testing.T) {
	db, factory := setupCaregiverFactory(t)
	ctx := testpkg.TenantContext(1)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, db, "Disable", "Caregiver")
	t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, account.ID) })
	t.Cleanup(func() {
		testpkg.CleanupActivityFixtures(
			t,
			db,
			teacher.ID,
			teacher.Staff.ID,
			teacher.Staff.Person.ID,
		)
	})
	testpkg.EnsureAccountTenant(t, db, account.ID, 1)
	assignSystemRoleToAccount(t, db, account.ID, 1, "admin")
	assignSystemRoleToAccount(t, db, account.ID, 1, "user")

	state, err := factory.CaregiverCapability.DisableCaregiverCapability(ctx, account.ID)

	require.NoError(t, err)
	require.NotNil(t, state)
	assert.False(t, state.HasUserRole)
	assert.True(t, state.HasAdminRole)
	assert.True(t, state.HasCaregiverProfile)
	assert.False(t, state.IsActiveCaregiver)
	assert.False(t, state.DisableBlocked)

	reloaded, err := factory.CaregiverCapability.GetCaregiverCapability(ctx, account.ID)
	require.NoError(t, err)
	assert.False(t, reloaded.HasUserRole)
	assert.True(t, reloaded.HasTeacher)
}

func TestCaregiverCapability_DisableUsesTenantScopedRoles(t *testing.T) {
	db, factory := setupCaregiverFactory(t)
	ctx := testpkg.TenantContext(1)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, db, "Scoped", "Caregiver")
	t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, account.ID) })
	t.Cleanup(func() {
		testpkg.CleanupActivityFixtures(
			t,
			db,
			teacher.ID,
			teacher.Staff.ID,
			teacher.Staff.Person.ID,
		)
	})

	testpkg.EnsureAccountTenant(t, db, account.ID, 1)
	testpkg.EnsureAccountTenant(t, db, account.ID, 2)
	assignSystemRoleToAccount(t, db, account.ID, 1, "user")
	assignSystemRoleToAccount(t, db, account.ID, 2, "admin")

	state, err := factory.CaregiverCapability.GetCaregiverCapability(ctx, account.ID)
	require.NoError(t, err)
	require.NotNil(t, state)
	assert.True(t, state.HasUserRole)
	assert.False(t, state.HasAdminRole)
	assert.True(t, state.DisableBlocked)
	assert.Contains(
		t,
		state.DisableBlockers,
		"Das Konto hat keine Verwaltungsrolle und würde ohne Betreuerfähigkeit keine nutzbare Systemrolle behalten.",
	)

	var blockedErr *usersSvc.CaregiverCapabilityBlockedError
	_, err = factory.CaregiverCapability.DisableCaregiverCapability(ctx, account.ID)
	require.ErrorAs(t, err, &blockedErr)
	assert.Contains(
		t,
		blockedErr.Reasons,
		"Das Konto hat keine Verwaltungsrolle und würde ohne Betreuerfähigkeit keine nutzbare Systemrolle behalten.",
	)
}

func TestCaregiverCapability_DisableReturnsDetailedBlockers(t *testing.T) {
	db, factory := setupCaregiverFactory(t)
	ctx := testpkg.TenantContext(1)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, db, "Blocked", "Caregiver")
	t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, account.ID) })
	t.Cleanup(func() {
		testpkg.CleanupActivityFixtures(
			t,
			db,
			teacher.ID,
			teacher.Staff.ID,
			teacher.Staff.Person.ID,
		)
	})
	testpkg.EnsureAccountTenant(t, db, account.ID, 1)
	assignSystemRoleToAccount(t, db, account.ID, 1, "admin")
	assignSystemRoleToAccount(t, db, account.ID, 1, "user")

	group := testpkg.CreateTestEducationGroupForTenant(t, db, 1, "Blocked Group")
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, db, "education.groups", group.ID) })
	groupTeacher := testpkg.CreateTestGroupTeacher(t, db, group.ID, teacher.ID)
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "education.group_teacher", groupTeacher.ID)
	})
	otherTeacher, otherAccount := testpkg.CreateTestTeacherWithAccount(
		t,
		db,
		"Second",
		"Leader",
	)
	t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, otherAccount.ID) })
	t.Cleanup(func() {
		testpkg.CleanupActivityFixtures(
			t,
			db,
			otherTeacher.ID,
			otherTeacher.Staff.ID,
			otherTeacher.Staff.Person.ID,
		)
	})
	otherGroupTeacher := testpkg.CreateTestGroupTeacher(t, db, group.ID, otherTeacher.ID)
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "education.group_teacher", otherGroupTeacher.ID)
	})

	room := testpkg.CreateTestRoomForTenant(t, db, 1, "Blocked Room")
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, db, "facilities.rooms", room.ID) })
	activityGroup := testpkg.CreateTestActivityGroupForTenant(t, db, 1, "Blocked Activity")
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, db, "activities.groups", activityGroup.ID) })
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activityGroup.ID, room.ID)
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, db, "active.groups", activeGroup.ID) })

	supervision := testpkg.CreateTestGroupSupervisor(
		t,
		db,
		teacher.Staff.ID,
		activeGroup.ID,
		"lead",
	)
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "active.group_supervisors", supervision.ID)
	})

	substitution := testpkg.CreateTestGroupSubstitution(
		t,
		db,
		group.ID,
		nil,
		teacher.Staff.ID,
		time.Now().Add(-24*time.Hour),
		time.Now().Add(24*time.Hour),
	)
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "education.group_substitution", substitution.ID)
	})

	plannedSupervisor := insertPlannedSupervisor(
		t,
		db,
		1,
		teacher.Staff.ID,
		activityGroup.ID,
	)
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "activities.supervisors", plannedSupervisor.ID)
	})

	state, err := factory.CaregiverCapability.GetCaregiverCapability(ctx, account.ID)
	require.NoError(t, err)
	require.NotNil(t, state)
	assert.True(t, state.DisableBlocked)
	assert.Len(t, state.DisableBlockers, 4)
	assert.Len(t, state.ActiveSupervisions, 1)
	assert.Len(t, state.ActiveSubstitutions, 1)
	assert.Len(t, state.ActivitySupervisions, 1)
	assert.Len(t, state.GroupAssignments, 1)
	assert.Equal(t, teacher.ID, state.GroupAssignments[0].TeacherID)
	assert.ElementsMatch(
		t,
		[]int64{teacher.ID, otherTeacher.ID},
		state.GroupAssignments[0].TeacherIDs,
	)

	var blockedErr *usersSvc.CaregiverCapabilityBlockedError
	_, err = factory.CaregiverCapability.DisableCaregiverCapability(ctx, account.ID)
	require.ErrorAs(t, err, &blockedErr)
	assert.Equal(t, state.DisableBlockers, blockedErr.Reasons)
}

func TestCaregiverCapability_RequiresTenantContextAndMapping(t *testing.T) {
	db, factory := setupCaregiverFactory(t)
	var requestedTenantID int64 = 1
	var mappedTenantID int64 = 2

	account := testpkg.CreateTestAccount(t, db, "caregiver-mapping")
	t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, account.ID) })

	_, err := factory.CaregiverCapability.GetCaregiverCapability(context.Background(), account.ID)
	require.Error(t, err)
	assert.ErrorContains(t, err, "tenant context is required")

	testpkg.EnsureAccountTenant(t, db, account.ID, mappedTenantID)

	var mappingErr *usersSvc.AccountNotAssignedToTenantError
	_, err = factory.CaregiverCapability.GetCaregiverCapability(
		testpkg.TenantContext(requestedTenantID),
		account.ID,
	)
	require.ErrorAs(t, err, &mappingErr)
	assert.Equal(t, account.ID, mappingErr.AccountID)
	assert.Equal(t, requestedTenantID, mappingErr.TenantID)
}

func TestCaregiverCapability_RejectsInvalidAccountIDs(t *testing.T) {
	_, factory := setupCaregiverFactory(t)
	ctx := testpkg.TenantContext(1)

	_, err := factory.CaregiverCapability.EnableCaregiverCapability(
		ctx,
		0,
		userModels.EnableCaregiverCapabilityInput{},
	)
	require.Error(t, err)
	assert.ErrorContains(t, err, "account ID is required")

	_, err = factory.CaregiverCapability.DisableCaregiverCapability(ctx, 0)
	require.Error(t, err)
	assert.ErrorContains(t, err, "account ID is required")
}

func TestCaregiverCapability_EnableRequiresNamesWhenProfileDoesNotExist(t *testing.T) {
	db, factory := setupCaregiverFactory(t)
	ctx := testpkg.TenantContext(1)

	account := testpkg.CreateTestAccount(t, db, "caregiver-missing-names")
	t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, account.ID) })
	testpkg.EnsureAccountTenant(t, db, account.ID, 1)

	_, err := factory.CaregiverCapability.EnableCaregiverCapability(
		ctx,
		account.ID,
		userModels.EnableCaregiverCapabilityInput{FirstName: "Ada"},
	)

	require.Error(t, err)
	assert.ErrorContains(
		t,
		err,
		"first_name and last_name are required when the account has no person profile",
	)
}

func TestCaregiverCapability_GetSupportsPersonWithoutStaffProfile(t *testing.T) {
	db, factory := setupCaregiverFactory(t)
	ctx := testpkg.TenantContext(1)

	account := testpkg.CreateTestAccount(t, db, "caregiver-person-only")
	t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, account.ID) })
	testpkg.EnsureAccountTenant(t, db, account.ID, 1)

	person := testpkg.CreateTestPersonWithAccountID(
		t,
		db,
		"Profile",
		"Only",
		account.ID,
	)
	t.Cleanup(func() { testpkg.CleanupActivityFixtures(t, db, person.ID) })

	state, err := factory.CaregiverCapability.GetCaregiverCapability(ctx, account.ID)

	require.NoError(t, err)
	require.NotNil(t, state)
	assert.True(t, state.HasPerson)
	assert.False(t, state.HasStaff)
	assert.False(t, state.HasTeacher)
	assert.False(t, state.HasCaregiverProfile)
	assert.False(t, state.IsActiveCaregiver)
}

func TestCaregiverCapability_DisableReturnsStateWhenUserRoleIsAlreadyMissing(t *testing.T) {
	db, factory := setupCaregiverFactory(t)
	ctx := testpkg.TenantContext(1)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, db, "Admin", "ProfileOnly")
	t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, account.ID) })
	t.Cleanup(func() {
		testpkg.CleanupActivityFixtures(
			t,
			db,
			teacher.ID,
			teacher.Staff.ID,
			teacher.Staff.Person.ID,
		)
	})
	testpkg.EnsureAccountTenant(t, db, account.ID, 1)
	assignSystemRoleToAccount(t, db, account.ID, 1, "admin")

	state, err := factory.CaregiverCapability.DisableCaregiverCapability(ctx, account.ID)

	require.NoError(t, err)
	require.NotNil(t, state)
	assert.True(t, state.HasAdminRole)
	assert.False(t, state.HasUserRole)
	assert.True(t, state.HasCaregiverProfile)
	assert.False(t, state.IsActiveCaregiver)
}

func TestCaregiverDirectory_ListAndFindOnlyActiveCaregivers(t *testing.T) {
	db, factory := setupCaregiverFactory(t)
	ctx := testpkg.TenantContext(1)

	activeTeacher, activeAccount := testpkg.CreateTestTeacherWithAccount(t, db, "Active", "Caregiver")
	t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, activeAccount.ID) })
	t.Cleanup(func() {
		testpkg.CleanupActivityFixtures(
			t,
			db,
			activeTeacher.ID,
			activeTeacher.Staff.ID,
			activeTeacher.Staff.Person.ID,
		)
	})
	testpkg.EnsureAccountTenant(t, db, activeAccount.ID, 1)
	assignSystemRoleToAccount(t, db, activeAccount.ID, 1, "user")

	adminOnlyTeacher, adminOnlyAccount := testpkg.CreateTestTeacherWithAccount(t, db, "Admin", "Only")
	t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, adminOnlyAccount.ID) })
	t.Cleanup(func() {
		testpkg.CleanupActivityFixtures(
			t,
			db,
			adminOnlyTeacher.ID,
			adminOnlyTeacher.Staff.ID,
			adminOnlyTeacher.Staff.Person.ID,
		)
	})
	testpkg.EnsureAccountTenant(t, db, adminOnlyAccount.ID, 1)
	assignSystemRoleToAccount(t, db, adminOnlyAccount.ID, 1, "admin")

	inactiveTeacher, inactiveAccount := testpkg.CreateTestTeacherWithAccount(t, db, "Inactive", "Caregiver")
	t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, inactiveAccount.ID) })
	t.Cleanup(func() {
		testpkg.CleanupActivityFixtures(
			t,
			db,
			inactiveTeacher.ID,
			inactiveTeacher.Staff.ID,
			inactiveTeacher.Staff.Person.ID,
		)
	})
	testpkg.EnsureAccountTenant(t, db, inactiveAccount.ID, 1)
	assignSystemRoleToAccount(t, db, inactiveAccount.ID, 1, "user")

	_, err := db.ExecContext(context.Background(), `UPDATE auth.accounts SET active = false WHERE id = ?`, inactiveAccount.ID)
	require.NoError(t, err)

	directory, err := usersSvc.CaregiverDirectoryFromPersonService(factory.Users)
	require.NoError(t, err)

	caregivers, err := directory.ListActiveCaregivers(ctx)
	require.NoError(t, err)
	require.Len(t, caregivers, 1)
	assert.Equal(t, activeAccount.ID, caregivers[0].AccountID)
	assert.Equal(t, "Active Caregiver", caregivers[0].FullName())

	found, err := directory.FindActiveCaregiverByAccountID(ctx, activeAccount.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, activeTeacher.ID, found.TeacherID)

	missing, err := directory.FindActiveCaregiverByAccountID(ctx, adminOnlyAccount.ID)
	require.NoError(t, err)
	assert.Nil(t, missing)

	inactive, err := directory.FindActiveCaregiverByAccountID(ctx, inactiveAccount.ID)
	require.NoError(t, err)
	assert.Nil(t, inactive)
}
