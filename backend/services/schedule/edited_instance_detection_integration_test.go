package schedule_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Hermetic integration test for DetectEditedInWindow (#1875): real fixtures via
// makeScenario, real materialization, then a direct DB edit that simulates a
// "Nur diesen Termin" change, asserting the detector flags exactly the diverged
// field categories. The fixture template runs Mondays 08:00–16:00 in one room
// with one primary supervisor and two valid enrollments.

var editWindowStart = timezone.NewDate(2026, time.April, 20) // Monday

// materializeSingleInstance runs the scenario's week and returns its one
// planned instance, registering it for cleanup.
func materializeSingleInstance(t *testing.T, s *scenarioSetup) *scheduleModels.ActivityInstance {
	t.Helper()
	_, err := s.svc.MaterializeForTenant(s.ctx, editWindowStart, editWindowStart.AddDays(6), scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	rows := listInstancesForDate(t, s.db, s.template.ID, editWindowStart)
	require.Len(t, rows, 1)
	return rows[0]
}

// setInstanceColumn writes one column on the instance row, tenant-scoped.
func setInstanceColumn(t *testing.T, s *scenarioSetup, instanceID int64, col string, val any) {
	t.Helper()
	_, err := s.db.NewUpdate().
		Model((*scheduleModels.ActivityInstance)(nil)).
		ModelTableExpr(`schedule.activity_instances AS "activity_instance"`).
		Set(col+" = ?", val).
		Where(`"activity_instance".id = ?`, instanceID).
		Where("tenant_id = ?", s.tenantID).
		Exec(s.ctx)
	require.NoError(t, err)
}

func detect(t *testing.T, s *scenarioSetup) []scheduleSvc.EditedOccurrence {
	return detectWindow(t, s, false)
}

func detectWindow(t *testing.T, s *scenarioSetup, includeDeletions bool) []scheduleSvc.EditedOccurrence {
	t.Helper()
	edited, err := s.svc.DetectEditedInWindow(
		s.ctx, s.template.ID, editWindowStart, editWindowStart.AddDays(6), includeDeletions,
	)
	require.NoError(t, err)
	return edited
}

func TestDetectEditedInWindow_Pristine(t *testing.T) {
	t.Parallel()

	s := makeScenario(t, activitiesModels.WeekdayMonday, editWindowStart)
	defer s.runCleanup(t)
	materializeSingleInstance(t, s)

	assert.Empty(t, detect(t, s), "a freshly materialized occurrence matches its template")
}

func TestDetectEditedInWindow_DynamicTargetRosterMatchesMaterialization(t *testing.T) {
	t.Parallel()

	for _, targetType := range []string{
		activitiesModels.TargetGroupTypeKlasse,
		activitiesModels.TargetGroupTypeJahrgang,
		activitiesModels.TargetGroupTypeGruppe,
	} {
		t.Run(targetType, func(t *testing.T) {
			s := makeScenario(t, activitiesModels.WeekdayMonday, editWindowStart)
			replaceScenarioTarget(t, s, targetType)

			result, err := s.svc.MaterializeForTenant(
				s.ctx, editWindowStart, editWindowStart.AddDays(6), scheduleSvc.MaterializationSourceManual,
			)
			require.NoError(t, err)
			assert.Equal(t, 3, result.InstanceStudentsCreated,
				"manual and dynamic memberships form one deduplicated roster")
			assert.Empty(t, detect(t, s),
				"automatically targeted students are not single-occurrence edits")
		})
	}
}

func replaceScenarioTarget(t *testing.T, s *scenarioSetup, targetType string) {
	t.Helper()
	target := &activitiesModels.GroupTarget{TargetGroupType: targetType}
	switch targetType {
	case activitiesModels.TargetGroupTypeKlasse:
		schoolClass := "3a"
		target.TargetSchoolClass = &schoolClass
	case activitiesModels.TargetGroupTypeJahrgang:
		grade := int16(3)
		target.TargetGradeLevel = &grade
	case activitiesModels.TargetGroupTypeGruppe:
		group := testpkg.CreateTestEducationGroupForTenant(t, s.db, s.tenantID, "Zielgruppe-2526")
		target.EducationGroupID = &group.ID
		_, err := s.db.NewUpdate().
			Table("users.students").
			Set("group_id = ?", group.ID).
			Where("tenant_id = ?", s.tenantID).
			Where("id = ?", s.students[2]).
			Exec(s.ctx)
		require.NoError(t, err)
	}
	targetRepo, ok := repositories.NewFactory(s.db).ActivityGroup.(activitiesModels.GroupTargetRepository)
	require.True(t, ok)
	require.NoError(t, targetRepo.ReplaceTargets(s.ctx, s.template.ID, []*activitiesModels.GroupTarget{target}))
}

func TestDetectEditedInWindow_TitleEdit(t *testing.T) {
	t.Parallel()

	s := makeScenario(t, activitiesModels.WeekdayMonday, editWindowStart)
	defer s.runCleanup(t)
	inst := materializeSingleInstance(t, s)

	setInstanceColumn(t, s, inst.ID, "title", "Sonder-Titel")

	edited := detect(t, s)
	require.Len(t, edited, 1)
	assert.Equal(t, inst.ID, edited[0].InstanceID)
	assert.Equal(t, editWindowStart, edited[0].Date)
	assert.Equal(t, []string{scheduleSvc.EditedChangeTitle}, edited[0].Changes)
	assert.Equal(t, "Sonder-Titel", edited[0].Title)
}

func TestDetectEditedInWindow_NotesEdit(t *testing.T) {
	t.Parallel()

	s := makeScenario(t, activitiesModels.WeekdayMonday, editWindowStart)
	defer s.runCleanup(t)
	inst := materializeSingleInstance(t, s)

	setInstanceColumn(t, s, inst.ID, "notes", "Heute im Turnraum")

	edited := detect(t, s)
	require.Len(t, edited, 1)
	assert.Equal(t, []string{scheduleSvc.EditedChangeNotes}, edited[0].Changes)
}

func TestDetectEditedInWindow_DescriptionEdit(t *testing.T) {
	t.Parallel()

	s := makeScenario(t, activitiesModels.WeekdayMonday, editWindowStart)
	defer s.runCleanup(t)
	inst := materializeSingleInstance(t, s)

	// description is settable via the instance PUT but never written by
	// materialization, so a per-occurrence description must be flagged as lost.
	setInstanceColumn(t, s, inst.ID, "description", "Bitte Sportkleidung mitbringen")

	edited := detect(t, s)
	require.Len(t, edited, 1)
	assert.Equal(t, []string{scheduleSvc.EditedChangeDescription}, edited[0].Changes)
}

func TestDetectEditedInWindow_RoomEdit(t *testing.T) {
	t.Parallel()

	s := makeScenario(t, activitiesModels.WeekdayMonday, editWindowStart)
	defer s.runCleanup(t)
	inst := materializeSingleInstance(t, s)

	room2 := testpkg.CreateTestRoomForTenant(t, s.db, s.tenantID, "Turnraum-1875")
	s.extraCleanups = append(s.extraCleanups, func() {
	})
	setInstanceColumn(t, s, inst.ID, "room_id", room2.ID)

	edited := detect(t, s)
	require.Len(t, edited, 1)
	assert.Equal(t, []string{scheduleSvc.EditedChangeRoom}, edited[0].Changes)
}

func TestDetectEditedInWindow_TimeMove(t *testing.T) {
	t.Parallel()

	s := makeScenario(t, activitiesModels.WeekdayMonday, editWindowStart)
	defer s.runCleanup(t)
	inst := materializeSingleInstance(t, s)

	// Shift the start off the template slot (08:00) → no matching slot → moved.
	setInstanceColumn(t, s, inst.ID, "start_time", time.Date(2000, 1, 1, 9, 0, 0, 0, time.UTC))

	edited := detect(t, s)
	require.Len(t, edited, 1)
	assert.Equal(t, []string{scheduleSvc.EditedChangeTime}, edited[0].Changes)
}

func TestDetectEditedInWindow_StudentRosterEdit(t *testing.T) {
	t.Parallel()

	s := makeScenario(t, activitiesModels.WeekdayMonday, editWindowStart)
	defer s.runCleanup(t)
	inst := materializeSingleInstance(t, s)

	// Add the expired-enrollment student (s.students[2]) directly onto the
	// occurrence — the roster now diverges from the template's valid enrollments.
	extra := &scheduleModels.InstanceStudent{
		InstanceID: inst.ID,
		StudentID:  s.students[2],
		Status:     scheduleModels.AttendanceStatusExpected,
	}
	extra.SetTenantID(s.tenantID)
	_, err := s.db.NewInsert().Model(extra).ModelTableExpr(`schedule.instance_students`).Exec(s.ctx)
	require.NoError(t, err)

	edited := detect(t, s)
	require.Len(t, edited, 1)
	assert.Equal(t, []string{scheduleSvc.EditedChangeStudents}, edited[0].Changes)
}

func TestDetectEditedInWindow_StudentRosterRemoval(t *testing.T) {
	t.Parallel()

	s := makeScenario(t, activitiesModels.WeekdayMonday, editWindowStart)
	defer s.runCleanup(t)
	inst := materializeSingleInstance(t, s)

	_, err := s.db.NewDelete().
		Model((*scheduleModels.InstanceStudent)(nil)).
		ModelTableExpr(`schedule.instance_students AS "instance_student"`).
		Where(`"instance_student".instance_id = ?`, inst.ID).
		Where(`"instance_student".student_id = ?`, s.students[0]).
		Where("tenant_id = ?", s.tenantID).
		Exec(s.ctx)
	require.NoError(t, err)

	edited := detect(t, s)
	require.Len(t, edited, 1)
	assert.Equal(t, []string{scheduleSvc.EditedChangeStudents}, edited[0].Changes)
}

func TestDetectEditedInWindow_StatusDayAbsenceIsRosterMembership(t *testing.T) {
	t.Parallel()

	statuses := []struct {
		name      string
		status    string
		substatus string
	}{
		{name: "sick", status: activeModels.StudentStatusDaySick, substatus: scheduleModels.AttendanceSubstatusSick},
		{name: "excused", status: activeModels.StudentStatusDayExcused, substatus: scheduleModels.AttendanceSubstatusExcused},
		{name: "class trip", status: activeModels.StudentStatusDayClassTrip, substatus: scheduleModels.AttendanceSubstatusFieldTrip},
	}

	for _, tc := range statuses {
		t.Run(tc.name, func(t *testing.T) {
			s := makeScenario(t, activitiesModels.WeekdayMonday, editWindowStart)
			defer s.runCleanup(t)
			inst := materializeSingleInstance(t, s)

			statusDay := &activeModels.StudentStatusDay{
				StudentID:  s.students[0],
				Date:       editWindowStart,
				Status:     tc.status,
				ReportedAt: time.Now(),
				Source:     activeModels.StudentStatusSourcePlanned,
			}
			require.NoError(t, s.factory.StudentStatusDays.UpsertReported(s.ctx, statusDay))
			s.extraCleanups = append(s.extraCleanups, func() {
			})

			studentRepo := testInstanceStudents(s.db)
			before, err := studentRepo.FindByInstanceAndStudent(s.ctx, inst.ID, s.students[0])
			require.NoError(t, err)
			assert.Equal(t, scheduleModels.AttendanceStatusAbsent, before.Status)
			require.NotNil(t, before.Substatus)
			assert.Equal(t, tc.substatus, *before.Substatus)
			require.NotNil(t, before.StudentStatusDayID)
			assert.Equal(t, statusDay.ID, *before.StudentStatusDayID)

			assert.Empty(t, detect(t, s), "a status-owned absence does not change roster membership")

			_, err = s.factory.Instance.ReplanWeek(s.ctx, editWindowStart, editWindowStart, &s.template.ID, nil)
			require.NoError(t, err)
			regenerated := listInstancesForDate(t, s.db, s.template.ID, editWindowStart)
			require.Len(t, regenerated, 1)

			after, err := studentRepo.FindByInstanceAndStudent(s.ctx, regenerated[0].ID, s.students[0])
			require.NoError(t, err)
			assert.Equal(t, scheduleModels.AttendanceStatusAbsent, after.Status)
			require.NotNil(t, after.Substatus)
			assert.Equal(t, tc.substatus, *after.Substatus)
			require.NotNil(t, after.StudentStatusDayID)
			assert.Equal(t, statusDay.ID, *after.StudentStatusDayID)
		})
	}
}

func TestDetectEditedInWindow_AttendanceStateDoesNotChangeRosterMembership(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		apply func(*scheduleModels.InstanceStudent)
	}{
		{name: "observed present", apply: func(row *scheduleModels.InstanceStudent) {
			now := time.Now()
			row.Status = scheduleModels.AttendanceStatusPresent
			row.CheckedInAt = &now
		}},
		{name: "manual absent", apply: func(row *scheduleModels.InstanceStudent) {
			now := time.Now()
			row.Status = scheduleModels.AttendanceStatusAbsent
			row.ManualStatusAt = &now
		}},
		{name: "not scheduled", apply: func(row *scheduleModels.InstanceStudent) {
			row.Status = scheduleModels.AttendanceStatusExpected
			row.NotScheduled = true
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := makeScenario(t, activitiesModels.WeekdayMonday, editWindowStart)
			defer s.runCleanup(t)
			inst := materializeSingleInstance(t, s)
			studentRepo := testInstanceStudents(s.db)
			row, err := studentRepo.FindByInstanceAndStudent(s.ctx, inst.ID, s.students[0])
			require.NoError(t, err)
			tc.apply(row)
			require.NoError(t, studentRepo.Update(s.ctx, row))

			edited := detect(t, s)
			require.Len(t, edited, 1)
			assert.Equal(t, []string{scheduleSvc.EditedChangeAttendance}, edited[0].Changes,
				"unrestored attendance is a lost edit, but not a roster-membership change")
			assert.NotContains(t, edited[0].Changes, scheduleSvc.EditedChangeStudents)

			_, err = s.factory.Instance.ReplanWeek(s.ctx, editWindowStart, editWindowStart, &s.template.ID, nil)
			require.NoError(t, err)
			regenerated := listInstancesForDate(t, s.db, s.template.ID, editWindowStart)
			require.Len(t, regenerated, 1)

			after, err := studentRepo.FindByInstanceAndStudent(s.ctx, regenerated[0].ID, s.students[0])
			require.NoError(t, err)
			assert.Equal(t, scheduleModels.AttendanceStatusExpected, after.Status)
			assert.False(t, after.NotScheduled)
			assert.Nil(t, after.ManualStatusAt)
			assert.Nil(t, after.CheckedInAt)
		})
	}
}

func TestDetectEditedInWindow_StaffRosterEdit(t *testing.T) {
	t.Parallel()

	s := makeScenario(t, activitiesModels.WeekdayMonday, editWindowStart)
	defer s.runCleanup(t)
	inst := materializeSingleInstance(t, s)

	// Remove the only planned supervisor row → the planned roster diverges.
	_, err := s.db.NewDelete().
		Model((*scheduleModels.InstanceStaff)(nil)).
		ModelTableExpr(`schedule.instance_staff AS "instance_staff"`).
		Where(`"instance_staff".instance_id = ?`, inst.ID).
		Where("tenant_id = ?", s.tenantID).
		Exec(s.ctx)
	require.NoError(t, err)

	edited := detect(t, s)
	require.Len(t, edited, 1)
	assert.Equal(t, []string{scheduleSvc.EditedChangeStaff}, edited[0].Changes)
}

func TestDetectEditedInWindow_SubstituteDeviationNotFlagged(t *testing.T) {
	t.Parallel()

	s := makeScenario(t, activitiesModels.WeekdayMonday, editWindowStart)
	defer s.runCleanup(t)
	inst := materializeSingleInstance(t, s)

	// Mark the planned supervisor absent (row kept) and add a substitute (a
	// fresh staff, since instance_staff is unique per staff) — the exact
	// Vertretungsplan shape ReplanWeek preserves. Must NOT be flagged.
	setInstanceStaffAbsent(t, s, inst.ID, s.staffID)
	subStaff := testpkg.CreateTestStaffForTenant(t, s.db, s.tenantID, "Ersatz", "Kraft-1875")
	s.extraCleanups = append(s.extraCleanups, func() {
	})
	sub := &scheduleModels.InstanceStaff{
		InstanceID:   inst.ID,
		StaffID:      subStaff.ID,
		IsSubstitute: true,
	}
	sub.SetTenantID(s.tenantID)
	_, err := s.db.NewInsert().Model(sub).ModelTableExpr(`schedule.instance_staff`).Exec(s.ctx)
	require.NoError(t, err)

	assert.Empty(t, detect(t, s), "absence + substitute is a preserved deviation, not a lost edit")
}

// TestDetectEditedInWindow_DeletedOccurrence covers the #1907 review critical:
// an individually-deleted future occurrence (a cancelled exception) is
// resurrected by a following-series split, so it must be reported — but only
// when the caller asks for deletions (a same-template re-plan preserves it).
func TestDetectEditedInWindow_DeletedOccurrence(t *testing.T) {
	t.Parallel()

	s := makeScenario(t, activitiesModels.WeekdayMonday, editWindowStart)
	defer s.runCleanup(t)
	inst := materializeSingleInstance(t, s)

	// Delete the occurrence: a cancelled exception is written and the row removed
	// (mirrors DeleteCancelled). Materialization then skips the date.
	reason := "Einzeltermin gelöscht"
	exc := &scheduleModels.ActivityException{
		ActivityGroupID: s.template.ID,
		ExceptionDate:   editWindowStart,
		ExceptionType:   scheduleModels.ActivityExceptionCancelled,
		Reason:          &reason,
	}
	exc.SetTenantID(s.tenantID)
	_, err := s.db.NewInsert().Model(exc).ModelTableExpr(`schedule.activity_exceptions`).Exec(s.ctx)
	require.NoError(t, err)
	_, err = s.db.NewDelete().
		Model((*scheduleModels.ActivityInstance)(nil)).
		ModelTableExpr(`schedule.activity_instances AS "activity_instance"`).
		Where(`"activity_instance".id = ?`, inst.ID).
		Where("tenant_id = ?", s.tenantID).
		Exec(s.ctx)
	require.NoError(t, err)

	// Same-template re-plan ("Alle Termine"): deletion preserved → not reported.
	assert.Empty(t, detectWindow(t, s, false),
		"deletions are preserved by a same-template re-plan")

	// Following-series split: successor resurrects it → reported as deleted.
	edited := detectWindow(t, s, true)
	require.Len(t, edited, 1)
	assert.Equal(t, editWindowStart, edited[0].Date)
	assert.Equal(t, []string{scheduleSvc.EditedChangeDeleted}, edited[0].Changes)
}

func TestDetectEditedInWindow_TenantIsolation(t *testing.T) {
	t.Parallel()

	s := makeScenario(t, activitiesModels.WeekdayMonday, editWindowStart)
	defer s.runCleanup(t)
	inst := materializeSingleInstance(t, s)
	setInstanceColumn(t, s, inst.ID, "title", "Sonder-Titel")

	// Another tenant must see nothing for this template.
	otherTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, s.db, otherTenant)
	otherCtx := testpkg.TenantContext(otherTenant)
	edited, err := s.svc.DetectEditedInWindow(otherCtx, s.template.ID, editWindowStart, editWindowStart.AddDays(6), false)
	require.NoError(t, err)
	assert.Empty(t, edited, "detection is tenant-scoped")
}

// TestDetectEditedInWindow_ExceptionShiftedStartNotFlagged covers the #1907
// review false-positive: a template-level modified exception moves the start
// time, materialization creates the occurrence at the shifted start, and the
// detector must NOT report it as a lost `time` edit (the re-plan re-applies the
// same exception). The old (weekday, start) slot map flagged it.
func TestDetectEditedInWindow_ExceptionShiftedStartNotFlagged(t *testing.T) {
	t.Parallel()

	s := makeScenario(t, activitiesModels.WeekdayMonday, editWindowStart)
	defer s.runCleanup(t)

	// Modified exception: shift the start to 13:00 for the target date.
	newStart := time.Date(1, 1, 1, 13, 0, 0, 0, time.UTC)
	exc := &scheduleModels.ActivityException{
		ActivityGroupID: s.template.ID,
		ExceptionDate:   editWindowStart,
		ExceptionType:   scheduleModels.ActivityExceptionModified,
		StartTime:       &newStart,
	}
	exc.SetTenantID(s.tenantID)
	_, err := s.db.NewInsert().Model(exc).ModelTableExpr(`schedule.activity_exceptions`).Exec(s.ctx)
	require.NoError(t, err)

	// Materialize so the occurrence lands at the exception-shifted 13:00 start.
	materializeSingleInstance(t, s)

	assert.Empty(t, detect(t, s),
		"an occurrence at the exception-shifted start is not a lost edit")
}

// TestDetectEditedInWindow_OffRecurrenceDateFlagged covers the #1907 review
// false-negative: an occurrence sits on a date the current schedule would no
// longer materialize (here: after its valid_until). The old date-agnostic slot
// map matched it on (weekday, start) and missed the loss; the date-aware
// projection returns no slot, so it is correctly flagged as a lost `time` edit.
func TestDetectEditedInWindow_OffRecurrenceDateFlagged(t *testing.T) {
	t.Parallel()

	s := makeScenario(t, activitiesModels.WeekdayMonday, editWindowStart)
	defer s.runCleanup(t)
	inst := materializeSingleInstance(t, s)

	// Narrow the schedule so it has ended by the occurrence's date: valid_until
	// is exclusive, so valid_until == date means the schedule no longer
	// materializes on that date.
	_, err := s.db.NewUpdate().
		Model((*activitiesModels.Schedule)(nil)).
		ModelTableExpr(`activities.schedules AS "schedule"`).
		Set("valid_until = ?", editWindowStart).
		Where(`"schedule".id = ?`, s.schedule.ID).
		Where("tenant_id = ?", s.tenantID).
		Exec(s.ctx)
	require.NoError(t, err)

	edited := detect(t, s)
	require.Len(t, edited, 1)
	assert.Equal(t, inst.ID, edited[0].InstanceID)
	assert.Equal(t, []string{scheduleSvc.EditedChangeTime}, edited[0].Changes)
}

// setInstanceStaffAbsent flips the planned staff row to absent, tenant-scoped.
func setInstanceStaffAbsent(t *testing.T, s *scenarioSetup, instanceID, staffID int64) {
	t.Helper()
	_, err := s.db.NewUpdate().
		Model((*scheduleModels.InstanceStaff)(nil)).
		ModelTableExpr(`schedule.instance_staff AS "instance_staff"`).
		Set("is_absent = ?", true).
		Where(`"instance_staff".instance_id = ?`, instanceID).
		Where(`"instance_staff".staff_id = ?`, staffID).
		Where("tenant_id = ?", s.tenantID).
		Exec(s.ctx)
	require.NoError(t, err)
}
