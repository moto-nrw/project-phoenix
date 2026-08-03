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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/schoolclass"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
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
	phase := &enrollmentModels.Phase{
		Name:                      fmt.Sprintf("Quell-Phase %d", suffix),
		Kind:                      enrollmentModels.PhaseKindCustom,
		ServiceStartDate:          serviceStart,
		ServiceEndDate:            serviceEnd,
		CareOverflowMode:          enrollmentModels.PhaseCareOverflowWaitlist,
		CareOfferingSelectionMode: enrollmentModels.PhaseCareOfferingSelectionOptional,
		AvailableSchoolClasses:    []string{},
		IsActive:                  true,
	}
	phase.SetTenantID(s.tenantID)
	repos := repositories.NewFactory(s.db)
	require.NoError(t, repos.Phase.Create(s.ctx, phase))

	offering := &enrollmentModels.CareOffering{
		PhaseID:            phase.ID,
		Name:               fmt.Sprintf("Frühbetreuung %d", suffix),
		DaysOfWeekMode:     enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:      []string{"mon"},
		IsActive:           true,
		CountsAsCare:       true,
		CountsAsCareSet:    true,
		SelectionRule:      enrollmentModels.SelectionRuleOptional,
		AutoAddGradeLevels: []int{},
	}
	var created *enrollmentModels.CareOffering
	require.NoError(t, tenant.WithTenantTx(s.ctx, s.db, s.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		var createErr error
		created, createErr = s.factory.EnrollmentCareOffering.Create(txCtx, offering)
		return createErr
	}))

	s.extraCleanups = append([]func(){func() {
		testpkg.CleanupTableRecords(t, s.db, "enrollment.care_offerings", created.ID)
		testpkg.CleanupTableRecords(t, s.db, "enrollment.phases", phase.ID)
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
		if len(timeframeIDs) > 0 {
			testpkg.CleanupTableRecords(t, s.db, "schedule.timeframes", timeframeIDs...)
		}
	}}, s.extraCleanups...)
}

func loadTemplateGroup(t *testing.T, s *scenarioSetup, templateID int64) *activitiesModels.Group {
	t.Helper()
	group, err := repositories.NewFactory(s.db).ActivityGroup.FindByID(s.ctx, templateID)
	require.NoError(t, err)
	require.NotNil(t, group)
	return group
}

func TestTemplateOfferingSource_CreateStoresTheRuleAndIsFoundByOffering(t *testing.T) {
	monday := futureMonday(1)
	s := makeScenario(t, activitiesModels.WeekdayMonday, monday)
	defer s.runCleanup(t)

	offering := createSourceCareOffering(t, s, s.period.StartDate, s.period.EndDate)

	result, err := s.factory.TimetableData.CreateTemplate(s.ctx, scheduleSvc.CreateTemplateInput{
		Name:                 fmt.Sprintf("Angebots-Termin-%d", time.Now().UnixNano()),
		Type:                 activitiesModels.GroupTypeCare,
		Weekdays:             []int{activitiesModels.WeekdayMonday},
		StartTime:            time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC),
		EndTime:              time.Date(2000, 1, 1, 16, 0, 0, 0, time.UTC),
		RoomID:               s.roomID,
		CategoryID:           s.categoryID,
		MaxParticipants:      20,
		CalendarPeriodID:     &s.period.ID,
		TargetGroupType:      activitiesModels.TargetGroupTypeAngebot,
		SourceCareOfferingID: &offering.ID,
		SourceGradeLevels:    []int{1, 2},
		RosterValidFrom:      monday.AddDays(-30),
		GradeLevelMax:        schoolclass.MaxGradeLevel,
	})
	require.NoError(t, err)
	registerSourcedTemplateCleanup(t, s, result.TemplateID, result.TimeframeID)

	group := loadTemplateGroup(t, s, result.TemplateID)
	require.NotNil(t, group.SourceCareOfferingID)
	assert.Equal(t, offering.ID, *group.SourceCareOfferingID)
	assert.Equal(t, []int{1, 2}, group.SourceGradeLevels)

	// The reverse lookup is what the editor's overlap hint and the decision
	// fan-out both read: one offering, every live template sourcing it.
	sourced, err := repositories.NewFactory(s.db).ActivityGroup.FindTemplatesBySourceOffering(s.ctx, offering.ID)
	require.NoError(t, err)
	require.Len(t, sourced, 1)
	assert.Equal(t, result.TemplateID, sourced[0].ID)
}

func TestTemplateOfferingSource_UpdateRewritesAndClearsTheRule(t *testing.T) {
	monday := futureMonday(1)
	s := makeScenario(t, activitiesModels.WeekdayMonday, monday)
	defer s.runCleanup(t)

	offering := createSourceCareOffering(t, s, s.period.StartDate, s.period.EndDate)
	name := fmt.Sprintf("Angebots-Termin-%d", time.Now().UnixNano())

	result, err := s.factory.TimetableData.CreateTemplate(s.ctx, scheduleSvc.CreateTemplateInput{
		Name:                 name,
		Type:                 activitiesModels.GroupTypeCare,
		Weekdays:             []int{activitiesModels.WeekdayMonday},
		StartTime:            time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC),
		EndTime:              time.Date(2000, 1, 1, 16, 0, 0, 0, time.UTC),
		RoomID:               s.roomID,
		CategoryID:           s.categoryID,
		MaxParticipants:      20,
		CalendarPeriodID:     &s.period.ID,
		TargetGroupType:      activitiesModels.TargetGroupTypeAngebot,
		SourceCareOfferingID: &offering.ID,
		RosterValidFrom:      monday.AddDays(-30),
		GradeLevelMax:        schoolclass.MaxGradeLevel,
	})
	require.NoError(t, err)
	registerSourcedTemplateCleanup(t, s, result.TemplateID, result.TimeframeID)

	updateInput := scheduleSvc.TemplateUpdateInput{
		TemplateID: result.TemplateID,
		Fields: activitiesModels.TemplateFieldsUpdate{
			Name:                 name,
			Type:                 activitiesModels.GroupTypeCare,
			CategoryID:           s.categoryID,
			RoomID:               s.roomID,
			MaxParticipants:      20,
			CalendarPeriodID:     &s.period.ID,
			TargetGroupType:      activitiesModels.TargetGroupTypeAngebot,
			SourceCareOfferingID: &offering.ID,
			SourceGradeLevels:    []int{3},
		},
		Weekdays:         []int{activitiesModels.WeekdayMonday},
		TimeframeID:      result.TimeframeID,
		CalendarPeriodID: &s.period.ID,
		RosterValidFrom:  monday.AddDays(-30),
		GradeLevelMax:    schoolclass.MaxGradeLevel,
	}
	require.NoError(t, s.factory.TimetableData.UpdateTemplate(s.ctx, updateInput))

	narrowed := loadTemplateGroup(t, s, result.TemplateID)
	require.NotNil(t, narrowed.SourceCareOfferingID)
	assert.Equal(t, []int{3}, narrowed.SourceGradeLevels,
		"narrowing the Jahrgang filter must persist, not merge with the old one")

	// Removing the source degrades the Regeltermin to a manually curated
	// roster; the filter must not survive its source (DB CHECK).
	updateInput.Fields.SourceCareOfferingID = nil
	updateInput.Fields.SourceGradeLevels = nil
	require.NoError(t, s.factory.TimetableData.UpdateTemplate(s.ctx, updateInput))

	cleared := loadTemplateGroup(t, s, result.TemplateID)
	assert.Nil(t, cleared.SourceCareOfferingID)
	assert.Empty(t, cleared.SourceGradeLevels)

	sourced, err := repositories.NewFactory(s.db).ActivityGroup.FindTemplatesBySourceOffering(s.ctx, offering.ID)
	require.NoError(t, err)
	assert.Empty(t, sourced, "a cleared source must drop out of the offering's template list")
}

func TestTemplateOfferingSource_SplitSuccessorInheritsTheRule(t *testing.T) {
	monday := futureMonday(1)
	s := makeScenario(t, activitiesModels.WeekdayMonday, monday)
	defer s.runCleanup(t)

	offering := createSourceCareOffering(t, s, s.period.StartDate, s.period.EndDate)
	name := fmt.Sprintf("Angebots-Termin-%d", time.Now().UnixNano())

	result, err := s.factory.TimetableData.CreateTemplate(s.ctx, scheduleSvc.CreateTemplateInput{
		Name:                 name,
		Type:                 activitiesModels.GroupTypeCare,
		Weekdays:             []int{activitiesModels.WeekdayMonday},
		StartTime:            time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC),
		EndTime:              time.Date(2000, 1, 1, 16, 0, 0, 0, time.UTC),
		RoomID:               s.roomID,
		CategoryID:           s.categoryID,
		MaxParticipants:      20,
		CalendarPeriodID:     &s.period.ID,
		TargetGroupType:      activitiesModels.TargetGroupTypeAngebot,
		SourceCareOfferingID: &offering.ID,
		SourceGradeLevels:    []int{1, 2},
		RosterValidFrom:      monday.AddDays(-30),
		GradeLevelMax:        schoolclass.MaxGradeLevel,
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
	require.NotNil(t, successor.SourceCareOfferingID)
	assert.Equal(t, offering.ID, *successor.SourceCareOfferingID)
	assert.Equal(t, []int{1, 2}, successor.SourceGradeLevels)
}

func TestTemplateOfferingSource_SplitAwayFromAngebotDropsTheRule(t *testing.T) {
	monday := futureMonday(1)
	s := makeScenario(t, activitiesModels.WeekdayMonday, monday)
	defer s.runCleanup(t)

	offering := createSourceCareOffering(t, s, s.period.StartDate, s.period.EndDate)
	name := fmt.Sprintf("Angebots-Termin-%d", time.Now().UnixNano())

	result, err := s.factory.TimetableData.CreateTemplate(s.ctx, scheduleSvc.CreateTemplateInput{
		Name:                 name,
		Type:                 activitiesModels.GroupTypeCare,
		Weekdays:             []int{activitiesModels.WeekdayMonday},
		StartTime:            time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC),
		EndTime:              time.Date(2000, 1, 1, 16, 0, 0, 0, time.UTC),
		RoomID:               s.roomID,
		CategoryID:           s.categoryID,
		MaxParticipants:      20,
		CalendarPeriodID:     &s.period.ID,
		TargetGroupType:      activitiesModels.TargetGroupTypeAngebot,
		SourceCareOfferingID: &offering.ID,
		SourceGradeLevels:    []int{1, 2},
		RosterValidFrom:      monday.AddDays(-30),
		GradeLevelMax:        schoolclass.MaxGradeLevel,
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
	assert.Nil(t, successor.SourceCareOfferingID)
	assert.Empty(t, successor.SourceGradeLevels)
}

// A source whose Anmeldephase reaches beyond the template's Kalenderzeitraum
// could never materialize its children, so the template save rejects it
// instead of persisting a dead rule (#2137).
func TestTemplateOfferingSource_RejectsOfferingOutsideTheTemplatePeriod(t *testing.T) {
	monday := futureMonday(1)
	s := makeScenario(t, activitiesModels.WeekdayMonday, monday)
	defer s.runCleanup(t)

	fitting := createSourceCareOffering(t, s, s.period.StartDate, s.period.EndDate)
	overhanging := createSourceCareOffering(t, s, s.period.StartDate, s.period.EndDate.AddDays(30))
	name := fmt.Sprintf("Angebots-Termin-%d", time.Now().UnixNano())

	createInput := scheduleSvc.CreateTemplateInput{
		Name:                 name,
		Type:                 activitiesModels.GroupTypeCare,
		Weekdays:             []int{activitiesModels.WeekdayMonday},
		StartTime:            time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC),
		EndTime:              time.Date(2000, 1, 1, 16, 0, 0, 0, time.UTC),
		RoomID:               s.roomID,
		CategoryID:           s.categoryID,
		MaxParticipants:      20,
		CalendarPeriodID:     &s.period.ID,
		TargetGroupType:      activitiesModels.TargetGroupTypeAngebot,
		SourceCareOfferingID: &overhanging.ID,
		RosterValidFrom:      monday.AddDays(-30),
		GradeLevelMax:        schoolclass.MaxGradeLevel,
	}

	_, err := s.factory.TimetableData.CreateTemplate(s.ctx, createInput)
	require.ErrorIs(t, err, scheduleSvc.ErrOfferingSourceInvalid)

	// Nothing may survive the rejected create — the whole save is one tx.
	sourced, err := repositories.NewFactory(s.db).ActivityGroup.FindTemplatesBySourceOffering(s.ctx, overhanging.ID)
	require.NoError(t, err)
	assert.Empty(t, sourced)

	// Same rule on the edit path: an existing sourced template cannot be
	// re-pointed at an offering that does not fit its period.
	createInput.SourceCareOfferingID = &fitting.ID
	result, err := s.factory.TimetableData.CreateTemplate(s.ctx, createInput)
	require.NoError(t, err)
	registerSourcedTemplateCleanup(t, s, result.TemplateID, result.TimeframeID)

	err = s.factory.TimetableData.UpdateTemplate(s.ctx, scheduleSvc.TemplateUpdateInput{
		TemplateID: result.TemplateID,
		Fields: activitiesModels.TemplateFieldsUpdate{
			Name:                 name,
			Type:                 activitiesModels.GroupTypeCare,
			CategoryID:           s.categoryID,
			RoomID:               s.roomID,
			MaxParticipants:      20,
			CalendarPeriodID:     &s.period.ID,
			TargetGroupType:      activitiesModels.TargetGroupTypeAngebot,
			SourceCareOfferingID: &overhanging.ID,
		},
		Weekdays:         []int{activitiesModels.WeekdayMonday},
		TimeframeID:      result.TimeframeID,
		CalendarPeriodID: &s.period.ID,
		RosterValidFrom:  monday.AddDays(-30),
		GradeLevelMax:    schoolclass.MaxGradeLevel,
	})
	require.ErrorIs(t, err, scheduleSvc.ErrOfferingSourceInvalid)

	kept := loadTemplateGroup(t, s, result.TemplateID)
	require.NotNil(t, kept.SourceCareOfferingID)
	assert.Equal(t, fitting.ID, *kept.SourceCareOfferingID,
		"a rejected edit must leave the previous source in place")
}
