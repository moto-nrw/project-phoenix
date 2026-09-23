package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"reflect"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// End-to-end coverage of operator provisioning (#3253) against the real
// composition: the factory binds Organisation & Tenancy provisioning to the
// retained owners exactly as production does. The legacy-composition test
// policy keeps organizationtenancy out of this package's imports, so input
// values come from the capability's own signatures and typed errors are
// matched by their package and name.

const organizationTenancyPackage = "github.com/moto-nrw/project-phoenix/modules/organizationtenancy"

var provisioningTestClientIP = net.IPv4(127, 0, 0, 1)

// buildOperatorProvisioning returns the factory whose OperatorProvisioning
// field is the production composition over db.
func buildOperatorProvisioning(t *testing.T, db *bun.DB) *Factory {
	t.Helper()
	repoFactory := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	serviceFactory, err := NewFactoryForTests(repoFactory, db, slog.Default())
	require.NoError(t, err, "Failed to create service factory")
	require.NoError(t, serviceFactory.SetTenantRuntime(testpkg.TenantRuntime(t, db)))
	return serviceFactory
}

// provisioningContext mirrors the runtime middleware of the operator routes.
func provisioningContext(t *testing.T, db *bun.DB) context.Context {
	t.Helper()
	return testpkg.WithTenantRuntime(t, context.Background(), db)
}

// newInput allocates the pointer input a create command takes.
func newInput[T, R any](_ func(context.Context, *T, int64, net.IP) (R, error)) *T {
	return new(T)
}

// schoolScopedInput returns the zero input of a school-scoped command such
// as CreateSchoolAccount or InviteSchoolAdmin.
func schoolScopedInput[T, R any](_ func(context.Context, int64, int64, net.IP, T) (R, error)) T {
	var zero T
	return zero
}

// requireProvisioningError asserts that err's chain holds the typed
// organizationtenancy error name and returns that error's struct value.
func requireProvisioningError(t *testing.T, err error, name string) reflect.Value {
	t.Helper()
	require.Error(t, err)
	pending := []error{err}
	for len(pending) > 0 {
		current := pending[0]
		pending = pending[1:]
		if current == nil {
			continue
		}
		typ := reflect.TypeOf(current)
		if typ.Kind() == reflect.Pointer && typ.Elem().PkgPath() == organizationTenancyPackage && typ.Elem().Name() == name {
			return reflect.ValueOf(current).Elem()
		}
		switch wrapped := current.(type) {
		case interface{ Unwrap() []error }:
			pending = append(pending, wrapped.Unwrap()...)
		default:
			pending = append(pending, errors.Unwrap(current))
		}
	}
	t.Fatalf("expected *organizationtenancy.%s in error chain, got %T: %v", name, err, err)
	return reflect.Value{}
}

func countOperatorAudit(t *testing.T, db *bun.DB, action, resourceType string, resourceID int64) int {
	t.Helper()
	count, err := db.NewSelect().
		TableExpr("platform.operator_audit_log").
		Where("action = ?", action).
		Where("resource_type = ?", resourceType).
		Where("resource_id = ?", resourceID).
		Count(testpkg.Ctx(t))
	require.NoError(t, err)
	return count
}

// =============================================================================
// ListSchoolPersons
// =============================================================================

func TestOperatorProvisioningIntegration_ListSchoolPersons_Success(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := buildOperatorProvisioning(t, db).OperatorProvisioning
	ctx := provisioningContext(t, db)

	person1 := testpkg.CreateTestPerson(t, db, "Alice", "Schmidt")
	person2 := testpkg.CreateTestPerson(t, db, "Bob", "Mueller")

	result, err := service.ListSchoolPersons(ctx, testpkg.Tenant(t))
	require.NoError(t, err)
	require.NotNil(t, result)

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

func TestOperatorProvisioningIntegration_ListSchoolPersons_ExcludesSoftDeleted(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := buildOperatorProvisioning(t, db).OperatorProvisioning
	ctx := provisioningContext(t, db)

	person := testpkg.CreateTestPerson(t, db, "ToDelete", "Person")

	operatorID := testpkg.CreateTestOperator(t, db).ID
	require.NoError(t, service.SoftDeletePerson(ctx, person.ID, operatorID, provisioningTestClientIP))

	result, err := service.ListSchoolPersons(ctx, testpkg.Tenant(t))
	require.NoError(t, err)
	for _, p := range result {
		assert.NotEqual(t, person.ID, p.ID, "soft-deleted person should not appear in list")
	}
}

func TestOperatorProvisioningIntegration_ListSchoolPersons_WithStaffAndStudent(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := buildOperatorProvisioning(t, db).OperatorProvisioning
	ctx := provisioningContext(t, db)

	staff := testpkg.CreateTestStaff(t, db, "Supervisor", "Staff")
	student := testpkg.CreateTestStudent(t, db, "Student", "Kid", "3a")

	result, err := service.ListSchoolPersons(ctx, testpkg.Tenant(t))
	require.NoError(t, err)

	var foundStaff, foundStudent bool
	for _, p := range result {
		if p.ID == staff.PersonID {
			foundStaff = true
			assert.True(t, p.IsStaff, "staff person should have is_staff=true")
			assert.False(t, p.IsStudent, "staff person should have is_student=false")
		}
		if p.ID == student.PersonID {
			foundStudent = true
			assert.False(t, p.IsStaff, "student person should have is_staff=false")
			assert.True(t, p.IsStudent, "student person should have is_student=true")
		}
	}
	assert.True(t, foundStaff, "staff person should be in result")
	assert.True(t, foundStudent, "student person should be in result")
}

func TestOperatorProvisioningIntegration_ListSchoolPersons_WithAccount(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := buildOperatorProvisioning(t, db).OperatorProvisioning
	ctx := provisioningContext(t, db)

	person, _ := testpkg.CreateTestPersonWithAccount(t, db, "With", "Account")

	result, err := service.ListSchoolPersons(ctx, testpkg.Tenant(t))
	require.NoError(t, err)

	var found bool
	for _, p := range result {
		if p.ID == person.ID {
			found = true
			assert.True(t, p.HasAccount, "person with account should have has_account=true")
			assert.NotNil(t, p.AccountEmail, "person with account should have email")
		}
	}
	assert.True(t, found, "person with account should be in result")
}

func TestOperatorProvisioningIntegration_ListSchoolPersons_EmptySchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := buildOperatorProvisioning(t, db).OperatorProvisioning
	ctx := provisioningContext(t, db)

	emptySchoolID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, emptySchoolID)
	result, err := service.ListSchoolPersons(ctx, emptySchoolID)
	require.NoError(t, err)
	assert.Empty(t, result)
}

// =============================================================================
// SoftDeletePerson
// =============================================================================

func TestOperatorProvisioningIntegration_SoftDeletePerson_Success(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := buildOperatorProvisioning(t, db).OperatorProvisioning
	ctx := provisioningContext(t, db)

	person := testpkg.CreateTestPerson(t, db, "Delete", "Me")

	operatorID := testpkg.CreateTestOperator(t, db).ID
	require.NoError(t, service.SoftDeletePerson(ctx, person.ID, operatorID, provisioningTestClientIP))

	var dbPerson struct {
		FirstName string  `bun:"first_name"`
		LastName  string  `bun:"last_name"`
		DeletedAt *string `bun:"deleted_at"`
	}
	err := db.NewSelect().
		TableExpr("users.persons").
		Column("first_name", "last_name", "deleted_at").
		Where("id = ?", person.ID).
		Scan(testpkg.Ctx(t), &dbPerson)
	require.NoError(t, err)
	assert.Equal(t, "Gelöscht", dbPerson.FirstName)
	assert.Equal(t, "Benutzer", dbPerson.LastName)
	assert.NotNil(t, dbPerson.DeletedAt)
}

func TestOperatorProvisioningIntegration_SoftDeletePerson_WithAccount(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := buildOperatorProvisioning(t, db).OperatorProvisioning
	ctx := provisioningContext(t, db)

	person, account := testpkg.CreateTestPersonWithAccount(t, db, "WithAccount", "Delete")

	operatorID := testpkg.CreateTestOperator(t, db).ID
	require.NoError(t, service.SoftDeletePerson(ctx, person.ID, operatorID, provisioningTestClientIP))

	var dbAccount struct {
		Email  string `bun:"email"`
		Active bool   `bun:"active"`
	}
	err := db.NewSelect().
		TableExpr("auth.accounts").
		Column("email", "active").
		Where("id = ?", account.ID).
		Scan(testpkg.Ctx(t), &dbAccount)
	require.NoError(t, err)
	assert.False(t, dbAccount.Active, "account should be deactivated")
	assert.Equal(t, fmt.Sprintf("deleted-%d@anonymized.local", person.ID), dbAccount.Email)

	var accountID *int64
	err = db.NewSelect().
		TableExpr("users.persons").
		Column("account_id").
		Where("id = ?", person.ID).
		Scan(testpkg.Ctx(t), &accountID)
	require.NoError(t, err)
	assert.Nil(t, accountID, "account_id should be NULL after soft delete")
}

func TestOperatorProvisioningIntegration_SoftDeletePerson_WithRFID(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := buildOperatorProvisioning(t, db).OperatorProvisioning
	ctx := provisioningContext(t, db)
	dbCtx := testpkg.Ctx(t)

	person := testpkg.CreateTestPerson(t, db, "RFID", "Person")

	rfidTag := fmt.Sprintf("TAG-SOFTDEL-%d", person.ID)
	_, err := db.ExecContext(dbCtx,
		`INSERT INTO users.rfid_cards (id, tenant_id) VALUES (?, ?)`,
		rfidTag, testpkg.Tenant(t))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users.rfid_cards WHERE id = ?`, rfidTag)
	})

	_, err = db.ExecContext(dbCtx,
		`UPDATE users.persons SET tag_id = ? WHERE id = ?`, rfidTag, person.ID)
	require.NoError(t, err)

	operatorID := testpkg.CreateTestOperator(t, db).ID
	require.NoError(t, service.SoftDeletePerson(ctx, person.ID, operatorID, provisioningTestClientIP))

	var tagID *string
	err = db.NewSelect().
		TableExpr("users.persons").
		Column("tag_id").
		Where("id = ?", person.ID).
		Scan(dbCtx, &tagID)
	require.NoError(t, err)
	assert.Nil(t, tagID, "tag_id should be NULL after soft delete")
}

func TestOperatorProvisioningIntegration_SoftDeletePerson_NotFound(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := buildOperatorProvisioning(t, db).OperatorProvisioning
	ctx := provisioningContext(t, db)

	operatorID := testpkg.CreateTestOperator(t, db).ID
	err := service.SoftDeletePerson(ctx, int64(99999999), operatorID, provisioningTestClientIP)
	requireProvisioningError(t, err, "PersonNotFoundError")
}

func TestOperatorProvisioningIntegration_SoftDeletePerson_AlreadyDeleted(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := buildOperatorProvisioning(t, db).OperatorProvisioning
	ctx := provisioningContext(t, db)

	person := testpkg.CreateTestPerson(t, db, "AlreadyDeleted", "Person")

	operatorID := testpkg.CreateTestOperator(t, db).ID
	require.NoError(t, service.SoftDeletePerson(ctx, person.ID, operatorID, provisioningTestClientIP))

	// A soft-deleted person is not found again.
	err := service.SoftDeletePerson(ctx, person.ID, operatorID, provisioningTestClientIP)
	requireProvisioningError(t, err, "PersonNotFoundError")
}

func TestOperatorProvisioningIntegration_SoftDeletePerson_WithActiveSupervisionsBlocked(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := buildOperatorProvisioning(t, db).OperatorProvisioning
	ctx := provisioningContext(t, db)

	staff := testpkg.CreateTestStaff(t, db, "Blocked", "Supervisor")
	activity := testpkg.CreateTestActivityGroup(t, db, "supervision-block-test")
	room := testpkg.CreateTestRoom(t, db, "supervision-block-room")
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	_ = testpkg.CreateTestGroupSupervisor(t, db, staff.ID, activeGroup.ID, "primary")

	operatorID := testpkg.CreateTestOperator(t, db).ID
	err := service.SoftDeletePerson(ctx, staff.PersonID, operatorID, provisioningTestClientIP)
	blocked := requireProvisioningError(t, err, "PersonHasActiveSupervisionsError")
	assert.Equal(t, staff.PersonID, blocked.FieldByName("PersonID").Int())
	assert.EqualValues(t, 1, blocked.FieldByName("Count").Int(), "one active supervision")
}

func TestOperatorProvisioningIntegration_SoftDeletePerson_StaffWithoutSupervision(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := buildOperatorProvisioning(t, db).OperatorProvisioning
	ctx := provisioningContext(t, db)

	staff := testpkg.CreateTestStaff(t, db, "FreeStaff", "NoSupervision")

	operatorID := testpkg.CreateTestOperator(t, db).ID
	require.NoError(t, service.SoftDeletePerson(ctx, staff.PersonID, operatorID, provisioningTestClientIP))

	var deletedAt *string
	err := db.NewSelect().
		TableExpr("users.persons").
		Column("deleted_at").
		Where("id = ?", staff.PersonID).
		Scan(testpkg.Ctx(t), &deletedAt)
	require.NoError(t, err)
	assert.NotNil(t, deletedAt)
}

func TestOperatorProvisioningIntegration_SoftDeletePerson_AuditLogCreated(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := buildOperatorProvisioning(t, db).OperatorProvisioning
	ctx := provisioningContext(t, db)

	person := testpkg.CreateTestPerson(t, db, "Audit", "LogTest")

	operatorID := testpkg.CreateTestOperator(t, db).ID
	require.NoError(t, service.SoftDeletePerson(ctx, person.ID, operatorID, provisioningTestClientIP))

	assert.Equal(t, 1, countOperatorAudit(t, db, "soft_delete", "person", person.ID),
		"should have exactly one audit log entry")
}

func TestOperatorProvisioningIntegration_SoftDeletePerson_StudentSuccess(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := buildOperatorProvisioning(t, db).OperatorProvisioning
	ctx := provisioningContext(t, db)

	student := testpkg.CreateTestStudent(t, db, "Student", "ToDelete", "2b")

	operatorID := testpkg.CreateTestOperator(t, db).ID
	require.NoError(t, service.SoftDeletePerson(ctx, student.PersonID, operatorID, provisioningTestClientIP))

	var dbPerson struct {
		FirstName string  `bun:"first_name"`
		DeletedAt *string `bun:"deleted_at"`
	}
	err := db.NewSelect().
		TableExpr("users.persons").
		Column("first_name", "deleted_at").
		Where("id = ?", student.PersonID).
		Scan(testpkg.Ctx(t), &dbPerson)
	require.NoError(t, err)
	assert.Equal(t, "Gelöscht", dbPerson.FirstName)
	assert.NotNil(t, dbPerson.DeletedAt)
}

// =============================================================================
// Global operator listings: deleted-school Papierkorb hiding
// =============================================================================

// TestOperatorProvisioningIntegration_ListAllDevices_HidesDeletedSchoolDevices
// verifies that the global operator device listing excludes devices whose
// school is soft-deleted and re-includes them after restore: everything tied
// to a deleted school disappears from the UI while the school itself stays
// restorable via Papierkorb.
func TestOperatorProvisioningIntegration_ListAllDevices_HidesDeletedSchoolDevices(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := buildOperatorProvisioning(t, db).OperatorProvisioning
	ctx := provisioningContext(t, db)
	dbCtx := testpkg.Ctx(t)

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	device := testpkg.CreateTestDeviceForTenant(t, db, tenantID, "trash-test")

	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = db.ExecContext(cleanupCtx, `DELETE FROM iot.devices WHERE id = ?`, device.ID)
		_, _ = db.ExecContext(cleanupCtx, `DELETE FROM platform.schools WHERE id = ?`, tenantID)
		_, _ = db.ExecContext(cleanupCtx, `DELETE FROM platform.organizations WHERE id = ?`, tenantID)
	})

	containsDevice := func(ids []int64) bool {
		for _, id := range ids {
			if id == device.ID {
				return true
			}
		}
		return false
	}
	listAll := func() []int64 {
		devices, err := service.ListAllDevices(ctx)
		require.NoError(t, err)
		ids := make([]int64, 0, len(devices))
		for _, d := range devices {
			ids = append(ids, d.ID)
		}
		return ids
	}

	require.True(t, containsDevice(listAll()),
		"baseline: device should be visible while school is active")

	// Soft-delete the school directly (avoids SoftDeleteSchool's token and
	// invitation side effects, which are orthogonal to the read filter).
	_, err := db.ExecContext(dbCtx, `UPDATE platform.schools SET deleted_at = NOW() WHERE id = ?`, tenantID)
	require.NoError(t, err)

	require.False(t, containsDevice(listAll()),
		"device whose school is soft-deleted must not appear in ListAllDevices")

	// The organization filter goes through the same listing; no separate
	// school pre-check protects ListOrganizationDevices.
	orgDevices, err := service.ListOrganizationDevices(ctx, tenantID)
	require.NoError(t, err)
	orgIDs := make([]int64, 0, len(orgDevices))
	for _, d := range orgDevices {
		orgIDs = append(orgIDs, d.ID)
	}
	require.False(t, containsDevice(orgIDs),
		"device whose school is soft-deleted must not appear in ListOrganizationDevices")

	_, err = db.ExecContext(dbCtx, `UPDATE platform.schools SET deleted_at = NULL WHERE id = ?`, tenantID)
	require.NoError(t, err)

	assert.True(t, containsDevice(listAll()),
		"restore: device should be visible again in ListAllDevices")
}

// TestOperatorProvisioningIntegration_ListAllDevices_HidesDeletedOrgDevices
// verifies the second filter layer: devices of a soft-deleted organization
// disappear even while its school row is still live (a corrupt state; under
// normal flow an organization is deleted only after all its schools).
func TestOperatorProvisioningIntegration_ListAllDevices_HidesDeletedOrgDevices(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := buildOperatorProvisioning(t, db).OperatorProvisioning
	ctx := provisioningContext(t, db)

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	device := testpkg.CreateTestDeviceForTenant(t, db, tenantID, "org-trash")

	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = db.ExecContext(cleanupCtx, `DELETE FROM iot.devices WHERE id = ?`, device.ID)
		_, _ = db.ExecContext(cleanupCtx, `DELETE FROM platform.schools WHERE id = ?`, tenantID)
		_, _ = db.ExecContext(cleanupCtx, `DELETE FROM platform.organizations WHERE id = ?`, tenantID)
	})

	_, err := db.ExecContext(testpkg.Ctx(t), `UPDATE platform.organizations SET deleted_at = NOW() WHERE id = ?`, tenantID)
	require.NoError(t, err)

	devices, err := service.ListAllDevices(ctx)
	require.NoError(t, err)
	for _, d := range devices {
		assert.NotEqual(t, device.ID, d.ID,
			"device whose organization is soft-deleted must not appear in ListAllDevices")
	}
}

// TestOperatorProvisioningIntegration_ListOrganizationDevices_RejectsDeletedOrg
// verifies the explicit deleted-organization pre-check, mirroring
// ListSchoolDevices for soft-deleted schools.
func TestOperatorProvisioningIntegration_ListOrganizationDevices_RejectsDeletedOrg(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := buildOperatorProvisioning(t, db).OperatorProvisioning
	ctx := provisioningContext(t, db)

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = db.ExecContext(cleanupCtx, `DELETE FROM platform.schools WHERE id = ?`, tenantID)
		_, _ = db.ExecContext(cleanupCtx, `DELETE FROM platform.organizations WHERE id = ?`, tenantID)
	})

	// EnsureTestTenant creates school and organization sharing tenantID as
	// their primary key.
	_, err := db.ExecContext(testpkg.Ctx(t), `UPDATE platform.organizations SET deleted_at = NOW() WHERE id = ?`, tenantID)
	require.NoError(t, err)

	_, err = service.ListOrganizationDevices(ctx, tenantID)
	deleted := requireProvisioningError(t, err, "OrganizationDeletedError")
	assert.Equal(t, tenantID, deleted.FieldByName("OrganizationID").Int())
}

// =============================================================================
// Organisation, school and account provisioning
// =============================================================================

// provisionTestSchool creates an organisation and a school through the
// capability and removes both afterwards.
func provisionTestSchool(t *testing.T, db *bun.DB, factory *Factory, operatorID int64, label string) (organizationID, schoolID int64) {
	t.Helper()
	service := factory.OperatorProvisioning
	ctx := provisioningContext(t, db)
	suffix := testpkg.UniqueTestTenantID(t)

	organizationInput := newInput(service.CreateOrganization)
	organizationInput.Name = fmt.Sprintf("Provisioning %s %d", label, suffix)
	organizationInput.Slug = fmt.Sprintf("prov-%s-%d", label, suffix)
	organizationInput.Active = true
	organization, err := service.CreateOrganization(ctx, organizationInput, operatorID, provisioningTestClientIP)
	require.NoError(t, err)
	organizationID = organization.ID

	schoolInput := newInput(service.CreateSchool)
	schoolInput.OrganizationID = organizationID
	schoolInput.Name = fmt.Sprintf("Schule %s %d", label, suffix)
	schoolInput.Slug = fmt.Sprintf("schule-%s-%d", label, suffix)
	schoolInput.Subdomain = fmt.Sprintf("schule-%s-%d", label, suffix)
	schoolInput.Active = true
	school, err := service.CreateSchool(ctx, schoolInput, operatorID, provisioningTestClientIP)
	require.NoError(t, err)
	schoolID = school.ID

	t.Cleanup(func() {
		cleanupCtx := context.Background()
		for _, statement := range []string{
			`DELETE FROM iot.devices WHERE tenant_id = ?`,
			`DELETE FROM activities.categories WHERE tenant_id = ?`,
		} {
			_, _ = db.ExecContext(cleanupCtx, statement, schoolID)
		}
		_, _ = db.ExecContext(cleanupCtx, `DELETE FROM platform.schools WHERE id = ?`, schoolID)
		_, _ = db.ExecContext(cleanupCtx, `DELETE FROM platform.organizations WHERE id = ?`, organizationID)
	})
	return organizationID, schoolID
}

func TestOperatorProvisioningIntegration_CreateSchool_SeedsCategoriesAndWebManualDevice(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	factory := buildOperatorProvisioning(t, db)
	operatorID := testpkg.CreateTestOperator(t, db).ID
	dbCtx := testpkg.Ctx(t)

	organizationID, schoolID := provisionTestSchool(t, db, factory, operatorID, "create")

	var categories []string
	err := db.NewSelect().
		TableExpr("activities.categories").
		Column("name").
		Where("tenant_id = ?", schoolID).
		Order("name").
		Scan(dbCtx, &categories)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{
		"Sport", "Kunst & Basteln", "Musik", "Spiele", "Lesen", "Hausaufgabenhilfe",
		"Natur & Forschen", "Computer", "Gruppenraum", "Mensa",
	}, categories)

	var device struct {
		DeviceType string  `bun:"device_type"`
		Name       *string `bun:"name"`
		Status     string  `bun:"status"`
	}
	err = db.NewSelect().
		TableExpr("iot.devices").
		Column("device_type", "name", "status").
		Where("tenant_id = ?", schoolID).
		Where("device_id = ?", "WEB-MANUAL-001").
		Scan(dbCtx, &device)
	require.NoError(t, err, "school must own the web-manual device")
	assert.Equal(t, "virtual", device.DeviceType)
	require.NotNil(t, device.Name)
	assert.Equal(t, "Web-Portal (Manuell)", *device.Name)
	assert.Equal(t, "active", device.Status)

	assert.Equal(t, 1, countOperatorAudit(t, db, "create", "organization", organizationID))
	assert.Equal(t, 1, countOperatorAudit(t, db, "create", "school", schoolID))

	// The web-manual device is protected from transfer.
	service := factory.OperatorProvisioning
	devices, err := service.ListSchoolDevices(provisioningContext(t, db), schoolID)
	require.NoError(t, err)
	require.Len(t, devices, 1)
	status, err := service.GetDeviceTransferStatus(provisioningContext(t, db), devices[0].ID)
	require.NoError(t, err)
	assert.True(t, status.IsProtected)
	assert.False(t, status.CanTransfer)
}

func TestOperatorProvisioningIntegration_InviteSchoolAdmin_CreatesInvitation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	factory := buildOperatorProvisioning(t, db)
	service := factory.OperatorProvisioning
	operatorID := testpkg.CreateTestOperator(t, db).ID
	ctx := provisioningContext(t, db)

	_, schoolID := provisionTestSchool(t, db, factory, operatorID, "invite")

	input := schoolScopedInput(service.InviteSchoolAdmin)
	input.Email = fmt.Sprintf("  Invited-Admin-%d@Example.test ", schoolID)
	firstName := "Erika"
	input.FirstName = &firstName
	invitation, err := service.InviteSchoolAdmin(ctx, schoolID, operatorID, provisioningTestClientIP, input)
	require.NoError(t, err)
	require.NotNil(t, invitation)
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM auth.invitation_tokens WHERE id = ?`, invitation.ID)
	})
	assert.Equal(t, fmt.Sprintf("invited-admin-%d@example.test", schoolID), invitation.Email)
	assert.Equal(t, "admin", invitation.RoleName)
	assert.NotEmpty(t, invitation.Token)

	var stored struct {
		TenantID int64 `bun:"tenant_id"`
		RoleID   int64 `bun:"role_id"`
	}
	require.NoError(t, db.NewSelect().TableExpr("auth.invitation_tokens").
		Column("tenant_id", "role_id").
		Where("id = ?", invitation.ID).Scan(testpkg.Ctx(t), &stored))
	assert.Equal(t, schoolID, stored.TenantID)
	assert.Equal(t, invitation.RoleID, stored.RoleID)
	assert.Equal(t, 1, countOperatorAudit(t, db, "create", "invitation", invitation.ID))
}

func TestOperatorProvisioningIntegration_CreateSchoolAccount_BuildsIdentityChain(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	factory := buildOperatorProvisioning(t, db)
	service := factory.OperatorProvisioning
	operatorID := testpkg.CreateTestOperator(t, db).ID
	ctx := provisioningContext(t, db)
	dbCtx := testpkg.Ctx(t)

	t.Run("role projection uses public identity capability", func(t *testing.T) {
		identity := provisioningIdentity{roles: factory.Auth}
		custom := testpkg.CreateTestRole(t, db, "operator-custom")
		system := testpkg.CreateTestSystemRole(t, db, "operator-system")
		roles, err := identity.ListSystemRoles(ctx)
		require.NoError(t, err)
		var foundSystem bool
		for _, role := range roles {
			require.True(t, role.IsSystem)
			require.NotEqual(t, custom.ID, role.ID)
			foundSystem = foundSystem || role.ID == system.ID
		}
		require.True(t, foundSystem)
		role, found, err := identity.FindSystemRole(ctx, strings.ToUpper(system.Name))
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, system.ID, role.ID)
		_, found, err = identity.FindSystemRole(ctx, custom.Name)
		require.NoError(t, err)
		require.False(t, found)
		role, found, err = identity.FindRole(ctx, custom.ID)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, custom.ID, role.ID)
		require.False(t, role.IsSystem)
		_, found, err = identity.FindRole(ctx, 0)
		require.NoError(t, err, "a missing role is not a lookup failure")
		require.False(t, found)
	})

	_, schoolID := provisionTestSchool(t, db, factory, operatorID, "account")

	t.Run("unknown role is invalid data, not an internal error", func(t *testing.T) {
		unknownRoleID := int64(0)
		input := schoolScopedInput(service.CreateSchoolAccount)
		input.Email = fmt.Sprintf("unknown-role-%d@example.test", schoolID)
		input.Password = "Provisioning-Test-9!"
		input.FirstName = "Erika"
		input.LastName = "Leitung"
		input.RoleID = &unknownRoleID
		_, err := service.CreateSchoolAccount(ctx, schoolID, operatorID, provisioningTestClientIP, input)
		requireProvisioningError(t, err, "InvalidProvisioningDataError")
		require.ErrorContains(t, err, "role with ID 0 not found")
	})

	input := schoolScopedInput(service.CreateSchoolAccount)
	input.Email = fmt.Sprintf("school-admin-%d@example.test", schoolID)
	input.Password = "Provisioning-Test-9!"
	input.FirstName = "Erika"
	input.LastName = "Leitung"
	account, err := service.CreateSchoolAccount(ctx, schoolID, operatorID, provisioningTestClientIP, input)
	require.NoError(t, err)
	require.NotNil(t, account)
	assert.Equal(t, input.Email, account.Email)
	assert.True(t, account.Active)
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM auth.accounts WHERE id = ?`, account.ID)
	})

	membership, err := db.NewSelect().
		TableExpr("auth.account_tenants").
		Where("account_id = ?", account.ID).
		Where("tenant_id = ?", schoolID).
		Count(dbCtx)
	require.NoError(t, err)
	assert.Equal(t, 1, membership, "account must be a member of the school")

	var person struct {
		ID        int64  `bun:"id"`
		FirstName string `bun:"first_name"`
		LastName  string `bun:"last_name"`
	}
	err = db.NewSelect().
		TableExpr("users.persons").
		Column("id", "first_name", "last_name").
		Where("account_id = ?", account.ID).
		Where("tenant_id = ?", schoolID).
		Scan(dbCtx, &person)
	require.NoError(t, err, "school identity must create the person")
	assert.Equal(t, "Erika", person.FirstName)
	assert.Equal(t, "Leitung", person.LastName)

	staffCount, err := db.NewSelect().
		TableExpr("users.staff").
		Where("person_id = ?", person.ID).
		Count(dbCtx)
	require.NoError(t, err)
	assert.Equal(t, 1, staffCount, "admin account must get a staff record")

	accounts, err := service.ListSchoolAccounts(ctx, schoolID)
	require.NoError(t, err)
	var listed bool
	for _, listedAccount := range accounts {
		if listedAccount.AccountID == account.ID {
			listed = true
			assert.True(t, listedAccount.HasAdminRole, "default role is the school admin role")
		}
	}
	assert.True(t, listed, "created account must be listed for the school")
	assert.Equal(t, 1, countOperatorAudit(t, db, "create", "account", account.ID))
}

// The caregiver upgrade of an admin account reads and assigns the platform
// user role inside the school's transaction (#3313).
func TestOperatorProvisioningIntegration_CreateSchoolAccount_CaregiverUpgradeAssignsUserRole(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	factory := buildOperatorProvisioning(t, db)
	service := factory.OperatorProvisioning
	operatorID := testpkg.CreateTestOperator(t, db).ID
	ctx := provisioningContext(t, db)

	_, schoolID := provisionTestSchool(t, db, factory, operatorID, "caregiver")

	input := schoolScopedInput(service.CreateSchoolAccount)
	input.Email = fmt.Sprintf("caregiver-admin-%d@example.test", schoolID)
	input.Password = "Provisioning-Test-9!"
	input.FirstName = "Clara"
	input.LastName = "Betreuung"
	input.CaregiverEnabled = true
	account, err := service.CreateSchoolAccount(ctx, schoolID, operatorID, provisioningTestClientIP, input)
	require.NoError(t, err)
	require.NotNil(t, account)
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM auth.accounts WHERE id = ?`, account.ID)
	})

	accounts, err := service.ListSchoolAccounts(ctx, schoolID)
	require.NoError(t, err)
	var listed bool
	for _, listedAccount := range accounts {
		if listedAccount.AccountID == account.ID {
			listed = true
			assert.True(t, listedAccount.HasAdminRole, "the requested admin role stays")
			assert.True(t, listedAccount.HasUserRole, "the caregiver upgrade hands out the user role")
			assert.True(t, listedAccount.HasCaregiverProfile, "the caregiver upgrade creates the profile")
		}
	}
	assert.True(t, listed, "created account must be listed for the school")
	assert.Equal(t, 1, countOperatorAudit(t, db, "create", "account", account.ID))
}

// A step failing after registration rolls the account back (#3313): blank
// names pass registration and fail the school identity.
func TestOperatorProvisioningIntegration_CreateSchoolAccount_FailedIdentityLeavesNoAccount(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	factory := buildOperatorProvisioning(t, db)
	service := factory.OperatorProvisioning
	operatorID := testpkg.CreateTestOperator(t, db).ID
	ctx := provisioningContext(t, db)

	_, schoolID := provisionTestSchool(t, db, factory, operatorID, "rollback")

	input := schoolScopedInput(service.CreateSchoolAccount)
	input.Email = fmt.Sprintf("rolled-back-%d@example.test", schoolID)
	input.Password = "Provisioning-Test-9!"
	input.FirstName = " "
	input.LastName = " "
	account, err := service.CreateSchoolAccount(ctx, schoolID, operatorID, provisioningTestClientIP, input)
	requireProvisioningError(t, err, "InvalidProvisioningDataError")
	assert.Nil(t, account)

	accounts, err := db.NewSelect().
		TableExpr("auth.accounts").
		Where("email = ?", input.Email).
		Count(testpkg.Ctx(t))
	require.NoError(t, err)
	assert.Zero(t, accounts, "the registered account must roll back")

	var audited int
	audited, err = db.NewSelect().
		TableExpr("platform.operator_audit_log").
		Where("operator_id = ?", operatorID).
		Where("resource_type = ?", "account").
		Count(testpkg.Ctx(t))
	require.NoError(t, err)
	assert.Zero(t, audited, "a failed creation is not audited")
}

func TestOperatorProvisioningIntegration_SoftDeleteAndRestoreSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	factory := buildOperatorProvisioning(t, db)
	service := factory.OperatorProvisioning
	operatorID := testpkg.CreateTestOperator(t, db).ID
	ctx := provisioningContext(t, db)

	_, schoolID := provisionTestSchool(t, db, factory, operatorID, "trash")

	require.NoError(t, service.SoftDeleteSchool(ctx, schoolID, operatorID, provisioningTestClientIP))

	var deletedAt *string
	require.NoError(t, db.NewSelect().TableExpr("platform.schools").Column("deleted_at").
		Where("id = ?", schoolID).Scan(testpkg.Ctx(t), &deletedAt))
	assert.NotNil(t, deletedAt, "school must be soft-deleted")

	err := service.SoftDeleteSchool(ctx, schoolID, operatorID, provisioningTestClientIP)
	requireProvisioningError(t, err, "SchoolAlreadyDeletedError")

	_, err = service.ListSchoolDevices(ctx, schoolID)
	require.Error(t, err, "a deleted school's devices are not listable")

	require.NoError(t, service.RestoreSchool(ctx, schoolID, operatorID, provisioningTestClientIP))

	deletedAt = nil
	require.NoError(t, db.NewSelect().TableExpr("platform.schools").Column("deleted_at").
		Where("id = ?", schoolID).Scan(testpkg.Ctx(t), &deletedAt))
	assert.Nil(t, deletedAt, "school must be restored")

	err = service.RestoreSchool(ctx, schoolID, operatorID, provisioningTestClientIP)
	requireProvisioningError(t, err, "SchoolNotDeletedError")

	assert.Equal(t, 1, countOperatorAudit(t, db, "soft_delete", "school", schoolID))
	assert.Equal(t, 1, countOperatorAudit(t, db, "restore", "school", schoolID))
}

func TestOperatorProvisioningIntegration_TransferDevice_ArchivesSourceAndMovesKey(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	factory := buildOperatorProvisioning(t, db)
	service := factory.OperatorProvisioning
	operatorID := testpkg.CreateTestOperator(t, db).ID
	ctx := provisioningContext(t, db)
	dbCtx := testpkg.Ctx(t)

	organizationID, sourceSchoolID := provisionTestSchool(t, db, factory, operatorID, "source")

	targetInput := newInput(service.CreateSchool)
	targetInput.OrganizationID = organizationID
	targetInput.Name = fmt.Sprintf("Zielschule %d", sourceSchoolID)
	targetInput.Slug = fmt.Sprintf("ziel-%d", sourceSchoolID)
	targetInput.Subdomain = fmt.Sprintf("ziel-%d", sourceSchoolID)
	targetInput.Active = true
	targetSchool, err := service.CreateSchool(ctx, targetInput, operatorID, provisioningTestClientIP)
	require.NoError(t, err)
	targetSchoolID := targetSchool.ID
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = db.ExecContext(cleanupCtx, `DELETE FROM iot.devices WHERE tenant_id = ?`, targetSchoolID)
		_, _ = db.ExecContext(cleanupCtx, `DELETE FROM activities.categories WHERE tenant_id = ?`, targetSchoolID)
		_, _ = db.ExecContext(cleanupCtx, `DELETE FROM platform.schools WHERE id = ?`, targetSchoolID)
	})

	var roomID int64
	require.NoError(t, db.NewRaw(
		`INSERT INTO facilities.rooms (tenant_id, name, building, capacity) VALUES (?, ?, ?, ?) RETURNING id`,
		sourceSchoolID, fmt.Sprintf("Transferraum %d", sourceSchoolID), "Haupthaus", 20,
	).Scan(dbCtx, &roomID))
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM facilities.rooms WHERE id = ?`, roomID)
	})

	deviceName := "Terminal Flur"
	apiKey := fmt.Sprintf("transfer-key-%d", sourceSchoolID)
	deviceID := fmt.Sprintf("transfer-device-%d", sourceSchoolID)
	created, err := service.CreateDevice(ctx, sourceSchoolID, deviceID, "rfid_terminal", &deviceName, &apiKey, operatorID, provisioningTestClientIP)
	require.NoError(t, err)
	_, err = db.ExecContext(dbCtx, `UPDATE iot.devices SET room_id = ? WHERE id = ?`, roomID, created.ID)
	require.NoError(t, err)

	status, err := service.GetDeviceTransferStatus(ctx, created.ID)
	require.NoError(t, err)
	assert.True(t, status.CanTransfer, "an offline device without session is transferable")

	transferred, err := service.TransferDevice(ctx, created.ID, targetSchoolID, operatorID, provisioningTestClientIP)
	require.NoError(t, err)
	require.NotNil(t, transferred)
	assert.NotEqual(t, created.ID, transferred.ID, "transfer creates a new device row")
	assert.Equal(t, deviceID, transferred.DeviceID)
	assert.Equal(t, targetSchoolID, transferred.SchoolID)
	require.NotNil(t, transferred.Name)
	assert.Equal(t, deviceName, *transferred.Name)

	var source struct {
		ArchivedAt            *string `bun:"archived_at"`
		APIKey                *string `bun:"api_key"`
		Status                string  `bun:"status"`
		RoomID                *int64  `bun:"room_id"`
		TransferredToDeviceID *int64  `bun:"transferred_to_device_id"`
	}
	require.NoError(t, db.NewSelect().TableExpr("iot.devices").
		Column("archived_at", "api_key", "status", "room_id", "transferred_to_device_id").
		Where("id = ?", created.ID).Scan(dbCtx, &source))
	assert.NotNil(t, source.ArchivedAt, "source device must be archived")
	assert.Nil(t, source.APIKey, "source device must release its API key")
	assert.Equal(t, "inactive", source.Status)
	assert.Nil(t, source.RoomID, "source device must release the source school's room")
	require.NotNil(t, source.TransferredToDeviceID)
	assert.Equal(t, transferred.ID, *source.TransferredToDeviceID)

	var target struct {
		TenantID int64   `bun:"tenant_id"`
		APIKey   *string `bun:"api_key"`
		Status   string  `bun:"status"`
		RoomID   *int64  `bun:"room_id"`
	}
	require.NoError(t, db.NewSelect().TableExpr("iot.devices").
		Column("tenant_id", "api_key", "status", "room_id").
		Where("id = ?", transferred.ID).Scan(dbCtx, &target))
	assert.Equal(t, targetSchoolID, target.TenantID)
	require.NotNil(t, target.APIKey)
	assert.Equal(t, apiKey, *target.APIKey, "the kiosk keeps its API key")
	assert.Equal(t, "active", target.Status)
	assert.Nil(t, target.RoomID, "a room of the source school must not follow the device")

	sourceDevices, err := service.ListSchoolDevices(ctx, sourceSchoolID)
	require.NoError(t, err)
	for _, listed := range sourceDevices {
		assert.NotEqual(t, created.ID, listed.ID, "archived source must not be listed")
	}

	assert.Equal(t, 1, countOperatorAudit(t, db, "create", "device", created.ID))
	assert.Equal(t, 1, countOperatorAudit(t, db, "transfer", "device", created.ID))
}
