package integration

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	importModels "github.com/moto-nrw/project-phoenix/models/import"
	"github.com/moto-nrw/project-phoenix/services"
	importService "github.com/moto-nrw/project-phoenix/services/import"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Exercise real owner writes through the production batch entry point.
// SQL failures must roll back every preceding owner write and the checkpoint.
func TestDataImportCutover_StudentOwnerFailuresRollbackAndReplay(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	for _, table := range []string{
		"users.persons", "users.students", "users.privacy_consents",
		"users.guardian_profiles", "users.guardian_phone_numbers", "users.students_guardians",
		"schedule.student_arrival_schedules", "schedule.student_pickup_schedules",
		"audit.student_consent_changes",
	} {
		t.Run(table, func(t *testing.T) {
			tenantID := testpkg.OwnTenant(t)
			module, err := services.NewImportTestModule(db, testpkg.TenantRuntime(t, db))
			require.NoError(t, err)
			actor := newImporter(t, db)
			before := countImportRows(t, db, tenantID)
			row := importModels.StudentImportRow{
				FirstName: "Mila", LastName: "Ownerfailure", SchoolClass: "1A",
				PrivacyAccepted: true, DataRetentionDays: 30, AGBAcceptedAt: "2026-08-01",
				Guardians: []importModels.GuardianImportData{{
					FirstName: "Karin", LastName: "Ownerfailure", Email: "ownerfailure@example.test", RelationshipType: "Mutter",
					PhoneNumbers: []importModels.PhoneImportData{{PhoneNumber: "0171 1234567", PhoneType: "mobile", IsPrimary: true}},
				}},
				ArrivalSchedules: []importModels.ArrivalScheduleImportData{{Weekday: 1, ExpectedArrival: "08:00"}},
				PickupSchedules:  []importModels.PickupScheduleImportData{{Weekday: 2, PickupTime: "15:30"}},
			}
			removeFailure := refuseOwnerInsert(t, db, table)
			ctx := tenant.WithUnitOfWork(testpkg.Ctx(t), testpkg.TenantRuntime(t, db))
			run := func() (*importModels.ImportResult[importModels.StudentImportRow], error) {
				return module.Import.ImportBatches(ctx, importModels.ImportRequest[importModels.StudentImportRow]{Rows: []importModels.StudentImportRow{row}, Mode: importModels.ImportModeCreate, UserID: actor.staffID, SkipInvalidRows: true}, importService.BatchAudit{EntityType: "student", Filename: "failure.csv", AccountID: actor.accountID})
			}
			result, err := run()
			require.Error(t, err)
			require.Equal(t, 1, result.ErrorCount)
			assert.Zero(t, result.CreatedCount)
			require.Len(t, result.Errors, 1)
			assert.Equal(t, 2, result.Errors[0].RowNumber)
			require.NotEmpty(t, result.Errors[0].Errors)
			var blocking []importModels.ValidationError
			for _, rowError := range result.Errors[0].Errors {
				if rowError.Severity == importModels.ErrorSeverityError {
					blocking = append(blocking, rowError)
				}
			}
			require.Len(t, blocking, 1)
			assert.Equal(t, "creation_failed", blocking[0].Code)
			assert.Contains(t, blocking[0].Message, "injected owner write failure")
			after := countImportRows(t, db, tenantID)
			assert.Equal(t, before, after, "no earlier owner write may escape the failed row")

			removeFailure()
			replay, err := run()
			require.NoError(t, err)
			requireNoRowErrors(t, replay)
			assert.Equal(t, 1, replay.CreatedCount, "rollback must not leave stale matching state")
			assert.Equal(t, before.students+1, countImportRows(t, db, tenantID).students)
		})
	}
}

func TestDataImportCutover_StaffOwnerFailuresRollbackAndReplay(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	tables := []string{"users.persons", "users.staff", "users.teachers", "users.staff_master_data", "users.staff_qualifications", "auth.invitation_tokens"}
	for _, table := range tables {
		t.Run(table, func(t *testing.T) {
			tenantID := testpkg.OwnTenant(t)
			module, err := services.NewImportTestModule(db, testpkg.TenantRuntime(t, db))
			require.NoError(t, err)
			actor := newImporter(t, db)
			role := testpkg.CreateTestRoleForTenant(t, db, "Betreuungskraft", tenantID)
			counts := func() []int {
				t.Helper()
				result := make([]int, len(tables))
				for i, name := range tables {
					require.NoError(t, db.NewSelect().TableExpr(name).ColumnExpr("count(*)").Where("tenant_id = ?", tenantID).Scan(testpkg.Ctx(t), &result[i]))
				}
				return result
			}
			run := func() (*importModels.ImportResult[importModels.StaffImportRow], error) {
				t.Helper()
				ctx := tenant.WithUnitOfWork(testpkg.Ctx(t), testpkg.TenantRuntime(t, db))
				ctx = importService.ContextWithImporterPermissions(ctx, []string{"admin:*"})
				return module.StaffImport.ImportBatches(ctx, importModels.ImportRequest[importModels.StaffImportRow]{
					Rows: []importModels.StaffImportRow{{
						FirstName: "Anna", LastName: "Ownerfailure", RoleName: role.Name, Position: "Gruppenleitung", Email: "staff-ownerfailure@example.test",
						PersonnelNumber: "P-2708", WeeklyHours: "19,5", Qualifications: "Erste Hilfe",
					}},
					Mode: importModels.ImportModeCreate, UserID: actor.accountID, SkipInvalidRows: true,
				}, importService.BatchAudit{EntityType: "staff", Filename: "failure.csv", AccountID: actor.accountID})
			}
			before := counts()
			removeFailure := refuseOwnerInsert(t, db, table)
			result, err := run()
			require.Error(t, err)
			require.Equal(t, 1, result.ErrorCount)
			assert.Zero(t, result.CreatedCount)
			require.Len(t, result.Errors, 1)
			assert.Equal(t, 2, result.Errors[0].RowNumber)
			require.NotEmpty(t, result.Errors[0].Errors)
			var blocking []importModels.ValidationError
			for _, rowError := range result.Errors[0].Errors {
				if rowError.Severity == importModels.ErrorSeverityError {
					blocking = append(blocking, rowError)
				}
			}
			require.Len(t, blocking, 1)
			assert.Equal(t, "creation_failed", blocking[0].Code)
			assert.Contains(t, blocking[0].Message, "injected owner write failure")
			assert.Equal(t, before, counts(), "all earlier owner writes must roll back")
			removeFailure()
			replay, err := run()
			require.NoError(t, err)
			require.Zero(t, replay.ErrorCount, "%+v", replay.Errors)
			assert.Equal(t, 1, replay.CreatedCount)
			for i, count := range counts() {
				assert.Equal(t, before[i]+1, count, "%s is created exactly once after retry", tables[i])
			}
		})
	}
}

// table is selected from the test's fixed owner-table list, never input data.
// The function and trigger exist only in this test's isolated database clone.
func refuseOwnerInsert(t *testing.T, db *bun.DB, table string) func() {
	t.Helper()
	ctx := context.Background()
	_, err := db.ExecContext(ctx, `CREATE OR REPLACE FUNCTION public.refuse_import_owner_write() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected owner write failure' USING ERRCODE = '23514'; END $$`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, fmt.Sprintf(`CREATE TRIGGER refuse_import_owner_write AFTER INSERT ON %s FOR EACH ROW EXECUTE FUNCTION public.refuse_import_owner_write()`, table))
	require.NoError(t, err)
	remove := func() {
		t.Helper()
		_, err := db.ExecContext(ctx, fmt.Sprintf(`DROP TRIGGER IF EXISTS refuse_import_owner_write ON %s`, table))
		require.NoError(t, err)
	}
	t.Cleanup(remove)
	return remove
}
