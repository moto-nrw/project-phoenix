// Issue #5313: a Regeltermin keeps a stored source offering that turned
// inactive. Saving the Regeltermin must not fail on it: the inactive source
// contributes nothing, as on every resync (#5305), and the valid source keeps
// planning its children. A newly selected inactive offering and a Regeltermin
// left without any valid source are still refused, and nothing is written.
package httpintegration_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// inactiveSourceTemplate is a Regeltermin sourced from A and B of one
// phase: the first child holds A, the second B.
type inactiveSourceTemplate struct {
	s       *scenarioSetup
	a, b    *testpkg.CareOffering
	update  timetable.UpdateTemplateCommand
	created []StudentEnrollment
}

func setupInactiveSourceTemplate(t *testing.T) inactiveSourceTemplate {
	t.Helper()
	monday := futureMonday(1)
	s := makeScenario(t, activityContract.WeekdayMonday, monday)
	t.Cleanup(func() { s.runCleanup(t) })

	a := createSourceCareOffering(t, s, calendar.Date(s.period.StartDate), calendar.Date(s.period.EndDate))
	b := createPhaseSibling(t, s, a, "Spätbetreuung")
	linkApprovedChildToOffering(t, s, a, s.students[0], "1a")
	linkApprovedChildToOffering(t, s, b, s.students[1], "1a")

	name := fmt.Sprintf("Mehrquellen-Termin-%d", time.Now().UnixNano())
	result, err := s.timetableData.Templates.CreateTemplate(s.ctx, timetable.CreateTemplateCommand{
		Name:                  name,
		Type:                  activityContract.GroupTypeCare,
		Weekdays:              []int{activityContract.WeekdayMonday},
		StartTime:             time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC),
		EndTime:               time.Date(2000, 1, 1, 16, 0, 0, 0, time.UTC),
		RoomID:                s.roomID,
		CategoryID:            s.categoryID,
		MaxParticipants:       20,
		CalendarPeriodID:      &s.period.ID,
		TargetGroupType:       activityContract.TargetGroupTypeAngebot,
		SourceCareOfferingIDs: []int64{a.ID, b.ID},
		RosterValidFrom:       monday.AddDays(-30),
		GradeLevelMax:         testpkg.MaxSchoolGradeLevel,
	})
	require.NoError(t, err)
	registerSourcedTemplateCleanup(t, s, result.TemplateID, result.TimeframeID)

	f := inactiveSourceTemplate{s: s, a: a, b: b, update: timetable.UpdateTemplateCommand{
		TemplateID: result.TemplateID,
		Fields: timetable.TemplateFields{
			Name:                  name + " (neu)",
			Type:                  activityContract.GroupTypeCare,
			CategoryID:            s.categoryID,
			RoomID:                s.roomID,
			MaxParticipants:       20,
			CalendarPeriodID:      &s.period.ID,
			TargetGroupType:       activityContract.TargetGroupTypeAngebot,
			SourceCareOfferingIDs: []int64{a.ID, b.ID},
		},
		Weekdays:         []int{activityContract.WeekdayMonday},
		TimeframeID:      result.TimeframeID,
		CalendarPeriodID: &s.period.ID,
		RosterValidFrom:  monday.AddDays(-30),
		GradeLevelMax:    testpkg.MaxSchoolGradeLevel,
	}}
	f.created = f.rows(t)
	require.Len(t, f.created, 2, "sanity: both sources seed their child")
	return f
}

// createPhaseSibling creates a second Monday offering in the phase of
// offering.
func createPhaseSibling(t *testing.T, s *scenarioSetup, offering *testpkg.CareOffering, name string) *testpkg.CareOffering {
	t.Helper()
	sibling := testpkg.NewSourceCareOffering(offering.PhaseID, fmt.Sprintf("%s %d", name, time.Now().UnixNano()), []string{"mon"}, nil)
	sibling.TenantID = s.tenantID
	testpkg.InsertTestCareOffering(t, s.db, s.ctx, sibling)
	return sibling
}

// deactivate switches offerings off behind the catalog's guard: the drift a
// stored source carries.
func (f inactiveSourceTemplate) deactivate(t *testing.T, offerings ...*testpkg.CareOffering) {
	t.Helper()
	for _, offering := range offerings {
		_, err := f.s.db.NewRaw(`UPDATE enrollment.care_offerings SET is_active = FALSE WHERE id = ?`, offering.ID).Exec(f.s.ctx)
		require.NoError(t, err)
	}
}

func (f inactiveSourceTemplate) rows(t *testing.T) []StudentEnrollment {
	t.Helper()
	var rows []StudentEnrollment
	require.NoError(t, f.s.db.NewSelect().
		Model(&rows).
		ModelTableExpr(`activities.student_enrollments AS "student_enrollment"`).
		Where(`"student_enrollment".activity_group_id = ?`, f.update.TemplateID).
		OrderExpr(`"student_enrollment".id`).
		Scan(f.s.ctx))
	return rows
}

func (f inactiveSourceTemplate) rowsOf(t *testing.T, studentID int64) []StudentEnrollment {
	t.Helper()
	var rows []StudentEnrollment
	for _, row := range f.rows(t) {
		if row.StudentID == studentID {
			rows = append(rows, row)
		}
	}
	return rows
}

// requireRefusedInactive asserts the refusal names the inactive offering
// and the save wrote nothing.
func (f inactiveSourceTemplate) requireRefusedInactive(t *testing.T, err error, offering *testpkg.CareOffering, before []StudentEnrollment, storedIDs []int64) {
	t.Helper()
	require.ErrorIs(t, err, timetable.ErrOfferingSourceInvalid)
	coded, ok := timetable.AsCoded(err)
	require.True(t, ok, "the refusal carries a code")
	assert.Equal(t, timetable.CodeOfferingSourceInactive, coded.Code)
	assert.Equal(t, offering.Name, coded.Values.Offering, "the refusal names the inactive offering")
	group := loadTemplateGroup(t, f.s, f.update.TemplateID)
	assert.Equal(t, storedIDs, group.SourceCareOfferingIDs, "the refused save keeps the stored sources")
	assert.NotEqual(t, f.update.Fields.Name, group.Name, "the refused save writes no field")
	assert.Equal(t, before, f.rows(t), "the refused save writes no roster row")
}

// Saving the Regeltermin with its unchanged sources succeeds although A is
// inactive: A's child leaves the roster, B's child keeps its row, and a
// second save changes nothing.
func TestTemplateOfferingSource_SaveSkipsStoredInactiveSource(t *testing.T) {
	t.Parallel()
	f := setupInactiveSourceTemplate(t)
	f.deactivate(t, f.a)

	require.NoError(t, f.s.timetableData.Templates.UpdateTemplate(f.s.ctx, f.update),
		"a stored inactive source must not block the save")

	group := loadTemplateGroup(t, f.s, f.update.TemplateID)
	assert.Equal(t, f.update.Fields.Name, group.Name, "the save writes its fields")
	assert.Equal(t, []int64{f.a.ID, f.b.ID}, group.SourceCareOfferingIDs, "the inactive source stays stored")
	assert.Empty(t, f.rowsOf(t, f.s.students[0]), "the inactive source plans its child no more")
	kept := f.rowsOf(t, f.s.students[1])
	require.Len(t, kept, 1, "the valid source keeps its child")
	assert.Equal(t, f.created[1], kept[0], "the valid source's row stays untouched")

	settled := f.rows(t)
	require.NoError(t, f.s.timetableData.Templates.UpdateTemplate(f.s.ctx, f.update))
	assert.Equal(t, settled, f.rows(t), "a second save changes nothing")
}

// A stored inactive source does not open the door for a newly selected one.
func TestTemplateOfferingSource_SaveRefusesNewlySelectedInactiveSource(t *testing.T) {
	t.Parallel()
	f := setupInactiveSourceTemplate(t)
	c := createPhaseSibling(t, f.s, f.a, "Ferienbetreuung")
	f.deactivate(t, f.a, c)
	before := f.rows(t)

	f.update.Fields.SourceCareOfferingIDs = []int64{f.a.ID, f.b.ID, c.ID}
	err := f.s.timetableData.Templates.UpdateTemplate(f.s.ctx, f.update)
	f.requireRefusedInactive(t, err, c, before, []int64{f.a.ID, f.b.ID})
}

// Without any valid source the save is refused as before.
func TestTemplateOfferingSource_SaveRefusesWhenEverySourceIsInactive(t *testing.T) {
	t.Parallel()
	f := setupInactiveSourceTemplate(t)
	f.deactivate(t, f.a, f.b)
	before := f.rows(t)

	err := f.s.timetableData.Templates.UpdateTemplate(f.s.ctx, f.update)
	f.requireRefusedInactive(t, err, f.a, before, []int64{f.a.ID, f.b.ID})
}

// "Dieser und alle folgenden" saves the Regeltermin as well: the successor
// inherits both stored sources, skips the inactive one and plans B's child.
func TestTemplateOfferingSource_SplitSkipsStoredInactiveSource(t *testing.T) {
	t.Parallel()
	f := setupInactiveSourceTemplate(t)
	f.deactivate(t, f.a)

	split, err := f.s.timetableData.Templates.SplitTemplate(f.s.ctx, timetable.SplitTemplateCommand{
		TemplateID:       f.update.TemplateID,
		EffectiveDate:    futureMonday(2),
		Name:             f.update.Fields.Name,
		Type:             activityContract.GroupTypeCare,
		Weekdays:         []int{activityContract.WeekdayMonday},
		StartTime:        time.Date(2000, 1, 1, 16, 0, 0, 0, time.UTC),
		EndTime:          time.Date(2000, 1, 1, 17, 0, 0, 0, time.UTC),
		RoomID:           f.s.roomID,
		CategoryID:       f.s.categoryID,
		CalendarPeriodID: &f.s.period.ID,
		TargetGroupType:  activityContract.TargetGroupTypeAngebot,
		GradeLevelMax:    testpkg.MaxSchoolGradeLevel,
	})
	require.NoError(t, err, "a stored inactive source must not block the split")
	registerSourcedTemplateCleanup(t, f.s, split.NewTemplateID)

	successor := loadTemplateGroup(t, f.s, split.NewTemplateID)
	assert.Equal(t, []int64{f.a.ID, f.b.ID}, successor.SourceCareOfferingIDs, "the successor inherits both stored sources")
	successorRows := inactiveSourceTemplate{s: f.s, update: timetable.UpdateTemplateCommand{TemplateID: split.NewTemplateID}}
	assert.Empty(t, successorRows.rowsOf(t, f.s.students[0]), "the inactive source plans its child no more")
	assert.Len(t, successorRows.rowsOf(t, f.s.students[1]), 1, "the valid source plans its child on the successor")
}
