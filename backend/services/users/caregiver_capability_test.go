package users_test

import (
	"context"
	"database/sql"
	"log/slog"
	"testing"
	"time"

	jwtPkg "github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
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

	factory := setupServiceFactory(t, db)
	return db, factory
}

func setupServiceFactory(t *testing.T, db *bun.DB) *services.Factory {
	t.Helper()

	factory, err := services.NewFactory(repositories.NewFactory(db), db, slog.Default())
	require.NoError(t, err)
	return factory
}

func createTestTeacherWithAccountForTenant(
	t *testing.T,
	db *bun.DB,
	tenantID int64,
	firstName string,
	lastName string,
) (*userModels.Teacher, *authModels.Account) {
	t.Helper()

	testpkg.EnsureTestTenant(t, db, tenantID)

	account := testpkg.CreateTestAccount(t, db, firstName+"."+lastName)
	person := testpkg.CreateTestPersonForTenant(t, db, tenantID, firstName, lastName)
	person.AccountID = &account.ID
	_, err := db.ExecContext(
		context.Background(),
		`UPDATE users.persons SET account_id = ? WHERE tenant_id = ? AND id = ?`,
		account.ID,
		tenantID,
		person.ID,
	)
	require.NoError(t, err)

	staff := &userModels.Staff{PersonID: person.ID}
	staff.SetTenantID(tenantID)
	err = db.NewInsert().
		Model(staff).
		ModelTableExpr(`users.staff`).
		Scan(context.Background())
	require.NoError(t, err)
	staff.Person = person

	teacher := &userModels.Teacher{StaffID: staff.ID}
	teacher.SetTenantID(tenantID)
	err = db.NewInsert().
		Model(teacher).
		ModelTableExpr(`users.teachers`).
		Scan(context.Background())
	require.NoError(t, err)
	teacher.Staff = staff

	return teacher, account
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

func ensureSystemRoleExists(t *testing.T, db *bun.DB, name string) {
	t.Helper()

	var role authModels.Role
	err := db.NewSelect().
		Model(&role).
		ModelTableExpr(`auth.roles AS "role"`).
		Where(`LOWER("role".name) = LOWER(?)`, name).
		Where(`"role".is_system = TRUE`).
		Where(`"role".tenant_id IS NULL`).
		Limit(1).
		Scan(context.Background())
	if err == nil {
		return
	}
	if err != nil && err != sql.ErrNoRows {
		require.NoError(t, err)
	}

	role = authModels.Role{
		Name:        name,
		Description: "Test system role: " + name,
		IsSystem:    true,
	}
	err = db.NewInsert().
		Model(&role).
		ModelTableExpr(`auth.roles`).
		Scan(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, db, "auth.roles", role.ID) })
}

func loadLatestAuthEvent(
	t *testing.T,
	db *bun.DB,
	accountID int64,
	eventType string,
) *auditModels.AuthEvent {
	t.Helper()

	var event auditModels.AuthEvent
	err := db.NewSelect().
		Model(&event).
		ModelTableExpr(`audit.auth_events AS "auth_event"`).
		Where(`"auth_event".account_id = ?`, accountID).
		Where(`"auth_event".event_type = ?`, eventType).
		OrderExpr(`"auth_event".created_at DESC`).
		Limit(1).
		Scan(context.Background())
	require.NoError(t, err)

	return &event
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

func accountHasSystemRole(
	t *testing.T,
	db *bun.DB,
	accountID int64,
	tenantID int64,
	roleName string,
) bool {
	t.Helper()

	var count int
	err := db.NewSelect().
		TableExpr(`auth.account_roles AS "ar"`).
		Join(`INNER JOIN auth.roles AS "role" ON "role".id = "ar".role_id`).
		ColumnExpr(`COUNT(*)`).
		Where(`"ar".account_id = ?`, accountID).
		Where(`"ar".tenant_id = ?`, tenantID).
		Where(`"role".is_system = TRUE`).
		Where(`"role".tenant_id IS NULL`).
		Where(`LOWER("role".name) = LOWER(?)`, roleName).
		Scan(context.Background(), &count)
	require.NoError(t, err)

	return count > 0
}

func setAccountTenantStatus(
	t *testing.T,
	db *bun.DB,
	accountID int64,
	tenantID int64,
	status string,
) {
	t.Helper()

	_, err := db.ExecContext(
		context.Background(),
		`UPDATE auth.account_tenants
		 SET status = ?, deactivated_at = CASE WHEN ? = 'inactive' THEN NOW() ELSE NULL END
		 WHERE account_id = ? AND tenant_id = ?`,
		status,
		status,
		accountID,
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
	ctx := context.WithValue(
		testpkg.Ctx(t),
		jwtPkg.CtxClaims,
		jwtPkg.AppClaims{ID: 91, Scope: "tenant", TenantID: testpkg.Tenant(t)},
	)

	account := testpkg.CreateTestAccount(t, db, "caregiver-enable")
	t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, account.ID) })
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

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

	staff, err := repositories.NewFactory(db).Staff.FindByPersonID(ctx, person.ID)
	require.NoError(t, err)
	require.NotNil(t, staff)
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, db, "users.staff", staff.ID) })

	teacher, err := repositories.NewFactory(db).Teacher.FindByStaffID(ctx, staff.ID)
	require.NoError(t, err)
	require.NotNil(t, teacher)
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, db, "users.teachers", teacher.ID) })
	assert.Equal(t, "Betreuungskraft", teacher.Role)

	event := loadLatestAuthEvent(
		t,
		db,
		account.ID,
		auditModels.EventTypeCaregiverCapabilityEnabled,
	)
	assert.Equal(t, "0.0.0.0", event.IPAddress)
	assert.Equal(t, float64(91), event.Metadata["actor_account_id"])
	assert.Equal(t, "tenant", event.Metadata["actor_scope"])
	assert.Equal(t, float64(testpkg.Tenant(t)), event.Metadata["tenant_id"])
	assert.Equal(t, true, event.Metadata["person_created"])
	assert.Equal(t, true, event.Metadata["staff_created"])
	assert.Equal(t, true, event.Metadata["teacher_created"])
	assert.Equal(t, "Betreuungskraft", event.Metadata["requested_position"])

	before, ok := event.Metadata["before"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, before["has_person"])
	assert.Equal(t, false, before["has_user_role"])

	after, ok := event.Metadata["after"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, true, after["has_person"])
	assert.Equal(t, true, after["has_user_role"])
	assert.Equal(t, true, after["has_teacher"])
}

func TestCaregiverCapability_DisableRemovesUserRoleWithoutDeletingProfile(t *testing.T) {
	db, factory := setupCaregiverFactory(t)
	ctx := context.WithValue(
		testpkg.Ctx(t),
		jwtPkg.CtxClaims,
		jwtPkg.AppClaims{ID: 92, Scope: "tenant", TenantID: testpkg.Tenant(t)},
	)

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
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))
	assignSystemRoleToAccount(t, db, account.ID, testpkg.Tenant(t), "admin")
	assignSystemRoleToAccount(t, db, account.ID, testpkg.Tenant(t), "user")

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

	event := loadLatestAuthEvent(
		t,
		db,
		account.ID,
		auditModels.EventTypeCaregiverCapabilityDisabled,
	)
	assert.Equal(t, "0.0.0.0", event.IPAddress)
	assert.Equal(t, float64(92), event.Metadata["actor_account_id"])
	assert.Equal(t, "tenant", event.Metadata["actor_scope"])
	assert.Equal(t, float64(testpkg.Tenant(t)), event.Metadata["tenant_id"])

	before, ok := event.Metadata["before"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, true, before["has_user_role"])
	assert.Equal(t, true, before["has_teacher"])

	after, ok := event.Metadata["after"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, after["has_user_role"])
	assert.Equal(t, true, after["has_teacher"])
}

func TestCaregiverCapability_DisableUsesTenantScopedRoles(t *testing.T) {
	db, factory := setupCaregiverFactory(t)
	ctx := testpkg.Ctx(t)

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

	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))
	testpkg.EnsureAccountTenant(t, db, account.ID, 2)
	assignSystemRoleToAccount(t, db, account.ID, testpkg.Tenant(t), "user")
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
		userModels.CaregiverCapabilityBlockerMissingUsableRole,
	)

	var blockedErr *usersSvc.CaregiverCapabilityBlockedError
	_, err = factory.CaregiverCapability.DisableCaregiverCapability(ctx, account.ID)
	require.ErrorAs(t, err, &blockedErr)
	assert.Contains(
		t,
		blockedErr.Reasons,
		userModels.CaregiverCapabilityBlockerMissingUsableRole,
	)
}

func TestCaregiverCapability_DisableAllowsCustomTenantRoleToRemain(t *testing.T) {
	db, factory := setupCaregiverFactory(t)
	ctx := testpkg.Ctx(t)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, db, "Custom", "Role")
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

	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))
	assignSystemRoleToAccount(t, db, account.ID, testpkg.Tenant(t), "user")

	tenantID := teacher.GetTenantID()
	customRole := &authModels.Role{
		Name:        "coordinator",
		Description: "Tenant-scoped coordinator role",
		IsSystem:    false,
		TenantID:    &tenantID,
	}
	err := db.NewInsert().
		Model(customRole).
		ModelTableExpr(`auth.roles`).
		Scan(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, db, "auth.roles", customRole.ID) })

	_, err = db.ExecContext(
		context.Background(),
		`INSERT INTO auth.account_roles (account_id, role_id, tenant_id, created_at, updated_at)
		 VALUES (?, ?, ?, NOW(), NOW())
		 ON CONFLICT (account_id, role_id, tenant_id) DO NOTHING`,
		account.ID,
		customRole.ID,
		tenantID,
	)
	require.NoError(t, err)

	state, err := factory.CaregiverCapability.GetCaregiverCapability(ctx, account.ID)
	require.NoError(t, err)
	require.NotNil(t, state)
	assert.False(t, state.DisableBlocked)
	assert.NotContains(
		t,
		state.DisableBlockers,
		userModels.CaregiverCapabilityBlockerMissingUsableRole,
	)

	state, err = factory.CaregiverCapability.DisableCaregiverCapability(ctx, account.ID)
	require.NoError(t, err)
	require.NotNil(t, state)
	assert.False(t, state.HasUserRole)
	assert.False(t, state.IsActiveCaregiver)
	assert.False(t, accountHasSystemRole(t, db, account.ID, testpkg.Tenant(t), "user"))
}

func TestCaregiverCapability_DisableReturnsDetailedBlockers(t *testing.T) {
	db, factory := setupCaregiverFactory(t)
	ctx := testpkg.Ctx(t)

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
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))
	assignSystemRoleToAccount(t, db, account.ID, testpkg.Tenant(t), "admin")
	assignSystemRoleToAccount(t, db, account.ID, testpkg.Tenant(t), "user")

	group := testpkg.CreateTestEducationGroupForTenant(t, db, testpkg.Tenant(t), "Blocked Group")
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

	room := testpkg.CreateTestRoomForTenant(t, db, testpkg.Tenant(t), "Blocked Room")
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, db, "facilities.rooms", room.ID) })
	activityGroup := testpkg.CreateTestActivityGroupForTenant(t, db, testpkg.Tenant(t), "Blocked Activity")
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
		timezone.TodayDate().AddDays(-1),
		timezone.TodayDate().AddDays(1),
	)
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "education.group_substitution", substitution.ID)
	})

	plannedSupervisor := insertPlannedSupervisor(
		t,
		db,
		testpkg.Tenant(t),
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

func TestCaregiverCapability_DisableWaitsForConcurrentBindings(t *testing.T) {
	db, factory := setupCaregiverFactory(t)
	ctx := testpkg.Ctx(t)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, db, "Concurrent", "Disable")
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
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))
	assignSystemRoleToAccount(t, db, account.ID, testpkg.Tenant(t), "admin")
	assignSystemRoleToAccount(t, db, account.ID, testpkg.Tenant(t), "user")

	group := testpkg.CreateTestEducationGroupForTenant(t, db, testpkg.Tenant(t), "Concurrent Group")
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, db, "education.groups", group.ID) })

	tx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)

	pendingRelation := &educationModels.GroupTeacher{
		GroupID:   group.ID,
		TeacherID: teacher.ID,
	}
	pendingRelation.SetTenantID(testpkg.Tenant(t))

	err = tx.NewInsert().
		Model(pendingRelation).
		ModelTableExpr(`education.group_teacher`).
		Scan(context.Background())
	require.NoError(t, err)

	resultCh := make(chan error, 1)
	go func() {
		_, disableErr := factory.CaregiverCapability.DisableCaregiverCapability(ctx, account.ID)
		resultCh <- disableErr
	}()

	select {
	case err := <-resultCh:
		t.Fatalf("disable completed before concurrent binding committed: %v", err)
	case <-time.After(150 * time.Millisecond):
	}

	require.NoError(t, tx.Commit())
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "education.group_teacher", pendingRelation.ID)
	})

	var blockedErr *usersSvc.CaregiverCapabilityBlockedError
	err = <-resultCh
	require.ErrorAs(t, err, &blockedErr)
	require.Len(t, blockedErr.Reasons, 1)
	assert.Equal(t, userModels.CaregiverCapabilityBlockerGroupAssignments, blockedErr.Reasons[0])

	state, err := factory.CaregiverCapability.GetCaregiverCapability(ctx, account.ID)
	require.NoError(t, err)
	assert.True(t, state.HasUserRole)
	assert.True(t, state.DisableBlocked)
	assert.Len(t, state.GroupAssignments, 1)
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
	ctx := testpkg.Ctx(t)

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
	ctx := testpkg.Ctx(t)

	account := testpkg.CreateTestAccount(t, db, "caregiver-missing-names")
	t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, account.ID) })
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

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
	ctx := testpkg.Ctx(t)

	account := testpkg.CreateTestAccount(t, db, "caregiver-person-only")
	t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, account.ID) })
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

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
	ctx := testpkg.Ctx(t)

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
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))
	assignSystemRoleToAccount(t, db, account.ID, testpkg.Tenant(t), "admin")

	state, err := factory.CaregiverCapability.DisableCaregiverCapability(ctx, account.ID)

	require.NoError(t, err)
	require.NotNil(t, state)
	assert.True(t, state.HasAdminRole)
	assert.False(t, state.HasUserRole)
	assert.True(t, state.HasCaregiverProfile)
	assert.False(t, state.IsActiveCaregiver)
}

func TestCaregiverCapability_DisableRemovesLegacyTeacherRoleWhenAdminRemains(t *testing.T) {
	db, factory := setupCaregiverFactory(t)
	ctx := testpkg.Ctx(t)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, db, "Legacy", "Disable")
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
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))
	ensureSystemRoleExists(t, db, "teacher")
	assignSystemRoleToAccount(t, db, account.ID, testpkg.Tenant(t), "admin")
	assignSystemRoleToAccount(t, db, account.ID, testpkg.Tenant(t), "teacher")

	state, err := factory.CaregiverCapability.DisableCaregiverCapability(ctx, account.ID)

	require.NoError(t, err)
	require.NotNil(t, state)
	assert.True(t, state.HasAdminRole)
	assert.False(t, state.HasUserRole)
	assert.True(t, state.HasCaregiverProfile)
	assert.False(t, state.IsActiveCaregiver)
	assert.False(t, accountHasSystemRole(t, db, account.ID, testpkg.Tenant(t), "teacher"))
}

func TestCaregiverCapability_DisableBlocksLegacyTeacherOnlyAccount(t *testing.T) {
	db, factory := setupCaregiverFactory(t)
	ctx := testpkg.Ctx(t)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, db, "Legacy", "TeacherOnly")
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
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))
	ensureSystemRoleExists(t, db, "teacher")
	assignSystemRoleToAccount(t, db, account.ID, testpkg.Tenant(t), "teacher")

	state, err := factory.CaregiverCapability.GetCaregiverCapability(ctx, account.ID)
	require.NoError(t, err)
	require.NotNil(t, state)
	assert.True(t, state.DisableBlocked)
	assert.Contains(
		t,
		state.DisableBlockers,
		userModels.CaregiverCapabilityBlockerMissingUsableRole,
	)

	var blockedErr *usersSvc.CaregiverCapabilityBlockedError
	_, err = factory.CaregiverCapability.DisableCaregiverCapability(ctx, account.ID)
	require.ErrorAs(t, err, &blockedErr)
	assert.Contains(
		t,
		blockedErr.Reasons,
		userModels.CaregiverCapabilityBlockerMissingUsableRole,
	)
	assert.True(t, accountHasSystemRole(t, db, account.ID, testpkg.Tenant(t), "teacher"))
}

func TestCaregiverDirectory_ListAndFindActiveCaregiversIncludingLegacyTeacherRole(t *testing.T) {
	db, factory := setupCaregiverFactory(t)
	tenantID := testpkg.UniqueTestTenantID(t)
	ctx := testpkg.TenantContext(tenantID)

	activeTeacher, activeAccount := createTestTeacherWithAccountForTenant(t, db, tenantID, "Active", "Caregiver")
	t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, activeAccount.ID) })
	t.Cleanup(func() {
		testpkg.CleanupActivityFixturesForTenant(
			t,
			db,
			tenantID,
			activeTeacher.ID,
			activeTeacher.Staff.ID,
			activeTeacher.Staff.Person.ID,
		)
	})
	testpkg.EnsureAccountTenant(t, db, activeAccount.ID, tenantID)
	assignSystemRoleToAccount(t, db, activeAccount.ID, tenantID, "user")

	legacyTeacherRoleTeacher, legacyTeacherRoleAccount := createTestTeacherWithAccountForTenant(t, db, tenantID, "Legacy", "Teacher")
	t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, legacyTeacherRoleAccount.ID) })
	t.Cleanup(func() {
		testpkg.CleanupActivityFixturesForTenant(
			t,
			db,
			tenantID,
			legacyTeacherRoleTeacher.ID,
			legacyTeacherRoleTeacher.Staff.ID,
			legacyTeacherRoleTeacher.Staff.Person.ID,
		)
	})
	testpkg.EnsureAccountTenant(t, db, legacyTeacherRoleAccount.ID, tenantID)
	ensureSystemRoleExists(t, db, "teacher")
	assignSystemRoleToAccount(t, db, legacyTeacherRoleAccount.ID, tenantID, "teacher")

	adminOnlyTeacher, adminOnlyAccount := createTestTeacherWithAccountForTenant(t, db, tenantID, "Admin", "Only")
	t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, adminOnlyAccount.ID) })
	t.Cleanup(func() {
		testpkg.CleanupActivityFixturesForTenant(
			t,
			db,
			tenantID,
			adminOnlyTeacher.ID,
			adminOnlyTeacher.Staff.ID,
			adminOnlyTeacher.Staff.Person.ID,
		)
	})
	testpkg.EnsureAccountTenant(t, db, adminOnlyAccount.ID, tenantID)
	assignSystemRoleToAccount(t, db, adminOnlyAccount.ID, tenantID, "admin")

	inactiveTeacher, inactiveAccount := createTestTeacherWithAccountForTenant(t, db, tenantID, "Inactive", "Caregiver")
	t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, inactiveAccount.ID) })
	t.Cleanup(func() {
		testpkg.CleanupActivityFixturesForTenant(
			t,
			db,
			tenantID,
			inactiveTeacher.ID,
			inactiveTeacher.Staff.ID,
			inactiveTeacher.Staff.Person.ID,
		)
	})
	testpkg.EnsureAccountTenant(t, db, inactiveAccount.ID, tenantID)
	assignSystemRoleToAccount(t, db, inactiveAccount.ID, tenantID, "user")

	_, err := db.ExecContext(context.Background(), `UPDATE auth.accounts SET active = false WHERE id = ?`, inactiveAccount.ID)
	require.NoError(t, err)

	inactiveMembershipTeacher, inactiveMembershipAccount := createTestTeacherWithAccountForTenant(
		t,
		db,
		tenantID,
		"Former",
		"Member",
	)
	t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, inactiveMembershipAccount.ID) })
	t.Cleanup(func() {
		testpkg.CleanupActivityFixturesForTenant(
			t,
			db,
			tenantID,
			inactiveMembershipTeacher.ID,
			inactiveMembershipTeacher.Staff.ID,
			inactiveMembershipTeacher.Staff.Person.ID,
		)
	})
	testpkg.EnsureAccountTenant(t, db, inactiveMembershipAccount.ID, tenantID)
	assignSystemRoleToAccount(t, db, inactiveMembershipAccount.ID, tenantID, "user")
	setAccountTenantStatus(
		t,
		db,
		inactiveMembershipAccount.ID,
		tenantID,
		authModels.AccountTenantStatusInactive,
	)

	directory, err := usersSvc.CaregiverDirectoryFromPersonService(factory.Users)
	require.NoError(t, err)

	caregivers, err := directory.ListActiveCaregivers(ctx)
	require.NoError(t, err)
	require.Len(t, caregivers, 2)

	accountIDs := []int64{caregivers[0].AccountID, caregivers[1].AccountID}
	assert.ElementsMatch(t, []int64{activeAccount.ID, legacyTeacherRoleAccount.ID}, accountIDs)

	found, err := directory.FindActiveCaregiverByAccountID(ctx, activeAccount.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, activeTeacher.ID, found.TeacherID)

	legacyFound, err := directory.FindActiveCaregiverByAccountID(ctx, legacyTeacherRoleAccount.ID)
	require.NoError(t, err)
	require.NotNil(t, legacyFound)
	assert.Equal(t, legacyTeacherRoleTeacher.ID, legacyFound.TeacherID)

	missing, err := directory.FindActiveCaregiverByAccountID(ctx, adminOnlyAccount.ID)
	require.NoError(t, err)
	assert.Nil(t, missing)

	inactive, err := directory.FindActiveCaregiverByAccountID(ctx, inactiveAccount.ID)
	require.NoError(t, err)
	assert.Nil(t, inactive)

	inactiveMembership, err := directory.FindActiveCaregiverByAccountID(ctx, inactiveMembershipAccount.ID)
	require.NoError(t, err)
	assert.Nil(t, inactiveMembership)
}

func TestCaregiverDirectory_ExcludesTenantScopedUserRole(t *testing.T) {
	db, factory := setupCaregiverFactory(t)
	tenantID := testpkg.UniqueTestTenantID(t)
	ctx := testpkg.TenantContext(tenantID)

	teacher, account := createTestTeacherWithAccountForTenant(t, db, tenantID, "Tenant", "UserRole")
	t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, account.ID) })
	t.Cleanup(func() {
		testpkg.CleanupActivityFixturesForTenant(
			t,
			db,
			tenantID,
			teacher.ID,
			teacher.Staff.ID,
			teacher.Staff.Person.ID,
		)
	})
	testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)

	customUserRole := &authModels.Role{
		Name:        "user",
		Description: "Tenant-scoped custom user role",
		IsSystem:    false,
		TenantID:    &tenantID,
	}
	err := db.NewInsert().
		Model(customUserRole).
		ModelTableExpr(`auth.roles`).
		Scan(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, db, "auth.roles", customUserRole.ID) })

	_, err = db.ExecContext(
		context.Background(),
		`INSERT INTO auth.account_roles (account_id, role_id, tenant_id, created_at, updated_at)
		 VALUES (?, ?, ?, NOW(), NOW())
		 ON CONFLICT (account_id, role_id, tenant_id) DO NOTHING`,
		account.ID,
		customUserRole.ID,
		tenantID,
	)
	require.NoError(t, err)

	directory, err := usersSvc.CaregiverDirectoryFromPersonService(factory.Users)
	require.NoError(t, err)

	caregivers, err := directory.ListActiveCaregivers(ctx)
	require.NoError(t, err)
	assert.Empty(t, caregivers)

	found, err := directory.FindActiveCaregiverByAccountID(ctx, account.ID)
	require.NoError(t, err)
	assert.Nil(t, found)
}
