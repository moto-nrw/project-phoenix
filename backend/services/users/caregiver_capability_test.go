package users_test

import (
	"context"
	"database/sql"
	"errors"
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
	// A system role has no tenant, so the row this test had to create is
	// shared state for every other test in the binary (#2419).
	t.Cleanup(func() {
		_, _ = db.NewDelete().TableExpr("auth.account_roles").Where("role_id = ?", role.ID).
			Exec(context.Background())
		_, _ = db.NewDelete().TableExpr("auth.roles").Where("id = ?", role.ID).
			Exec(context.Background())
	})
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
	t.Parallel()

	db, factory := setupCaregiverFactory(t)
	ctx := context.WithValue(
		testpkg.Ctx(t),
		jwtPkg.CtxClaims,
		jwtPkg.AppClaims{ID: 91, Scope: "tenant", TenantID: testpkg.Tenant(t)},
	)

	account := testpkg.CreateTestAccount(t, db, "caregiver-enable")
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
	assert.Equal(t, "Ada", person.FirstName)
	assert.Equal(t, "Lovelace", person.LastName)

	staff, err := repositories.NewFactory(db).Staff.FindByPersonID(ctx, person.ID)
	require.NoError(t, err)
	require.NotNil(t, staff)

	teacher, err := repositories.NewFactory(db).Teacher.FindByStaffID(ctx, staff.ID)
	require.NoError(t, err)
	require.NotNil(t, teacher)
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

// Deliberately NOT parallel: assigning a SYSTEM role creates a row in
// auth.roles, whose name is unique across the whole clone (idx_roles_name_system
// has no tenant in it). Two tests inserting the same system role collide.
func TestCaregiverCapability_DisableRemovesUserRoleWithoutDeletingProfile(t *testing.T) {
	db, factory := setupCaregiverFactory(t)
	ctx := context.WithValue(
		testpkg.Ctx(t),
		jwtPkg.CtxClaims,
		jwtPkg.AppClaims{ID: 92, Scope: "tenant", TenantID: testpkg.Tenant(t)},
	)

	_, account := testpkg.CreateTestTeacherWithAccount(t, db, "Disable", "Caregiver")
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

// The nested request transaction verifies that disable retains its
// independently committed retry boundary.
func TestCaregiverCapability_DisableCommitsIndependentlyOfAmbientTransaction(t *testing.T) {
	t.Parallel()

	db, factory := setupCaregiverFactory(t)
	tenantID := testpkg.Tenant(t)
	ctx := testpkg.Ctx(t)

	_, account := testpkg.CreateTestTeacherWithAccount(t, db, "Independent", "Disable")
	testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)
	assignSystemRoleToAccount(t, db, account.ID, tenantID, "admin")
	assignSystemRoleToAccount(t, db, account.ID, tenantID, "user")

	outerRollback := errors.New("rollback outer request transaction")
	err := testpkg.WithTenantTx(t, ctx, db, tenantID, func(outerCtx context.Context, _ bun.Tx) error {
		state, disableErr := factory.CaregiverCapability.DisableCaregiverCapability(outerCtx, account.ID)
		require.NoError(t, disableErr)
		assert.False(t, state.HasUserRole)
		return outerRollback
	})
	require.ErrorIs(t, err, outerRollback)

	state, err := factory.CaregiverCapability.GetCaregiverCapability(testpkg.Ctx(t), account.ID)
	require.NoError(t, err)
	assert.False(t, state.HasUserRole)
}

// Deliberately NOT parallel: assigning a SYSTEM role creates a row in
// auth.roles, whose name is unique across the whole clone (idx_roles_name_system
// has no tenant in it). Two tests inserting the same system role collide.
func TestCaregiverCapability_DisableUsesTenantScopedRoles(t *testing.T) {
	db, factory := setupCaregiverFactory(t)
	ctx := testpkg.Ctx(t)

	_, account := testpkg.CreateTestTeacherWithAccount(t, db, "Scoped", "Caregiver")

	otherTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenant)
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))
	testpkg.EnsureAccountTenant(t, db, account.ID, otherTenant)
	assignSystemRoleToAccount(t, db, account.ID, testpkg.Tenant(t), "user")
	assignSystemRoleToAccount(t, db, account.ID, otherTenant, "admin")

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

// Deliberately NOT parallel: assigning a SYSTEM role creates a row in
// auth.roles, whose name is unique across the whole clone (idx_roles_name_system
// has no tenant in it). Two tests inserting the same system role collide.
func TestCaregiverCapability_DisableAllowsCustomTenantRoleToRemain(t *testing.T) {
	db, factory := setupCaregiverFactory(t)
	ctx := testpkg.Ctx(t)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, db, "Custom", "Role")

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

// Deliberately NOT parallel: assigning a SYSTEM role creates a row in
// auth.roles, whose name is unique across the whole clone (idx_roles_name_system
// has no tenant in it). Two tests inserting the same system role collide.
func TestCaregiverCapability_DisableReturnsDetailedBlockers(t *testing.T) {
	db, factory := setupCaregiverFactory(t)
	ctx := testpkg.Ctx(t)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, db, "Blocked", "Caregiver")
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))
	assignSystemRoleToAccount(t, db, account.ID, testpkg.Tenant(t), "admin")
	assignSystemRoleToAccount(t, db, account.ID, testpkg.Tenant(t), "user")

	group := testpkg.CreateTestEducationGroupForTenant(t, db, testpkg.Tenant(t), "Blocked Group")
	testpkg.CreateTestGroupTeacher(t, db, group.ID, teacher.ID)
	otherTeacher, _ := testpkg.CreateTestTeacherWithAccount(
		t,
		db,
		"Second",
		"Leader",
	)
	testpkg.CreateTestGroupTeacher(t, db, group.ID, otherTeacher.ID)

	room := testpkg.CreateTestRoomForTenant(t, db, testpkg.Tenant(t), "Blocked Room")
	activityGroup := testpkg.CreateTestActivityGroupForTenant(t, db, testpkg.Tenant(t), "Blocked Activity")
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activityGroup.ID, room.ID)

	testpkg.CreateTestGroupSupervisor(
		t,
		db,
		teacher.Staff.ID,
		activeGroup.ID,
		"lead",
	)

	testpkg.CreateTestGroupSubstitution(
		t,
		db,
		group.ID,
		nil,
		teacher.Staff.ID,
		timezone.TodayDate().AddDays(-1),
		timezone.TodayDate().AddDays(1),
	)

	insertPlannedSupervisor(
		t,
		db,
		testpkg.Tenant(t),
		teacher.Staff.ID,
		activityGroup.ID,
	)

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

// Deliberately NOT parallel: assigning a SYSTEM role creates a row in
// auth.roles, whose name is unique across the whole clone (idx_roles_name_system
// has no tenant in it). Two tests inserting the same system role collide.
func TestCaregiverCapability_DisableWaitsForConcurrentBindings(t *testing.T) {
	db, factory := setupCaregiverFactory(t)
	ctx := testpkg.Ctx(t)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, db, "Concurrent", "Disable")
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))
	assignSystemRoleToAccount(t, db, account.ID, testpkg.Tenant(t), "admin")
	assignSystemRoleToAccount(t, db, account.ID, testpkg.Tenant(t), "user")

	group := testpkg.CreateTestEducationGroupForTenant(t, db, testpkg.Tenant(t), "Concurrent Group")

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
	t.Parallel()

	db, factory := setupCaregiverFactory(t)
	requestedTenantID := testpkg.UniqueTestTenantID(t)
	mappedTenantID := testpkg.UniqueTestTenantID(t)

	account := testpkg.CreateTestAccount(t, db, "caregiver-mapping")

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
	t.Parallel()

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
	t.Parallel()

	db, factory := setupCaregiverFactory(t)
	ctx := testpkg.Ctx(t)

	account := testpkg.CreateTestAccount(t, db, "caregiver-missing-names")
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
	t.Parallel()

	db, factory := setupCaregiverFactory(t)
	ctx := testpkg.Ctx(t)

	account := testpkg.CreateTestAccount(t, db, "caregiver-person-only")
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

	testpkg.CreateTestPersonWithAccountID(
		t,
		db,
		"Profile",
		"Only",
		account.ID,
	)

	state, err := factory.CaregiverCapability.GetCaregiverCapability(ctx, account.ID)

	require.NoError(t, err)
	require.NotNil(t, state)
	assert.True(t, state.HasPerson)
	assert.False(t, state.HasStaff)
	assert.False(t, state.HasTeacher)
	assert.False(t, state.HasCaregiverProfile)
	assert.False(t, state.IsActiveCaregiver)
}

// Deliberately NOT parallel: assigning a SYSTEM role creates a row in
// auth.roles, whose name is unique across the whole clone (idx_roles_name_system
// has no tenant in it). Two tests inserting the same system role collide.
func TestCaregiverCapability_DisableReturnsStateWhenUserRoleIsAlreadyMissing(t *testing.T) {
	db, factory := setupCaregiverFactory(t)
	ctx := testpkg.Ctx(t)

	_, account := testpkg.CreateTestTeacherWithAccount(t, db, "Admin", "ProfileOnly")
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

// Deliberately NOT parallel: assigning a SYSTEM role creates a row in
// auth.roles, whose name is unique across the whole clone (idx_roles_name_system
// has no tenant in it). Two tests inserting the same system role collide.
func TestCaregiverCapability_DisableRemovesLegacyTeacherRoleWhenAdminRemains(t *testing.T) {
	db, factory := setupCaregiverFactory(t)
	ctx := testpkg.Ctx(t)

	_, account := testpkg.CreateTestTeacherWithAccount(t, db, "Legacy", "Disable")
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

// Deliberately NOT parallel: assigning a SYSTEM role creates a row in
// auth.roles, whose name is unique across the whole clone (idx_roles_name_system
// has no tenant in it). Two tests inserting the same system role collide.
func TestCaregiverCapability_DisableBlocksLegacyTeacherOnlyAccount(t *testing.T) {
	db, factory := setupCaregiverFactory(t)
	ctx := testpkg.Ctx(t)

	_, account := testpkg.CreateTestTeacherWithAccount(t, db, "Legacy", "TeacherOnly")
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

// Deliberately NOT parallel: assigning a SYSTEM role creates a row in
// auth.roles, whose name is unique across the whole clone (idx_roles_name_system
// has no tenant in it). Two tests inserting the same system role collide.
func TestCaregiverDirectory_ListAndFindActiveCaregiversIncludingLegacyTeacherRole(t *testing.T) {
	db, factory := setupCaregiverFactory(t)
	tenantID := testpkg.UniqueTestTenantID(t)
	ctx := testpkg.TenantContext(tenantID)

	activeTeacher, activeAccount := createTestTeacherWithAccountForTenant(t, db, tenantID, "Active", "Caregiver")
	testpkg.EnsureAccountTenant(t, db, activeAccount.ID, tenantID)
	assignSystemRoleToAccount(t, db, activeAccount.ID, tenantID, "user")

	legacyTeacherRoleTeacher, legacyTeacherRoleAccount := createTestTeacherWithAccountForTenant(t, db, tenantID, "Legacy", "Teacher")
	testpkg.EnsureAccountTenant(t, db, legacyTeacherRoleAccount.ID, tenantID)
	ensureSystemRoleExists(t, db, "teacher")
	assignSystemRoleToAccount(t, db, legacyTeacherRoleAccount.ID, tenantID, "teacher")

	_, adminOnlyAccount := createTestTeacherWithAccountForTenant(t, db, tenantID, "Admin", "Only")
	testpkg.EnsureAccountTenant(t, db, adminOnlyAccount.ID, tenantID)
	assignSystemRoleToAccount(t, db, adminOnlyAccount.ID, tenantID, "admin")

	_, inactiveAccount := createTestTeacherWithAccountForTenant(t, db, tenantID, "Inactive", "Caregiver")
	testpkg.EnsureAccountTenant(t, db, inactiveAccount.ID, tenantID)
	assignSystemRoleToAccount(t, db, inactiveAccount.ID, tenantID, "user")

	_, err := db.ExecContext(context.Background(), `UPDATE auth.accounts SET active = false WHERE id = ?`, inactiveAccount.ID)
	require.NoError(t, err)

	_, inactiveMembershipAccount := createTestTeacherWithAccountForTenant(
		t,
		db,
		tenantID,
		"Former",
		"Member",
	)
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
	t.Parallel()

	db, factory := setupCaregiverFactory(t)
	tenantID := testpkg.UniqueTestTenantID(t)
	ctx := testpkg.TenantContext(tenantID)

	_, account := createTestTeacherWithAccountForTenant(t, db, tenantID, "Tenant", "UserRole")
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
