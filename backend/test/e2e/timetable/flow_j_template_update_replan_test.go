// Flow J — a whole-series template update re-plans its planned occurrences
// in its own transaction (#6838). The client used to re-plan in 56-day
// chunks after the PUT; a failed chunk or a closed tab left the old
// occurrences beside the new ones.
package e2e_timetable

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

func TestFlowJ_TemplateUpdateReplansTheWholePeriod(t *testing.T) {
	t.Parallel()

	s := setupTimetableScenarioModule(t)

	const farDays, beyondDays = 10 * 7, 11 * 7
	near := s.plannableWeekday(3, 7, farDays, beyondDays) // Wednesday, >=7 days out
	far := near.AddDays(farDays)                          // beyond the first 56-day window
	beyond := near.AddDays(beyondDays)                    // never materialized before the edit
	period := s.createActivePeriod(fmt.Sprintf("E2E-Flow-J-%d", time.Now().UnixNano()), near)

	room := testpkg.CreateTestRoom(t, s.db, "FlowJ-Room")
	category := testpkg.CreateTestActivityCategory(t, s.db, "FlowJ-Category")
	staffA := testpkg.CreateTestStaff(t, s.db, "FlowJ", "Alpha")
	staffB := testpkg.CreateTestStaff(t, s.db, "FlowJ", "Bravo")

	body := func(start, end string) map[string]any {
		return map[string]any{
			"name":               "FlowJ-Serie",
			"type":               "care",
			"weekdays":           []int{3},
			"start_time":         start,
			"end_time":           end,
			"room_id":            room.ID,
			"category_id":        category.ID,
			"calendar_period_id": period.ID,
			"staff_ids":          []int64{staffA.ID, staffB.ID},
			"primary_staff_id":   staffA.ID,
		}
	}
	create := body("14:00", "15:00")
	create["materialize_from"] = near.String()
	create["materialize_to"] = near.String()
	rr := s.do("POST", "/templates", create, s.primaryAdminClaims())
	require.Equal(t, http.StatusCreated, rr.Code, "create template body=%s", rr.Body.String())
	var created struct {
		TemplateID int64 `json:"template_id"`
	}
	decodeResponse(t, rr, &created)

	rr = s.do("POST", "/materialize", map[string]any{"from_date": far.String(), "to_date": far.String()}, s.primaryAdminClaims())
	require.Equal(t, http.StatusOK, rr.Code, "materialize body=%s", rr.Body.String())
	farBefore := fetchOneInstance(t, s, created.TemplateID, far)
	require.Equal(t, "14:00", farBefore.StartTime.Format("15:04"))

	devReq := map[string]any{
		"absences":          []map[string]any{{"staff_id": staffA.ID, "reason": "Krank"}},
		"understaffed_ack":  true,
		"understaffed_note": "Kein Ersatz verfügbar",
	}
	rr = s.do("POST", fmt.Sprintf("/instances/%d/deviations", farBefore.ID), devReq, s.primaryAdminClaims())
	require.Equal(t, http.StatusOK, rr.Code, "deviations body=%s", rr.Body.String())

	update := body("15:00", "16:00")
	update["replan_future"] = true
	rr = s.do("PUT", fmt.Sprintf("/templates/%d", created.TemplateID), update, s.primaryAdminClaims())
	require.Equal(t, http.StatusOK, rr.Code, "update template body=%s", rr.Body.String())

	// fetchOneInstance fails on a stale 14:00 row beside the new 15:00 one.
	for _, day := range []calendar.Date{near, far, beyond} {
		inst := fetchOneInstance(t, s, created.TemplateID, day)
		assert.Equal(t, "15:00", inst.StartTime.Format("15:04"), "occurrence on %s follows the edit", day)
	}
	stale := testpkg.ActivityInstancesWhere(t, s.tenantCtx(), s.db,
		`"activity_instance".activity_group_id = ? AND "activity_instance".start_time = '14:00' AND "activity_instance".tenant_id = ?`,
		created.TemplateID, s.primaryTenant)
	assert.Empty(t, stale, "no occurrence keeps the old start time")

	farAfter := fetchOneInstance(t, s, created.TemplateID, far)
	assert.True(t, farAfter.UnderstaffedAck, "acknowledgement reapplied beyond the first window")
	absent := map[int64]bool{}
	for _, row := range fetchInstanceStaff(t, s, farAfter.ID) {
		absent[row.StaffID] = row.IsAbsent
	}
	assert.Equal(t, map[int64]bool{staffA.ID: true, staffB.ID: false}, absent, "absence reapplied")
}
