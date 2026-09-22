package staffintegration_test

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Inventory source literals rather than grep output: comments, the owner
// tables and migration compatibility SQL are not application callers (#2753).
// The one allowed literal is the table tag of the retained models/users.Staff
// DTO, which only test fixtures still bind; #2754 removes it with the view.
func TestStaffCutoverCallerInventory(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..", "..")
	files, literals, allowed := 0, 0, 0
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if relative == "cmd/backfill.go" {
			return nil // Operator-only backfill command, not an application caller.
		}
		if entry.IsDir() {
			if relative == "database/migrations" || relative == "test" || relative == "internal/architecture" || entry.Name() == "testdata" || entry.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		positions := token.NewFileSet()
		file, err := parser.ParseFile(positions, path, nil, 0)
		if err != nil {
			return err
		}
		files++
		ast.Inspect(file, func(node ast.Node) bool {
			literal, ok := node.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			value, err := strconv.Unquote(literal.Value)
			if err != nil {
				t.Fatal(err)
			}
			literals++
			if !referencesOldStaffStorage(value) {
				return true
			}
			if relative == "models/users/staff.go" && value == `bun:"schema:users,table:staff"` {
				allowed++
				return true
			}
			t.Errorf("old application table literal at %s: %q", positions.Position(literal.Pos()), value)
			return true
		})
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 1, allowed, "the retained DTO tag is the only documented exception")
	t.Logf("Inventory: %d application Go files, %d string literals, zero old application staff table callers", files, literals)
}

var staffStorageSQLComment = regexp.MustCompile(`(?m)^\s*--[^\n]*`)

// Application SQL names the schema, so the qualified name, a BUN table tag
// and the compatibility objects are the whole surface. An unqualified
// "staff" is ordinary prose in error and operation strings.
var staffStorageReference = regexp.MustCompile(`(?i)\busers\s*\.\s*staff(?:_legacy)?\b|\btable:staff(?:_legacy)?\b|\b(?:staff_legacy|staff_compatibility_reads|staff_compatibility_writes|route_staff_compatibility)\b`)

func referencesOldStaffStorage(value string) bool {
	value = staffStorageSQLComment.ReplaceAllString(value, "")
	value = strings.ReplaceAll(value, `"`, "")
	return staffStorageReference.MatchString(value)
}

func TestStaffStorageCallerInventoryRecognizesStorageNotOwnerTables(t *testing.T) {
	t.Parallel()
	for _, value := range []string{
		`SELECT * FROM "users"."staff"`, `UPDATE users.staff_legacy SET staff_notes = ''`,
		`bun:"schema:users,table:staff"`, `bun:"table:staff,alias:staff"`, `users.staff AS "staff"`,
		`staff_compatibility_reads`, `route_staff_compatibility`,
	} {
		assert.True(t, referencesOldStaffStorage(value), "missed retired storage: %s", value)
	}
	for _, value := range []string{
		`json:"staff"`, `users.staff_school_memberships AS "staff"`, `users.staff_employment_profiles`,
		`bun:"table:staff_school_memberships,alias:staff"`, `users.staff_messages`, `"staff".deleted_at IS NULL`,
		"-- users.staff is historical\nSELECT 42", "update staff absence", "RFID tag unassigned from staff",
	} {
		assert.False(t, referencesOldStaffStorage(value), "not a retired storage reference: %s", value)
	}
}

// TestStaffOwnersWorkWithoutCompatibilityView drops the rollback view in an
// isolated database: every current staff path has to keep working on the two
// owner tables alone.
func TestStaffOwnersWorkWithoutCompatibilityView(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.Ctx(t)
	_, err := db.ExecContext(ctx, `DROP VIEW users.staff`)
	require.NoError(t, err)
	membership, err := repositories.NewSchoolMembership(db)
	require.NoError(t, err)
	employment := repositories.MustNewStaffEmployment(db)
	workTime, err := repositories.NewWorkforce(db, membership)
	require.NoError(t, err)

	model, err := workTime.CreateWorkTimeModel(ctx, workforce.CreateWorkTimeModel{WorkTimeModelFields: workforce.WorkTimeModelFields{
		Name: "Ohne View", RotationLength: 1, RotationAnchorDate: "2026-01-05",
	}})
	require.NoError(t, err)
	person := testpkg.CreateTestPerson(t, db, "Owner", "Only")
	created, err := membership.CreateStaff(ctx, schoolmembership.CreateStaff{StaffFields: schoolmembership.StaffFields{
		PersonID: person.ID, StaffNotes: "Anfang", EmploymentType: testpkg.StrPtr(schoolmembership.EmploymentTypePartTime),
		WorkTimeModelID: &model.ID, PersonnelNumber: testpkg.StrPtr("OV-1"),
	}})
	require.NoError(t, err)

	found, err := membership.FindStaff(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "Anfang", found.StaffNotes)
	assert.Equal(t, &model.ID, found.WorkTimeModelID)
	listed, err := membership.ListStaff(ctx, schoolmembership.StaffFilter{PersonIDs: []int64{person.ID}})
	require.NoError(t, err)
	require.Len(t, listed, 1)

	_, err = membership.UpdateStaff(ctx, schoolmembership.UpdateStaff{ID: created.ID, StaffFields: schoolmembership.StaffFields{
		PersonID: person.ID, StaffNotes: "Anfang", WorkTimeModelID: &model.ID, BirthdayDisplayOptOut: true,
	}})
	require.NoError(t, err)
	_, err = employment.AppendStaffNotes(ctx, created.ID, "Ergänzt")
	require.NoError(t, err)
	_, err = workTime.UpdateWorkTimeModel(ctx, workforce.UpdateWorkTimeModel{ID: model.ID, WorkTimeModelFields: workforce.WorkTimeModelFields{
		Name: "Ohne View", RotationLength: 1, RotationAnchorDate: "2026-02-02",
	}})
	require.NoError(t, err)
	found, err = membership.FindStaff(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "Anfang\nErgänzt", found.StaffNotes)
	assert.True(t, found.BirthdayDisplayOptOut)
	assert.Equal(t, "2026-02-02", found.RotationAnchorDate)

	require.NoError(t, membership.DeleteStaff(ctx, created.ID))
	require.NoError(t, employment.ClearStaffWorkTimeModel(ctx, created.ID))
	require.NoError(t, workTime.DeleteWorkTimeModel(ctx, model.ID), "the retired staff member no longer holds the template")
}

func staffOwnerRows(t *testing.T, db *bun.DB, personID int64) (memberships, profiles int) {
	t.Helper()
	require.NoError(t, db.NewRaw(`SELECT
		(SELECT count(*) FROM users.staff_school_memberships WHERE person_id = ?0),
		(SELECT count(*) FROM users.staff_employment_profiles p JOIN users.staff_school_memberships m
			ON m.tenant_id = p.tenant_id AND m.id = p.membership_id WHERE m.person_id = ?0)`, personID).
		Scan(context.Background(), &memberships, &profiles))
	return memberships, profiles
}

// refuseEmploymentWrites makes the Workforce half of a staff write fail for one
// school: BEFORE fires after the membership command wrote its row, AFTER once
// the employment command wrote its own too.
func refuseEmploymentWrites(t *testing.T, db *bun.DB, tenantID int64, timing string) func() {
	t.Helper()
	ctx := context.Background()
	_, err := db.ExecContext(ctx, `CREATE OR REPLACE FUNCTION public.refuse_staff_employment() RETURNS trigger
		LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected owner write failure'; END $$`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, fmt.Sprintf(`CREATE TRIGGER refuse_staff_employment %s INSERT ON users.staff_employment_profiles
		FOR EACH ROW WHEN (NEW.tenant_id = %d) EXECUTE FUNCTION public.refuse_staff_employment()`, timing, tenantID))
	require.NoError(t, err)
	return func() {
		_, err := db.ExecContext(ctx, `DROP TRIGGER refuse_staff_employment ON users.staff_employment_profiles`)
		require.NoError(t, err)
	}
}

func TestStaffOwnerWritesRollBackWhenTheCallerCatchesTheFailure(t *testing.T) {
	t.Parallel()
	// One isolated database; the stages run in turn because the injected
	// trigger is table-wide DDL.
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.Ctx(t)
	membership, err := repositories.NewSchoolMembership(db)
	require.NoError(t, err)
	for _, stage := range []struct{ name, timing string }{{"after membership", "BEFORE"}, {"after employment", "AFTER"}} {
		person := testpkg.CreateTestPerson(t, db, "Atomic", stage.name)
		allow := refuseEmploymentWrites(t, db, testpkg.Tenant(t), stage.timing)

		// The caller swallows the error and commits its own transaction:
		// the command-local savepoint must still undo both owners.
		require.NoError(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
			_, createErr := membership.CreateStaff(txCtx, schoolmembership.CreateStaff{StaffFields: schoolmembership.StaffFields{
				PersonID: person.ID, StaffNotes: "verworfen",
			}})
			require.ErrorContains(t, createErr, "injected owner write failure", stage.name)
			return nil
		}))
		memberships, profiles := staffOwnerRows(t, db, person.ID)
		assert.Zero(t, memberships, "%s: no membership may outlive the failed staff write", stage.name)
		assert.Zero(t, profiles, stage.name)

		// The retry is clean and writes both owners.
		allow()
		created, err := membership.CreateStaff(ctx, schoolmembership.CreateStaff{StaffFields: schoolmembership.StaffFields{
			PersonID: person.ID, StaffNotes: "übernommen",
		}})
		require.NoError(t, err, stage.name)
		assert.Equal(t, "übernommen", created.StaffNotes)
		memberships, profiles = staffOwnerRows(t, db, person.ID)
		assert.Equal(t, 1, memberships, stage.name)
		assert.Equal(t, 1, profiles, stage.name)
	}
}

func TestStaffOwnerUpdateRollsBackBothOwnersWithTheOuterTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	membership, err := repositories.NewSchoolMembership(db)
	require.NoError(t, err)
	person := testpkg.CreateTestPerson(t, db, "Outer", "Rollback")
	created, err := membership.CreateStaff(ctx, schoolmembership.CreateStaff{StaffFields: schoolmembership.StaffFields{PersonID: person.ID, StaffNotes: "vorher"}})
	require.NoError(t, err)

	outer := errors.New("later owner command failed")
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		_, updateErr := membership.UpdateStaff(txCtx, schoolmembership.UpdateStaff{ID: created.ID, StaffFields: schoolmembership.StaffFields{
			PersonID: person.ID, StaffNotes: "nachher", EmploymentType: testpkg.StrPtr(schoolmembership.EmploymentTypeMinijob),
		}})
		require.NoError(t, updateErr)
		return outer
	})
	require.ErrorIs(t, err, outer)
	found, err := membership.FindStaff(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "vorher", found.StaffNotes)
	assert.Nil(t, found.EmploymentType)
	assert.Equal(t, created.UpdatedAt, found.UpdatedAt, "the membership write rolled back with the profile")
}

func TestStaffOwnersClassifyAPersonnelNumberConflict(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	membership, err := repositories.NewSchoolMembership(db)
	require.NoError(t, err)
	holder := testpkg.CreateTestPerson(t, db, "Number", "Holder")
	_, err = membership.CreateStaff(ctx, schoolmembership.CreateStaff{StaffFields: schoolmembership.StaffFields{PersonID: holder.ID, PersonnelNumber: testpkg.StrPtr("PN-7")}})
	require.NoError(t, err)

	second := testpkg.CreateTestPerson(t, db, "Number", "Clash")
	_, err = membership.CreateStaff(ctx, schoolmembership.CreateStaff{StaffFields: schoolmembership.StaffFields{PersonID: second.ID, PersonnelNumber: testpkg.StrPtr("PN-7")}})
	require.ErrorIs(t, err, schoolmembership.ErrPersonnelNumberConflict)
	memberships, _ := staffOwnerRows(t, db, second.ID)
	assert.Zero(t, memberships, "the refused number takes the membership with it")
}

func TestStaffOwnersAreTenantIsolated(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	membership, err := repositories.NewSchoolMembership(db)
	require.NoError(t, err)
	employment := repositories.MustNewStaffEmployment(db)
	person := testpkg.CreateTestPerson(t, db, "Tenant", "Bound")
	created, err := membership.CreateStaff(ctx, schoolmembership.CreateStaff{StaffFields: schoolmembership.StaffFields{PersonID: person.ID, StaffNotes: "vertraulich"}})
	require.NoError(t, err)

	otherTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenantID)
	otherCtx := testpkg.TenantContext(otherTenantID)

	_, err = membership.FindStaff(otherCtx, created.ID)
	require.ErrorIs(t, err, schoolmembership.ErrStaffNotFound)
	listed, err := membership.ListStaff(otherCtx, schoolmembership.StaffFilter{IDs: []int64{created.ID}, IncludeDeleted: true})
	require.NoError(t, err)
	assert.Empty(t, listed)
	_, err = membership.UpdateStaff(otherCtx, schoolmembership.UpdateStaff{ID: created.ID, StaffFields: schoolmembership.StaffFields{PersonID: person.ID}})
	require.ErrorIs(t, err, schoolmembership.ErrStaffNotFound)
	profiles, err := employment.StaffEmployments(otherCtx, []int64{created.ID})
	require.NoError(t, err)
	assert.Empty(t, profiles)
	require.ErrorIs(t, employment.SaveStaffEmployment(otherCtx, workforce.StaffEmployment{MembershipID: created.ID}), workforce.ErrStaffEmploymentNotFound)

	found, err := membership.FindStaff(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "vertraulich", found.StaffNotes)
}
