package common_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// StudentLocationInfo Tests
// =============================================================================

func TestStudentLocationInfo_Fields(t *testing.T) {
	now := time.Now()
	info := common.StudentLocationInfo{
		Location: "Anwesend - Room 101",
		Since:    &now,
	}

	assert.Equal(t, "Anwesend - Room 101", info.Location)
	assert.NotNil(t, info.Since)
	assert.Equal(t, now, *info.Since)
}

func TestStudentLocationInfo_NilSince(t *testing.T) {
	info := common.StudentLocationInfo{
		Location: "Abwesend",
		Since:    nil,
	}

	assert.Equal(t, "Abwesend", info.Location)
	assert.Nil(t, info.Since)
}

// =============================================================================
// StudentLocationSnapshot Tests
// =============================================================================

func TestStudentLocationSnapshot_ResolveStudentLocation_NilSnapshot(t *testing.T) {
	var snapshot *common.StudentLocationSnapshot = nil

	location := snapshot.ResolveStudentLocation(123, true)

	assert.Equal(t, "Abwesend", location)
}

func TestStudentLocationSnapshot_ResolveStudentLocation_EmptySnapshot(t *testing.T) {
	snapshot := &common.StudentLocationSnapshot{
		Attendances: make(map[int64]*activeService.AttendanceStatus),
		Visits:      make(map[int64]*activeModels.Visit),
		Groups:      make(map[int64]*activeModels.Group),
	}

	location := snapshot.ResolveStudentLocation(123, true)

	assert.Equal(t, "Abwesend", location)
}

func TestStudentLocationSnapshot_ResolveStudentLocation_NotCheckedIn(t *testing.T) {
	snapshot := &common.StudentLocationSnapshot{
		Attendances: map[int64]*activeService.AttendanceStatus{
			123: {
				StudentID: 123,
				Status:    "not_checked_in",
			},
		},
		Visits: make(map[int64]*activeModels.Visit),
		Groups: make(map[int64]*activeModels.Group),
	}

	location := snapshot.ResolveStudentLocation(123, true)

	assert.Equal(t, "Abwesend", location)
}

func TestStudentLocationSnapshot_ResolveStudentLocation_CheckedOut(t *testing.T) {
	checkoutTime := time.Now().Add(-30 * time.Minute)
	snapshot := &common.StudentLocationSnapshot{
		Attendances: map[int64]*activeService.AttendanceStatus{
			123: {
				StudentID:    123,
				Status:       "checked_out",
				CheckOutTime: &checkoutTime,
			},
		},
		Visits: make(map[int64]*activeModels.Visit),
		Groups: make(map[int64]*activeModels.Group),
	}

	location := snapshot.ResolveStudentLocation(123, true)

	assert.Equal(t, "Abwesend", location)
}

func TestStudentLocationSnapshot_ResolveStudentLocation_CheckedOut_NoFullAccess(t *testing.T) {
	checkoutTime := time.Now().Add(-30 * time.Minute)
	snapshot := &common.StudentLocationSnapshot{
		Attendances: map[int64]*activeService.AttendanceStatus{
			123: {
				StudentID:    123,
				Status:       "checked_out",
				CheckOutTime: &checkoutTime,
			},
		},
		Visits: make(map[int64]*activeModels.Visit),
		Groups: make(map[int64]*activeModels.Group),
	}

	location := snapshot.ResolveStudentLocation(123, false)

	assert.Equal(t, "Abwesend", location)
}

func TestStudentLocationSnapshot_ResolveStudentLocation_CheckedIn_NoFullAccess(t *testing.T) {
	checkinTime := time.Now().Add(-1 * time.Hour)
	snapshot := &common.StudentLocationSnapshot{
		Attendances: map[int64]*activeService.AttendanceStatus{
			123: {
				StudentID:   123,
				Status:      "checked_in",
				CheckInTime: &checkinTime,
			},
		},
		Visits: make(map[int64]*activeModels.Visit),
		Groups: make(map[int64]*activeModels.Group),
	}

	location := snapshot.ResolveStudentLocation(123, false)

	assert.Equal(t, "Anwesend", location)
}

func TestStudentLocationSnapshot_ResolveStudentLocation_CheckedIn_NoVisit(t *testing.T) {
	checkinTime := time.Now().Add(-1 * time.Hour)
	snapshot := &common.StudentLocationSnapshot{
		Attendances: map[int64]*activeService.AttendanceStatus{
			123: {
				StudentID:   123,
				Status:      "checked_in",
				CheckInTime: &checkinTime,
			},
		},
		Visits: make(map[int64]*activeModels.Visit), // No visit
		Groups: make(map[int64]*activeModels.Group),
	}

	location := snapshot.ResolveStudentLocation(123, true)

	assert.Equal(t, "Unterwegs", location)
}

func TestStudentLocationSnapshot_ResolveStudentLocation_CheckedIn_NilVisit(t *testing.T) {
	checkinTime := time.Now().Add(-1 * time.Hour)
	snapshot := &common.StudentLocationSnapshot{
		Attendances: map[int64]*activeService.AttendanceStatus{
			123: {
				StudentID:   123,
				Status:      "checked_in",
				CheckInTime: &checkinTime,
			},
		},
		Visits: map[int64]*activeModels.Visit{
			123: nil, // Explicit nil visit
		},
		Groups: make(map[int64]*activeModels.Group),
	}

	location := snapshot.ResolveStudentLocation(123, true)

	assert.Equal(t, "Unterwegs", location)
}

func TestStudentLocationSnapshot_ResolveStudentLocation_CheckedIn_VisitNoGroupID(t *testing.T) {
	checkinTime := time.Now().Add(-1 * time.Hour)
	entryTime := time.Now().Add(-30 * time.Minute)
	snapshot := &common.StudentLocationSnapshot{
		Attendances: map[int64]*activeService.AttendanceStatus{
			123: {
				StudentID:   123,
				Status:      "checked_in",
				CheckInTime: &checkinTime,
			},
		},
		Visits: map[int64]*activeModels.Visit{
			123: {
				StudentID:     123,
				ActiveGroupID: 0, // No group ID
				EntryTime:     entryTime,
			},
		},
		Groups: make(map[int64]*activeModels.Group),
	}

	location := snapshot.ResolveStudentLocation(123, true)

	assert.Equal(t, "Unterwegs", location)
}

func TestStudentLocationSnapshot_ResolveStudentLocation_CheckedIn_GroupNotFound(t *testing.T) {
	checkinTime := time.Now().Add(-1 * time.Hour)
	entryTime := time.Now().Add(-30 * time.Minute)
	snapshot := &common.StudentLocationSnapshot{
		Attendances: map[int64]*activeService.AttendanceStatus{
			123: {
				StudentID:   123,
				Status:      "checked_in",
				CheckInTime: &checkinTime,
			},
		},
		Visits: map[int64]*activeModels.Visit{
			123: {
				StudentID:     123,
				ActiveGroupID: 456,
				EntryTime:     entryTime,
			},
		},
		Groups: make(map[int64]*activeModels.Group), // Group 456 not in map
	}

	location := snapshot.ResolveStudentLocation(123, true)

	assert.Equal(t, "Unterwegs", location)
}

func TestStudentLocationSnapshot_ResolveStudentLocation_CheckedIn_NilGroup(t *testing.T) {
	checkinTime := time.Now().Add(-1 * time.Hour)
	entryTime := time.Now().Add(-30 * time.Minute)
	snapshot := &common.StudentLocationSnapshot{
		Attendances: map[int64]*activeService.AttendanceStatus{
			123: {
				StudentID:   123,
				Status:      "checked_in",
				CheckInTime: &checkinTime,
			},
		},
		Visits: map[int64]*activeModels.Visit{
			123: {
				StudentID:     123,
				ActiveGroupID: 456,
				EntryTime:     entryTime,
			},
		},
		Groups: map[int64]*activeModels.Group{
			456: nil, // Explicit nil
		},
	}

	location := snapshot.ResolveStudentLocation(123, true)

	assert.Equal(t, "Unterwegs", location)
}

func TestStudentLocationSnapshot_ResolveStudentLocation_CheckedIn_GroupNoRoom(t *testing.T) {
	checkinTime := time.Now().Add(-1 * time.Hour)
	entryTime := time.Now().Add(-30 * time.Minute)
	startTime := time.Now().Add(-2 * time.Hour)
	snapshot := &common.StudentLocationSnapshot{
		Attendances: map[int64]*activeService.AttendanceStatus{
			123: {
				StudentID:   123,
				Status:      "checked_in",
				CheckInTime: &checkinTime,
			},
		},
		Visits: map[int64]*activeModels.Visit{
			123: {
				StudentID:     123,
				ActiveGroupID: 456,
				EntryTime:     entryTime,
			},
		},
		Groups: map[int64]*activeModels.Group{
			456: {
				GroupID:   789,
				RoomID:    1,
				StartTime: startTime,
				Room:      nil, // No room loaded
			},
		},
	}

	location := snapshot.ResolveStudentLocation(123, true)

	assert.Equal(t, "Unterwegs", location)
}

func TestStudentLocationSnapshot_ResolveStudentLocation_CheckedIn_GroupEmptyRoomName(t *testing.T) {
	checkinTime := time.Now().Add(-1 * time.Hour)
	entryTime := time.Now().Add(-30 * time.Minute)
	startTime := time.Now().Add(-2 * time.Hour)
	snapshot := &common.StudentLocationSnapshot{
		Attendances: map[int64]*activeService.AttendanceStatus{
			123: {
				StudentID:   123,
				Status:      "checked_in",
				CheckInTime: &checkinTime,
			},
		},
		Visits: map[int64]*activeModels.Visit{
			123: {
				StudentID:     123,
				ActiveGroupID: 456,
				EntryTime:     entryTime,
			},
		},
		Groups: map[int64]*activeModels.Group{
			456: {
				GroupID:   789,
				RoomID:    1,
				StartTime: startTime,
				Room: &facilities.Room{
					Name: "", // Empty name
				},
			},
		},
	}

	location := snapshot.ResolveStudentLocation(123, true)

	assert.Equal(t, "Unterwegs", location)
}

func TestStudentLocationSnapshot_ResolveStudentLocation_CheckedIn_WithRoom(t *testing.T) {
	checkinTime := time.Now().Add(-1 * time.Hour)
	entryTime := time.Now().Add(-30 * time.Minute)
	startTime := time.Now().Add(-2 * time.Hour)
	snapshot := &common.StudentLocationSnapshot{
		Attendances: map[int64]*activeService.AttendanceStatus{
			123: {
				StudentID:   123,
				Status:      "checked_in",
				CheckInTime: &checkinTime,
			},
		},
		Visits: map[int64]*activeModels.Visit{
			123: {
				StudentID:     123,
				ActiveGroupID: 456,
				EntryTime:     entryTime,
			},
		},
		Groups: map[int64]*activeModels.Group{
			456: {
				GroupID:   789,
				RoomID:    1,
				StartTime: startTime,
				Room: &facilities.Room{
					Name:     "Room 101",
					Building: "Main Building",
				},
			},
		},
	}

	location := snapshot.ResolveStudentLocation(123, true)

	assert.Equal(t, "Anwesend - Room 101", location)
}

// =============================================================================
// ResolveStudentLocationWithTime Tests
// =============================================================================

func TestStudentLocationSnapshot_ResolveStudentLocationWithTime_NilSnapshot(t *testing.T) {
	var snapshot *common.StudentLocationSnapshot = nil

	info := snapshot.ResolveStudentLocationWithTime(123, true)

	assert.Equal(t, "Abwesend", info.Location)
	assert.Nil(t, info.Since)
}

func TestStudentLocationSnapshot_ResolveStudentLocationWithTime_CheckedOut_FullAccess(t *testing.T) {
	checkoutTime := time.Now().Add(-30 * time.Minute)
	snapshot := &common.StudentLocationSnapshot{
		Attendances: map[int64]*activeService.AttendanceStatus{
			123: {
				StudentID:    123,
				Status:       "checked_out",
				CheckOutTime: &checkoutTime,
			},
		},
		Visits: make(map[int64]*activeModels.Visit),
		Groups: make(map[int64]*activeModels.Group),
	}

	info := snapshot.ResolveStudentLocationWithTime(123, true)

	assert.Equal(t, "Abwesend", info.Location)
	require.NotNil(t, info.Since)
	assert.Equal(t, checkoutTime, *info.Since)
}

func TestStudentLocationSnapshot_ResolveStudentLocationWithTime_CheckedOut_NoFullAccess(t *testing.T) {
	checkoutTime := time.Now().Add(-30 * time.Minute)
	snapshot := &common.StudentLocationSnapshot{
		Attendances: map[int64]*activeService.AttendanceStatus{
			123: {
				StudentID:    123,
				Status:       "checked_out",
				CheckOutTime: &checkoutTime,
			},
		},
		Visits: make(map[int64]*activeModels.Visit),
		Groups: make(map[int64]*activeModels.Group),
	}

	info := snapshot.ResolveStudentLocationWithTime(123, false)

	assert.Equal(t, "Abwesend", info.Location)
	assert.Nil(t, info.Since) // No time for non-full-access users
}

func TestStudentLocationSnapshot_ResolveStudentLocationWithTime_CheckedIn_WithRoom(t *testing.T) {
	checkinTime := time.Now().Add(-1 * time.Hour)
	entryTime := time.Now().Add(-30 * time.Minute)
	startTime := time.Now().Add(-2 * time.Hour)
	snapshot := &common.StudentLocationSnapshot{
		Attendances: map[int64]*activeService.AttendanceStatus{
			123: {
				StudentID:   123,
				Status:      "checked_in",
				CheckInTime: &checkinTime,
			},
		},
		Visits: map[int64]*activeModels.Visit{
			123: {
				StudentID:     123,
				ActiveGroupID: 456,
				EntryTime:     entryTime,
			},
		},
		Groups: map[int64]*activeModels.Group{
			456: {
				GroupID:   789,
				RoomID:    1,
				StartTime: startTime,
				Room: &facilities.Room{
					Name: "Art Room",
				},
			},
		},
	}

	info := snapshot.ResolveStudentLocationWithTime(123, true)

	assert.Equal(t, "Anwesend - Art Room", info.Location)
	require.NotNil(t, info.Since)
	assert.Equal(t, entryTime, *info.Since)
}

func TestStudentLocationSnapshot_ResolveStudentLocationWithTime_Unterwegs(t *testing.T) {
	checkinTime := time.Now().Add(-1 * time.Hour)
	snapshot := &common.StudentLocationSnapshot{
		Attendances: map[int64]*activeService.AttendanceStatus{
			123: {
				StudentID:   123,
				Status:      "checked_in",
				CheckInTime: &checkinTime,
			},
		},
		Visits: make(map[int64]*activeModels.Visit),
		Groups: make(map[int64]*activeModels.Group),
	}

	info := snapshot.ResolveStudentLocationWithTime(123, true)

	assert.Equal(t, "Unterwegs", info.Location)
	assert.Nil(t, info.Since)
}

// =============================================================================
// Multiple Students Tests
// =============================================================================

func TestStudentLocationSnapshot_MultipleStudents(t *testing.T) {
	checkinTime := time.Now().Add(-1 * time.Hour)
	checkoutTime := time.Now().Add(-30 * time.Minute)
	entryTime := time.Now().Add(-15 * time.Minute)
	startTime := time.Now().Add(-2 * time.Hour)

	snapshot := &common.StudentLocationSnapshot{
		Attendances: map[int64]*activeService.AttendanceStatus{
			1: {StudentID: 1, Status: "not_checked_in"},
			2: {StudentID: 2, Status: "checked_out", CheckOutTime: &checkoutTime},
			3: {StudentID: 3, Status: "checked_in", CheckInTime: &checkinTime},
			4: {StudentID: 4, Status: "checked_in", CheckInTime: &checkinTime},
		},
		Visits: map[int64]*activeModels.Visit{
			4: {StudentID: 4, ActiveGroupID: 10, EntryTime: entryTime},
		},
		Groups: map[int64]*activeModels.Group{
			10: {
				GroupID: 100, RoomID: 1, StartTime: startTime,
				Room: &facilities.Room{Name: "Cafeteria"},
			},
		},
	}

	tests := []struct {
		studentID      int64
		hasFullAccess  bool
		expectedLoc    string
		expectSince    bool
	}{
		{1, true, "Abwesend", false},
		{2, true, "Abwesend", true},
		{2, false, "Abwesend", false},
		{3, true, "Unterwegs", false},
		{3, false, "Anwesend", false},
		{4, true, "Anwesend - Cafeteria", true},
		{4, false, "Anwesend", false},
		{999, true, "Abwesend", false}, // Unknown student
	}

	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			info := snapshot.ResolveStudentLocationWithTime(tc.studentID, tc.hasFullAccess)
			assert.Equal(t, tc.expectedLoc, info.Location)
			if tc.expectSince {
				assert.NotNil(t, info.Since)
			} else {
				assert.Nil(t, info.Since)
			}
		})
	}
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestStudentLocationSnapshot_NilAttendanceInMap(t *testing.T) {
	snapshot := &common.StudentLocationSnapshot{
		Attendances: map[int64]*activeService.AttendanceStatus{
			123: nil, // Explicit nil value
		},
		Visits: make(map[int64]*activeModels.Visit),
		Groups: make(map[int64]*activeModels.Group),
	}

	location := snapshot.ResolveStudentLocation(123, true)

	assert.Equal(t, "Abwesend", location)
}

func TestStudentLocationSnapshot_UnknownStatus(t *testing.T) {
	snapshot := &common.StudentLocationSnapshot{
		Attendances: map[int64]*activeService.AttendanceStatus{
			123: {
				StudentID: 123,
				Status:    "unknown_status", // Invalid status
			},
		},
		Visits: make(map[int64]*activeModels.Visit),
		Groups: make(map[int64]*activeModels.Group),
	}

	location := snapshot.ResolveStudentLocation(123, true)

	// Unknown status should return "Abwesend"
	assert.Equal(t, "Abwesend", location)
}
