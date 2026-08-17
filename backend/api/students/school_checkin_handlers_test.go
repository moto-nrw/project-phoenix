package students

import (
	"testing"
	"time"

	activeService "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLabelForAttendanceStatus(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{"checked_in", "Anwesend"},
		{"on_yard", "Schulhof"},
		{"checked_out", "Abwesend"},
		{"not_checked_in", "Abwesend"},
		{"", "Abwesend"},
		{"garbage", "Abwesend"},
	}
	for _, tc := range tests {
		t.Run(tc.status, func(t *testing.T) {
			assert.Equal(t, tc.want, labelForAttendanceStatus(tc.status))
		})
	}
}

func TestBuildSchoolCheckinResponse_CheckedIn(t *testing.T) {
	checkin := time.Now().Add(-1 * time.Hour)
	status := &activeService.AttendanceStatus{
		StudentID:   42,
		Status:      "checked_in",
		CheckInTime: &checkin,
	}

	resp := buildSchoolCheckinResponse(42, status, true)

	require.NotNil(t, resp)
	assert.Equal(t, int64(42), resp.StudentID)
	assert.Equal(t, "checked_in", resp.Status)
	assert.Equal(t, "Anwesend", resp.Location)
	assert.True(t, resp.Changed)
	require.NotNil(t, resp.CheckInTime)
	assert.Equal(t, checkin, *resp.CheckInTime)
	assert.Nil(t, resp.CheckOutTime)
	assert.Nil(t, resp.YardSince)
}

func TestBuildSchoolCheckinResponse_OnYard(t *testing.T) {
	checkin := time.Now().Add(-2 * time.Hour)
	yard := time.Now().Add(-15 * time.Minute)
	status := &activeService.AttendanceStatus{
		StudentID:   42,
		Status:      "on_yard",
		CheckInTime: &checkin,
		YardSince:   &yard,
	}

	resp := buildSchoolCheckinResponse(42, status, false)

	assert.Equal(t, "Schulhof", resp.Location)
	assert.False(t, resp.Changed, "idempotent no-op should report changed=false")
	require.NotNil(t, resp.YardSince)
	assert.Equal(t, yard, *resp.YardSince)
}

func TestBuildSchoolCheckinResponse_CheckedOut(t *testing.T) {
	checkout := time.Now().Add(-5 * time.Minute)
	status := &activeService.AttendanceStatus{
		StudentID:    42,
		Status:       "checked_out",
		CheckOutTime: &checkout,
	}

	resp := buildSchoolCheckinResponse(42, status, true)

	assert.Equal(t, "Abwesend", resp.Location)
	require.NotNil(t, resp.CheckOutTime)
}

func TestBuildSchoolCheckinResponse_NotCheckedIn(t *testing.T) {
	status := &activeService.AttendanceStatus{
		StudentID: 42,
		Status:    "not_checked_in",
	}

	resp := buildSchoolCheckinResponse(42, status, false)

	assert.Equal(t, "Abwesend", resp.Location)
	assert.Nil(t, resp.CheckInTime)
	assert.Nil(t, resp.CheckOutTime)
	assert.Nil(t, resp.YardSince)
}

// =============================================================================
// activeService.IsSchoolCheckinNoop — pure decision logic, no mocks needed
// (moved from the handler into services/active with the batch orchestration,
// review #2372; cases and assertions unchanged)
// =============================================================================

func TestIsIdempotentSchoolCheckin(t *testing.T) {
	cases := []struct {
		name          string
		action        string
		currentStatus string
		wantNoOp      bool
	}{
		{"in + checked_in → no-op", "in", "checked_in", true},
		{"in + on_yard → no-op (yard is sub-state of present)", "in", "on_yard", true},
		{"in + checked_out → write", "in", "checked_out", false},
		{"in + not_checked_in → write", "in", "not_checked_in", false},
		{"out + checked_out → no-op", "out", "checked_out", true},
		{"out + not_checked_in → no-op", "out", "not_checked_in", true},
		{"out + checked_in → write", "out", "checked_in", false},
		{"out + on_yard → write (yard still counts as present)", "out", "on_yard", false},
		{"unknown action → write (handler-level validation catches this)", "toggle", "checked_in", false},
		{"unknown status → write (defensive)", "in", "garbage", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.wantNoOp, activeService.IsSchoolCheckinNoop(tc.action, tc.currentStatus))
		})
	}
}
