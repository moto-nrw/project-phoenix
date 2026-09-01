// Endpoint-level tests for the Änderungsprotokoll (#1886): every deviation
// write through POST /instances/{id}/deviations appends its
// audit.deviation_events row; idempotent no-ops append nothing.
package timetable

import (
	"context"
	"net/http"
	"testing"
	"time"

	activeRepo "github.com/moto-nrw/project-phoenix/database/repositories/active"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// loadEventsForInstance returns the protocol rows pointing at one instance,
// oldest first.
func loadEventsForInstance(t *testing.T, db *bun.DB, ctx context.Context, instanceID int64) []*auditModels.DeviationEvent {
	t.Helper()
	var rows []*auditModels.DeviationEvent
	err := db.NewSelect().
		Model(&rows).
		ModelTableExpr(`audit.deviation_events AS "deviation_event"`).
		Where(`"deviation_event".instance_id = ?`, instanceID).
		Order("id ASC").
		Scan(ctx)
	require.NoError(t, err)
	return rows
}

// TestApplyDeviations_WritesAbsenceAndSubstitutionEvents: one save with an
// absence (X) and a substitution (A→Y) writes exactly one event per touched
// row with the right types and staff references.
func TestApplyDeviations_WritesAbsenceAndSubstitutionEvents(t *testing.T) {
	t.Parallel()

	s := buildDevModule(t)
	router := devRouter(s.ctx, s.res)
	_, date := futureSubDate(1)

	inst := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{Title: "Protokoll"})

	testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.staffA, testpkg.InstanceStaffOpts{})
	testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.staffX, testpkg.InstanceStaffOpts{})

	w := doDev(t, router, inst.ID, map[string]any{
		"absences":      []map[string]any{{"staff_id": s.staffX, "reason": "krank"}},
		"substitutions": []map[string]any{{"absent_staff_id": s.staffA, "substitute_staff_id": s.staffY}},
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	events := loadEventsForInstance(t, s.db, s.ctx, inst.ID)
	require.Len(t, events, 2, "one absence + one substitution event")

	var absence, substitution *auditModels.DeviationEvent
	for _, ev := range events {
		switch ev.EventType {
		case auditModels.DeviationEventAbsence:
			absence = ev
		case auditModels.DeviationEventSubstitution:
			substitution = ev
		}
	}
	require.NotNil(t, absence, "absence event written")
	require.NotNil(t, substitution, "substitution event written")

	require.NotNil(t, absence.SubjectStaffID)
	assert.Equal(t, s.staffX, *absence.SubjectStaffID)
	require.NotNil(t, absence.Reason)
	assert.Equal(t, "krank", *absence.Reason)
	assert.Equal(t, date, absence.OccurrenceDate)

	require.NotNil(t, substitution.SubjectStaffID)
	assert.Equal(t, s.staffA, *substitution.SubjectStaffID)
	require.NotNil(t, substitution.RelatedStaffID)
	assert.Equal(t, s.staffY, *substitution.RelatedStaffID)
	assert.Contains(t, string(substitution.NewValue), "substitute_row_created")
}

// TestApplyDeviations_PresenceWritesReturnEvent: restoring a wrongly-marked
// absence writes a return_to_presence event carrying the old reason.
func TestApplyDeviations_PresenceWritesReturnEvent(t *testing.T) {
	t.Parallel()

	s := buildDevModule(t)
	router := devRouter(s.ctx, s.res)
	_, date := futureSubDate(1)

	inst := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{Title: "Rückkehr"})

	testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.staffA, testpkg.InstanceStaffOpts{IsAbsent: true})
	testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.staffB, testpkg.InstanceStaffOpts{})

	w := doDev(t, router, inst.ID, map[string]any{
		"presences": []int64{s.staffA},
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	events := loadEventsForInstance(t, s.db, s.ctx, inst.ID)
	require.Len(t, events, 1)
	assert.Equal(t, auditModels.DeviationEventReturnToPresence, events[0].EventType)
	require.NotNil(t, events[0].SubjectStaffID)
	assert.Equal(t, s.staffA, *events[0].SubjectStaffID)
}

// TestApplyDeviations_IdempotentReplayWritesNoEvent: re-marking an
// already-absent person is a no-op and must not pollute the Verlauf.
func TestApplyDeviations_IdempotentReplayWritesNoEvent(t *testing.T) {
	t.Parallel()

	s := buildDevModule(t)
	router := devRouter(s.ctx, s.res)
	_, date := futureSubDate(1)

	inst := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{Title: "Replay"})

	testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.staffA, testpkg.InstanceStaffOpts{IsAbsent: true})
	testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.staffB, testpkg.InstanceStaffOpts{})

	w := doDev(t, router, inst.ID, map[string]any{
		"absences": []map[string]any{{"staff_id": s.staffA}},
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	events := loadEventsForInstance(t, s.db, s.ctx, inst.ID)
	assert.Empty(t, events, "idempotent replay writes no protocol row")
}

// TestApplyDeviations_ActiveInstance_EndsAndCreatesSupervisor: ported from the
// removed POST /substitute endpoint tests (#1886) — the live-session supervisor
// swap must keep working through the consolidated deviations path.
func TestApplyDeviations_ActiveInstance_EndsAndCreatesSupervisor(t *testing.T) {
	t.Parallel()

	s := buildDevModule(t)
	router := devRouter(s.ctx, s.res)
	_, date := futureSubDate(1)

	activeGroupRepo := activeRepo.NewGroupRepository(s.db)
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	ag := &activeModel.Group{
		StartTime:      now,
		LastActivity:   now,
		TimeoutMinutes: 30,
		RoomID:         s.roomID,
	}
	require.NoError(t, activeGroupRepo.Create(s.ctx, ag))

	inst := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "14:00", EndHHMM: "15:00", Title: "Active-Inst",
		Status:        scheduleModel.InstanceStatusActive,
		ActiveGroupID: &ag.ID,
	})

	testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.staffA, testpkg.InstanceStaffOpts{})

	supervisorRepo := activeRepo.NewGroupSupervisorRepository(s.db)
	absentSup := &activeModel.GroupSupervisor{
		StaffID:   s.staffA,
		GroupID:   ag.ID,
		Role:      "supervisor",
		StartDate: timezone.DateFromTime(now),
	}
	require.NoError(t, supervisorRepo.Create(s.ctx, absentSup))

	w := doDev(t, router, inst.ID, map[string]any{
		"substitutions": []map[string]any{{"absent_staff_id": s.staffA, "substitute_staff_id": s.staffY}},
	})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	absentSupAfter, err := supervisorRepo.FindByID(s.ctx, absentSup.ID)
	require.NoError(t, err)
	assert.NotNil(t, absentSupAfter.EndDate, "absent supervisor must be ended")

	subSups, err := supervisorRepo.FindActiveByStaffID(s.ctx, s.staffY)
	require.NoError(t, err)
	found := false
	for _, sup := range subSups {
		if sup.GroupID == ag.ID {
			found = true
		}
	}
	assert.True(t, found, "substitute must have a new active supervisor row on this group")
	t.Cleanup(func() {
	})

	rows := devInstanceStaff(t, s.db, s.ctx, inst.ID)
	var newSubRowIDs []int64
	for _, r := range rows {
		if r.IsSubstitute && r.StaffID == s.staffY {
			newSubRowIDs = append(newSubRowIDs, r.ID)
		}
	}
	require.NotEmpty(t, newSubRowIDs, "substitute instance_staff row created")
}
