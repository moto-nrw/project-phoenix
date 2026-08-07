// Hermetic integration tests for the #2187 "Nachzügler" roster reconciliation:
// a roster change saved on the living segment with series_roster_from = D must
// also be true on the occurrences of an already-capped predecessor segment
// from D on — both in activities.student_enrollments / activities.supervisors
// AND on the occurrences that were already materialized from that predecessor.
//
// Chain shape (makeSeriesChain): T → S1[b1, b2) → S2[b2, ∞), with S1's
// occurrences materialized. The anchor D is a scheduled day inside [b1, b2).
package schedule_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rosterChain is a seriesChain whose middle segment already has materialized
// occurrences, plus the two dates the assertions use: `anchor` is the day the
// roster change takes effect from, `earlier` a scheduled day before it.
type rosterChain struct {
	*seriesChain
	anchor  timezone.Date
	earlier timezone.Date
}

func makeRosterChain(t *testing.T, weekdays []int) *rosterChain {
	t.Helper()
	firstBoundary := futureMonday(1)
	secondBoundary := futureMonday(3)
	chain := makeSeriesChain(t, weekdays, firstBoundary, secondBoundary)
	// Materialize the middle segment's whole window (exclusive end), so every
	// occurrence on the grid belongs to the bounded predecessor.
	_, err := chain.svc.MaterializeForTenant(
		chain.ctx, firstBoundary, secondBoundary.AddDays(-1), scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	return &rosterChain{
		seriesChain: chain,
		anchor:      firstBoundary.AddDays(7),
		earlier:     firstBoundary,
	}
}

// livingUpdateInput builds the service-level equivalent of a PUT on the
// chain's living segment: same fields, same timeframe, the given roster.
func (c *seriesChain) livingUpdateInput(
	t *testing.T,
	studentIDs, staffIDs []int64,
	primaryStaffID *int64,
) scheduleSvc.TemplateUpdateInput {
	t.Helper()
	group := reloadSplitGroup(t, c.scenarioSetup, c.livingID)
	schedules := loadSplitSchedules(t, c.scenarioSetup, c.livingID)
	require.NotEmpty(t, schedules)
	require.NotNil(t, schedules[0].TimeframeID)
	return scheduleSvc.TemplateUpdateInput{
		TemplateID: c.livingID,
		Fields: activitiesModels.TemplateFieldsUpdate{
			Name:            group.Name,
			Type:            group.Type,
			CategoryID:      group.CategoryID,
			RoomID:          c.roomID,
			MaxParticipants: group.MaxParticipants,
			TargetGroupType: group.TargetGroupType,
		},
		Weekdays:        c.weekdays,
		TimeframeID:     *schedules[0].TimeframeID,
		RosterValidFrom: c.secondBoundary,
		GradeLevelMax:   4,
		StudentIDs:      studentIDs,
		StaffIDs:        staffIDs,
		PrimaryStaffID:  primaryStaffID,
	}
}

// addStaff creates one extra staff member for the chain's tenant.
func (c *seriesChain) addStaff(t *testing.T, label string) int64 {
	t.Helper()
	staff := testpkg.CreateTestStaffForTenant(t, c.db, c.tenantID, "Extra",
		fmt.Sprintf("%s-%d", label, time.Now().UnixNano()))
	c.extraCleanups = append(c.extraCleanups, func() {
		testpkg.CleanupTableRecords(t, c.db, "users.staff", staff.ID)
		testpkg.CleanupTableRecords(t, c.db, "users.persons", staff.PersonID)
	})
	return staff.ID
}

func enrollmentsFor(t *testing.T, s *scenarioSetup, groupID, studentID int64) []*activitiesModels.StudentEnrollment {
	t.Helper()
	var out []*activitiesModels.StudentEnrollment
	for _, row := range loadSplitEnrollments(t, s, groupID) {
		if row.StudentID == studentID {
			out = append(out, row)
		}
	}
	return out
}

func supervisorsFor(t *testing.T, s *scenarioSetup, groupID, staffID int64) []*activitiesModels.SupervisorPlanned {
	t.Helper()
	var out []*activitiesModels.SupervisorPlanned
	for _, row := range loadSplitSupervisors(t, s, groupID) {
		if row.StaffID == staffID {
			out = append(out, row)
		}
	}
	return out
}

// instanceStudentIDsOn returns the students planned on the group's occurrence
// of that date (empty when the occurrence has no roster row at all).
func instanceStudentIDsOn(t *testing.T, s *scenarioSetup, groupID int64, date timezone.Date) []int64 {
	t.Helper()
	ids := make([]int64, 0)
	err := s.db.NewSelect().
		TableExpr(`schedule.instance_students AS "row"`).
		ColumnExpr(`"row".student_id`).
		Join(`JOIN schedule.activity_instances AS "inst" ON "inst".id = "row".instance_id`).
		Where(`"inst".activity_group_id = ?`, groupID).
		Where(`"inst".date = ?`, date).
		Where(`"inst".tenant_id = ?`, s.tenantID).
		OrderExpr(`"row".student_id ASC`).
		Scan(s.ctx, &ids)
	require.NoError(t, err)
	return ids
}

func instanceStaffIDsOn(t *testing.T, s *scenarioSetup, groupID int64, date timezone.Date) []int64 {
	t.Helper()
	ids := make([]int64, 0)
	err := s.db.NewSelect().
		TableExpr(`schedule.instance_staff AS "row"`).
		ColumnExpr(`"row".staff_id`).
		Join(`JOIN schedule.activity_instances AS "inst" ON "inst".id = "row".instance_id`).
		Where(`"inst".activity_group_id = ?`, groupID).
		Where(`"inst".date = ?`, date).
		Where(`"inst".tenant_id = ?`, s.tenantID).
		OrderExpr(`"row".staff_id ASC`).
		Scan(s.ctx, &ids)
	require.NoError(t, err)
	return ids
}

// deleteInstanceStudentRow removes ONE student's row from ONE occurrence, the
// way a supervisor does it by hand in the planner.
func deleteInstanceStudentRow(t *testing.T, s *scenarioSetup, groupID, studentID int64, date timezone.Date) {
	t.Helper()
	res, err := s.db.NewDelete().
		TableExpr(`schedule.instance_students`).
		Where(`student_id = ?`, studentID).
		Where(`instance_id IN (
			SELECT id FROM schedule.activity_instances
			WHERE activity_group_id = ? AND date = ? AND tenant_id = ?
		)`, groupID, date, s.tenantID).
		Exec(s.ctx)
	require.NoError(t, err)
	affected, err := res.RowsAffected()
	require.NoError(t, err)
	require.Equal(t, int64(1), affected, "expected exactly one hand-removable roster row")
}

// TestSeriesRoster_AddsChildAcrossCappedPredecessor is the core acceptance
// criterion of #2187: a child added "ab D" stands on the D list even though
// that day still materialized from the capped predecessor segment.
func TestSeriesRoster_AddsChildAcrossCappedPredecessor(t *testing.T) {
	c := makeRosterChain(t, []int{activitiesModels.WeekdayMonday})
	defer c.runCleanup(t)

	kept := []int64{c.students[0], c.students[1]}
	added := c.students[2]
	rootBefore := loadSplitEnrollments(t, c.scenarioSetup, c.rootID)
	require.Empty(t, enrollmentsFor(t, c.scenarioSetup, c.middleID, added),
		"the added child must not already be on the predecessor")

	in := c.livingUpdateInput(t, append(append([]int64{}, kept...), added), []int64{c.staffID}, &c.staffID)
	in.SeriesRosterFrom = &c.anchor
	in.SeriesRosterScopeStudentIDs = []int64{added}
	require.NoError(t, c.factory.TimetableData.UpdateTemplate(c.ctx, in))

	predecessorRows := enrollmentsFor(t, c.scenarioSetup, c.middleID, added)
	require.Len(t, predecessorRows, 1, "one bounded row per scheduled weekday")
	row := predecessorRows[0]
	assert.Equal(t, c.anchor, row.ValidFrom, "the predecessor row starts at the anchor")
	require.NotNil(t, row.ValidUntil)
	assert.Equal(t, c.secondBoundary, *row.ValidUntil, "and ends with the predecessor segment")
	require.NotNil(t, row.Weekday, "predecessor rows are written weekday-explicit")
	assert.Equal(t, activitiesModels.WeekdayMonday, *row.Weekday)

	livingRows := enrollmentsFor(t, c.scenarioSetup, c.livingID, added)
	require.Len(t, livingRows, 1)
	assert.Nil(t, livingRows[0].ValidUntil, "the living segment keeps its own open roster row")

	assert.Equal(t, rootBefore, loadSplitEnrollments(t, c.scenarioSetup, c.rootID),
		"segments that ended before the anchor are never rewritten")

	assert.Contains(t, instanceStudentIDsOn(t, c.scenarioSetup, c.middleID, c.anchor), added,
		"the already-materialized occurrence on the anchor must gain the child")
	assert.NotContains(t, instanceStudentIDsOn(t, c.scenarioSetup, c.middleID, c.earlier), added,
		"occurrences before the anchor stay untouched")
}

func TestSeriesRoster_RemovesChildAcrossCappedPredecessor(t *testing.T) {
	c := makeRosterChain(t, []int{activitiesModels.WeekdayMonday})
	defer c.runCleanup(t)

	removed := c.students[1]
	require.Contains(t, instanceStudentIDsOn(t, c.scenarioSetup, c.middleID, c.anchor), removed)

	in := c.livingUpdateInput(t, []int64{c.students[0]}, []int64{c.staffID}, &c.staffID)
	in.SeriesRosterFrom = &c.anchor
	in.SeriesRosterScopeStudentIDs = []int64{removed}
	require.NoError(t, c.factory.TimetableData.UpdateTemplate(c.ctx, in))

	rows := enrollmentsFor(t, c.scenarioSetup, c.middleID, removed)
	require.Len(t, rows, 1, "the carried row is closed, not duplicated")
	assert.Equal(t, c.firstBoundary, rows[0].ValidFrom, "its start stays where it was")
	require.NotNil(t, rows[0].ValidUntil)
	assert.Equal(t, c.anchor, *rows[0].ValidUntil, "the row now ends at the anchor")

	assert.NotContains(t, instanceStudentIDsOn(t, c.scenarioSetup, c.middleID, c.anchor), removed,
		"the anchor occurrence loses the child")
	assert.Contains(t, instanceStudentIDsOn(t, c.scenarioSetup, c.middleID, c.earlier), removed,
		"the occurrence before the anchor keeps them")
}

// TestSeriesRoster_KeepsPredecessorOnlyChildOutsideScope pins the rule that
// makes the mirroring safe: the saved roster describes the LIVING segment, so
// a child who only ever stood on the predecessor (a split may have dropped
// them) must survive a save that removes somebody else.
func TestSeriesRoster_KeepsPredecessorOnlyChildOutsideScope(t *testing.T) {
	c := makeRosterChain(t, []int{activitiesModels.WeekdayMonday})
	defer c.runCleanup(t)

	predecessorOnly := c.insertPredecessorEnrollment(t, c.students[2], func(*activitiesModels.StudentEnrollment) {})
	removed := c.students[1]

	in := c.livingUpdateInput(t, []int64{c.students[0]}, []int64{c.staffID}, &c.staffID)
	in.SeriesRosterFrom = &c.anchor
	in.SeriesRosterScopeStudentIDs = []int64{removed}
	require.NoError(t, c.factory.TimetableData.UpdateTemplate(c.ctx, in))

	kept := c.reloadEnrollment(t, predecessorOnly.ID)
	assert.Equal(t, predecessorOnly.ValidFrom, kept.ValidFrom,
		"a child outside the edit's scope keeps their predecessor row")
	assert.Equal(t, predecessorOnly.ValidUntil, kept.ValidUntil)
	require.Len(t, enrollmentsFor(t, c.scenarioSetup, c.middleID, c.students[2]), 1,
		"and gains no second row either")

	removedRows := enrollmentsFor(t, c.scenarioSetup, c.middleID, removed)
	require.Len(t, removedRows, 1)
	require.NotNil(t, removedRows[0].ValidUntil)
	assert.Equal(t, c.anchor, *removedRows[0].ValidUntil,
		"the child the edit did touch is still closed at the anchor")
}

func TestSeriesRoster_NoopWithoutSeriesRosterFrom(t *testing.T) {
	c := makeRosterChain(t, []int{activitiesModels.WeekdayMonday})
	defer c.runCleanup(t)

	added := c.students[2]
	before := loadSplitEnrollments(t, c.scenarioSetup, c.middleID)

	in := c.livingUpdateInput(t,
		[]int64{c.students[0], c.students[1], added}, []int64{c.staffID}, &c.staffID)
	// SeriesRosterFrom deliberately unset.
	require.NoError(t, c.factory.TimetableData.UpdateTemplate(c.ctx, in))

	assert.Equal(t, before, loadSplitEnrollments(t, c.scenarioSetup, c.middleID),
		"without the anchor the predecessor segment is not touched at all")
	assert.NotContains(t, instanceStudentIDsOn(t, c.scenarioSetup, c.middleID, c.anchor), added)
}

// TestSeriesRoster_SkipsProtectedPredecessorEnrollments pins that rows owned by
// another workflow (an enrollment request child, a weekday-selection row) are
// never rewritten by the series pass.
func TestSeriesRoster_SkipsProtectedPredecessorEnrollments(t *testing.T) {
	c := makeRosterChain(t, []int{activitiesModels.WeekdayMonday})
	defer c.runCleanup(t)

	added := c.students[2]
	requestChildID := createSplitRequestChild(t, c.scenarioSetup, added)
	protectedByRequest := c.insertPredecessorEnrollment(t, added, func(row *activitiesModels.StudentEnrollment) {
		row.EnrollmentRequestChildID = &requestChildID
	})
	protectedByWeekdays := c.insertPredecessorEnrollment(t, c.students[0], func(row *activitiesModels.StudentEnrollment) {
		row.SelectedWeekdays = []int{activitiesModels.WeekdayMonday}
	})

	in := c.livingUpdateInput(t,
		[]int64{c.students[0], c.students[1], added}, []int64{c.staffID}, &c.staffID)
	in.SeriesRosterFrom = &c.anchor
	in.SeriesRosterScopeStudentIDs = []int64{added, c.students[0]}
	require.NoError(t, c.factory.TimetableData.UpdateTemplate(c.ctx, in))

	for _, want := range []*activitiesModels.StudentEnrollment{protectedByRequest, protectedByWeekdays} {
		got := c.reloadEnrollment(t, want.ID)
		assert.Equal(t, want.ValidFrom, got.ValidFrom, "protected row start must survive")
		assert.Equal(t, want.ValidUntil, got.ValidUntil, "protected row end must survive")
		assert.Equal(t, want.Weekday, got.Weekday)
		assert.Equal(t, want.SelectedWeekdays, got.SelectedWeekdays)
		assert.Equal(t, want.EnrollmentRequestChildID, got.EnrollmentRequestChildID)
	}

	// The weekday the protected row already supplies must not gain a second,
	// editor-owned row — a duplicate would keep the child in the group after
	// the Betreuungsanfrage is withdrawn.
	assert.Len(t, enrollmentsFor(t, c.scenarioSetup, c.middleID, added), 1,
		"a protected row is not supplemented on the weekday it already covers")
}

// insertPredecessorEnrollment writes one bounded [firstBoundary, secondBoundary)
// enrollment row onto the middle segment, customized by mutate.
func (c *seriesChain) insertPredecessorEnrollment(
	t *testing.T,
	studentID int64,
	mutate func(*activitiesModels.StudentEnrollment),
) *activitiesModels.StudentEnrollment {
	t.Helper()
	validUntil := c.secondBoundary
	row := &activitiesModels.StudentEnrollment{
		StudentID:       studentID,
		ActivityGroupID: c.middleID,
		ValidFrom:       c.firstBoundary,
		ValidUntil:      &validUntil,
	}
	mutate(row)
	row.SetTenantID(c.tenantID)
	_, err := c.db.NewInsert().Model(row).ModelTableExpr(`activities.student_enrollments`).Exec(c.ctx)
	require.NoError(t, err)
	c.registerCleanup("activities.student_enrollments", row.ID)
	return row
}

func (c *seriesChain) reloadEnrollment(t *testing.T, id int64) *activitiesModels.StudentEnrollment {
	t.Helper()
	var row activitiesModels.StudentEnrollment
	err := c.db.NewSelect().Model(&row).
		ModelTableExpr(`activities.student_enrollments AS "student_enrollment"`).
		Where(`"student_enrollment".id = ?`, id).
		Where(`"student_enrollment".tenant_id = ?`, c.tenantID).
		Scan(c.ctx)
	require.NoError(t, err)
	return &row
}

// TestSeriesRoster_PreservesHandRemovedChildOnPredecessorOccurrence: once staff
// removed a child from ONE occurrence by hand, a later series roster save must
// not resurrect that row, while newly gained coverage still creates rows.
func TestSeriesRoster_PreservesHandRemovedChildOnPredecessorOccurrence(t *testing.T) {
	c := makeRosterChain(t, []int{activitiesModels.WeekdayMonday})
	defer c.runCleanup(t)

	added := c.students[2]
	roster := []int64{c.students[0], c.students[1], added}

	first := c.livingUpdateInput(t, roster, []int64{c.staffID}, &c.staffID)
	first.SeriesRosterFrom = &c.anchor
	first.SeriesRosterScopeStudentIDs = []int64{added}
	require.NoError(t, c.factory.TimetableData.UpdateTemplate(c.ctx, first))
	require.Contains(t, instanceStudentIDsOn(t, c.scenarioSetup, c.middleID, c.anchor), added)

	// Staff strikes the child from that single occurrence.
	deleteInstanceStudentRow(t, c.scenarioSetup, c.middleID, added, c.anchor)

	// Same roster, but the anchor now reaches back to the segment start: the
	// child's predecessor row is rewritten, so the instance pass runs again.
	second := c.livingUpdateInput(t, roster, []int64{c.staffID}, &c.staffID)
	second.SeriesRosterFrom = &c.firstBoundary
	second.SeriesRosterScopeStudentIDs = []int64{added}
	require.NoError(t, c.factory.TimetableData.UpdateTemplate(c.ctx, second))

	rows := enrollmentsFor(t, c.scenarioSetup, c.middleID, added)
	require.Len(t, rows, 1)
	assert.Equal(t, c.firstBoundary, rows[0].ValidFrom, "the widened anchor moved the row start")

	assert.NotContains(t, instanceStudentIDsOn(t, c.scenarioSetup, c.middleID, c.anchor), added,
		"a hand-removed occurrence row must not be resurrected")
	assert.Contains(t, instanceStudentIDsOn(t, c.scenarioSetup, c.middleID, c.earlier), added,
		"newly gained coverage still reaches the earlier occurrence")
}

// TestSeriesRoster_WeekdayScopedAddOnlyTouchesThatWeekday: a per-weekday
// deviation must retro-apply to that weekday only.
func TestSeriesRoster_WeekdayScopedAddOnlyTouchesThatWeekday(t *testing.T) {
	c := makeRosterChain(t, []int{activitiesModels.WeekdayMonday, activitiesModels.WeekdayWednesday})
	defer c.runCleanup(t)

	added := c.students[2]
	shared := []int64{c.students[0], c.students[1]}
	wednesdayAfterAnchor := c.anchor.AddDays(2)
	require.Equal(t, time.Wednesday, wednesdayAfterAnchor.Weekday())

	in := c.livingUpdateInput(t, shared, []int64{c.staffID}, &c.staffID)
	in.WeekdayAssignments = []scheduleSvc.WeekdayRosterAssignment{{
		Weekday:        activitiesModels.WeekdayMonday,
		StudentIDs:     append(append([]int64{}, shared...), added),
		StaffIDs:       []int64{c.staffID},
		PrimaryStaffID: &c.staffID,
	}}
	in.SeriesRosterFrom = &c.anchor
	in.SeriesRosterScopeStudentIDs = []int64{added}
	require.NoError(t, c.factory.TimetableData.UpdateTemplate(c.ctx, in))

	rows := enrollmentsFor(t, c.scenarioSetup, c.middleID, added)
	require.Len(t, rows, 1, "only the deviating weekday gets a predecessor row")
	require.NotNil(t, rows[0].Weekday)
	assert.Equal(t, activitiesModels.WeekdayMonday, *rows[0].Weekday)

	assert.Contains(t, instanceStudentIDsOn(t, c.scenarioSetup, c.middleID, c.anchor), added,
		"the Monday occurrence gains the child")
	assert.NotContains(t, instanceStudentIDsOn(t, c.scenarioSetup, c.middleID, wednesdayAfterAnchor), added,
		"the Wednesday occurrence after the anchor must stay unchanged")
}

// TestSeriesRoster_ReconcilesSupervisorsAcrossPredecessor covers the staff twin
// of the roster pass, including the rule that an instance_staff row carrying a
// real decision (here: an absence) is never removed.
func TestSeriesRoster_ReconcilesSupervisorsAcrossPredecessor(t *testing.T) {
	c := makeRosterChain(t, []int{activitiesModels.WeekdayMonday})
	defer c.runCleanup(t)

	extraStaff := c.addStaff(t, "Sub")
	students := []int64{c.students[0], c.students[1]}

	added := c.livingUpdateInput(t, students, []int64{c.staffID, extraStaff}, &c.staffID)
	added.SeriesRosterFrom = &c.anchor
	added.SeriesRosterScopeStaffIDs = []int64{extraStaff}
	require.NoError(t, c.factory.TimetableData.UpdateTemplate(c.ctx, added))

	rows := supervisorsFor(t, c.scenarioSetup, c.middleID, extraStaff)
	require.Len(t, rows, 1)
	assert.Equal(t, c.anchor, rows[0].ValidFrom)
	require.NotNil(t, rows[0].ValidUntil)
	assert.Equal(t, c.secondBoundary, *rows[0].ValidUntil)
	require.NotNil(t, rows[0].Weekday)
	assert.Equal(t, activitiesModels.WeekdayMonday, *rows[0].Weekday)
	assert.False(t, rows[0].IsPrimary, "the primary supervisor did not change")

	assert.Contains(t, instanceStaffIDsOn(t, c.scenarioSetup, c.middleID, c.anchor), extraStaff,
		"the anchor occurrence gains the staff member")
	assert.NotContains(t, instanceStaffIDsOn(t, c.scenarioSetup, c.middleID, c.earlier), extraStaff,
		"occurrences before the anchor stay untouched")

	// The staff member is marked absent on that one occurrence — a recorded
	// decision the removal below must not delete.
	c.markInstanceStaffAbsent(t, extraStaff, c.anchor)

	removed := c.livingUpdateInput(t, students, []int64{c.staffID}, &c.staffID)
	removed.SeriesRosterFrom = &c.anchor
	removed.SeriesRosterScopeStaffIDs = []int64{extraStaff}
	require.NoError(t, c.factory.TimetableData.UpdateTemplate(c.ctx, removed))

	rows = supervisorsFor(t, c.scenarioSetup, c.middleID, extraStaff)
	require.Len(t, rows, 1)
	require.NotNil(t, rows[0].ValidUntil)
	assert.Equal(t, c.anchor, *rows[0].ValidUntil, "the predecessor supervision closes at the anchor")

	assert.Contains(t, instanceStaffIDsOn(t, c.scenarioSetup, c.middleID, c.anchor), extraStaff,
		"an absence row is a recorded decision and survives the removal")
	assert.Contains(t, instanceStaffIDsOn(t, c.scenarioSetup, c.middleID, c.anchor), c.staffID,
		"the remaining supervisor keeps their assignment")
}

func (c *seriesChain) markInstanceStaffAbsent(t *testing.T, staffID int64, date timezone.Date) {
	t.Helper()
	res, err := c.db.NewUpdate().
		TableExpr(`schedule.instance_staff`).
		Set(`is_absent = TRUE`).
		Where(`staff_id = ?`, staffID).
		Where(`instance_id IN (
			SELECT id FROM schedule.activity_instances
			WHERE activity_group_id = ? AND date = ? AND tenant_id = ?
		)`, c.middleID, date, c.tenantID).
		Exec(c.ctx)
	require.NoError(t, err)
	affected, err := res.RowsAffected()
	require.NoError(t, err)
	require.Equal(t, int64(1), affected)
}
