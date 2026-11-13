package contracttest_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/api"
	"github.com/moto-ogs/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-ogs/project-phoenix/modules/careplan/compose"
	"github.com/moto-ogs/project-phoenix/modules/careplan/masterdatarequests"
	"github.com/moto-ogs/project-phoenix/modules/careplan/parentrequests"
	"github.com/moto-ogs/project-phoenix/modules/peopledirectory"
	timezone "github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// classRosterFixture plans Regeltermine with a Klasse target around a fixed
// Monday, 2026-08-24, for the Stammdaten child (Klasse 1a).
type classRosterFixture struct {
	masterDataFixture
	periodID int64
	roomID   int64
	past1a   int64
	next1a   int64
	next2b   int64
}

func newClassRosterFixture(t *testing.T) classRosterFixture {
	t.Helper()
	f := classRosterFixture{masterDataFixture: newMasterDataFixture(t, testpkg.SetupTestDB(t))}
	require.Equal(t, testpkg.Tenant(t), f.chain.TenantID)
	f.periodID = testpkg.CreateTestCalendarPeriod(t, f.db, "Schuljahr Stammdaten", testpkg.Date(2026, time.August, 1), testpkg.Date(2027, time.July, 31)).ID
	f.roomID = testpkg.CreateTestRoom(t, f.db, "Stammdaten").ID
	class1a, class2b := f.classTemplate(t, "1a"), f.classTemplate(t, "2b")
	f.past1a, f.next1a, f.next2b = f.occurrence(t, class1a, 21), f.occurrence(t, class1a, 25), f.occurrence(t, class2b, 25)
	testpkg.CreateTestInstanceStudent(t, f.db, f.past1a, f.chain.StudentID, "")
	testpkg.CreateTestInstanceStudent(t, f.db, f.next1a, f.chain.StudentID, "")
	return f
}

func (f classRosterFixture) classTemplate(t *testing.T, class string) int64 {
	t.Helper()
	template := testpkg.CreateTestActivityGroup(t, f.db, fmt.Sprintf("AG %s-%d", class, time.Now().UnixNano()))
	ctx := testpkg.Ctx(t)
	_, err := f.db.NewUpdate().TableExpr("activities.groups").Set("is_template = true").Set("calendar_period_id = ?", f.periodID).
		Where("id = ?", template.ID).Exec(ctx)
	require.NoError(t, err)
	_, err = f.db.NewRaw(`INSERT INTO activities.group_targets (tenant_id, activity_group_id, target_group_type, target_school_class)
		VALUES (?, ?, 'klasse', ?)`, f.chain.TenantID, template.ID, class).Exec(ctx)
	require.NoError(t, err)
	return template.ID
}

func (f classRosterFixture) occurrence(t *testing.T, templateID int64, day int) int64 {
	t.Helper()
	return testpkg.CreateTestActivityInstance(t, f.db, testpkg.Date(2026, time.August, day), f.roomID, testpkg.ActivityInstanceOpts{
		ActivityGroupID: &templateID, CalendarPeriodID: &f.periodID,
	}).ID
}

func (f classRosterFixture) roster(t *testing.T, instanceID int64) []int64 {
	t.Helper()
	var ids []int64
	require.NoError(t, f.db.NewRaw(`SELECT student_id FROM schedule.instance_students WHERE instance_id = ? ORDER BY student_id`, instanceID).
		Scan(testpkg.Ctx(t), &ids))
	return ids
}

// decisions binds the class follow-up the production root binds and the
// fixed school day.
func (f classRosterFixture) decisions(t *testing.T) *careplanCompose.MasterDataDecisions {
	t.Helper()
	classChanges, err := api.NewMasterDataTestClassChanges(testpkg.NewDatabase(t, f.db))
	require.NoError(t, err)
	return f.masterDataFixture.decisions(t, func(o *api.MasterDataDecisionTestOptions, _ *api.MasterDataDecisionTestPeople) {
		o.ClassChanges = classChanges
		o.Today = func() timezone.Date { return timezone.NewDate(2026, time.August, 24) }
	})
}

func (f classRosterFixture) assertFollowed(t *testing.T) {
	t.Helper()
	child := f.chain.StudentID
	assert.Equal(t, []int64{child}, f.roster(t, f.past1a), "history stays untouched")
	assert.Empty(t, f.roster(t, f.next1a), "the child leaves the future 1a occurrence")
	assert.Equal(t, []int64{child}, f.roster(t, f.next2b), "the child joins the future 2b occurrence")
}

// TestMasterDataReview_ClassApprovalFollowsTargetedTemplates pins #6951: an
// approved Klasse moves the child between the planned occurrences of the
// Regeltermine with a Klasse target in the decision's transaction.
func TestMasterDataReview_ClassApprovalFollowsTargetedTemplates(t *testing.T) {
	t.Parallel()
	f := newClassRosterFixture(t)
	row := f.insert(t, peopledirectory.DataChangeTargetStudent, "school_class", `"1a"`, `"2b"`)

	_, err := f.decide(t, f.decisions(t), masterdatarequests.DecideInput{RequestID: row.ID, Approve: true, ReviewedBy: f.chain.AccountID})
	require.NoError(t, err)

	f.assertFollowed(t)
}

// TestParentRequestCoordinator_BulkClassApprovalFollowsTargetedTemplates
// pins the same for a bulk approval (#6951).
func TestParentRequestCoordinator_BulkClassApprovalFollowsTargetedTemplates(t *testing.T) {
	t.Parallel()
	f := newClassRosterFixture(t)
	excused, err := api.NewTestExcusedAbsenceRequests(api.ExcusedRequestTestOptions{
		CarePlan: f.reposCare.CarePlan(), Logger: slog.Default(),
	}, api.ExcusedRequestTestPeople{Students: f.repos.Student, Persons: f.repos.Person}, nil, nil)
	require.NoError(t, err)
	coordinator, err := careplanCompose.NewParentRequestCoordinator(careplanCompose.ParentRequestCoordinatorDependencies{
		Rights: func(context.Context) careplanCompose.ParentRequestRights {
			return careplanCompose.ParentRequestRights{WriteQueues: true, Absences: true}
		},
		MasterData: f.decisions(t), Excused: excused, Care: unusedConflictPort{}, Offering: unusedConflictPort{},
	})
	require.NoError(t, err)
	row := f.insert(t, peopledirectory.DataChangeTargetStudent, "school_class", `"1a"`, `"2b"`)
	rename := f.insert(t, peopledirectory.DataChangeTargetPerson, "first_name", `"Felix"`, `"Max"`)

	require.NoError(t, f.inTx(t, func(ctx context.Context) error {
		return coordinator.BulkApprove(ctx, parentrequests.BulkApproveInput{
			Requests: []parentrequests.Ref{
				{Kind: parentrequests.KindMasterData, ID: row.ID, ExpectedVersion: careplan.ParentRequestVersion(row.UpdatedAt)},
				{Kind: parentrequests.KindMasterData, ID: rename.ID, ExpectedVersion: careplan.ParentRequestVersion(rename.UpdatedAt)},
			},
			Reason: "Gemeinsam geprüft", ReviewerID: f.chain.AccountID,
		})
	}))

	f.assertFollowed(t)
}
