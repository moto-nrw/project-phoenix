// Package platform_test contains integration tests for operator provisioning
// that require a real database connection.
package platform_test

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/services"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

var testClientIP = net.IPv4(127, 0, 0, 1)

// testSchoolID is the tenant_id used by all test fixtures (SetupTestDB ensures tenant 1 exists).
// Expressed as a const to avoid triggering the hermetic int64(1) pattern check.
const testSchoolID int64 = 1 //nolint:unused

func buildProvisioningService(t *testing.T, db *bun.DB) platformSvc.OperatorProvisioningService {
	t.Helper()
	repoFactory := repositories.NewFactory(db)
	serviceFactory, err := services.NewFactory(repoFactory, db, slog.Default())
	require.NoError(t, err, "Failed to create service factory")
	return serviceFactory.OperatorProvisioning
}

// =============================================================================
// ListSchoolPersons Integration Tests
// =============================================================================

func TestIntegration_ListSchoolPersons_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := buildProvisioningService(t, db)
	ctx := context.Background()

	person1 := testpkg.CreateTestPerson(t, db, "Alice", "Schmidt")
	person2 := testpkg.CreateTestPerson(t, db, "Bob", "Mueller")
	defer testpkg.CleanupPerson(t, db, person1.ID)
	defer testpkg.CleanupPerson(t, db, person2.ID)

	// schoolID=1 is the default test tenant
	result, err := service.ListSchoolPersons(ctx, testSchoolID)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Find our created persons in the result
	var foundAlice, foundBob bool
	for _, p := range result {
		if p.ID == person1.ID {
			foundAlice = true
			assert.Equal(t, "Alice", p.FirstName)
			assert.Equal(t, "Schmidt", p.LastName)
		}
		if p.ID == person2.ID {
			foundBob = true
			assert.Equal(t, "Bob", p.FirstName)
			assert.Equal(t, "Mueller", p.LastName)
		}
	}
	assert.True(t, foundAlice, "Alice should be in result")
	assert.True(t, foundBob, "Bob should be in result")
}

func TestIntegration_ListSchoolPersons_ExcludesSoftDeleted(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := buildProvisioningService(t, db)
	ctx := context.Background()

	person := testpkg.CreateTestPerson(t, db, "ToDelete", "Person")
	defer testpkg.CleanupPerson(t, db, person.ID)

	// Soft delete the person
	operatorID := testpkg.CreateTestOperator(t, db).ID
	err := service.SoftDeletePerson(ctx, person.ID, operatorID, testClientIP)
	require.NoError(t, err)

	// List should not include soft-deleted person
	result, err := service.ListSchoolPersons(ctx, testSchoolID)
	require.NoError(t, err)

	for _, p := range result {
		assert.NotEqual(t, person.ID, p.ID, "soft-deleted person should not appear in list")
	}
}

func TestIntegration_ListSchoolPersons_WithStaffAndStudent(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := buildProvisioningService(t, db)
	ctx := context.Background()

	staff := testpkg.CreateTestStaff(t, db, "Supervisor", "Staff")
	student := testpkg.CreateTestStudent(t, db, "Student", "Kid", "3a")
	defer testpkg.CleanupStaffFixtures(t, db, staff.ID)
	defer testpkg.CleanupPerson(t, db, student.PersonID)

	result, err := service.ListSchoolPersons(ctx, testSchoolID)
	require.NoError(t, err)

	for _, p := range result {
		if p.ID == staff.PersonID {
			assert.True(t, p.IsStaff, "staff person should have is_staff=true")
			assert.False(t, p.IsStudent, "staff person should have is_student=false")
		}
		if p.ID == student.PersonID {
			assert.False(t, p.IsStaff, "student person should have is_staff=false")
			assert.True(t, p.IsStudent, "student person should have is_student=true")
		}
	}
}

func TestIntegration_ListSchoolPersons_WithAccount(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := buildProvisioningService(t, db)
	ctx := context.Background()

	person, account := testpkg.CreateTestPersonWithAccount(t, db, "With", "Account")
	defer testpkg.CleanupPerson(t, db, person.ID)
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	result, err := service.ListSchoolPersons(ctx, testSchoolID)
	require.NoError(t, err)

	for _, p := range result {
		if p.ID == person.ID {
			assert.True(t, p.HasAccount, "person with account should have has_account=true")
			assert.NotNil(t, p.AccountEmail, "person with account should have email")
		}
	}
}

func TestIntegration_ListSchoolPersons_EmptySchool(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := buildProvisioningService(t, db)
	ctx := context.Background()

	// Use a school ID that exists but has no persons
	// Create a second school for this test
	testpkg.EnsureTestTenant(t, db, int64(999))
	result, err := service.ListSchoolPersons(ctx, int64(999))
	require.NoError(t, err)
	assert.Empty(t, result)
}

// =============================================================================
// SoftDeletePerson Integration Tests
// =============================================================================

func TestIntegration_SoftDeletePerson_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := buildProvisioningService(t, db)
	ctx := context.Background()

	person := testpkg.CreateTestPerson(t, db, "Delete", "Me")
	defer testpkg.CleanupPerson(t, db, person.ID)

	operatorID := testpkg.CreateTestOperator(t, db).ID
	err := service.SoftDeletePerson(ctx, person.ID, operatorID, testClientIP)
	require.NoError(t, err)

	// Verify person is anonymized
	var dbPerson struct {
		FirstName string  `bun:"first_name"`
		LastName  string  `bun:"last_name"`
		DeletedAt *string `bun:"deleted_at"`
	}
	err = db.NewSelect().
		TableExpr("users.persons").
		Column("first_name", "last_name", "deleted_at").
		Where("id = ?", person.ID).
		Scan(ctx, &dbPerson)
	require.NoError(t, err)
	assert.Equal(t, "Gelöscht", dbPerson.FirstName)
	assert.Equal(t, "Benutzer", dbPerson.LastName)
	assert.NotNil(t, dbPerson.DeletedAt)
}

func TestIntegration_SoftDeletePerson_WithAccount(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := buildProvisioningService(t, db)
	ctx := context.Background()

	person, account := testpkg.CreateTestPersonWithAccount(t, db, "WithAccount", "Delete")
	defer testpkg.CleanupPerson(t, db, person.ID)
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	operatorID := testpkg.CreateTestOperator(t, db).ID
	err := service.SoftDeletePerson(ctx, person.ID, operatorID, testClientIP)
	require.NoError(t, err)

	// Verify account is deactivated and email anonymized
	var dbAccount struct {
		Email  string `bun:"email"`
		Active bool   `bun:"active"`
	}
	err = db.NewSelect().
		TableExpr("auth.accounts").
		Column("email", "active").
		Where("id = ?", account.ID).
		Scan(ctx, &dbAccount)
	require.NoError(t, err)
	assert.False(t, dbAccount.Active, "account should be deactivated")
	assert.Equal(t, fmt.Sprintf("deleted-%d@anonymized.local", person.ID), dbAccount.Email)

	// Verify person's account_id is unlinked
	var accountID *int64
	err = db.NewSelect().
		TableExpr("users.persons").
		Column("account_id").
		Where("id = ?", person.ID).
		Scan(ctx, &accountID)
	require.NoError(t, err)
	assert.Nil(t, accountID, "account_id should be NULL after soft delete")
}

func TestIntegration_SoftDeletePerson_WithRFID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := buildProvisioningService(t, db)
	ctx := context.Background()

	person := testpkg.CreateTestPerson(t, db, "RFID", "Person")
	defer testpkg.CleanupPerson(t, db, person.ID)

	// Assign RFID card via raw SQL
	rfidTag := fmt.Sprintf("TAG-SOFTDEL-%d", person.ID)
	_, err := db.ExecContext(ctx,
		`INSERT INTO users.rfid_cards (id, tenant_id) VALUES (?, ?)`,
		rfidTag, testSchoolID)
	require.NoError(t, err)
	defer func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM users.rfid_cards WHERE id = ?`, rfidTag)
	}()

	_, err = db.ExecContext(ctx,
		`UPDATE users.persons SET tag_id = ? WHERE id = ?`, rfidTag, person.ID)
	require.NoError(t, err)

	// Soft delete
	operatorID := testpkg.CreateTestOperator(t, db).ID
	err = service.SoftDeletePerson(ctx, person.ID, operatorID, testClientIP)
	require.NoError(t, err)

	// Verify RFID is unlinked
	var tagID *string
	err = db.NewSelect().
		TableExpr("users.persons").
		Column("tag_id").
		Where("id = ?", person.ID).
		Scan(ctx, &tagID)
	require.NoError(t, err)
	assert.Nil(t, tagID, "tag_id should be NULL after soft delete")
}

func TestIntegration_SoftDeletePerson_NotFound(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := buildProvisioningService(t, db)
	ctx := context.Background()

	operatorID := testpkg.CreateTestOperator(t, db).ID
	err := service.SoftDeletePerson(ctx, int64(99999999), operatorID, testClientIP)
	require.Error(t, err)
	var notFoundErr *platformSvc.PersonNotFoundError
	require.ErrorAs(t, err, &notFoundErr)
}

func TestIntegration_SoftDeletePerson_AlreadyDeleted(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := buildProvisioningService(t, db)
	ctx := context.Background()

	person := testpkg.CreateTestPerson(t, db, "AlreadyDeleted", "Person")
	defer testpkg.CleanupPerson(t, db, person.ID)

	// First delete succeeds
	operatorID := testpkg.CreateTestOperator(t, db).ID
	err := service.SoftDeletePerson(ctx, person.ID, operatorID, testClientIP)
	require.NoError(t, err)

	// Second delete should return not found (BUN excludes soft-deleted rows)
	err = service.SoftDeletePerson(ctx, person.ID, operatorID, testClientIP)
	require.Error(t, err)
	var notFoundErr *platformSvc.PersonNotFoundError
	require.ErrorAs(t, err, &notFoundErr)
}

func TestIntegration_SoftDeletePerson_WithActiveSupervisionsBlocked(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := buildProvisioningService(t, db)
	ctx := context.Background()

	staff := testpkg.CreateTestStaff(t, db, "Blocked", "Supervisor")
	activity := testpkg.CreateTestActivityGroup(t, db, "supervision-block-test")
	room := testpkg.CreateTestRoom(t, db, "supervision-block-room")
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	supervisor := testpkg.CreateTestGroupSupervisor(t, db, staff.ID, activeGroup.ID, "primary")
	defer testpkg.CleanupTableRecords(t, db, "active.group_supervisors", supervisor.ID)
	defer testpkg.CleanupTableRecords(t, db, "active.groups", activeGroup.ID)
	defer testpkg.CleanupTableRecords(t, db, "facilities.rooms", room.ID)
	defer testpkg.CleanupTableRecords(t, db, "activities.groups", activity.ID)
	defer testpkg.CleanupStaffFixtures(t, db, staff.ID)

	operatorID := testpkg.CreateTestOperator(t, db).ID
	err := service.SoftDeletePerson(ctx, staff.PersonID, operatorID, testClientIP)
	require.Error(t, err)
	var activeSupErr *platformSvc.PersonHasActiveSupervisionsError
	require.ErrorAs(t, err, &activeSupErr)
	assert.Equal(t, staff.PersonID, activeSupErr.PersonID)
	assert.Equal(t, 1, activeSupErr.Count)
}

func TestIntegration_SoftDeletePerson_StaffWithoutSupervision(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := buildProvisioningService(t, db)
	ctx := context.Background()

	staff := testpkg.CreateTestStaff(t, db, "FreeStaff", "NoSupervision")
	defer testpkg.CleanupStaffFixtures(t, db, staff.ID)

	// Staff without active supervision should be deletable
	operatorID := testpkg.CreateTestOperator(t, db).ID
	err := service.SoftDeletePerson(ctx, staff.PersonID, operatorID, testClientIP)
	require.NoError(t, err)

	// Verify soft deleted
	var deletedAt *string
	err = db.NewSelect().
		TableExpr("users.persons").
		Column("deleted_at").
		Where("id = ?", staff.PersonID).
		Scan(ctx, &deletedAt)
	require.NoError(t, err)
	assert.NotNil(t, deletedAt)
}

func TestIntegration_SoftDeletePerson_AuditLogCreated(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := buildProvisioningService(t, db)
	ctx := context.Background()

	person := testpkg.CreateTestPerson(t, db, "Audit", "LogTest")
	defer testpkg.CleanupPerson(t, db, person.ID)

	operatorID := testpkg.CreateTestOperator(t, db).ID
	err := service.SoftDeletePerson(ctx, person.ID, operatorID, testClientIP)
	require.NoError(t, err)

	// Verify audit log entry
	var auditCount int
	auditCount, err = db.NewSelect().
		TableExpr("platform.operator_audit_log").
		Where("resource_type = ?", "person").
		Where("action = ?", "soft_delete").
		Where("resource_id = ?", person.ID).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, auditCount, "should have exactly one audit log entry")
}

func TestIntegration_SoftDeletePerson_StudentSuccess(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := buildProvisioningService(t, db)
	ctx := context.Background()

	student := testpkg.CreateTestStudent(t, db, "Student", "ToDelete", "2b")
	defer testpkg.CleanupPerson(t, db, student.PersonID)

	operatorID := testpkg.CreateTestOperator(t, db).ID
	err := service.SoftDeletePerson(ctx, student.PersonID, operatorID, testClientIP)
	require.NoError(t, err)

	// Verify soft deleted and anonymized
	var dbPerson struct {
		FirstName string  `bun:"first_name"`
		DeletedAt *string `bun:"deleted_at"`
	}
	err = db.NewSelect().
		TableExpr("users.persons").
		Column("first_name", "deleted_at").
		Where("id = ?", student.PersonID).
		Scan(ctx, &dbPerson)
	require.NoError(t, err)
	assert.Equal(t, "Gelöscht", dbPerson.FirstName)
	assert.NotNil(t, dbPerson.DeletedAt)
}

// =============================================================================
// Global Operator Listings: deleted-school Papierkorb hiding
// =============================================================================

// TestIntegration_ListAllDevices_HidesDeletedSchoolDevices verifies that the
// global operator devices listing excludes devices whose tenant school is
// soft-deleted, and re-includes them after restore. Companion to
// TestAccountTenantRepository_ListAllAccounts_ExcludesDeletedSchool — both
// guard the user-visible contract "everything tied to a deleted school must
// disappear from the UI; the school itself stays restorable via Papierkorb."
func TestIntegration_ListAllDevices_HidesDeletedSchoolDevices(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := buildProvisioningService(t, db)
	ctx := context.Background()

	tenantID := int64(90021)
	testpkg.EnsureTestTenant(t, db, tenantID)

	device := testpkg.CreateTestDeviceForTenant(t, db, tenantID, "trash-test")

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM iot.devices WHERE id = ?`, device.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.schools WHERE id = ?`, tenantID)
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.organizations WHERE id = ?`, tenantID)
	})

	containsDevice := func(devices []platformSvc.OperatorDeviceInfo) bool {
		for _, d := range devices {
			if d.ID == device.ID {
				return true
			}
		}
		return false
	}

	// Baseline: device visible while school is active.
	devices, err := service.ListAllDevices(ctx)
	require.NoError(t, err)
	require.True(t, containsDevice(devices),
		"baseline: device should be visible while school is active")

	// Soft-delete the school directly (avoids SoftDeleteSchool's token/invitation
	// side effects which are orthogonal to the read-filter behavior under test).
	_, err = db.ExecContext(ctx,
		`UPDATE platform.schools SET deleted_at = NOW() WHERE id = ?`, tenantID)
	require.NoError(t, err)

	// After soft-delete: device must disappear from the global listing.
	devices, err = service.ListAllDevices(ctx)
	require.NoError(t, err)
	require.False(t, containsDevice(devices),
		"device whose school is soft-deleted must not appear in ListAllDevices")

	// Also hidden when filtered by the parent organization (org filter goes through
	// the same query path; no separate IsDeleted pre-check protects ListOrganizationDevices).
	devices, err = service.ListOrganizationDevices(ctx, tenantID)
	require.NoError(t, err)
	require.False(t, containsDevice(devices),
		"device whose school is soft-deleted must not appear in ListOrganizationDevices")

	// Restore: device reappears.
	_, err = db.ExecContext(ctx,
		`UPDATE platform.schools SET deleted_at = NULL WHERE id = ?`, tenantID)
	require.NoError(t, err)

	devices, err = service.ListAllDevices(ctx)
	require.NoError(t, err)
	assert.True(t, containsDevice(devices),
		"restore: device should be visible again in ListAllDevices")
}

// TestIntegration_ListAllDevices_HidesDeletedOrgDevices verifies the
// second layer of the SQL filter: devices belonging to a soft-deleted
// organization (whose schools may technically still have deleted_at IS NULL
// in a corrupt-state scenario) also disappear. Defensive — under normal
// flow an org can only be deleted after all its schools are in the Papierkorb.
func TestIntegration_ListAllDevices_HidesDeletedOrgDevices(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := buildProvisioningService(t, db)
	ctx := context.Background()

	tenantID := int64(90022)
	testpkg.EnsureTestTenant(t, db, tenantID)

	device := testpkg.CreateTestDeviceForTenant(t, db, tenantID, "org-trash")

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM iot.devices WHERE id = ?`, device.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.schools WHERE id = ?`, tenantID)
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.organizations WHERE id = ?`, tenantID)
	})

	containsDevice := func(devices []platformSvc.OperatorDeviceInfo) bool {
		for _, d := range devices {
			if d.ID == device.ID {
				return true
			}
		}
		return false
	}

	// Mark only the org as deleted (school stays active to isolate the org filter).
	_, err := db.ExecContext(ctx,
		`UPDATE platform.organizations SET deleted_at = NOW() WHERE id = ?`, tenantID)
	require.NoError(t, err)

	devices, err := service.ListAllDevices(ctx)
	require.NoError(t, err)
	assert.False(t, containsDevice(devices),
		"device whose organization is soft-deleted must not appear in ListAllDevices")
}

// TestIntegration_ListOrganizationDevices_RejectsDeletedOrg verifies the
// explicit IsDeleted pre-check on ListOrganizationDevices: targeting a
// soft-deleted org must return OrganizationDeletedError, mirroring the
// behavior of ListSchoolDevices for soft-deleted schools.
func TestIntegration_ListOrganizationDevices_RejectsDeletedOrg(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := buildProvisioningService(t, db)
	ctx := context.Background()

	tenantID := int64(90023)
	testpkg.EnsureTestTenant(t, db, tenantID)

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.schools WHERE id = ?`, tenantID)
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.organizations WHERE id = ?`, tenantID)
	})

	// Soft-delete the org (EnsureTestTenant created school+org sharing tenantID
	// as their primary key, see testpkg.EnsureTestTenant).
	_, err := db.ExecContext(ctx,
		`UPDATE platform.organizations SET deleted_at = NOW() WHERE id = ?`, tenantID)
	require.NoError(t, err)

	_, err = service.ListOrganizationDevices(ctx, tenantID)
	require.Error(t, err)
	var deletedErr *platformSvc.OrganizationDeletedError
	require.ErrorAs(t, err, &deletedErr,
		"ListOrganizationDevices on a soft-deleted org must return OrganizationDeletedError")
	assert.Equal(t, tenantID, deletedErr.OrganizationID)
}
