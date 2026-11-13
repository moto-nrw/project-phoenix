package integration

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/api"
	importModels "github.com/moto-ogs/project-phoenix/modules/dataimport"
	timezone "github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// TestImportBatches_UpdateFollowsTargetedTemplates pins #6951: an update
// import that changes a child's Klasse and Gruppe moves the child between
// the planned occurrences of the Regeltermine with a Klasse or Gruppe
// target in the import's transaction, as the student edit does (#6841).
func TestImportBatches_UpdateFollowsTargetedTemplates(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.OwnTenant(t)
	module, err := api.NewImportTestModuleWithOptions(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), api.ImportTestOptions{
		FollowCohorts: true, Today: func() timezone.Date { return timezone.NewDate(2026, time.August, 24) },
	})
	require.NoError(t, err)
	actor := newImporter(t, db)
	ctx := testpkg.Ctx(t)

	periodID := testpkg.CreateTestCalendarPeriod(t, db, "Schuljahr Import", testpkg.Date(2026, time.August, 1), testpkg.Date(2027, time.July, 31)).ID
	roomID := testpkg.CreateTestRoom(t, db, "Import").ID
	foxes := testpkg.CreateTestEducationGroup(t, db, fmt.Sprintf("Füchse-%d", time.Now().UnixNano()))
	template := func(column string, value any) int64 {
		t.Helper()
		group := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("AG %v-%d", value, time.Now().UnixNano()))
		_, err := db.NewUpdate().TableExpr("activities.groups").Set("is_template = true").Set("calendar_period_id = ?", periodID).
			Where("id = ?", group.ID).Exec(ctx)
		require.NoError(t, err)
		targetType := map[string]string{"target_school_class": "klasse", "education_group_id": "gruppe"}[column]
		_, err = db.NewRaw(fmt.Sprintf(`INSERT INTO activities.group_targets (tenant_id, activity_group_id, target_group_type, %s)
			VALUES (?, ?, ?, ?)`, column), tenantID, group.ID, targetType, value).Exec(ctx)
		require.NoError(t, err)
		return group.ID
	}
	occurrence := func(templateID int64) int64 {
		t.Helper()
		return testpkg.CreateTestActivityInstance(t, db, testpkg.Date(2026, time.August, 25), roomID, testpkg.ActivityInstanceOpts{
			ActivityGroupID: &templateID, CalendarPeriodID: &periodID,
		}).ID
	}
	roster := func(instanceID int64) []int64 {
		t.Helper()
		var ids []int64
		require.NoError(t, db.NewRaw(`SELECT student_id FROM schedule.instance_students WHERE instance_id = ? ORDER BY student_id`, instanceID).Scan(ctx, &ids))
		return ids
	}

	row := importModels.StudentImportRow{FirstName: "Kohorte", LastName: "Import", Birthday: "2018-03-04", SchoolClass: "1a", DataRetentionDays: 30}
	result, err := runStudentImport(t, db, module, actor, importModels.ImportModeCreate, []importModels.StudentImportRow{row}, nil)
	require.NoError(t, err)
	requireNoRowErrors(t, result)
	var childID int64
	require.NoError(t, db.NewSelect().TableExpr("users.student_profiles AS s").Column("s.id").
		Join("JOIN users.persons p ON p.id = s.person_id").Where("s.tenant_id = ? AND p.last_name = ?", tenantID, "Import").Scan(ctx, &childID))

	next1a := occurrence(template("target_school_class", "1a"))
	testpkg.CreateTestInstanceStudent(t, db, next1a, childID, "")
	next2b := occurrence(template("target_school_class", "2b"))
	nextFoxes := occurrence(template("education_group_id", foxes.ID))

	row.SchoolClass, row.GroupName = "2b", foxes.Name
	audit := importModels.BatchAudit{EntityType: "student", Filename: "klassenwechsel.csv", AccountID: actor.accountID}
	result, err = module.Import.ImportBatches(ctx, importModels.ImportRequest[importModels.StudentImportRow]{
		Rows: []importModels.StudentImportRow{row}, Mode: importModels.ImportModeUpdate, UserID: actor.staffID, SkipInvalidRows: true,
	}, audit)
	require.NoError(t, err)
	requireNoRowErrors(t, result)
	require.Equal(t, 1, result.UpdatedCount)

	assert.Empty(t, roster(next1a), "the child leaves the future 1a occurrence")
	assert.Equal(t, []int64{childID}, roster(next2b), "the child joins the future 2b occurrence")
	assert.Equal(t, []int64{childID}, roster(nextFoxes), "the child joins the future Gruppe occurrence")
}
