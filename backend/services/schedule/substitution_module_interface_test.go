package schedule_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/services"
	substitution "github.com/moto-nrw/project-phoenix/services/education"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestScheduleSubstitutionExternalInterfacePartialCoverage(t *testing.T) {
	t.Parallel()
	db, repos, service := newScheduleSubstitutionModule(t)
	var err error

	room := testpkg.CreateTestRoom(t, db, "Teilvertretung")
	absent := testpkg.CreateTestStaff(t, db, "Anna", "Abwesend")
	replacement := testpkg.CreateTestStaff(t, db, "Bela", "Vertretung")
	date := timezone.TodayDate().AddDays(1)
	first := testpkg.CreateTestActivityInstance(t, db, date, room.ID, testpkg.ActivityInstanceOpts{Title: "Erster Termin"})
	second := testpkg.CreateTestActivityInstance(t, db, date, room.ID, testpkg.ActivityInstanceOpts{Title: "Zweiter Termin", StartHHMM: "16:00", EndHHMM: "17:00"})
	testpkg.CreateTestInstanceStaff(t, db, first.ID, absent.ID, testpkg.InstanceStaffOpts{})
	testpkg.CreateTestInstanceStaff(t, db, second.ID, absent.ID, testpkg.InstanceStaffOpts{})

	selected := []int64{first.ID}
	result, err := service.Assign(testpkg.Ctx(t), scheduleSubstitutionCaller(t), substitution.Assignment{
		Type: substitution.TargetScheduleSubstitution,
		ScheduleSubstitution: &substitution.ScheduleSubstitutionAssignment{
			InstanceID: first.ID,
			Substitutions: []substitution.ScheduleSubstitutionChange{{
				AbsentStaffID: absent.ID, SubstituteStaffID: replacement.ID, InstanceIDs: &selected,
			}},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, result.ScheduleSubstitution)
	require.Equal(t, 1, result.ScheduleSubstitution.TotalAffected)

	firstRows, err := repos.InstanceStaff.FindByInstanceID(testpkg.Ctx(t), first.ID)
	require.NoError(t, err)
	requireScheduleStaffState(t, firstRows, absent.ID, true, false)
	requireScheduleStaffState(t, firstRows, replacement.ID, false, true)

	secondRows, err := repos.InstanceStaff.FindByInstanceID(testpkg.Ctx(t), second.ID)
	require.NoError(t, err)
	requireScheduleStaffState(t, secondRows, absent.ID, false, false)
	for _, row := range secondRows {
		require.NotEqual(t, replacement.ID, row.StaffID)
	}
}

func TestScheduleSubstitutionExternalInterfaceDayWideAbsence(t *testing.T) {
	t.Parallel()
	db, repos, service := newScheduleSubstitutionModule(t)
	var err error

	room := testpkg.CreateTestRoom(t, db, "Tagesweite Abwesenheit")
	absent := testpkg.CreateTestStaff(t, db, "Tara", "Ganztägig")
	date := timezone.TodayDate().AddDays(1)
	first := testpkg.CreateTestActivityInstance(t, db, date, room.ID, testpkg.ActivityInstanceOpts{Title: "Früher Termin"})
	second := testpkg.CreateTestActivityInstance(t, db, date, room.ID, testpkg.ActivityInstanceOpts{Title: "Später Termin", StartHHMM: "16:00", EndHHMM: "17:00"})
	testpkg.CreateTestInstanceStaff(t, db, first.ID, absent.ID, testpkg.InstanceStaffOpts{})
	testpkg.CreateTestInstanceStaff(t, db, second.ID, absent.ID, testpkg.InstanceStaffOpts{})

	result, err := service.Assign(testpkg.Ctx(t), scheduleSubstitutionCaller(t), substitution.Assignment{
		Type: substitution.TargetScheduleSubstitution,
		ScheduleSubstitution: &substitution.ScheduleSubstitutionAssignment{
			InstanceID: first.ID,
			Absences:   []substitution.ScheduleAbsenceChange{{StaffID: absent.ID}},
		},
	})
	require.NoError(t, err)
	require.Equal(t, 2, result.ScheduleSubstitution.TotalAffected)

	for _, instanceID := range []int64{first.ID, second.ID} {
		rows, findErr := repos.InstanceStaff.FindByInstanceID(testpkg.Ctx(t), instanceID)
		require.NoError(t, findErr)
		requireScheduleStaffState(t, rows, absent.ID, true, false)
	}
}

func TestScheduleSubstitutionExternalInterfaceActiveAppointmentSyncsOnce(t *testing.T) {
	t.Parallel()
	db, repos, service := newScheduleSubstitutionModule(t)
	var err error

	room := testpkg.CreateTestRoom(t, db, "Aktiver Termin")
	activity := testpkg.CreateTestActivityGroup(t, db, "Aktive Betreuung")
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	absent := testpkg.CreateTestStaff(t, db, "Alex", "Aktiv")
	replacement := testpkg.CreateTestStaff(t, db, "Rita", "Ersatz")
	instance := testpkg.CreateTestActivityInstance(t, db, timezone.TodayDate(), room.ID, testpkg.ActivityInstanceOpts{
		Title: "Laufender Termin", Status: scheduleModels.InstanceStatusActive, ActiveGroupID: &activeGroup.ID,
	})
	testpkg.CreateTestInstanceStaff(t, db, instance.ID, absent.ID, testpkg.InstanceStaffOpts{})
	testpkg.CreateTestGroupSupervisor(t, db, absent.ID, activeGroup.ID, "supervisor")
	selected := []int64{instance.ID}

	_, err = service.Assign(testpkg.Ctx(t), scheduleSubstitutionCaller(t), substitution.Assignment{
		Type: substitution.TargetScheduleSubstitution,
		ScheduleSubstitution: &substitution.ScheduleSubstitutionAssignment{
			InstanceID: instance.ID,
			Substitutions: []substitution.ScheduleSubstitutionChange{{
				AbsentStaffID: absent.ID, SubstituteStaffID: replacement.ID, InstanceIDs: &selected,
			}},
		},
	})
	require.NoError(t, err)

	activeSupervisors, err := repos.GroupSupervisor.FindByActiveGroupID(testpkg.Ctx(t), activeGroup.ID, true)
	require.NoError(t, err)
	require.Len(t, activeSupervisors, 1)
	require.Equal(t, replacement.ID, activeSupervisors[0].StaffID)
}

func TestScheduleSubstitutionExternalInterfaceConflictRollsBack(t *testing.T) {
	t.Parallel()
	db, repos, service := newScheduleSubstitutionModule(t)
	var err error

	room := testpkg.CreateTestRoom(t, db, "Konflikt")
	firstAbsent := testpkg.CreateTestStaff(t, db, "Erste", "Abwesend")
	secondAbsent := testpkg.CreateTestStaff(t, db, "Zweite", "Abwesend")
	existingReplacement := testpkg.CreateTestStaff(t, db, "Schon", "Eingetragen")
	newReplacement := testpkg.CreateTestStaff(t, db, "Neu", "Gewählt")
	date := timezone.TodayDate().AddDays(1)
	first := testpkg.CreateTestActivityInstance(t, db, date, room.ID, testpkg.ActivityInstanceOpts{Title: "Schreibbarer Termin"})
	conflicting := testpkg.CreateTestActivityInstance(t, db, date, room.ID, testpkg.ActivityInstanceOpts{Title: "Bereits vertreten", StartHHMM: "16:00", EndHHMM: "17:00"})
	testpkg.CreateTestInstanceStaff(t, db, first.ID, firstAbsent.ID, testpkg.InstanceStaffOpts{})
	testpkg.CreateTestInstanceStaff(t, db, conflicting.ID, secondAbsent.ID, testpkg.InstanceStaffOpts{IsAbsent: true})
	testpkg.CreateTestInstanceStaff(t, db, conflicting.ID, existingReplacement.ID, testpkg.InstanceStaffOpts{IsSubstitute: true})
	firstOnly := []int64{first.ID}
	conflictingOnly := []int64{conflicting.ID}

	_, err = service.Assign(testpkg.Ctx(t), scheduleSubstitutionCaller(t), substitution.Assignment{
		Type: substitution.TargetScheduleSubstitution,
		ScheduleSubstitution: &substitution.ScheduleSubstitutionAssignment{
			InstanceID: first.ID,
			Substitutions: []substitution.ScheduleSubstitutionChange{
				{AbsentStaffID: firstAbsent.ID, SubstituteStaffID: newReplacement.ID, InstanceIDs: &firstOnly},
				{AbsentStaffID: secondAbsent.ID, SubstituteStaffID: newReplacement.ID, InstanceIDs: &conflictingOnly},
			},
		},
	})
	require.ErrorIs(t, err, substitution.ErrConflict)

	firstRows, err := repos.InstanceStaff.FindByInstanceID(testpkg.Ctx(t), first.ID)
	require.NoError(t, err)
	requireScheduleStaffState(t, firstRows, firstAbsent.ID, false, false)
	for _, row := range firstRows {
		require.NotEqual(t, newReplacement.ID, row.StaffID)
	}
}

func TestScheduleSubstitutionExternalInterfaceWriteFailureRollsBack(t *testing.T) {
	t.Parallel()
	db, repos, service := newScheduleSubstitutionModule(t)
	var err error

	room := testpkg.CreateTestRoom(t, db, "Rollback")
	absent := testpkg.CreateTestStaff(t, db, "Romy", "Rollback")
	replacement := testpkg.CreateTestStaff(t, db, "Fritz", "Fehler")
	date := timezone.TodayDate().AddDays(1)
	instance := testpkg.CreateTestActivityInstance(t, db, date, room.ID, testpkg.ActivityInstanceOpts{Title: "Rollback-Termin"})
	testpkg.CreateTestInstanceStaff(t, db, instance.ID, absent.ID, testpkg.InstanceStaffOpts{})
	selected := []int64{instance.ID}
	caller := scheduleSubstitutionCaller(t)
	_, err = db.NewDelete().TableExpr(`auth.accounts`).Where("id = ?", caller.AccountID).Exec(testpkg.Ctx(t))
	require.NoError(t, err)

	_, err = service.Assign(testpkg.Ctx(t), caller, substitution.Assignment{
		Type: substitution.TargetScheduleSubstitution,
		ScheduleSubstitution: &substitution.ScheduleSubstitutionAssignment{
			InstanceID: instance.ID,
			Substitutions: []substitution.ScheduleSubstitutionChange{{
				AbsentStaffID: absent.ID, SubstituteStaffID: replacement.ID, InstanceIDs: &selected,
			}},
		},
	})
	require.Error(t, err)

	rows, err := repos.InstanceStaff.FindByInstanceID(testpkg.Ctx(t), instance.ID)
	require.NoError(t, err)
	requireScheduleStaffState(t, rows, absent.ID, false, false)
	for _, row := range rows {
		require.NotEqual(t, replacement.ID, row.StaffID)
	}
}

func TestScheduleSubstitutionExternalInterfacePreservesStaffingRules(t *testing.T) {
	t.Parallel()
	db, _, service := newScheduleSubstitutionModule(t)
	var err error

	room := testpkg.CreateTestRoom(t, db, "Besetzungsregeln")
	absent := testpkg.CreateTestStaff(t, db, "Ulla", "Unterbesetzt")
	present := testpkg.CreateTestStaff(t, db, "Paul", "Anwesend")
	replacement := testpkg.CreateTestStaff(t, db, "Vera", "Vertretung")
	instance := testpkg.CreateTestActivityInstance(t, db, timezone.TodayDate().AddDays(1), room.ID, testpkg.ActivityInstanceOpts{Title: "Besetzungsregeln"})
	testpkg.CreateTestInstanceStaff(t, db, instance.ID, absent.ID, testpkg.InstanceStaffOpts{})
	testpkg.CreateTestInstanceStaff(t, db, instance.ID, present.ID, testpkg.InstanceStaffOpts{})
	selected := []int64{instance.ID}
	caller := scheduleSubstitutionCaller(t)
	acknowledged := true

	understaffed, err := service.Assign(testpkg.Ctx(t), caller, substitution.Assignment{
		Type: substitution.TargetScheduleSubstitution,
		ScheduleSubstitution: &substitution.ScheduleSubstitutionAssignment{
			InstanceID: instance.ID, UnderstaffedAck: &acknowledged,
			Absences: []substitution.ScheduleAbsenceChange{{StaffID: absent.ID, InstanceIDs: &selected}},
		},
	})
	require.NoError(t, err)
	require.True(t, understaffed.ScheduleSubstitution.UnderstaffedAck)

	covered, err := service.Assign(testpkg.Ctx(t), caller, substitution.Assignment{
		Type: substitution.TargetScheduleSubstitution,
		ScheduleSubstitution: &substitution.ScheduleSubstitutionAssignment{
			InstanceID: instance.ID,
			Substitutions: []substitution.ScheduleSubstitutionChange{{
				AbsentStaffID: absent.ID, SubstituteStaffID: replacement.ID, InstanceIDs: &selected,
			}},
		},
	})
	require.NoError(t, err)
	require.False(t, covered.ScheduleSubstitution.UnderstaffedAck)

	_, err = service.Assign(testpkg.Ctx(t), caller, substitution.Assignment{
		Type: substitution.TargetScheduleSubstitution,
		ScheduleSubstitution: &substitution.ScheduleSubstitutionAssignment{
			InstanceID: instance.ID,
			Presences:  []substitution.SchedulePresenceChange{{StaffID: absent.ID, InstanceIDs: &selected}},
		},
	})
	require.ErrorIs(t, err, substitution.ErrConflict)
	var operationError *substitution.OperationError
	require.ErrorAs(t, err, &operationError)
	require.Equal(t, "presence_would_overstaff", operationError.Code)
}

func TestScheduleSubstitutionExternalInterfaceReportsTimeConflicts(t *testing.T) {
	t.Parallel()
	db, _, service := newScheduleSubstitutionModule(t)
	var err error

	room := testpkg.CreateTestRoom(t, db, "Zeitkonflikt")
	absent := testpkg.CreateTestStaff(t, db, "Karla", "Abwesend")
	replacement := testpkg.CreateTestStaff(t, db, "Theo", "Doppelt")
	date := timezone.TodayDate().AddDays(1)
	target := testpkg.CreateTestActivityInstance(t, db, date, room.ID, testpkg.ActivityInstanceOpts{Title: "Zieltermin"})
	other := testpkg.CreateTestActivityInstance(t, db, date, room.ID, testpkg.ActivityInstanceOpts{Title: "Paralleler Termin", StartHHMM: "14:30", EndHHMM: "15:30"})
	testpkg.CreateTestInstanceStaff(t, db, target.ID, absent.ID, testpkg.InstanceStaffOpts{})
	testpkg.CreateTestInstanceStaff(t, db, other.ID, replacement.ID, testpkg.InstanceStaffOpts{})
	selected := []int64{target.ID}

	result, err := service.Assign(testpkg.Ctx(t), scheduleSubstitutionCaller(t), substitution.Assignment{
		Type: substitution.TargetScheduleSubstitution,
		ScheduleSubstitution: &substitution.ScheduleSubstitutionAssignment{
			InstanceID: target.ID,
			Substitutions: []substitution.ScheduleSubstitutionChange{{
				AbsentStaffID: absent.ID, SubstituteStaffID: replacement.ID, InstanceIDs: &selected,
			}},
		},
	})
	require.NoError(t, err)
	require.Len(t, result.ScheduleSubstitution.Warnings, 1)
	require.Equal(t, target.ID, result.ScheduleSubstitution.Warnings[0].InstanceID)
	require.Equal(t, other.ID, result.ScheduleSubstitution.Warnings[0].OtherID)
}

func TestScheduleSubstitutionExternalInterfacePermissionAndTenantIsolation(t *testing.T) {
	t.Parallel()
	db, _, service := newScheduleSubstitutionModule(t)
	var err error

	room := testpkg.CreateTestRoom(t, db, "Berechtigung")
	absent := testpkg.CreateTestStaff(t, db, "Pia", "Geplant")
	replacement := testpkg.CreateTestStaff(t, db, "Bert", "Ersatz")
	date := timezone.TodayDate().AddDays(1)
	instance := testpkg.CreateTestActivityInstance(t, db, date, room.ID, testpkg.ActivityInstanceOpts{Title: "Eigener Termin"})
	testpkg.CreateTestInstanceStaff(t, db, instance.ID, absent.ID, testpkg.InstanceStaffOpts{})
	selected := []int64{instance.ID}
	caller := scheduleSubstitutionCaller(t)
	caller.HasPermission = func(required string) bool { return required == "schedules:read" }

	_, err = service.Assign(testpkg.Ctx(t), caller, substitution.Assignment{
		Type: substitution.TargetScheduleSubstitution,
		ScheduleSubstitution: &substitution.ScheduleSubstitutionAssignment{
			InstanceID: instance.ID,
			Substitutions: []substitution.ScheduleSubstitutionChange{{
				AbsentStaffID: absent.ID, SubstituteStaffID: replacement.ID, InstanceIDs: &selected,
			}},
		},
	})
	require.ErrorIs(t, err, substitution.ErrForbidden)

	caller = scheduleSubstitutionCaller(t)
	caller.TenantID++
	_, err = service.Assign(testpkg.Ctx(t), caller, substitution.Assignment{
		Type: substitution.TargetScheduleSubstitution,
		ScheduleSubstitution: &substitution.ScheduleSubstitutionAssignment{
			InstanceID: instance.ID,
			Absences:   []substitution.ScheduleAbsenceChange{{StaffID: absent.ID, InstanceIDs: &selected}},
		},
	})
	require.ErrorIs(t, err, substitution.ErrForbidden)

	foreignTenant, _ := testpkg.CreateTestTenant(t, db)
	foreignRoom := testpkg.CreateTestRoomForTenant(t, db, foreignTenant, "Fremder Raum")
	foreignAbsent := testpkg.CreateTestStaffForTenant(t, db, foreignTenant, "Fremde", "Person")
	foreignInstance := &scheduleModels.ActivityInstance{
		Date: scheduleModels.Date(date), Title: "Fremder Termin", RoomID: foreignRoom.ID,
		StartTime: time.Date(2000, time.January, 1, 14, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2000, time.January, 1, 15, 0, 0, 0, time.UTC),
		Status:    scheduleModels.InstanceStatusPlanned,
	}
	foreignInstance.SetTenantID(foreignTenant)
	_, err = db.NewInsert().Model(foreignInstance).ModelTableExpr(`schedule.activity_instances`).Exec(context.Background())
	require.NoError(t, err)
	foreignRow := &scheduleModels.InstanceStaff{InstanceID: foreignInstance.ID, StaffID: foreignAbsent.ID}
	foreignRow.SetTenantID(foreignTenant)
	_, err = db.NewInsert().Model(foreignRow).ModelTableExpr(`schedule.instance_staff`).Exec(context.Background())
	require.NoError(t, err)

	caller = scheduleSubstitutionCaller(t)
	foreignSelected := []int64{foreignInstance.ID}
	_, err = service.Assign(testpkg.Ctx(t), caller, substitution.Assignment{
		Type: substitution.TargetScheduleSubstitution,
		ScheduleSubstitution: &substitution.ScheduleSubstitutionAssignment{
			InstanceID: foreignInstance.ID,
			Absences:   []substitution.ScheduleAbsenceChange{{StaffID: foreignAbsent.ID, InstanceIDs: &foreignSelected}},
		},
	})
	require.ErrorIs(t, err, substitution.ErrNotFound)
}

func TestScheduleSubstitutionExternalInterfaceUsesBulkPlanning(t *testing.T) {
	t.Parallel()
	db, repos, service := newScheduleSubstitutionModule(t)
	var err error

	room := testpkg.CreateTestRoom(t, db, "Sammelvertretung")
	absent := testpkg.CreateTestStaff(t, db, "Sam", "Mehrere Tage")
	replacement := testpkg.CreateTestStaff(t, db, "Mara", "Vertretung")
	firstDate := timezone.TodayDate().AddDays(1)
	secondDate := firstDate.AddDays(1)
	first := testpkg.CreateTestActivityInstance(t, db, firstDate, room.ID, testpkg.ActivityInstanceOpts{Title: "Erster Tag"})
	second := testpkg.CreateTestActivityInstance(t, db, secondDate, room.ID, testpkg.ActivityInstanceOpts{Title: "Zweiter Tag"})
	testpkg.CreateTestInstanceStaff(t, db, first.ID, absent.ID, testpkg.InstanceStaffOpts{})
	testpkg.CreateTestInstanceStaff(t, db, second.ID, absent.ID, testpkg.InstanceStaffOpts{})

	result, err := service.Assign(testpkg.Ctx(t), scheduleSubstitutionCaller(t), substitution.Assignment{
		Type: substitution.TargetScheduleSubstitution,
		ScheduleSubstitution: &substitution.ScheduleSubstitutionAssignment{
			WholeDays: &substitution.ScheduleWholeDayAssignment{
				AbsentStaffID: absent.ID, SubstituteStaffID: &replacement.ID,
				Dates: []timezone.Date{firstDate, secondDate},
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, result.ScheduleSubstitution.Days, 2)
	require.Equal(t, 2, result.ScheduleSubstitution.TotalAffected)
	for _, instanceID := range []int64{first.ID, second.ID} {
		rows, findErr := repos.InstanceStaff.FindByInstanceID(testpkg.Ctx(t), instanceID)
		require.NoError(t, findErr)
		requireScheduleStaffState(t, rows, absent.ID, true, false)
		requireScheduleStaffState(t, rows, replacement.ID, false, true)
	}
}

func TestScheduleSubstitutionExternalInterfaceEndsTypedAppointmentSubstitution(t *testing.T) {
	t.Parallel()
	db, repos, service := newScheduleSubstitutionModule(t)
	var err error

	room := testpkg.CreateTestRoom(t, db, "Vertretung beenden")
	absent := testpkg.CreateTestStaff(t, db, "Erna", "Abwesend")
	replacement := testpkg.CreateTestStaff(t, db, "Emil", "Ersatz")
	instance := testpkg.CreateTestActivityInstance(t, db, timezone.TodayDate().AddDays(1), room.ID, testpkg.ActivityInstanceOpts{Title: "Terminvertretung"})
	testpkg.CreateTestInstanceStaff(t, db, instance.ID, absent.ID, testpkg.InstanceStaffOpts{})
	selected := []int64{instance.ID}
	caller := scheduleSubstitutionCaller(t)

	_, err = service.Assign(testpkg.Ctx(t), caller, substitution.Assignment{
		Type: substitution.TargetScheduleSubstitution,
		ScheduleSubstitution: &substitution.ScheduleSubstitutionAssignment{
			InstanceID: instance.ID,
			Substitutions: []substitution.ScheduleSubstitutionChange{{
				AbsentStaffID: absent.ID, SubstituteStaffID: replacement.ID, InstanceIDs: &selected,
			}},
		},
	})
	require.NoError(t, err)
	rows, err := repos.InstanceStaff.FindByInstanceID(testpkg.Ctx(t), instance.ID)
	require.NoError(t, err)
	var substitutionID int64
	for _, row := range rows {
		if row.StaffID == replacement.ID && row.IsSubstitute {
			substitutionID = row.ID
		}
	}
	require.NotZero(t, substitutionID)

	require.NoError(t, service.End(testpkg.Ctx(t), caller, substitution.EndRequest{
		Type: substitution.TargetScheduleSubstitution, ID: substitutionID,
	}))
	rows, err = repos.InstanceStaff.FindByInstanceID(testpkg.Ctx(t), instance.ID)
	require.NoError(t, err)
	requireScheduleStaffState(t, rows, absent.ID, true, false)
	for _, row := range rows {
		require.NotEqual(t, replacement.ID, row.StaffID)
	}
}

func TestScheduleSubstitutionExternalInterfaceReturnsNarrowOverview(t *testing.T) {
	t.Parallel()
	db, _, service := newScheduleSubstitutionModule(t)
	var err error

	room := testpkg.CreateTestRoom(t, db, "Schmale Übersicht")
	absent := testpkg.CreateTestStaff(t, db, "Nora", "Abwesend")
	replacement := testpkg.CreateTestStaff(t, db, "Nils", "Vertretung")
	date := timezone.TodayDate().AddDays(1)
	instance := testpkg.CreateTestActivityInstance(t, db, date, room.ID, testpkg.ActivityInstanceOpts{Title: "Schmaler Termin"})
	testpkg.CreateTestInstanceStaff(t, db, instance.ID, absent.ID, testpkg.InstanceStaffOpts{})
	selected := []int64{instance.ID}
	caller := scheduleSubstitutionCaller(t)
	_, err = service.Assign(testpkg.Ctx(t), caller, substitution.Assignment{
		Type: substitution.TargetScheduleSubstitution,
		ScheduleSubstitution: &substitution.ScheduleSubstitutionAssignment{
			InstanceID: instance.ID,
			Substitutions: []substitution.ScheduleSubstitutionChange{{
				AbsentStaffID: absent.ID, SubstituteStaffID: replacement.ID, InstanceIDs: &selected,
			}},
		},
	})
	require.NoError(t, err)

	overview, err := service.Overview(testpkg.Ctx(t), caller, substitution.OverviewQuery{
		ScheduleFrom: &date, ScheduleTo: &date, IncludeScheduleTargets: true,
	})
	require.NoError(t, err)
	require.Len(t, overview.ScheduleAppointments, 1)
	require.Equal(t, instance.ID, overview.ScheduleAppointments[0].ID)
	require.Len(t, overview.ScheduleAppointments[0].Staff, 2)
	canEnd := false
	for _, row := range overview.ScheduleAppointments[0].Staff {
		if row.Staff.ID == replacement.ID {
			canEnd = row.CanEnd
		}
	}
	require.True(t, canEnd)
	require.Contains(t, overview.ScheduleTargets, substitution.StaffRef{ID: replacement.ID, FullName: "Nils Vertretung"})
	readOnly := scheduleSubstitutionCaller(t)
	readOnly.HasPermission = func(required string) bool { return required == "schedules:read" }
	readOnlyOverview, err := service.Overview(testpkg.Ctx(t), readOnly, substitution.OverviewQuery{
		ScheduleFrom: &date, ScheduleTo: &date,
	})
	require.NoError(t, err)
	for _, row := range readOnlyOverview.ScheduleAppointments[0].Staff {
		require.False(t, row.CanEnd)
	}
	payload, err := json.Marshal(overview)
	require.NoError(t, err)
	require.NotContains(t, string(payload), "staff_notes")
	require.NotContains(t, string(payload), "employment_type")
	require.NotContains(t, string(payload), "account_id")
	require.NotContains(t, string(payload), "person_id")
}

func requireScheduleStaffState(t *testing.T, rows []*scheduleModels.InstanceStaff, staffID int64, absent, substitute bool) {
	t.Helper()
	for _, row := range rows {
		if row.StaffID == staffID {
			require.Equal(t, absent, row.IsAbsent)
			require.Equal(t, substitute, row.IsSubstitute)
			return
		}
	}
	require.Failf(t, "staff row missing", "staff_id=%d", staffID)
}

func scheduleSubstitutionCaller(t *testing.T) substitution.Caller {
	t.Helper()
	account := testpkg.CreateTestAccount(t, testpkg.SetupTestDB(t), "schedule-substitution-admin")
	return substitution.Caller{
		AccountID: account.ID, TenantID: testpkg.Tenant(t), Roles: []string{"admin"},
		HasPermission: func(required string) bool {
			return required == "schedules:read" || required == "schedules:manage"
		},
		Admin: true,
	}
}

func newScheduleSubstitutionModule(t *testing.T) (*bun.DB, *repositories.Factory, substitution.Module) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	factory, err := services.NewFactoryForTests(repos, db, slog.Default())
	require.NoError(t, err)
	return db, repos, factory.Substitution
}
