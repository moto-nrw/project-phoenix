package careplanhttp_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/api"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cohortRosterFixture holds the templates and occurrences one cohort test
// plans around a fixed Monday, 2026-08-24.
type cohortRosterFixture struct {
	tc       *testContext
	periodID int64
	roomID   int64
}

// targetedTemplate creates a Regeltermin with one dynamic target; column and
// value name the target ('klasse' → target_school_class and so on).
func (f *cohortRosterFixture) targetedTemplate(t *testing.T, name, targetType, column string, value any) int64 {
	t.Helper()
	template := testpkg.CreateTestActivityGroup(t, f.tc.db, fmt.Sprintf("%s-%d", name, time.Now().UnixNano()))
	ctx := testpkg.Ctx(t)
	_, err := f.tc.db.NewUpdate().TableExpr("activities.groups").Set("is_template = true").Set("calendar_period_id = ?", f.periodID).
		Where("id = ?", template.ID).Exec(ctx)
	require.NoError(t, err)
	_, err = f.tc.db.NewRaw(fmt.Sprintf(`INSERT INTO activities.group_targets (tenant_id, activity_group_id, target_group_type, %s)
		VALUES (?, ?, ?, ?)`, column), testpkg.Tenant(t), template.ID, targetType, value).Exec(ctx)
	require.NoError(t, err)
	return template.ID
}

// occurrence materializes one planned occurrence of the template.
func (f *cohortRosterFixture) occurrence(t *testing.T, templateID int64, day int) int64 {
	t.Helper()
	instance := testpkg.CreateTestActivityInstance(t, f.tc.db, testpkg.Date(2026, time.August, day), f.roomID, testpkg.ActivityInstanceOpts{
		ActivityGroupID: &templateID, CalendarPeriodID: &f.periodID,
	})
	return instance.ID
}

func (f *cohortRosterFixture) roster(t *testing.T, instanceID int64) []int64 {
	t.Helper()
	var ids []int64
	require.NoError(t, f.tc.db.NewRaw(`SELECT student_id FROM schedule.instance_students WHERE instance_id = ? ORDER BY student_id`, instanceID).
		Scan(testpkg.Ctx(t), &ids))
	return ids
}

// TestUpdateStudent_CohortChangeFollowsTargetedTemplates pins #6841: a child
// who changes Klasse or Gruppe leaves the already materialized future
// occurrences of the Regeltermine whose target they left and joins those of
// the target they entered, in the same request. History, rows recording a
// hand decision, classmates and templates whose target the child still
// matches stay as they are.
func TestUpdateStudent_CohortChangeFollowsTargetedTemplates(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t, fixedCalendarClock)
	follow, err := api.NewStudentTestCohortRosters(testpkg.NewDatabase(t, tc.db))
	require.NoError(t, err)
	tc.resource.FollowCohortChanges = follow
	recurrenceLock, err := api.NewTimetableHTTPTestRecurrenceLock(testpkg.NewDatabase(t, tc.db))
	require.NoError(t, err)
	tc.resource.LockTemplateRecurrence = recurrenceLock.LockRecurrenceWrites

	f := &cohortRosterFixture{tc: tc}
	f.periodID = testpkg.CreateTestCalendarPeriod(t, tc.db, "Schuljahr Kohorte", testpkg.Date(2026, time.August, 1), testpkg.Date(2027, time.July, 31)).ID
	f.roomID = testpkg.CreateTestRoom(t, tc.db, "Kohorte").ID
	group := testpkg.CreateTestEducationGroup(t, tc.db, fmt.Sprintf("Füchse-%d", time.Now().UnixNano()))
	child := testpkg.CreateTestStudent(t, tc.db, "Kohorte", "Wechsel", "3a")
	classmate := testpkg.CreateTestStudent(t, tc.db, "Kohorte", "Bleibt", "3a")

	class3a := f.targetedTemplate(t, "AG 3a", "klasse", "target_school_class", "3a")
	class3b := f.targetedTemplate(t, "AG 3b", "klasse", "target_school_class", "3b")
	grade3 := f.targetedTemplate(t, "AG Jahrgang 3", "jahrgang", "target_grade_level", 3)
	foxes := f.targetedTemplate(t, "AG Füchse", "gruppe", "education_group_id", group.ID)

	past3a := f.occurrence(t, class3a, 21)
	testpkg.CreateTestInstanceStudent(t, tc.db, past3a, child.ID, "")
	next3a := f.occurrence(t, class3a, 25)
	testpkg.CreateTestInstanceStudent(t, tc.db, next3a, child.ID, "")
	testpkg.CreateTestInstanceStudent(t, tc.db, next3a, classmate.ID, "")
	decided3a := f.occurrence(t, class3a, 26)
	marked := fixedCalendarClock()
	testpkg.CreateTestInstanceStudent(t, tc.db, decided3a, child.ID, "", testpkg.InstanceStudentOpts{ManualStatusAt: &marked})
	next3b := f.occurrence(t, class3b, 25)
	// Staff removed the child from this Jahrgang occurrence by hand: the
	// class change keeps the child in Jahrgang 3, so the removal stands.
	handRemovedGrade3 := f.occurrence(t, grade3, 25)
	nextFoxes := f.occurrence(t, foxes, 25)

	putStudent(t, tc, child.ID, map[string]any{"school_class": "3b"})

	assert.Equal(t, []int64{child.ID}, f.roster(t, past3a), "history stays untouched")
	assert.Equal(t, []int64{classmate.ID}, f.roster(t, next3a), "the child leaves the future 3a occurrence, the classmate stays")
	assert.Equal(t, []int64{child.ID}, f.roster(t, decided3a), "a row with a hand decision is no longer a plan and stays")
	assert.Equal(t, []int64{child.ID}, f.roster(t, next3b), "the child joins the future 3b occurrence")
	assert.Empty(t, f.roster(t, handRemovedGrade3), "the Jahrgang target did not change, so a hand removal stands")
	assert.Empty(t, f.roster(t, nextFoxes), "the child is not in the Gruppe yet")

	putStudent(t, tc, child.ID, map[string]any{"group_id": group.ID})
	assert.Equal(t, []int64{child.ID}, f.roster(t, nextFoxes), "a Gruppe change reaches the Gruppe occurrence too")

	// Staff take the child off the 3b occurrence; an unchanged resubmission
	// of the class must not bring them back.
	_, err = tc.db.NewRaw(`DELETE FROM schedule.instance_students WHERE instance_id = ? AND student_id = ?`, next3b, child.ID).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	putStudent(t, tc, child.ID, map[string]any{"school_class": "3b"})
	assert.Empty(t, f.roster(t, next3b), "resubmitting the unchanged class keeps the hand removal")
}
