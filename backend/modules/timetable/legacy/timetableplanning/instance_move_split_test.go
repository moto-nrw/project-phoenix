package timetableplanning_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/schoolclass"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The single-week edit is the retained instance lifecycle's (UpdatePlanned,
// #3424 slice S1); the split, the full-series edit and the materialization
// are the Timetable owner's since slice S2. This suite pins how they meet.

// Mirrors the reported UI sequence end to end: edit one materialized week,
// split from the following occurrence, edit the complete successor segment,
// then re-plan across both dates. The moved single occurrence and its
// exception must not be joined by a backfilled successor occurrence.
func TestTemplateSplit_SingleEditThenSuccessorUpdateDoesNotDuplicate(t *testing.T) {
	t.Parallel()

	effective := futureMonday(2)
	singleDate := effective.AddDays(-7)
	s := makeScenario(t, activitiesModels.WeekdayMonday, singleDate)
	defer s.runCleanup(t)

	materialized, err := s.svc.MaterializeForTenant(s.ctx, singleDate, singleDate, timetable.MaterializationSourceManual)
	require.NoError(t, err)
	require.Equal(t, 1, materialized.InstancesCreated)
	instances := listInstancesForDate(t, s.db, s.template.ID, singleDate)
	require.Len(t, instances, 1)
	single := instances[0]

	// "Nur diese Woche": move the start by one hour. UpdatePlanned keeps the
	// instance on this date and writes a cancellation exception for the old
	// template slot so a re-plan cannot resurrect the original time.
	updatedSingle, err := s.factory.Instance.UpdatePlanned(
		s.ctx,
		single.ID,
		moveInput(s, single, singleDate, 1),
		nil,
	)
	require.NoError(t, err)
	assert.Equal(t, single.ID, updatedSingle.ID)
	exceptions := loadExceptions(t, s, s.template.ID)
	require.Len(t, exceptions, 1)
	assert.Equal(t, scheduleModels.Date(singleDate), exceptions[0].ExceptionDate)
	assert.Equal(t, scheduleModels.ActivityExceptionCancelled, exceptions[0].ExceptionType)

	// "Ab jetzt dauerhaft": split at the next Monday.
	in := baseSplitInput(s, effective, fmt.Sprintf("Split-Single-Following-%d", time.Now().UnixNano()))
	res, err := s.factory.TimetableData.Templates.SplitTemplate(s.ctx, in)
	require.NoError(t, err)

	// "Alle Termine der Serie" on the successor: a full schedule replacement
	// must retain valid_from=effective.
	successor := reloadSplitGroup(t, s, res.NewTemplateID)
	require.NotNil(t, successor.PlannedRoomID)
	err = s.factory.TimetableData.Templates.UpdateTemplate(s.ctx, timetable.UpdateTemplateCommand{
		TemplateID: res.NewTemplateID,
		Fields: timetable.TemplateFields{
			Name:              successor.Name + "-all-edited",
			Type:              successor.Type,
			CategoryID:        successor.CategoryID,
			RoomID:            *successor.PlannedRoomID,
			EducationGroupID:  successor.EducationGroupID,
			MaxParticipants:   successor.MaxParticipants,
			CalendarPeriodID:  successor.CalendarPeriodID,
			TargetGroupType:   successor.TargetGroupType,
			TargetGradeLevel:  successor.TargetGradeLevel,
			TargetSchoolClass: successor.TargetSchoolClass,
		},
		Weekdays:        []int{activitiesModels.WeekdayMonday},
		TimeframeID:     s.timeframe.ID,
		WeekPattern:     s.schedule.WeekPattern,
		RosterValidFrom: singleDate.AddDays(-30),
		StudentIDs:      []int64{s.students[0], s.students[1]},
		StaffIDs:        []int64{s.staffID},
		PrimaryStaffID:  &s.staffID,
		GradeLevelMax:   schoolclass.MaxGradeLevel,
	})
	require.NoError(t, err)
	successorSchedules := loadSplitSchedules(t, s, res.NewTemplateID)
	require.Len(t, successorSchedules, 1)
	require.NotNil(t, successorSchedules[0].ValidFrom)
	assert.Equal(t, activitiesModels.Date(effective), *successorSchedules[0].ValidFrom)

	// The frontend follows a full-series PUT with a successor-scoped re-plan.
	// Materialization still examines every template, making this the exact
	// point where an erased valid_from used to create the duplicate.
	result, err := s.factory.Instance.ReplanWeek(s.ctx, singleDate, effective, &res.NewTemplateID, nil)
	require.NoError(t, err)
	require.NotNil(t, result.Materialization)

	oldSingle := listInstancesForDate(t, s.db, s.template.ID, singleDate)
	require.Len(t, oldSingle, 1, "the explicitly edited week remains")
	assert.Equal(t, single.ID, oldSingle[0].ID)
	assert.Empty(t, listInstancesForDate(t, s.db, res.NewTemplateID, singleDate),
		"the successor must not backfill next to the single-week edit")
	assert.Empty(t, listInstancesForDate(t, s.db, s.template.ID, effective))
	assert.Len(t, listInstancesForDate(t, s.db, res.NewTemplateID, effective), 1)
}

// futureMonday returns the first Monday strictly after a fixed future date,
// shifted by offsetWeeks additional weeks. The split validates
// effective_date >= today, so split tests anchor on future dates.
func futureMonday(offsetWeeks int) timezone.Date {
	d := timezone.NewDate(2030, 8, 26).AddDays(1)
	for d.Weekday() != time.Monday {
		d = d.AddDays(1)
	}
	return d.AddDays(7 * offsetWeeks)
}

func baseSplitInput(s *scenarioSetup, effective timezone.Date, name string) timetable.SplitTemplateCommand {
	return timetable.SplitTemplateCommand{
		TemplateID:    s.template.ID,
		EffectiveDate: effective,
		Name:          name,
		Type:          activitiesModels.GroupTypeActivity,
		Weekdays:      []int{activitiesModels.WeekdayMonday},
		StartTime:     time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC),
		EndTime:       time.Date(2000, 1, 1, 16, 0, 0, 0, time.UTC),
		RoomID:        s.roomID,
		CategoryID:    s.categoryID,
		GradeLevelMax: schoolclass.MaxGradeLevel,
	}
}

func reloadSplitGroup(t *testing.T, s *scenarioSetup, id int64) *activitiesModels.Group {
	t.Helper()
	var row activitiesModels.Group
	err := s.db.NewSelect().Model(&row).
		ModelTableExpr(`activities.groups AS "group"`).
		Where(`"group".id = ?`, id).
		Where(`"group".tenant_id = ?`, s.tenantID).
		Scan(s.ctx)
	require.NoError(t, err)
	return &row
}

func loadSplitSchedules(t *testing.T, s *scenarioSetup, groupID int64) []*activitiesModels.Schedule {
	t.Helper()
	var rows []*activitiesModels.Schedule
	err := s.db.NewSelect().Model(&rows).
		ModelTableExpr(`activities.schedules AS "schedule"`).
		Where(`"schedule".activity_group_id = ?`, groupID).
		Where(`"schedule".tenant_id = ?`, s.tenantID).
		Order("weekday ASC").
		Scan(s.ctx)
	require.NoError(t, err)
	return rows
}
