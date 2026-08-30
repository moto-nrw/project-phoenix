// Flow I — Personalpool + atomarer Personal-Move (#1884).
//
// Drives the two new endpoints through the production router: the pool read
// categorizes staff against a block's window, the move relocates one person
// from Schulhof to Mensa in one save, the Änderungsprotokoll shows the single
// staff_moved entry, and a foreign tenant sees 404.
package e2e_timetable

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestFlowI_StaffPoolAndAtomicMove(t *testing.T) {
	t.Parallel()

	s := setupTimetableScenarioRoute(t)
	defer s.teardown()
	s.extraCleanup = append(s.extraCleanup, func() {
		_, _ = s.db.NewDelete().Model((*struct{})(nil)).
			ModelTableExpr("audit.deviation_events").
			Where("tenant_id = ?", s.primaryTenant).Exec(s.tenantCtx())
	})

	date := timezone.TodayDate().AddDays(7)
	room := testpkg.CreateTestRoom(t, s.db, "Flow-I-Raum")
	s.extraCleanup = append(s.extraCleanup, func() {
	})
	mover := testpkg.CreateTestStaff(t, s.db, "Ina", "Umzieherin")
	free := testpkg.CreateTestStaff(t, s.db, "Frido", "Verfuegbar")
	s.extraCleanup = append(s.extraCleanup, func() {
	})

	schulhof := testpkg.CreateTestActivityInstance(t, s.db, date, room.ID, testpkg.ActivityInstanceOpts{
		Title: "Schulhof", StartHHMM: "12:00", EndHHMM: "14:00",
	})
	mensa := testpkg.CreateTestActivityInstance(t, s.db, date, room.ID, testpkg.ActivityInstanceOpts{
		Title: "Mensa", StartHHMM: "12:30", EndHHMM: "13:30",
	})
	s.registerCleanup("schedule.activity_instances", schulhof.ID, mensa.ID)

	row := testpkg.CreateTestInstanceStaff(t, s.db, schulhof.ID, mover.ID, testpkg.InstanceStaffOpts{})
	s.registerCleanup("schedule.instance_staff", row.ID)
	testpkg.CreateTestStaffShift(t, s.db, mover.ID, date, testpkg.StaffShiftOpts{})
	testpkg.CreateTestStaffShift(t, s.db, free.ID, date, testpkg.StaffShiftOpts{})
	s.extraCleanup = append(s.extraCleanup, func() {
	})

	claims := s.primaryAdminClaims()

	// 1. Pool read: mover is planned elsewhere, free is on shift and free.
	rr := s.do(http.MethodGet, fmt.Sprintf("/instances/%d/staff-pool", mensa.ID), nil, claims)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var pool struct {
		DienstplanInUse bool `json:"dienstplan_in_use"`
		Entries         []struct {
			StaffID     int64  `json:"staff_id"`
			Category    string `json:"category"`
			Assignments []struct {
				InstanceID int64  `json:"instance_id"`
				Title      string `json:"title"`
			} `json:"assignments"`
		} `json:"entries"`
	}
	decodeResponse(t, rr, &pool)
	assert.True(t, pool.DienstplanInUse)
	categories := map[int64]string{}
	for _, e := range pool.Entries {
		categories[e.StaffID] = e.Category
		if e.StaffID == mover.ID {
			require.Len(t, e.Assignments, 1)
			assert.Equal(t, schulhof.ID, e.Assignments[0].InstanceID)
			assert.Equal(t, "Schulhof", e.Assignments[0].Title)
		}
	}
	assert.Equal(t, "assigned_elsewhere", categories[mover.ID])
	assert.Equal(t, "on_shift_free", categories[free.ID])

	// 2. Atomic move Schulhof → Mensa.
	moveBody := map[string]any{"staff_id": mover.ID, "source_instance_id": schulhof.ID}
	rr = s.do(http.MethodPost, fmt.Sprintf("/instances/%d/move-staff", mensa.ID), moveBody, claims)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var moved struct {
		TargetInstanceID int64  `json:"target_instance_id"`
		SourceInstanceID *int64 `json:"source_instance_id"`
		Action           string `json:"action"`
	}
	decodeResponse(t, rr, &moved)
	assert.Equal(t, "moved", moved.Action)
	assert.Equal(t, mensa.ID, moved.TargetInstanceID)

	var rows []*scheduleModels.InstanceStaff
	require.NoError(t, s.db.NewSelect().Model(&rows).
		ModelTableExpr(`schedule.instance_staff AS "instance_staff"`).
		Where(`"instance_staff".id = ?`, row.ID).Scan(s.tenantCtx()))
	require.Len(t, rows, 1)
	assert.Equal(t, mensa.ID, rows[0].InstanceID, "row relocated to the target block")

	// 3. Änderungsprotokoll: one connected staff_moved entry with names.
	rr = s.do(http.MethodGet, fmt.Sprintf("/deviations/history?date=%s", date.String()), nil, claims)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var history struct {
		Events []struct {
			EventType        string  `json:"event_type"`
			InstanceID       *int64  `json:"instance_id"`
			SubjectStaffName *string `json:"subject_staff_name"`
		} `json:"events"`
	}
	decodeResponse(t, rr, &history)
	require.Len(t, history.Events, 1, "exactly one protocol entry for the move")
	assert.Equal(t, "staff_moved", history.Events[0].EventType)
	require.NotNil(t, history.Events[0].InstanceID)
	assert.Equal(t, mensa.ID, *history.Events[0].InstanceID)
	require.NotNil(t, history.Events[0].SubjectStaffName)
	assert.Contains(t, *history.Events[0].SubjectStaffName, "Ina")
	// 4. Pool assign: the free person joins Mensa without a source block.
	rr = s.do(http.MethodPost, fmt.Sprintf("/instances/%d/move-staff", mensa.ID),
		map[string]any{"staff_id": free.ID}, claims)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	decodeResponse(t, rr, &moved)
	assert.Equal(t, "assigned", moved.Action)
	var mensaRows []*scheduleModels.InstanceStaff
	require.NoError(t, s.db.NewSelect().Model(&mensaRows).
		ModelTableExpr(`schedule.instance_staff AS "instance_staff"`).
		Where(`"instance_staff".instance_id = ?`, mensa.ID).Scan(s.tenantCtx()))
	assert.Len(t, mensaRows, 2)
	for _, r := range mensaRows {
		s.registerCleanup("schedule.instance_staff", r.ID)
	}

	// 5. Tenant isolation: a foreign tenant cannot see or move.
	foreign := adminClaimsForTenant(2, s.secondaryTenant)
	rr = s.do(http.MethodGet, fmt.Sprintf("/instances/%d/staff-pool", mensa.ID), nil, foreign)
	assert.Equal(t, http.StatusNotFound, rr.Code, rr.Body.String())
	rr = s.do(http.MethodPost, fmt.Sprintf("/instances/%d/move-staff", mensa.ID), moveBody, foreign)
	assert.Equal(t, http.StatusNotFound, rr.Code, rr.Body.String())
}
