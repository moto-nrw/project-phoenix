// Issue #2137: a Betreuungsangebot can feed several parallel Regeltermine,
// each with its own Jahrgang filter. These tests drive the real write path
// (TimetableData.CreateTemplate / UpdateTemplate and the split service) and
// assert what actually lands in activities.groups, because the source rule is
// what every later roster resync reads back.
//
// Fixtures via makeScenario (materialization_integration_test.go): unique
// tenant, real rows, full teardown. No hardcoded IDs.
package schedule_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/schoolclass"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// createSourceCareOffering creates a phase plus one care offering that is NOT
// linked to a template via the legacy CareOffering.ActivityGroupID bridge —
// #2137 inverts that link, the template names the offering.
func createSourceCareOffering(
	t *testing.T,
	s *scenarioSetup,
	serviceStart, serviceEnd timezone.Date,
) *enrollmentModels.CareOffering {
	t.Helper()

	suffix := time.Now().UnixNano()
	phase := &capability.Phase{
		Name:                      fmt.Sprintf("Quell-Phase %d", suffix),
		Kind:                      enrollmentModels.PhaseKindCustom,
		ServiceStartDate:          capability.Date(serviceStart),
		ServiceEndDate:            capability.Date(serviceEnd),
		CareOverflowMode:          enrollmentModels.PhaseCareOverflowWaitlist,
		CareOfferingSelectionMode: enrollmentModels.PhaseCareOfferingSelectionOptional,
		AvailableSchoolClasses:    []string{},
		IsActive:                  true,
	}
	phase.TenantID = s.tenantID
	repos := repositories.NewFactory(s.db, repositories.NewUnobservedTimetableDependencies(s.db))
	require.NoError(t, repos.Enrollment().InsertPhase(s.ctx, phase))

	offering := &enrollmentModels.CareOffering{
		PhaseID:            phase.ID,
		Name:               fmt.Sprintf("Frühbetreuung %d", suffix),
		DaysOfWeekMode:     enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:      []string{"mon"},
		PickupTimes:        map[string]string{"mon": "14:30"},
		IsActive:           true,
		CountsAsCare:       true,
		CountsAsCareSet:    true,
		SelectionRule:      enrollmentModels.SelectionRuleOptional,
		AutoAddGradeLevels: []int{},
	}
	var created *enrollmentModels.CareOffering
	require.NoError(t, testpkg.WithTenantTx(t, s.ctx, s.db, s.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		var createErr error
		created, createErr = s.factory.EnrollmentCareOffering.Create(txCtx, offering)
		return createErr
	}))

	s.extraCleanups = append([]func(){func() {
	}}, s.extraCleanups...)
	return created
}

// registerSourcedTemplateCleanup removes a template created during the test
// with everything hanging off it, regardless of what later edits added.
func registerSourcedTemplateCleanup(t *testing.T, s *scenarioSetup, templateID int64, timeframeIDs ...int64) {
	t.Helper()
	s.extraCleanups = append([]func(){func() {
		for _, stmt := range []string{
			`DELETE FROM activities.student_enrollments WHERE activity_group_id = ?`,
			`DELETE FROM activities.supervisors WHERE group_id = ?`,
			`DELETE FROM activities.schedules WHERE activity_group_id = ?`,
			`DELETE FROM activities.groups WHERE id = ?`,
		} {
			_, err := s.db.NewRaw(stmt, templateID).Exec(s.ctx)
			assert.NoError(t, err)
		}
	}}, s.extraCleanups...)
}

func loadTemplateGroup(t *testing.T, s *scenarioSetup, templateID int64) *activitiesModels.Group {
	t.Helper()
	group, err := repositories.NewFactory(s.db, repositories.NewUnobservedTimetableDependencies(s.db)).ActivityGroup.FindByID(s.ctx, templateID)
	require.NoError(t, err)
	require.NotNil(t, group)
	return group
}

// linkApprovedChildToOffering attaches an approved request-child (with a
// live users.students row) to the offering so the create-time resync can
// seed the template roster — the same path Franziska uses in the editor.
func linkApprovedChildToOffering(
	t *testing.T,
	s *scenarioSetup,
	offering *enrollmentModels.CareOffering,
	studentID int64,
	schoolClass string,
) {
	t.Helper()
	require.NotNil(t, offering)
	// Grade filter derives the Jahrgang from users.students.school_class.
	_, err := s.db.NewRaw(
		`UPDATE users.students SET school_class = ? WHERE id = ?`,
		schoolClass, studentID,
	).Exec(s.ctx)
	require.NoError(t, err)

	childID := createSplitRequestChildInPhase(t, s, offering.PhaseID, studentID)
	link := &capability.RequestChildOffering{
		RequestChildID: childID,
		CareOfferingID: offering.ID,
	}
	link.TenantID = s.tenantID
	require.NoError(t, repositories.NewFactory(s.db, repositories.NewUnobservedTimetableDependencies(s.db)).Enrollment().InsertRequestChildOffering(s.ctx, link))
	s.extraCleanups = append([]func(){func() {
	}}, s.extraCleanups...)
}

// Create with an offering source + materialize must land the approved
// children on the new occurrences. Empty instance_students after a
// successful editor create was the Schule-am-Berg report (kids selected
// via Angebote + Jahrgangsfilter, then "Keine Kinder geplant").
func TestTemplateOfferingSource_CreateAndMaterializeCopiesSourcedKids(t *testing.T) {
	t.Parallel()

	monday := futureMonday(1)
	s := makeScenario(t, activitiesModels.WeekdayMonday, monday)
	defer s.runCleanup(t)

	offering := createSourceCareOffering(t, s, timezone.Date(s.period.StartDate), timezone.Date(s.period.EndDate))
	// Grade filter 1 must match the child's school class.
	linkApprovedChildToOffering(t, s, offering, s.students[0], "1a")
	// A second child outside the filter must stay off the roster.
	linkApprovedChildToOffering(t, s, offering, s.students[1], "2b")

	name := fmt.Sprintf("Angebots-Termin-Kids-%d", time.Now().UnixNano())
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
		RosterValidFrom:       monday.AddDays(-30),
		GradeLevelMax:         schoolclass.MaxGradeLevel,
	})
	require.NoError(t, err)
	registerSourcedTemplateCleanup(t, s, result.TemplateID, result.TimeframeID)

	// Enrollments must be seeded before materialization (create-time resync).
	var enrollments []activitiesModels.StudentEnrollment
	require.NoError(t, s.db.NewSelect().
		Model(&enrollments).
		ModelTableExpr(`activities.student_enrollments AS "student_enrollment"`).
		Where(`"student_enrollment".activity_group_id = ?`, result.TemplateID).
		Scan(s.ctx))
	require.Len(t, enrollments, 1, "only the grade-1 child may be seeded")
	assert.Equal(t, s.students[0], enrollments[0].StudentID)

	mat, err := s.factory.Materialization.MaterializeForTenant(
		s.ctx, monday, monday, scheduleSvc.MaterializationSourceManual,
	)
	require.NoError(t, err)
	require.NotNil(t, mat)
	require.GreaterOrEqual(t, mat.InstancesCreated, 1)

	var instances []scheduleModels.ActivityInstance
	require.NoError(t, s.db.NewSelect().
		Model(&instances).
		ModelTableExpr(`schedule.activity_instances AS "activity_instance"`).
		Where(`"activity_instance".activity_group_id = ?`, result.TemplateID).
		Where(`"activity_instance".date = ?`, monday).
		Scan(s.ctx))
	require.Len(t, instances, 1)
	// Materialize is tenant-wide and also fills the scenario's baseline
	// template. Tear those rows down before makeScenario drops students/rooms.
	s.extraCleanups = append([]func(){func() {
		_, _ = s.db.NewRaw(`
			DELETE FROM schedule.instance_students
			WHERE instance_id IN (
				SELECT id FROM schedule.activity_instances WHERE tenant_id = ?
			)`, s.tenantID).Exec(s.ctx)
		_, _ = s.db.NewRaw(`
			DELETE FROM schedule.instance_staff
			WHERE instance_id IN (
				SELECT id FROM schedule.activity_instances WHERE tenant_id = ?
			)`, s.tenantID).Exec(s.ctx)
		_, _ = s.db.NewRaw(
			`DELETE FROM schedule.activity_instances WHERE tenant_id = ?`, s.tenantID,
		).Exec(s.ctx)
	}}, s.extraCleanups...)

	var rows []scheduleModels.InstanceStudent
	require.NoError(t, s.db.NewSelect().
		Model(&rows).
		ModelTableExpr(`schedule.instance_students AS "instance_student"`).
		Where(`"instance_student".instance_id = ?`, instances[0].ID).
		Scan(s.ctx))
	require.Len(t, rows, 1, "materialize must copy the sourced child onto the occurrence")
	assert.Equal(t, s.students[0], rows[0].StudentID)
	assert.Equal(t, scheduleModels.AttendanceStatusExpected, rows[0].Status)
}

func TestTemplateOfferingSource_PullForwardWidensSourcedRoster(t *testing.T) {
	t.Parallel()

	newStart := futureMonday(1)
	oldStart := newStart.AddDays(7)
	s := makeScenario(t, activitiesModels.WeekdayMonday, newStart)
	defer s.runCleanup(t)

	offering := createSourceCareOffering(t, s, timezone.Date(s.period.StartDate), timezone.Date(s.period.EndDate))
	linkApprovedChildToOffering(t, s, offering, s.students[0], "1a")
	name := fmt.Sprintf("Angebots-Pull-Forward-%d", time.Now().UnixNano())
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
		RosterValidFrom:       oldStart,
		ScheduleValidFrom:     &oldStart,
		GradeLevelMax:         schoolclass.MaxGradeLevel,
	})
	require.NoError(t, err)
	registerSourcedTemplateCleanup(t, s, result.TemplateID, result.TimeframeID)

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
		},
		Weekdays:         []int{activitiesModels.WeekdayMonday},
		TimeframeID:      result.TimeframeID,
		CalendarPeriodID: &s.period.ID,
		RosterValidFrom:  oldStart,
		StartDate:        &newStart,
		GradeLevelMax:    schoolclass.MaxGradeLevel,
	}))

	var sourced []activitiesModels.StudentEnrollment
	require.NoError(t, s.db.NewSelect().
		Model(&sourced).
		ModelTableExpr(`activities.student_enrollments AS "student_enrollment"`).
		Where(`"student_enrollment".activity_group_id = ?`, result.TemplateID).
		Where(`"student_enrollment".enrollment_request_child_id IS NOT NULL`).
		Scan(s.ctx))
	require.Len(t, sourced, 1)
	assert.Equal(t, activitiesModels.Date(newStart), sourced[0].ValidFrom,
		"offering-derived roster must follow the pulled-forward schedule boundary")
}

func TestTemplateOfferingSource_CreateStoresTheRuleAndIsFoundByOffering(t *testing.T) {
	t.Parallel()

	monday := futureMonday(1)
	s := makeScenario(t, activitiesModels.WeekdayMonday, monday)
	defer s.runCleanup(t)

	offering := createSourceCareOffering(t, s, timezone.Date(s.period.StartDate), timezone.Date(s.period.EndDate))

	result, err := s.factory.TimetableData.CreateTemplate(s.ctx, scheduleSvc.CreateTemplateInput{
		Name:                  fmt.Sprintf("Angebots-Termin-%d", time.Now().UnixNano()),
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
		SourceGradeLevels:     []int{1, 2},
		RosterValidFrom:       monday.AddDays(-30),
		GradeLevelMax:         schoolclass.MaxGradeLevel,
	})
	require.NoError(t, err)
	registerSourcedTemplateCleanup(t, s, result.TemplateID, result.TimeframeID)

	group := loadTemplateGroup(t, s, result.TemplateID)
	require.NotEmpty(t, group.SourceCareOfferingIDs)
	assert.Equal(t, []int64{offering.ID}, group.SourceCareOfferingIDs)
	assert.Equal(t, []int{1, 2}, group.SourceGradeLevels)

	// The reverse lookup is what the editor's overlap hint and the decision
	// fan-out both read: one offering, every live template sourcing it.
	sourced, err := repositories.NewFactory(s.db, repositories.NewUnobservedTimetableDependencies(s.db)).ActivityGroup.FindTemplatesBySourceOffering(s.ctx, offering.ID)
	require.NoError(t, err)
	require.Len(t, sourced, 1)
	assert.Equal(t, result.TemplateID, sourced[0].ID)
}

func TestTemplateOfferingSource_UpdateRewritesAndClearsTheRule(t *testing.T) {
	t.Parallel()

	monday := futureMonday(1)
	s := makeScenario(t, activitiesModels.WeekdayMonday, monday)
	defer s.runCleanup(t)

	offering := createSourceCareOffering(t, s, timezone.Date(s.period.StartDate), timezone.Date(s.period.EndDate))
	name := fmt.Sprintf("Angebots-Termin-%d", time.Now().UnixNano())

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
		RosterValidFrom:       monday.AddDays(-30),
		GradeLevelMax:         schoolclass.MaxGradeLevel,
	})
	require.NoError(t, err)
	registerSourcedTemplateCleanup(t, s, result.TemplateID, result.TimeframeID)

	updateInput := scheduleSvc.TemplateUpdateInput{
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
			SourceGradeLevels:     []int{3},
		},
		Weekdays:         []int{activitiesModels.WeekdayMonday},
		TimeframeID:      result.TimeframeID,
		CalendarPeriodID: &s.period.ID,
		RosterValidFrom:  monday.AddDays(-30),
		GradeLevelMax:    schoolclass.MaxGradeLevel,
	}
	require.NoError(t, s.factory.TimetableData.UpdateTemplate(s.ctx, updateInput))

	narrowed := loadTemplateGroup(t, s, result.TemplateID)
	require.NotEmpty(t, narrowed.SourceCareOfferingIDs)
	assert.Equal(t, []int{3}, narrowed.SourceGradeLevels,
		"narrowing the Jahrgang filter must persist, not merge with the old one")

	// Removing the source degrades the Regeltermin to a manually curated
	// roster; the filter must not survive its source (DB CHECK).
	updateInput.Fields.SourceCareOfferingIDs = nil
	updateInput.Fields.SourceGradeLevels = nil
	require.NoError(t, s.factory.TimetableData.UpdateTemplate(s.ctx, updateInput))

	cleared := loadTemplateGroup(t, s, result.TemplateID)
	assert.Empty(t, cleared.SourceCareOfferingIDs)
	assert.Empty(t, cleared.SourceGradeLevels)

	sourced, err := repositories.NewFactory(s.db, repositories.NewUnobservedTimetableDependencies(s.db)).ActivityGroup.FindTemplatesBySourceOffering(s.ctx, offering.ID)
	require.NoError(t, err)
	assert.Empty(t, sourced, "a cleared source must drop out of the offering's template list")
}

func TestTemplateOfferingSource_SplitSuccessorInheritsTheRule(t *testing.T) {
	t.Parallel()

	monday := futureMonday(1)
	s := makeScenario(t, activitiesModels.WeekdayMonday, monday)
	defer s.runCleanup(t)

	offering := createSourceCareOffering(t, s, timezone.Date(s.period.StartDate), timezone.Date(s.period.EndDate))
	name := fmt.Sprintf("Angebots-Termin-%d", time.Now().UnixNano())

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
		SourceGradeLevels:     []int{1, 2},
		RosterValidFrom:       monday.AddDays(-30),
		GradeLevelMax:         schoolclass.MaxGradeLevel,
	})
	require.NoError(t, err)
	registerSourcedTemplateCleanup(t, s, result.TemplateID, result.TimeframeID)

	// "Dieser und alle folgenden" does not expose the source rule, so the
	// successor must inherit it — otherwise the segment would silently lose
	// its Betreuungsangebot feed.
	split, err := s.factory.TemplateSplit.Split(s.ctx, scheduleSvc.TemplateSplitInput{
		TemplateID:       result.TemplateID,
		EffectiveDate:    monday,
		Name:             name,
		Type:             activitiesModels.GroupTypeCare,
		Weekdays:         []int{activitiesModels.WeekdayMonday},
		StartTime:        time.Date(2000, 1, 1, 16, 0, 0, 0, time.UTC),
		EndTime:          time.Date(2000, 1, 1, 17, 0, 0, 0, time.UTC),
		RoomID:           s.roomID,
		CategoryID:       s.categoryID,
		CalendarPeriodID: &s.period.ID,
		TargetGroupType:  activitiesModels.TargetGroupTypeAngebot,
		GradeLevelMax:    schoolclass.MaxGradeLevel,
	})
	require.NoError(t, err)
	registerSourcedTemplateCleanup(t, s, split.NewTemplateID)

	successor := loadTemplateGroup(t, s, split.NewTemplateID)
	require.NotEmpty(t, successor.SourceCareOfferingIDs)
	assert.Equal(t, []int64{offering.ID}, successor.SourceCareOfferingIDs)
	assert.Equal(t, []int{1, 2}, successor.SourceGradeLevels)
}

func TestTemplateOfferingSource_SplitAwayFromAngebotDropsTheRule(t *testing.T) {
	t.Parallel()

	monday := futureMonday(1)
	s := makeScenario(t, activitiesModels.WeekdayMonday, monday)
	defer s.runCleanup(t)

	offering := createSourceCareOffering(t, s, timezone.Date(s.period.StartDate), timezone.Date(s.period.EndDate))
	name := fmt.Sprintf("Angebots-Termin-%d", time.Now().UnixNano())

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
		SourceGradeLevels:     []int{1, 2},
		RosterValidFrom:       monday.AddDays(-30),
		GradeLevelMax:         schoolclass.MaxGradeLevel,
	})
	require.NoError(t, err)
	registerSourcedTemplateCleanup(t, s, result.TemplateID, result.TimeframeID)

	// The DB CHECK ties the source to the 'angebot' Zielgruppe: changing the
	// Zielgruppe in the split must drop the rule instead of failing the write.
	split, err := s.factory.TemplateSplit.Split(s.ctx, scheduleSvc.TemplateSplitInput{
		TemplateID:       result.TemplateID,
		EffectiveDate:    monday,
		Name:             name,
		Type:             activitiesModels.GroupTypeCare,
		Weekdays:         []int{activitiesModels.WeekdayMonday},
		StartTime:        time.Date(2000, 1, 1, 16, 0, 0, 0, time.UTC),
		EndTime:          time.Date(2000, 1, 1, 17, 0, 0, 0, time.UTC),
		RoomID:           s.roomID,
		CategoryID:       s.categoryID,
		CalendarPeriodID: &s.period.ID,
		TargetGroupType:  activitiesModels.TargetGroupTypeNone,
		GradeLevelMax:    schoolclass.MaxGradeLevel,
	})
	require.NoError(t, err)
	registerSourcedTemplateCleanup(t, s, split.NewTemplateID)

	successor := loadTemplateGroup(t, s, split.NewTemplateID)
	assert.Empty(t, successor.SourceCareOfferingIDs)
	assert.Empty(t, successor.SourceGradeLevels)
}

// Dropping the source on a split away from 'angebot' must also shed the
// source-derived roster rows the carry-over copies onto the successor: the
// now-manual template would otherwise keep provenance-tagged children the
// editor cannot replace (#2147 review). Manual rows carry over unchanged and
// the predecessor's rows stay untouched.
func TestTemplateOfferingSource_SplitAwayFromAngebotClearsSourcedRoster(t *testing.T) {
	t.Parallel()

	monday := futureMonday(1)
	s := makeScenario(t, activitiesModels.WeekdayMonday, monday)
	defer s.runCleanup(t)

	offering := createSourceCareOffering(t, s, timezone.Date(s.period.StartDate), timezone.Date(s.period.EndDate))
	name := fmt.Sprintf("Angebots-Termin-%d", time.Now().UnixNano())

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
		RosterValidFrom:       monday.AddDays(-30),
		GradeLevelMax:         schoolclass.MaxGradeLevel,
	})
	require.NoError(t, err)
	registerSourcedTemplateCleanup(t, s, result.TemplateID, result.TimeframeID)

	// One source-fed row (provenance-tagged, phase-bounded, the shape the
	// decision fan-out writes) and one manual row.
	enrollments := ownedStudentEnrollmentRepository(t, s.db)
	childID := createSplitRequestChild(t, s, s.students[0])
	sourcedUntil := activitiesModels.Date(s.period.EndDate.AddDays(1))
	sourced := &activitiesModels.StudentEnrollment{
		StudentID:                s.students[0],
		ActivityGroupID:          result.TemplateID,
		ValidFrom:                activitiesModels.Date(s.period.StartDate),
		ValidUntil:               &sourcedUntil,
		CalendarPeriodID:         &s.period.ID,
		EnrollmentRequestChildID: &childID,
		SelectedWeekdays:         []int{activitiesModels.WeekdayMonday},
	}
	sourced.SetTenantID(s.tenantID)
	require.NoError(t, enrollments.Create(s.ctx, sourced))
	manual := &activitiesModels.StudentEnrollment{
		StudentID:        s.students[1],
		ActivityGroupID:  result.TemplateID,
		ValidFrom:        activitiesModels.Date(monday.AddDays(-30)),
		CalendarPeriodID: &s.period.ID,
	}
	manual.SetTenantID(s.tenantID)
	require.NoError(t, enrollments.Create(s.ctx, manual))

	split, err := s.factory.TemplateSplit.Split(s.ctx, scheduleSvc.TemplateSplitInput{
		TemplateID:       result.TemplateID,
		EffectiveDate:    monday,
		Name:             name,
		Type:             activitiesModels.GroupTypeCare,
		Weekdays:         []int{activitiesModels.WeekdayMonday},
		StartTime:        time.Date(2000, 1, 1, 16, 0, 0, 0, time.UTC),
		EndTime:          time.Date(2000, 1, 1, 17, 0, 0, 0, time.UTC),
		RoomID:           s.roomID,
		CategoryID:       s.categoryID,
		CalendarPeriodID: &s.period.ID,
		TargetGroupType:  activitiesModels.TargetGroupTypeNone,
		GradeLevelMax:    schoolclass.MaxGradeLevel,
	})
	require.NoError(t, err)
	registerSourcedTemplateCleanup(t, s, split.NewTemplateID)

	successorRows := loadSplitEnrollments(t, s, split.NewTemplateID)
	require.Len(t, successorRows, 1, "the successor must carry only the manual row")
	assert.Equal(t, s.students[1], successorRows[0].StudentID)
	assert.Nil(t, successorRows[0].EnrollmentRequestChildID)

	// The predecessor keeps its rows: the bounded sourced row is history of
	// the capped segment, the manual row is capped at the split date.
	oldRows := loadSplitEnrollments(t, s, result.TemplateID)
	require.Len(t, oldRows, 2)
	byStudent := map[int64]*activitiesModels.StudentEnrollment{}
	for _, row := range oldRows {
		byStudent[row.StudentID] = row
	}
	require.NotNil(t, byStudent[s.students[0]])
	assert.NotNil(t, byStudent[s.students[0]].EnrollmentRequestChildID)
	require.NotNil(t, byStudent[s.students[1]])
	require.NotNil(t, byStudent[s.students[1]].ValidUntil)
	assert.Equal(t, activitiesModels.Date(monday), *byStudent[s.students[1]].ValidUntil)
}

// A split request may change the source rule itself (#2147 review round 14):
// the editor lets the admin adjust the Quelle or the Jahrgangsfilter and then
// pick the "following" scope. The provided fields must land on the successor,
// and the changed feed must reconcile the carried roster — a row of a child
// the new rule no longer wants may not survive the carry-over.
func TestTemplateOfferingSource_SplitAppliesRequestedSourceChange(t *testing.T) {
	t.Parallel()

	monday := futureMonday(1)
	s := makeScenario(t, activitiesModels.WeekdayMonday, monday)
	defer s.runCleanup(t)

	offering := createSourceCareOffering(t, s, timezone.Date(s.period.StartDate), timezone.Date(s.period.EndDate))
	name := fmt.Sprintf("Angebots-Termin-%d", time.Now().UnixNano())

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
		SourceGradeLevels:     []int{1, 2},
		RosterValidFrom:       monday.AddDays(-30),
		GradeLevelMax:         schoolclass.MaxGradeLevel,
	})
	require.NoError(t, err)
	registerSourcedTemplateCleanup(t, s, result.TemplateID, result.TimeframeID)

	// One source-fed row the old rule planned; the child holds no approved
	// offering link, so ANY reconciliation removes it — its survival therefore
	// tells apart the plain carry-over (unchanged rule, see the inheritance
	// test) from the resync a changed rule must trigger.
	enrollments := ownedStudentEnrollmentRepository(t, s.db)
	childID := createSplitRequestChild(t, s, s.students[0])
	sourcedUntil := activitiesModels.Date(s.period.EndDate.AddDays(1))
	sourced := &activitiesModels.StudentEnrollment{
		StudentID:                s.students[0],
		ActivityGroupID:          result.TemplateID,
		ValidFrom:                activitiesModels.Date(s.period.StartDate),
		ValidUntil:               &sourcedUntil,
		CalendarPeriodID:         &s.period.ID,
		EnrollmentRequestChildID: &childID,
		SelectedWeekdays:         []int{activitiesModels.WeekdayMonday},
	}
	sourced.SetTenantID(s.tenantID)
	require.NoError(t, enrollments.Create(s.ctx, sourced))

	split, err := s.factory.TemplateSplit.Split(s.ctx, scheduleSvc.TemplateSplitInput{
		TemplateID:                result.TemplateID,
		EffectiveDate:             monday,
		Name:                      name,
		Type:                      activitiesModels.GroupTypeCare,
		Weekdays:                  []int{activitiesModels.WeekdayMonday},
		StartTime:                 time.Date(2000, 1, 1, 16, 0, 0, 0, time.UTC),
		EndTime:                   time.Date(2000, 1, 1, 17, 0, 0, 0, time.UTC),
		RoomID:                    s.roomID,
		CategoryID:                s.categoryID,
		CalendarPeriodID:          &s.period.ID,
		TargetGroupType:           activitiesModels.TargetGroupTypeAngebot,
		SourceGradeLevels:         []int{3},
		SourceGradeLevelsProvided: true,
		GradeLevelMax:             schoolclass.MaxGradeLevel,
	})
	require.NoError(t, err)
	registerSourcedTemplateCleanup(t, s, split.NewTemplateID)

	successor := loadTemplateGroup(t, s, split.NewTemplateID)
	require.NotEmpty(t, successor.SourceCareOfferingIDs, "the omitted source id must still inherit")
	assert.Equal(t, []int64{offering.ID}, successor.SourceCareOfferingIDs)
	assert.Equal(t, []int{3}, successor.SourceGradeLevels,
		"the provided Jahrgangsfilter must land on the successor instead of the inherited one")

	assert.Empty(t, loadSplitEnrollments(t, s, split.NewTemplateID),
		"the changed rule must reconcile the carried roster, not keep the old feed's row")
}

// Keeping the 'angebot' Zielgruppe while explicitly clearing the source
// (source_care_offering_id: null) turns the successor into a manual template:
// the rule is gone and the source-fed rows the carry-over copied are removed,
// while the manual roster survives (#2147 review round 14).
func TestTemplateOfferingSource_SplitClearsSourceOnExplicitNull(t *testing.T) {
	t.Parallel()

	monday := futureMonday(1)
	s := makeScenario(t, activitiesModels.WeekdayMonday, monday)
	defer s.runCleanup(t)

	offering := createSourceCareOffering(t, s, timezone.Date(s.period.StartDate), timezone.Date(s.period.EndDate))
	name := fmt.Sprintf("Angebots-Termin-%d", time.Now().UnixNano())

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
		RosterValidFrom:       monday.AddDays(-30),
		GradeLevelMax:         schoolclass.MaxGradeLevel,
	})
	require.NoError(t, err)
	registerSourcedTemplateCleanup(t, s, result.TemplateID, result.TimeframeID)

	enrollments := ownedStudentEnrollmentRepository(t, s.db)
	childID := createSplitRequestChild(t, s, s.students[0])
	sourcedUntil := activitiesModels.Date(s.period.EndDate.AddDays(1))
	sourced := &activitiesModels.StudentEnrollment{
		StudentID:                s.students[0],
		ActivityGroupID:          result.TemplateID,
		ValidFrom:                activitiesModels.Date(s.period.StartDate),
		ValidUntil:               &sourcedUntil,
		CalendarPeriodID:         &s.period.ID,
		EnrollmentRequestChildID: &childID,
		SelectedWeekdays:         []int{activitiesModels.WeekdayMonday},
	}
	sourced.SetTenantID(s.tenantID)
	require.NoError(t, enrollments.Create(s.ctx, sourced))
	manual := &activitiesModels.StudentEnrollment{
		StudentID:        s.students[1],
		ActivityGroupID:  result.TemplateID,
		ValidFrom:        activitiesModels.Date(monday.AddDays(-30)),
		CalendarPeriodID: &s.period.ID,
	}
	manual.SetTenantID(s.tenantID)
	require.NoError(t, enrollments.Create(s.ctx, manual))

	split, err := s.factory.TemplateSplit.Split(s.ctx, scheduleSvc.TemplateSplitInput{
		TemplateID:                    result.TemplateID,
		EffectiveDate:                 monday,
		Name:                          name,
		Type:                          activitiesModels.GroupTypeCare,
		Weekdays:                      []int{activitiesModels.WeekdayMonday},
		StartTime:                     time.Date(2000, 1, 1, 16, 0, 0, 0, time.UTC),
		EndTime:                       time.Date(2000, 1, 1, 17, 0, 0, 0, time.UTC),
		RoomID:                        s.roomID,
		CategoryID:                    s.categoryID,
		CalendarPeriodID:              &s.period.ID,
		TargetGroupType:               activitiesModels.TargetGroupTypeAngebot,
		SourceCareOfferingIDsProvided: true,
		GradeLevelMax:                 schoolclass.MaxGradeLevel,
	})
	require.NoError(t, err)
	registerSourcedTemplateCleanup(t, s, split.NewTemplateID)

	successor := loadTemplateGroup(t, s, split.NewTemplateID)
	assert.Empty(t, successor.SourceCareOfferingIDs)
	assert.Empty(t, successor.SourceGradeLevels)

	successorRows := loadSplitEnrollments(t, s, split.NewTemplateID)
	require.Len(t, successorRows, 1, "the successor must carry only the manual row")
	assert.Equal(t, s.students[1], successorRows[0].StudentID)
	assert.Nil(t, successorRows[0].EnrollmentRequestChildID)
}

// A source whose Anmeldephase reaches beyond the template's Kalenderzeitraum
// could never materialize its children, so the template save rejects it
// instead of persisting a dead rule (#2137).
func TestTemplateOfferingSource_RejectsOfferingOutsideTheTemplatePeriod(t *testing.T) {
	t.Parallel()

	monday := futureMonday(1)
	s := makeScenario(t, activitiesModels.WeekdayMonday, monday)
	defer s.runCleanup(t)

	fitting := createSourceCareOffering(t, s, timezone.Date(s.period.StartDate), timezone.Date(s.period.EndDate))
	overhanging := createSourceCareOffering(t, s, timezone.Date(s.period.StartDate), timezone.Date(s.period.EndDate).AddDays(30))
	name := fmt.Sprintf("Angebots-Termin-%d", time.Now().UnixNano())

	createInput := scheduleSvc.CreateTemplateInput{
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
		SourceCareOfferingIDs: []int64{overhanging.ID},
		RosterValidFrom:       monday.AddDays(-30),
		GradeLevelMax:         schoolclass.MaxGradeLevel,
	}

	_, err := s.factory.TimetableData.CreateTemplate(s.ctx, createInput)
	require.ErrorIs(t, err, scheduleSvc.ErrOfferingSourceInvalid)

	// Nothing may survive the rejected create — the whole save is one tx.
	sourced, err := repositories.NewFactory(s.db, repositories.NewUnobservedTimetableDependencies(s.db)).ActivityGroup.FindTemplatesBySourceOffering(s.ctx, overhanging.ID)
	require.NoError(t, err)
	assert.Empty(t, sourced)

	// Same rule on the edit path: an existing sourced template cannot be
	// re-pointed at an offering that does not fit its period.
	createInput.SourceCareOfferingIDs = []int64{fitting.ID}
	result, err := s.factory.TimetableData.CreateTemplate(s.ctx, createInput)
	require.NoError(t, err)
	registerSourcedTemplateCleanup(t, s, result.TemplateID, result.TimeframeID)

	err = s.factory.TimetableData.UpdateTemplate(s.ctx, scheduleSvc.TemplateUpdateInput{
		TemplateID: result.TemplateID,
		Fields: activitiesModels.TemplateFieldsUpdate{
			Name:                  name,
			Type:                  activitiesModels.GroupTypeCare,
			CategoryID:            s.categoryID,
			RoomID:                s.roomID,
			MaxParticipants:       20,
			CalendarPeriodID:      &s.period.ID,
			TargetGroupType:       activitiesModels.TargetGroupTypeAngebot,
			SourceCareOfferingIDs: []int64{overhanging.ID},
		},
		Weekdays:         []int{activitiesModels.WeekdayMonday},
		TimeframeID:      result.TimeframeID,
		CalendarPeriodID: &s.period.ID,
		RosterValidFrom:  monday.AddDays(-30),
		GradeLevelMax:    schoolclass.MaxGradeLevel,
	})
	require.ErrorIs(t, err, scheduleSvc.ErrOfferingSourceInvalid)

	kept := loadTemplateGroup(t, s, result.TemplateID)
	require.NotEmpty(t, kept.SourceCareOfferingIDs)
	assert.Equal(t, []int64{fitting.ID}, kept.SourceCareOfferingIDs,
		"a rejected edit must leave the previous source in place")
}

// The editor's selector only offers active offerings; a direct request naming
// a retired offering must be rejected the same way (#2147 review round 9).
func TestTemplateOfferingSource_RejectsInactiveOffering(t *testing.T) {
	t.Parallel()

	monday := futureMonday(1)
	s := makeScenario(t, activitiesModels.WeekdayMonday, monday)
	defer s.runCleanup(t)

	offering := createSourceCareOffering(t, s, timezone.Date(s.period.StartDate), timezone.Date(s.period.EndDate))
	_, err := s.db.NewRaw(`UPDATE enrollment.care_offerings SET is_active = FALSE WHERE id = ?`, offering.ID).Exec(s.ctx)
	require.NoError(t, err)

	_, err = s.factory.TimetableData.CreateTemplate(s.ctx, scheduleSvc.CreateTemplateInput{
		Name:                  fmt.Sprintf("Angebots-Termin-%d", time.Now().UnixNano()),
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
		RosterValidFrom:       monday.AddDays(-30),
		GradeLevelMax:         schoolclass.MaxGradeLevel,
	})
	require.ErrorIs(t, err, scheduleSvc.ErrOfferingSourceInvalid)

	sourced, err := repositories.NewFactory(s.db, repositories.NewUnobservedTimetableDependencies(s.db)).ActivityGroup.FindTemplatesBySourceOffering(s.ctx, offering.ID)
	require.NoError(t, err)
	assert.Empty(t, sourced, "nothing may survive the rejected create")
}

// A split may re-pin the successor to another Kalenderzeitraum. Carrying the
// inherited source into a period the offering's phase does not fit would
// persist a state create/update reject, so the split must refuse it too
// (#2147 review).
func TestTemplateOfferingSource_SplitRejectsOfferingOutsideNewPeriod(t *testing.T) {
	t.Parallel()

	monday := futureMonday(1)
	s := makeScenario(t, activitiesModels.WeekdayMonday, monday)
	defer s.runCleanup(t)

	offering := createSourceCareOffering(t, s, timezone.Date(s.period.StartDate), timezone.Date(s.period.EndDate))
	name := fmt.Sprintf("Angebots-Termin-%d", time.Now().UnixNano())

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
		SourceGradeLevels:     []int{1, 2},
		RosterValidFrom:       monday.AddDays(-30),
		GradeLevelMax:         schoolclass.MaxGradeLevel,
	})
	require.NoError(t, err)
	registerSourcedTemplateCleanup(t, s, result.TemplateID, result.TimeframeID)

	// A period starting after the offering's phase begins — the phase no
	// longer fits, so the successor could never materialize the source.
	latePeriod := &scheduleModels.CalendarPeriod{
		Name:            fmt.Sprintf("Spaetzeitraum-%d", time.Now().UnixNano()),
		PeriodType:      scheduleModels.PeriodTypeSchoolYear,
		StartDate:       s.period.StartDate.AddDays(60),
		EndDate:         s.period.EndDate,
		WeekCycleLength: 1,
		IsActive:        true,
	}
	latePeriod.SetTenantID(s.tenantID)
	require.NoError(t, repositories.NewFactory(s.db, repositories.NewUnobservedTimetableDependencies(s.db)).CalendarPeriod.Create(s.ctx, latePeriod))
	s.extraCleanups = append([]func(){func() {
	}}, s.extraCleanups...)

	_, err = s.factory.TemplateSplit.Split(s.ctx, scheduleSvc.TemplateSplitInput{
		TemplateID:       result.TemplateID,
		EffectiveDate:    monday,
		Name:             name,
		Type:             activitiesModels.GroupTypeCare,
		Weekdays:         []int{activitiesModels.WeekdayMonday},
		StartTime:        time.Date(2000, 1, 1, 16, 0, 0, 0, time.UTC),
		EndTime:          time.Date(2000, 1, 1, 17, 0, 0, 0, time.UTC),
		RoomID:           s.roomID,
		CategoryID:       s.categoryID,
		CalendarPeriodID: &latePeriod.ID,
		TargetGroupType:  activitiesModels.TargetGroupTypeAngebot,
		GradeLevelMax:    schoolclass.MaxGradeLevel,
	})
	require.ErrorIs(t, err, scheduleSvc.ErrOfferingSourceInvalid)

	// The rejected split must leave no successor behind.
	sourced, err := repositories.NewFactory(s.db, repositories.NewUnobservedTimetableDependencies(s.db)).ActivityGroup.FindTemplatesBySourceOffering(s.ctx, offering.ID)
	require.NoError(t, err)
	require.Len(t, sourced, 1)
	assert.Equal(t, result.TemplateID, sourced[0].ID)
}

// Removing a template's source while re-picking a child by hand must leave
// that child on the template's already-materialized future occurrences: the
// source resync runs BEFORE the roster replacement and clears the child's
// still-planned instance rows, so the update has to reconcile the manual
// roster back onto existing occurrences afterwards (#2147 review, round 4).
func TestTemplateOfferingSource_SourceRemovalKeepsManualChildOnOccurrences(t *testing.T) {
	t.Parallel()

	monday := futureMonday(2)
	s := makeScenario(t, activitiesModels.WeekdayMonday, monday)
	defer s.runCleanup(t)

	offering := createSourceCareOffering(t, s, timezone.Date(s.period.StartDate), timezone.Date(s.period.EndDate))
	name := fmt.Sprintf("Angebots-Termin-%d", time.Now().UnixNano())

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
		RosterValidFrom:       monday.AddDays(-30),
		GradeLevelMax:         schoolclass.MaxGradeLevel,
	})
	require.NoError(t, err)
	registerSourcedTemplateCleanup(t, s, result.TemplateID, result.TimeframeID)

	// One occurrence is already materialized when the edit happens.
	instance := &scheduleModels.ActivityInstance{
		Date:             scheduleModels.Date(monday),
		ActivityGroupID:  &result.TemplateID,
		CalendarPeriodID: &s.period.ID,
		Title:            name,
		StartTime:        time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC),
		EndTime:          time.Date(2000, 1, 1, 16, 0, 0, 0, time.UTC),
		RoomID:           s.roomID,
		Status:           scheduleModels.InstanceStatusPlanned,
	}
	instance.SetTenantID(s.tenantID)
	_, err = s.db.NewInsert().Model(instance).ModelTableExpr(`schedule.activity_instances`).Exec(s.ctx)
	require.NoError(t, err)
	s.extraCleanups = append([]func(){func() {
		_, _ = s.db.NewRaw(`DELETE FROM schedule.instance_students WHERE instance_id = ?`, instance.ID).Exec(s.ctx)
	}}, s.extraCleanups...)

	manualStudentID := s.students[0]
	require.NoError(t, s.factory.TimetableData.UpdateTemplate(s.ctx, scheduleSvc.TemplateUpdateInput{
		TemplateID: result.TemplateID,
		Fields: activitiesModels.TemplateFieldsUpdate{
			Name:             name,
			Type:             activitiesModels.GroupTypeCare,
			CategoryID:       s.categoryID,
			RoomID:           s.roomID,
			MaxParticipants:  20,
			CalendarPeriodID: &s.period.ID,
			TargetGroupType:  activitiesModels.TargetGroupTypeNone,
		},
		Weekdays:         []int{activitiesModels.WeekdayMonday},
		TimeframeID:      result.TimeframeID,
		CalendarPeriodID: &s.period.ID,
		RosterValidFrom:  monday.AddDays(-30),
		StudentIDs:       []int64{manualStudentID},
		GradeLevelMax:    schoolclass.MaxGradeLevel,
	}))

	var rows []scheduleModels.InstanceStudent
	require.NoError(t, s.db.NewSelect().
		Model(&rows).
		ModelTableExpr(`schedule.instance_students AS "instance_student"`).
		Where(`"instance_student".instance_id = ?`, instance.ID).
		Scan(s.ctx))
	require.Len(t, rows, 1, "the manually re-picked child must land on the pre-existing occurrence")
	assert.Equal(t, manualStudentID, rows[0].StudentID)
	assert.Equal(t, scheduleModels.AttendanceStatusExpected, rows[0].Status)
}

// Converting a manual template to an offering-sourced one retires the manual
// enrollment rows — and the retired students must also leave the template's
// already-materialized future occurrences, because the materializer never
// revisits an existing instance (#2147 review, round 5).
func TestTemplateOfferingSource_ConversionRemovesRetiredManualChildFromOccurrences(t *testing.T) {
	t.Parallel()

	monday := futureMonday(3)
	s := makeScenario(t, activitiesModels.WeekdayMonday, monday)
	defer s.runCleanup(t)

	offering := createSourceCareOffering(t, s, timezone.Date(s.period.StartDate), timezone.Date(s.period.EndDate))
	name := fmt.Sprintf("Angebots-Termin-%d", time.Now().UnixNano())
	manualStudentID := s.students[0]

	result, err := s.factory.TimetableData.CreateTemplate(s.ctx, scheduleSvc.CreateTemplateInput{
		Name:             name,
		Type:             activitiesModels.GroupTypeCare,
		Weekdays:         []int{activitiesModels.WeekdayMonday},
		StartTime:        time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC),
		EndTime:          time.Date(2000, 1, 1, 16, 0, 0, 0, time.UTC),
		RoomID:           s.roomID,
		CategoryID:       s.categoryID,
		MaxParticipants:  20,
		CalendarPeriodID: &s.period.ID,
		StudentIDs:       []int64{manualStudentID},
		RosterValidFrom:  monday.AddDays(-30),
		GradeLevelMax:    schoolclass.MaxGradeLevel,
	})
	require.NoError(t, err)
	registerSourcedTemplateCleanup(t, s, result.TemplateID, result.TimeframeID)

	// One occurrence with the manual child is already materialized when the
	// conversion happens.
	instance := &scheduleModels.ActivityInstance{
		Date:             scheduleModels.Date(monday),
		ActivityGroupID:  &result.TemplateID,
		CalendarPeriodID: &s.period.ID,
		Title:            name,
		StartTime:        time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC),
		EndTime:          time.Date(2000, 1, 1, 16, 0, 0, 0, time.UTC),
		RoomID:           s.roomID,
		Status:           scheduleModels.InstanceStatusPlanned,
	}
	instance.SetTenantID(s.tenantID)
	_, err = s.db.NewInsert().Model(instance).ModelTableExpr(`schedule.activity_instances`).Exec(s.ctx)
	require.NoError(t, err)
	s.extraCleanups = append([]func(){func() {
		_, _ = s.db.NewRaw(`DELETE FROM schedule.instance_students WHERE instance_id = ?`, instance.ID).Exec(s.ctx)
	}}, s.extraCleanups...)
	instanceStudent := &scheduleModels.InstanceStudent{
		InstanceID: instance.ID,
		StudentID:  manualStudentID,
		Status:     scheduleModels.AttendanceStatusExpected,
	}
	instanceStudent.SetTenantID(s.tenantID)
	_, err = s.db.NewInsert().Model(instanceStudent).ModelTableExpr(`schedule.instance_students`).Exec(s.ctx)
	require.NoError(t, err)

	// The offering has no enrolled children, so the new source covers nobody:
	// the retired manual child must disappear from the existing occurrence.
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
		},
		Weekdays:         []int{activitiesModels.WeekdayMonday},
		TimeframeID:      result.TimeframeID,
		CalendarPeriodID: &s.period.ID,
		RosterValidFrom:  monday.AddDays(-30),
		GradeLevelMax:    schoolclass.MaxGradeLevel,
	}))

	var rows []scheduleModels.InstanceStudent
	require.NoError(t, s.db.NewSelect().
		Model(&rows).
		ModelTableExpr(`schedule.instance_students AS "instance_student"`).
		Where(`"instance_student".instance_id = ?`, instance.ID).
		Scan(s.ctx))
	require.Empty(t, rows, "the retired manual child must be removed from the already-materialized occurrence")
}
