package e2e_timetable

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlowG_TemplateClockTimesStayTimezoneFree(t *testing.T) {
	t.Parallel()

	s := setupTimetableScenarioModule(t)

	target := nextWeekday(timezone.TodayDate(), 1, 7) // Monday, at least 7 days out.
	period := s.createActivePeriod(fmt.Sprintf("E2E-Flow-G-%d", time.Now().UnixNano()), target)

	room := testpkg.CreateTestRoom(t, s.db, "FlowG-Room")
	category := testpkg.CreateTestActivityCategory(t, s.db, "FlowG-Category")

	fromS := target.String()
	createReq := map[string]any{
		"name":               "Mensa Clock Regression",
		"type":               "care",
		"weekdays":           []int{1},
		"start_time":         "11:30",
		"end_time":           "12:00",
		"room_id":            room.ID,
		"category_id":        category.ID,
		"calendar_period_id": period.ID,
		"materialize_from":   fromS,
		"materialize_to":     fromS,
	}

	rr := s.do("POST", "/templates", createReq, s.primaryAdminClaims())
	require.Equal(t, http.StatusCreated, rr.Code, "create template body=%s", rr.Body.String())

	var created struct {
		TemplateID       int64   `json:"template_id"`
		TimeframeID      int64   `json:"timeframe_id"`
		ScheduleIDs      []int64 `json:"schedule_ids"`
		InstancesCreated int     `json:"instances_created"`
	}
	decodeResponse(t, rr, &created)
	require.NotZero(t, created.TemplateID)
	require.NotZero(t, created.TimeframeID)
	require.Len(t, created.ScheduleIDs, 1)

	assert.Equal(t, 1, created.InstancesCreated)

	rr = s.do("GET", fmt.Sprintf("/templates/%d?period_id=%d", created.TemplateID, period.ID), nil, s.primaryAdminClaims())
	require.Equal(t, http.StatusOK, rr.Code, "get template body=%s", rr.Body.String())

	var templateResp struct {
		ID        int64 `json:"id"`
		Schedules []struct {
			StartTime string `json:"start_time"`
			EndTime   string `json:"end_time"`
		} `json:"schedules"`
	}
	decodeResponse(t, rr, &templateResp)
	require.Len(t, templateResp.Schedules, 1)
	assert.Equal(t, "11:30", templateResp.Schedules[0].StartTime)
	assert.Equal(t, "12:00", templateResp.Schedules[0].EndTime)

	rr = s.do("GET", fmt.Sprintf("/instances?from=%s&to=%s", fromS, fromS), nil, s.primaryAdminClaims())
	require.Equal(t, http.StatusOK, rr.Code, "list instances body=%s", rr.Body.String())

	var instancesResp struct {
		Instances []struct {
			ID              int64  `json:"id"`
			ActivityGroupID int64  `json:"activity_group_id"`
			StartTime       string `json:"start_time"`
			EndTime         string `json:"end_time"`
		} `json:"instances"`
	}
	decodeResponse(t, rr, &instancesResp)
	require.Len(t, instancesResp.Instances, 1)
	assert.Equal(t, created.TemplateID, instancesResp.Instances[0].ActivityGroupID)
	assert.Equal(t, "11:30", instancesResp.Instances[0].StartTime)
	assert.Equal(t, "12:00", instancesResp.Instances[0].EndTime)
}
