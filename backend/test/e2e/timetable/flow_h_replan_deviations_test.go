// Flow H — re-plan-week must reapply Vertretungsplan deviations onto the
// regenerated occurrence instead of freezing the stale occurrence in place
// (#1840), and a persisted absence must be clearable (present again).
package e2e_timetable

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestFlowH_ReplanReappliesAbsenceAndAckAcrossTemplateEdit is the regression
// for the freeze bug: an admin marks a planned block understaffed (one person
// absent, deliberately unstaffed acknowledged), then edits the template (title
// AND start time) and re-plans. The occurrence must be regenerated with the new
// template values — exactly ONE instance, no stale duplicate — and the absence
// and acknowledgement must be reapplied onto it.
func TestFlowH_ReplanReappliesAbsenceAndAckAcrossTemplateEdit(t *testing.T) {
	s := newScenario(t)
	defer s.teardown()

	target := nextWeekday(timezone.TodayDate(), 3, 7) // Wednesday, >=7 days out
	dateS := target.String()
	s.createActivePeriod(fmt.Sprintf("E2E-Flow-H-%d", time.Now().UnixNano()), target)

	room := testpkg.CreateTestRoom(t, s.db, "FlowH-Room")
	s.extraCleanup = append(s.extraCleanup, func() {
		testpkg.CleanupTableRecords(t, s.db, "facilities.rooms", room.ID)
	})
	staffA := testpkg.CreateTestStaff(t, s.db, "FlowH", "Alpha")
	staffB := testpkg.CreateTestStaff(t, s.db, "FlowH", "Bravo")
	s.extraCleanup = append(s.extraCleanup, func() {
		testpkg.CleanupStaffFixtures(t, s.db, staffA.ID, staffB.ID)
	})

	tmpl := s.buildTemplate(templateSpec{
		name:      fmt.Sprintf("FlowH-%d", time.Now().UnixNano()),
		weekday:   3,
		startHHMM: "08:00",
		endHHMM:   "09:00",
		roomID:    room.ID,
		staffIDs:  []int64{staffA.ID, staffB.ID},
	})

	matReq := map[string]any{"from_date": dateS, "to_date": dateS}
	rr := s.do("POST", "/materialize", matReq, s.primaryAdminClaims())
	require.Equal(t, http.StatusOK, rr.Code, "materialize body=%s", rr.Body.String())

	inst := fetchOneInstance(t, s, tmpl.group.ID, target)
	s.registerCleanup("schedule.activity_instances", inst.ID)

	// Mark staffA absent and acknowledge the block as deliberately unstaffed —
	// present(1) < planned(2), so the ack is valid.
	devReq := map[string]any{
		"absences":          []map[string]any{{"staff_id": staffA.ID, "reason": "Krank"}},
		"understaffed_ack":  true,
		"understaffed_note": "Kein Ersatz verfügbar",
	}
	rr = s.do("POST", fmt.Sprintf("/instances/%d/deviations", inst.ID), devReq, s.primaryAdminClaims())
	require.Equal(t, http.StatusOK, rr.Code, "deviations body=%s", rr.Body.String())

	// Edit the template as the "all occurrences" flow would: new title + new time.
	_, err := s.db.NewUpdate().Model((*activitiesModel.Group)(nil)).
		ModelTableExpr(`activities.groups AS "group"`).
		Set(`name = ?`, "FlowH-Renamed").
		Where(`"group".id = ?`, tmpl.group.ID).
		Where(`"group".tenant_id = ?`, s.primaryTenant).
		Exec(s.tenantCtx())
	require.NoError(t, err, "rename template")
	newStart := parseHHMM(t, "09:30")
	newEnd := parseHHMM(t, "10:30")
	_, err = s.db.NewUpdate().Model((*scheduleModel.Timeframe)(nil)).
		ModelTableExpr(`schedule.timeframes`).
		Set(`start_time = ?`, newStart).
		Set(`end_time = ?`, newEnd).
		Where(`id = ?`, tmpl.timeframe.ID).
		Where(`tenant_id = ?`, s.primaryTenant).
		Exec(s.tenantCtx())
	require.NoError(t, err, "retime template")

	// Re-plan.
	rr = s.do("POST", "/instances/re-plan-week", matReq, s.primaryAdminClaims())
	require.Equal(t, http.StatusOK, rr.Code, "re-plan body=%s", rr.Body.String())

	// Exactly one instance for (template, date) — fetchOneInstance's single-row
	// scan fails if a stale duplicate survived beside the regenerated block.
	newInst := fetchOneInstance(t, s, tmpl.group.ID, target)
	s.registerCleanup("schedule.activity_instances", newInst.ID)
	assert.NotEqual(t, inst.ID, newInst.ID, "occurrence was regenerated (new id)")
	assert.Equal(t, "FlowH-Renamed", newInst.Title, "regenerated with the edited template title")
	assert.Equal(t, "09:30", newInst.StartTime.Format("15:04"), "regenerated at the edited start time")

	// The absence and acknowledgement were reapplied onto the regenerated block.
	assert.True(t, newInst.UnderstaffedAck, "understaffed acknowledgement reapplied")
	rows := fetchInstanceStaff(t, s, newInst.ID)
	require.Len(t, rows, 2, "regenerated roster from the template")
	byStaff := map[int64]scheduleModel.InstanceStaff{}
	for _, r := range rows {
		byStaff[r.StaffID] = r
	}
	require.Contains(t, byStaff, staffA.ID)
	require.Contains(t, byStaff, staffB.ID)
	assert.True(t, byStaff[staffA.ID].IsAbsent, "staffA absence reapplied")
	assert.False(t, byStaff[staffB.ID].IsAbsent, "staffB stays present")
}

// TestFlowH_ReplanReappliesSubstitute proves a substitute assignment survives a
// re-plan: the absent's row is reapplied absent and the substitute row (a staff
// not on the template) is recreated on the regenerated occurrence.
func TestFlowH_ReplanReappliesSubstitute(t *testing.T) {
	s := newScenario(t)
	defer s.teardown()

	target := nextWeekday(timezone.TodayDate(), 4, 7) // Thursday
	dateS := target.String()
	s.createActivePeriod(fmt.Sprintf("E2E-Flow-H2-%d", time.Now().UnixNano()), target)

	room := testpkg.CreateTestRoom(t, s.db, "FlowH2-Room")
	s.extraCleanup = append(s.extraCleanup, func() {
		testpkg.CleanupTableRecords(t, s.db, "facilities.rooms", room.ID)
	})
	staffA := testpkg.CreateTestStaff(t, s.db, "FlowH2", "Alpha")
	staffB := testpkg.CreateTestStaff(t, s.db, "FlowH2", "Bravo")
	subC := testpkg.CreateTestStaff(t, s.db, "FlowH2", "Charlie") // not on the template
	s.extraCleanup = append(s.extraCleanup, func() {
		testpkg.CleanupStaffFixtures(t, s.db, staffA.ID, staffB.ID, subC.ID)
	})

	tmpl := s.buildTemplate(templateSpec{
		name:      fmt.Sprintf("FlowH2-%d", time.Now().UnixNano()),
		weekday:   4,
		startHHMM: "10:00",
		endHHMM:   "11:00",
		roomID:    room.ID,
		staffIDs:  []int64{staffA.ID, staffB.ID},
	})

	matReq := map[string]any{"from_date": dateS, "to_date": dateS}
	rr := s.do("POST", "/materialize", matReq, s.primaryAdminClaims())
	require.Equal(t, http.StatusOK, rr.Code, "materialize body=%s", rr.Body.String())
	inst := fetchOneInstance(t, s, tmpl.group.ID, target)
	s.registerCleanup("schedule.activity_instances", inst.ID)

	// Konsolidierung (#1886): Vertretung über den atomaren Deviations-Save.
	subReq := map[string]any{
		"substitutions": []map[string]any{
			{"absent_staff_id": staffA.ID, "substitute_staff_id": subC.ID},
		},
	}
	rr = s.do("POST", fmt.Sprintf("/instances/%d/deviations", inst.ID), subReq, s.primaryAdminClaims())
	require.Equal(t, http.StatusOK, rr.Code, "substitute body=%s", rr.Body.String())

	rr = s.do("POST", "/instances/re-plan-week", matReq, s.primaryAdminClaims())
	require.Equal(t, http.StatusOK, rr.Code, "re-plan body=%s", rr.Body.String())

	newInst := fetchOneInstance(t, s, tmpl.group.ID, target)
	s.registerCleanup("schedule.activity_instances", newInst.ID)
	rows := fetchInstanceStaff(t, s, newInst.ID)
	byStaff := map[int64]scheduleModel.InstanceStaff{}
	for _, r := range rows {
		byStaff[r.StaffID] = r
	}
	require.Contains(t, byStaff, staffA.ID)
	require.Contains(t, byStaff, subC.ID)
	assert.True(t, byStaff[staffA.ID].IsAbsent, "absent staff reapplied absent")
	assert.False(t, byStaff[staffA.ID].IsSubstitute, "absent staff is a planned position")
	assert.True(t, byStaff[subC.ID].IsSubstitute, "substitute row recreated")
	assert.False(t, byStaff[subC.ID].IsAbsent, "substitute is present")
}

// TestFlowH_ClearPersistedAbsence proves a persisted day-wide absence can be
// cleared through /deviations (present again), correcting a wrongly-marked
// employee — the row is set back to present.
func TestFlowH_ClearPersistedAbsence(t *testing.T) {
	s := newScenario(t)
	defer s.teardown()

	target := nextWeekday(timezone.TodayDate(), 2, 7) // Tuesday
	dateS := target.String()
	s.createActivePeriod(fmt.Sprintf("E2E-Flow-H3-%d", time.Now().UnixNano()), target)

	room := testpkg.CreateTestRoom(t, s.db, "FlowH3-Room")
	s.extraCleanup = append(s.extraCleanup, func() {
		testpkg.CleanupTableRecords(t, s.db, "facilities.rooms", room.ID)
	})
	staffA := testpkg.CreateTestStaff(t, s.db, "FlowH3", "Alpha")
	staffB := testpkg.CreateTestStaff(t, s.db, "FlowH3", "Bravo")
	s.extraCleanup = append(s.extraCleanup, func() {
		testpkg.CleanupStaffFixtures(t, s.db, staffA.ID, staffB.ID)
	})

	tmpl := s.buildTemplate(templateSpec{
		name:      fmt.Sprintf("FlowH3-%d", time.Now().UnixNano()),
		weekday:   2,
		startHHMM: "12:00",
		endHHMM:   "13:00",
		roomID:    room.ID,
		staffIDs:  []int64{staffA.ID, staffB.ID},
	})

	matReq := map[string]any{"from_date": dateS, "to_date": dateS}
	rr := s.do("POST", "/materialize", matReq, s.primaryAdminClaims())
	require.Equal(t, http.StatusOK, rr.Code, "materialize body=%s", rr.Body.String())
	inst := fetchOneInstance(t, s, tmpl.group.ID, target)
	s.registerCleanup("schedule.activity_instances", inst.ID)

	// Mark staffA absent, then reopen and clear it.
	rr = s.do("POST", fmt.Sprintf("/instances/%d/deviations", inst.ID),
		map[string]any{"absences": []map[string]any{{"staff_id": staffA.ID}}}, s.primaryAdminClaims())
	require.Equal(t, http.StatusOK, rr.Code, "mark absent body=%s", rr.Body.String())
	for _, r := range fetchInstanceStaff(t, s, inst.ID) {
		if r.StaffID == staffA.ID {
			require.True(t, r.IsAbsent, "precondition: staffA is absent")
		}
	}

	rr = s.do("POST", fmt.Sprintf("/instances/%d/deviations", inst.ID),
		map[string]any{"presences": []int64{staffA.ID}}, s.primaryAdminClaims())
	require.Equal(t, http.StatusOK, rr.Code, "clear absence body=%s", rr.Body.String())

	var staffARow *scheduleModel.InstanceStaff
	for _, r := range fetchInstanceStaff(t, s, inst.ID) {
		if r.StaffID == staffA.ID {
			row := r
			staffARow = &row
		}
	}
	require.NotNil(t, staffARow, "staffA row present")
	assert.False(t, staffARow.IsAbsent, "persisted absence cleared")
	assert.Nil(t, staffARow.AbsenceReason, "absence reason cleared")

	// Contradiction guard: present + absent for the same staff in one save → 400.
	rr = s.do("POST", fmt.Sprintf("/instances/%d/deviations", inst.ID),
		map[string]any{
			"absences":  []map[string]any{{"staff_id": staffA.ID}},
			"presences": []int64{staffA.ID},
		}, s.primaryAdminClaims())
	assert.Equal(t, http.StatusBadRequest, rr.Code, "present+absent contradiction rejected")
}
