// Package active_test tests the checkin-related functionality
package active_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/active"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Active Group Model Tests
// =============================================================================

func TestActiveGroup_IsActive(t *testing.T) {
	t.Run("group with no end time is active", func(t *testing.T) {
		group := &activeModels.Group{
			RoomID: 1,
		}
		assert.True(t, group.IsActive())
	})

	t.Run("group with end time is not active (regardless of time)", func(t *testing.T) {
		// IsActive() returns true only when EndTime is nil
		futureTime := time.Now().Add(1 * time.Hour)
		group := &activeModels.Group{
			RoomID:  1,
			EndTime: &futureTime,
		}
		assert.False(t, group.IsActive()) // EndTime is set, so not active
	})

	t.Run("group with past end time is not active", func(t *testing.T) {
		pastTime := time.Now().Add(-1 * time.Hour)
		group := &activeModels.Group{
			RoomID:  1,
			EndTime: &pastTime,
		}
		assert.False(t, group.IsActive())
	})
}

// =============================================================================
// Visit Model Tests
// =============================================================================

func TestVisit_Fields(t *testing.T) {
	t.Run("visit has required fields", func(t *testing.T) {
		now := time.Now()
		visit := &activeModels.Visit{
			StudentID:     123,
			ActiveGroupID: 456,
			EntryTime:     now,
		}

		assert.Equal(t, int64(123), visit.StudentID)
		assert.Equal(t, int64(456), visit.ActiveGroupID)
		assert.Equal(t, now, visit.EntryTime)
		assert.Nil(t, visit.ExitTime)
	})

	t.Run("visit can have exit time", func(t *testing.T) {
		now := time.Now()
		exitTime := now.Add(1 * time.Hour)
		visit := &activeModels.Visit{
			StudentID:     123,
			ActiveGroupID: 456,
			EntryTime:     now,
			ExitTime:      &exitTime,
		}

		require.NotNil(t, visit.ExitTime)
		assert.True(t, visit.ExitTime.After(visit.EntryTime))
	})

	t.Run("visit IsActive returns true when no exit time", func(t *testing.T) {
		visit := &activeModels.Visit{
			StudentID:     123,
			ActiveGroupID: 456,
			EntryTime:     time.Now(),
		}
		assert.True(t, visit.IsActive())
	})

	t.Run("visit IsActive returns false when exit time is set", func(t *testing.T) {
		exitTime := time.Now()
		visit := &activeModels.Visit{
			StudentID:     123,
			ActiveGroupID: 456,
			EntryTime:     time.Now().Add(-1 * time.Hour),
			ExitTime:      &exitTime,
		}
		assert.False(t, visit.IsActive())
	})
}

// =============================================================================
// CheckinRequest Tests
// =============================================================================

func TestCheckinRequest_Validation(t *testing.T) {
	t.Run("valid request with active_group_id", func(t *testing.T) {
		req := active.CheckinRequest{
			ActiveGroupID: 1,
		}
		assert.Greater(t, req.ActiveGroupID, int64(0))
	})

	t.Run("invalid request without active_group_id", func(t *testing.T) {
		req := active.CheckinRequest{}
		assert.Equal(t, int64(0), req.ActiveGroupID)
	})
}

func TestCheckinRequest_JSONDecoding(t *testing.T) {
	t.Run("decodes from JSON correctly", func(t *testing.T) {
		jsonData := `{"active_group_id": 456}`
		var req active.CheckinRequest
		err := json.Unmarshal([]byte(jsonData), &req)
		require.NoError(t, err)
		assert.Equal(t, int64(456), req.ActiveGroupID)
	})

	t.Run("decodes zero value when missing", func(t *testing.T) {
		jsonData := `{}`
		var req active.CheckinRequest
		err := json.Unmarshal([]byte(jsonData), &req)
		require.NoError(t, err)
		assert.Equal(t, int64(0), req.ActiveGroupID)
	})

	t.Run("encodes to JSON correctly", func(t *testing.T) {
		req := active.CheckinRequest{ActiveGroupID: 123}
		data, err := json.Marshal(req)
		require.NoError(t, err)
		assert.Contains(t, string(data), "123")
	})
}

// =============================================================================
// Attendance Model Tests
// =============================================================================

func TestAttendance_Fields(t *testing.T) {
	t.Run("attendance has required fields", func(t *testing.T) {
		now := time.Now()
		today := time.Now().UTC().Truncate(24 * time.Hour)
		attendance := &activeModels.Attendance{
			StudentID:   123,
			Date:        today,
			CheckInTime: now,
			CheckedInBy: 456,
			DeviceID:    789,
		}

		assert.Equal(t, int64(123), attendance.StudentID)
		assert.Equal(t, today, attendance.Date)
		assert.Equal(t, now, attendance.CheckInTime)
		assert.Equal(t, int64(456), attendance.CheckedInBy)
		assert.Equal(t, int64(789), attendance.DeviceID)
		assert.Nil(t, attendance.CheckOutTime)
		assert.Nil(t, attendance.CheckedOutBy)
	})

	t.Run("attendance can have checkout fields", func(t *testing.T) {
		now := time.Now()
		checkoutTime := now.Add(4 * time.Hour)
		checkedOutBy := int64(789)

		attendance := &activeModels.Attendance{
			StudentID:    123,
			Date:         time.Now().UTC().Truncate(24 * time.Hour),
			CheckInTime:  now,
			CheckedInBy:  456,
			DeviceID:     111,
			CheckOutTime: &checkoutTime,
			CheckedOutBy: &checkedOutBy,
		}

		require.NotNil(t, attendance.CheckOutTime)
		require.NotNil(t, attendance.CheckedOutBy)
		assert.True(t, attendance.CheckOutTime.After(attendance.CheckInTime))
		assert.Equal(t, int64(789), *attendance.CheckedOutBy)
	})
}

// =============================================================================
// Group Supervisor Model Tests
// =============================================================================

func TestGroupSupervisor_IsActive(t *testing.T) {
	t.Run("supervisor with no end date is active", func(t *testing.T) {
		supervisor := &activeModels.GroupSupervisor{
			StaffID:   1,
			GroupID:   2,
			Role:      "supervisor",
			StartDate: time.Now(),
		}
		assert.True(t, supervisor.IsActive())
	})

	t.Run("supervisor with future end date is active", func(t *testing.T) {
		futureDate := time.Now().Add(30 * 24 * time.Hour)
		supervisor := &activeModels.GroupSupervisor{
			StaffID:   1,
			GroupID:   2,
			Role:      "supervisor",
			StartDate: time.Now(),
			EndDate:   &futureDate,
		}
		assert.True(t, supervisor.IsActive())
	})

	t.Run("supervisor with past end date is not active", func(t *testing.T) {
		pastDate := time.Now().Add(-30 * 24 * time.Hour)
		supervisor := &activeModels.GroupSupervisor{
			StaffID:   1,
			GroupID:   2,
			Role:      "supervisor",
			StartDate: time.Now().Add(-60 * 24 * time.Hour),
			EndDate:   &pastDate,
		}
		assert.False(t, supervisor.IsActive())
	})
}

// =============================================================================
// Combined Group Model Tests
// =============================================================================

func TestCombinedGroup_IsActive(t *testing.T) {
	t.Run("combined group with no end time is active", func(t *testing.T) {
		combined := &activeModels.CombinedGroup{
			StartTime: time.Now(),
		}
		assert.True(t, combined.IsActive())
	})

	t.Run("combined group with end time is not active", func(t *testing.T) {
		endTime := time.Now()
		combined := &activeModels.CombinedGroup{
			StartTime: time.Now().Add(-1 * time.Hour),
			EndTime:   &endTime,
		}
		assert.False(t, combined.IsActive())
	})
}
