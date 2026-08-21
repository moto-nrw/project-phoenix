// Issue #2482: an offering-sourced Regeltermin can additionally be narrowed
// to concrete Schulklassen. This is the OGS-am-Berg Randstunden case — one
// shared Betreuungsangebot, six Regeltermine (1a…2c), each on its own
// weekdays — which the Jahrgang filter alone cannot express.
//
// The tests drive the real write path and the real resync hooks, because the
// stored rule is what every later reconcile reads back.
package schedule_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/schoolclass"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/services/education"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

// createClassSourcedTemplate creates one "Randstunde"-shaped Regeltermin fed
// by the offering and narrowed to the given Schulklassen.
func createClassSourcedTemplate(
	t *testing.T,
	s *scenarioSetup,
	offering *enrollmentModels.CareOffering,
	label string,
	schoolClasses []string,
	rosterValidFrom timezone.Date,
) *scheduleSvc.CreateTemplateResult {
	t.Helper()

	result, err := s.factory.TimetableData.CreateTemplate(s.ctx, scheduleSvc.CreateTemplateInput{
		Name:                  fmt.Sprintf("Randstunde-%s-%d", label, time.Now().UnixNano()),
		Type:                  activitiesModels.GroupTypeCare,
		Weekdays:              []int{activitiesModels.WeekdayMonday},
		StartTime:             time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC),
		EndTime:               time.Date(2000, 1, 1, 16, 0, 0, 0, time.UTC),
		RoomID:                s.roomID,
		CategoryID:            s.categoryID,
		MaxParticipants:       20,
		CalendarPeriodID:      &s.period.ID,
		TargetGroupType:       activitiesModels.TargetGroupTypeAngebot,
		SourceCareOfferingIDs: []int64{offering.ID},
		SourceSchoolClasses:   schoolClasses,
		RosterValidFrom:       rosterValidFrom,
		GradeLevelMax:         schoolclass.MaxGradeLevel,
	})
	require.NoError(t, err)
	registerSourcedTemplateCleanup(t, s, result.TemplateID, result.TimeframeID)
	return result
}

// sourcedStudentIDs returns the students the template's offering-derived
// roster still plans from today onward. A child that no longer matches is
// either deleted (row never took effect) or capped at the resync boundary —
// "entfernen ODER begrenzen" — so a plain row count would report a child the
// plan no longer holds. Rows that only cover past days are attendance
// history and must survive; see the ClassChange test's explicit assertion.
func sourcedStudentIDs(t *testing.T, s *scenarioSetup, templateID int64) []int64 {
	t.Helper()
	today := timezone.TodayDate()
	var rows []activitiesModels.StudentEnrollment
	require.NoError(t, s.db.NewSelect().
		Model(&rows).
		ModelTableExpr(`activities.student_enrollments AS "student_enrollment"`).
		Where(`"student_enrollment".activity_group_id = ?`, templateID).
		Where(`"student_enrollment".enrollment_request_child_id IS NOT NULL`).
		Where(`("student_enrollment".valid_until IS NULL OR "student_enrollment".valid_until > ?)`, today).
		OrderExpr(`"student_enrollment".student_id ASC`).
		Scan(s.ctx))
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.StudentID)
	}
	return ids
}

func offeringSourceResyncer(t *testing.T, s *scenarioSetup) education.OfferingSourceResyncer {
	t.Helper()
	resyncer, ok := s.factory.EnrollmentDecision.(education.OfferingSourceResyncer)
	require.True(t, ok, "the decision service must implement the offering-source resyncer")
	return resyncer
}

// The core case: one Angebot, one Regeltermin per Klasse. Only the children
// of the filtered class may be planned — a Jahrgang filter would have pulled
// 1a and 1b into the same Termin.
func TestTemplateSourceClassFilter_SeedsOnlyTheFilteredClass(t *testing.T) {
	t.Parallel()

	monday := futureMonday(1)
	s := makeScenario(t, activitiesModels.WeekdayMonday, monday)
	defer s.runCleanup(t)

	offering := createSourceCareOffering(t, s, s.period.StartDate, s.period.EndDate)
	linkApprovedChildToOffering(t, s, offering, s.students[0], "1a")
	linkApprovedChildToOffering(t, s, offering, s.students[1], "1b")
	// Same Jahrgang as the filtered class, different Klasse: the case the
	// Jahrgang filter cannot separate.
	linkApprovedChildToOffering(t, s, offering, s.students[2], "1c")

	result := createClassSourcedTemplate(t, s, offering, "1b", []string{"1b"}, monday.AddDays(-30))

	assert.Equal(t, []int64{s.students[1]}, sourcedStudentIDs(t, s, result.TemplateID),
		"only the 1b child may be planned")

	stored := loadTemplateGroup(t, s, result.TemplateID)
	assert.Equal(t, []string{"1b"}, stored.SourceSchoolClasses)
	assert.Nil(t, stored.SourceGradeLevels, "class and grade filter are mutually exclusive")
}

// The class name is free text in users.students.school_class. Matching must
// ignore case and padding, or a school typing "1B" silently loses children.
func TestTemplateSourceClassFilter_MatchesCaseInsensitively(t *testing.T) {
	t.Parallel()

	monday := futureMonday(1)
	s := makeScenario(t, activitiesModels.WeekdayMonday, monday)
	defer s.runCleanup(t)

	offering := createSourceCareOffering(t, s, s.period.StartDate, s.period.EndDate)
	linkApprovedChildToOffering(t, s, offering, s.students[0], " 1B ")

	result := createClassSourcedTemplate(t, s, offering, "1b", []string{"1b"}, monday.AddDays(-30))

	assert.Equal(t, []int64{s.students[0]}, sourcedStudentIDs(t, s, result.TemplateID))
}

// A child approved AFTER the Regeltermin was saved must be pulled in by the
// existing offering-source resync — the production incident of 2026-08-21 was
// exactly this, minus a stored source rule to resync against.
func TestTemplateSourceClassFilter_LaterApprovalJoinsTheTermin(t *testing.T) {
	t.Parallel()

	monday := futureMonday(1)
	s := makeScenario(t, activitiesModels.WeekdayMonday, monday)
	defer s.runCleanup(t)

	offering := createSourceCareOffering(t, s, s.period.StartDate, s.period.EndDate)
	linkApprovedChildToOffering(t, s, offering, s.students[0], "1b")

	result := createClassSourcedTemplate(t, s, offering, "1b", []string{"1b"}, monday.AddDays(-30))
	require.Equal(t, []int64{s.students[0]}, sourcedStudentIDs(t, s, result.TemplateID))

	// Freigabe nachträglich: a second 1b child joins the Angebot.
	linkApprovedChildToOffering(t, s, offering, s.students[1], "1b")
	require.NoError(t, offeringSourceResyncer(t, s).ResyncOfferingSourcedTemplates(s.ctx, timezone.TodayDate()))

	got := sourcedStudentIDs(t, s, result.TemplateID)
	assert.ElementsMatch(t, []int64{s.students[0], s.students[1]}, got,
		"a later approval must reach the already-saved Regeltermin")
}

// Klassenwechsel: the child leaves its old class Termin and joins the new
// one. Both directions in one test, because a resync that only adds is just
// as wrong as one that only removes.
func TestTemplateSourceClassFilter_ClassChangeMovesTheChild(t *testing.T) {
	t.Parallel()

	monday := futureMonday(1)
	s := makeScenario(t, activitiesModels.WeekdayMonday, monday)
	defer s.runCleanup(t)

	offering := createSourceCareOffering(t, s, s.period.StartDate, s.period.EndDate)
	linkApprovedChildToOffering(t, s, offering, s.students[0], "1a")

	rosterFrom := monday.AddDays(-30)
	terminA := createClassSourcedTemplate(t, s, offering, "1a", []string{"1a"}, rosterFrom)
	terminB := createClassSourcedTemplate(t, s, offering, "1b", []string{"1b"}, rosterFrom)

	require.Equal(t, []int64{s.students[0]}, sourcedStudentIDs(t, s, terminA.TemplateID))
	require.Empty(t, sourcedStudentIDs(t, s, terminB.TemplateID))

	_, err := s.db.NewRaw(
		`UPDATE users.students SET school_class = ? WHERE id = ?`, "1b", s.students[0],
	).Exec(s.ctx)
	require.NoError(t, err)
	require.NoError(t, offeringSourceResyncer(t, s).ResyncOfferingSourcedTemplates(s.ctx, timezone.TodayDate()))

	assert.Empty(t, sourcedStudentIDs(t, s, terminA.TemplateID),
		"the child must leave the Termin of its former Klasse")
	assert.Equal(t, []int64{s.students[0]}, sourcedStudentIDs(t, s, terminB.TemplateID),
		"the child must join the Termin of its new Klasse")

	// The already-started row is capped, not deleted: attendance history for
	// the days the child really was in the 1a Randstunde stays intact.
	var capped []activitiesModels.StudentEnrollment
	require.NoError(t, s.db.NewSelect().
		Model(&capped).
		ModelTableExpr(`activities.student_enrollments AS "student_enrollment"`).
		Where(`"student_enrollment".activity_group_id = ?`, terminA.TemplateID).
		Where(`"student_enrollment".student_id = ?`, s.students[0]).
		Scan(s.ctx))
	require.Len(t, capped, 1)
	require.NotNil(t, capped[0].ValidUntil)
	assert.False(t, capped[0].ValidUntil.After(timezone.TodayDate()),
		"the retired row must end at the resync boundary, keeping past days planned")
}

// Already-materialized future occurrences must follow the rule; attendance
// history must not. The resync reconciles instances from today onward only.
func TestTemplateSourceClassFilter_ReconcilesMaterializedFutureOccurrences(t *testing.T) {
	t.Parallel()

	monday := futureMonday(1)
	s := makeScenario(t, activitiesModels.WeekdayMonday, monday)
	defer s.runCleanup(t)

	offering := createSourceCareOffering(t, s, s.period.StartDate, s.period.EndDate)
	linkApprovedChildToOffering(t, s, offering, s.students[0], "1b")

	result := createClassSourcedTemplate(t, s, offering, "1b", []string{"1b"}, monday.AddDays(-30))

	mat, err := s.factory.Materialization.MaterializeForTenant(
		s.ctx, monday, monday, scheduleSvc.MaterializationSourceManual,
	)
	require.NoError(t, err)
	require.NotNil(t, mat)
	s.extraCleanups = append([]func(){func() {
		_, _ = s.db.NewRaw(`
			DELETE FROM schedule.instance_students
			WHERE instance_id IN (SELECT id FROM schedule.activity_instances WHERE tenant_id = ?)`,
			s.tenantID).Exec(s.ctx)
		_, _ = s.db.NewRaw(`
			DELETE FROM schedule.instance_staff
			WHERE instance_id IN (SELECT id FROM schedule.activity_instances WHERE tenant_id = ?)`,
			s.tenantID).Exec(s.ctx)
		_, _ = s.db.NewRaw(
			`DELETE FROM schedule.activity_instances WHERE tenant_id = ?`, s.tenantID).Exec(s.ctx)
	}}, s.extraCleanups...)

	instanceID := singleInstanceID(t, s, result.TemplateID, monday)
	require.Equal(t, []int64{s.students[0]}, instanceStudentIDs(t, s, instanceID))

	// Nachträgliche Freigabe eines zweiten 1b-Kindes.
	linkApprovedChildToOffering(t, s, offering, s.students[1], "1b")
	require.NoError(t, offeringSourceResyncer(t, s).ResyncOfferingSourcedTemplates(s.ctx, timezone.TodayDate()))

	assert.ElementsMatch(t, []int64{s.students[0], s.students[1]}, instanceStudentIDs(t, s, instanceID),
		"the already-materialized future occurrence must be reconciled, not left stale")
}

// Switching an existing Regeltermin from the Jahrgang filter to a
// Klassenfilter must swap both columns atomically and re-shape the roster.
func TestTemplateSourceClassFilter_UpdateSwitchesFromGradeToClass(t *testing.T) {
	t.Parallel()

	monday := futureMonday(1)
	s := makeScenario(t, activitiesModels.WeekdayMonday, monday)
	defer s.runCleanup(t)

	offering := createSourceCareOffering(t, s, s.period.StartDate, s.period.EndDate)
	linkApprovedChildToOffering(t, s, offering, s.students[0], "1a")
	linkApprovedChildToOffering(t, s, offering, s.students[1], "1b")

	rosterFrom := monday.AddDays(-30)
	name := fmt.Sprintf("Randstunde-Jahrgang-%d", time.Now().UnixNano())
	result, err := s.factory.TimetableData.CreateTemplate(s.ctx, scheduleSvc.CreateTemplateInput{
		Name:                  name,
		Type:                  activitiesModels.GroupTypeCare,
		Weekdays:              []int{activitiesModels.WeekdayMonday},
		StartTime:             time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC),
		EndTime:               time.Date(2000, 1, 1, 16, 0, 0, 0, time.UTC),
		RoomID:                s.roomID,
		CategoryID:            s.categoryID,
		MaxParticipants:       20,
		CalendarPeriodID:      &s.period.ID,
		TargetGroupType:       activitiesModels.TargetGroupTypeAngebot,
		SourceCareOfferingIDs: []int64{offering.ID},
		SourceGradeLevels:     []int{1},
		RosterValidFrom:       rosterFrom,
		GradeLevelMax:         schoolclass.MaxGradeLevel,
	})
	require.NoError(t, err)
	registerSourcedTemplateCleanup(t, s, result.TemplateID, result.TimeframeID)
	require.ElementsMatch(t, []int64{s.students[0], s.students[1]}, sourcedStudentIDs(t, s, result.TemplateID))

	require.NoError(t, s.factory.TimetableData.UpdateTemplate(s.ctx, scheduleSvc.TemplateUpdateInput{
		TemplateID: result.TemplateID,
		Fields: activitiesModels.TemplateFieldsUpdate{
			Name:                  name,
			Type:                  activitiesModels.GroupTypeCare,
			CategoryID:            s.categoryID,
			RoomID:                s.roomID,
			MaxParticipants:       20,
			CalendarPeriodID:      &s.period.ID,
			TargetGroupType:       activitiesModels.TargetGroupTypeAngebot,
			SourceCareOfferingIDs: []int64{offering.ID},
			SourceSchoolClasses:   []string{"1b"},
		},
		Weekdays:         []int{activitiesModels.WeekdayMonday},
		TimeframeID:      result.TimeframeID,
		CalendarPeriodID: &s.period.ID,
		RosterValidFrom:  rosterFrom,
		GradeLevelMax:    schoolclass.MaxGradeLevel,
	}))

	stored := loadTemplateGroup(t, s, result.TemplateID)
	assert.Equal(t, []string{"1b"}, stored.SourceSchoolClasses)
	assert.Nil(t, stored.SourceGradeLevels)
	assert.Equal(t, []int64{s.students[1]}, sourcedStudentIDs(t, s, result.TemplateID),
		"the 1a child must be retired when the filter narrows to 1b")
}

// The two filters are mutually exclusive: a save carrying both is a
// client-correctable error, never a silently applied AND or OR.
func TestTemplateSourceClassFilter_RejectsCombinedFilters(t *testing.T) {
	t.Parallel()

	monday := futureMonday(1)
	s := makeScenario(t, activitiesModels.WeekdayMonday, monday)
	defer s.runCleanup(t)

	offering := createSourceCareOffering(t, s, s.period.StartDate, s.period.EndDate)

	_, err := s.factory.TimetableData.CreateTemplate(s.ctx, scheduleSvc.CreateTemplateInput{
		Name:                  fmt.Sprintf("Randstunde-Konflikt-%d", time.Now().UnixNano()),
		Type:                  activitiesModels.GroupTypeCare,
		Weekdays:              []int{activitiesModels.WeekdayMonday},
		StartTime:             time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC),
		EndTime:               time.Date(2000, 1, 1, 16, 0, 0, 0, time.UTC),
		RoomID:                s.roomID,
		CategoryID:            s.categoryID,
		MaxParticipants:       20,
		CalendarPeriodID:      &s.period.ID,
		TargetGroupType:       activitiesModels.TargetGroupTypeAngebot,
		SourceCareOfferingIDs: []int64{offering.ID},
		SourceGradeLevels:     []int{1},
		SourceSchoolClasses:   []string{"1b"},
		RosterValidFrom:       monday.AddDays(-30),
		GradeLevelMax:         schoolclass.MaxGradeLevel,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, scheduleSvc.ErrOfferingSourceInvalid)
	assert.ErrorContains(t, err, "cannot be combined")
}

func singleInstanceID(t *testing.T, s *scenarioSetup, templateID int64, date timezone.Date) int64 {
	t.Helper()
	var instances []scheduleModels.ActivityInstance
	require.NoError(t, s.db.NewSelect().
		Model(&instances).
		ModelTableExpr(`schedule.activity_instances AS "activity_instance"`).
		Where(`"activity_instance".activity_group_id = ?`, templateID).
		Where(`"activity_instance".date = ?`, date).
		Scan(s.ctx))
	require.Len(t, instances, 1)
	return instances[0].ID
}

func instanceStudentIDs(t *testing.T, s *scenarioSetup, instanceID int64) []int64 {
	t.Helper()
	var rows []scheduleModels.InstanceStudent
	require.NoError(t, s.db.NewSelect().
		Model(&rows).
		ModelTableExpr(`schedule.instance_students AS "instance_student"`).
		Where(`"instance_student".instance_id = ?`, instanceID).
		Scan(s.ctx))
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.StudentID)
	}
	return ids
}
