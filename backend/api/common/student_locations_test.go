package common_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/ptrtest"
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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	var snapshot *common.StudentLocationSnapshot = nil

	location := snapshot.ResolveStudentLocation(123, true)

	assert.Equal(t, "Abwesend", location)
}

func TestStudentLocationSnapshot_ResolveStudentLocation_EmptySnapshot(t *testing.T) {
	t.Parallel()

	snapshot := &common.StudentLocationSnapshot{
		Attendances: make(map[int64]*activeService.AttendanceStatus),
		Visits:      make(map[int64]*activeModels.Visit),
		Groups:      make(map[int64]*activeModels.Group),
	}

	location := snapshot.ResolveStudentLocation(123, true)

	assert.Equal(t, "Abwesend", location)
}

func TestStudentLocationSnapshot_ResolveStudentLocation_NotCheckedIn(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
				GroupID:   ptrtest.Ptr(int64(789)),
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
	t.Parallel()

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
				GroupID:   ptrtest.Ptr(int64(789)),
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
	t.Parallel()

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
				GroupID:   ptrtest.Ptr(int64(789)),
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

// TestStudentLocationSnapshot_ResolveStudentLocation_RoomColor confirms the
// snapshot resolver populates StudentLocationInfo.RoomColor when the active
// group's room has a color configured. Frontend depends on this to render
// per-room badge colors instead of every "Anwesend - <Room>" being blue.
func TestStudentLocationSnapshot_ResolveStudentLocation_RoomColor(t *testing.T) {
	t.Parallel()

	checkinTime := time.Now().Add(-30 * time.Minute)
	entryTime := time.Now().Add(-10 * time.Minute)
	startTime := time.Now().Add(-1 * time.Hour)
	roomColor := "#A3D977"

	t.Run("populates RoomColor when room has color set", func(t *testing.T) {
		snapshot := &common.StudentLocationSnapshot{
			Mode: common.PresenceModeDetailed,
			Attendances: map[int64]*activeService.AttendanceStatus{
				123: {StudentID: 123, Status: "checked_in", CheckInTime: &checkinTime},
			},
			Visits: map[int64]*activeModels.Visit{
				123: {StudentID: 123, ActiveGroupID: 456, EntryTime: entryTime},
			},
			Groups: map[int64]*activeModels.Group{
				456: {
					GroupID:   ptrtest.Ptr(int64(789)),
					RoomID:    1,
					StartTime: startTime,
					Room: &facilities.Room{
						Name:     "Bibliothek",
						Building: "Main Building",
						Color:    &roomColor,
					},
				},
			},
		}

		info := snapshot.ResolveStudentLocationWithTime(123, true)
		assert.Equal(t, "Anwesend - Bibliothek", info.Location)
		require.NotNil(t, info.RoomColor)
		assert.Equal(t, "#A3D977", *info.RoomColor)
	})

	t.Run("RoomColor is nil when room has no color (fallback to blue)", func(t *testing.T) {
		snapshot := &common.StudentLocationSnapshot{
			Mode: common.PresenceModeDetailed,
			Attendances: map[int64]*activeService.AttendanceStatus{
				123: {StudentID: 123, Status: "checked_in", CheckInTime: &checkinTime},
			},
			Visits: map[int64]*activeModels.Visit{
				123: {StudentID: 123, ActiveGroupID: 456, EntryTime: entryTime},
			},
			Groups: map[int64]*activeModels.Group{
				456: {
					GroupID:   ptrtest.Ptr(int64(789)),
					RoomID:    1,
					StartTime: startTime,
					Room:      &facilities.Room{Name: "Sportraum"},
				},
			},
		}

		info := snapshot.ResolveStudentLocationWithTime(123, true)
		assert.Equal(t, "Anwesend - Sportraum", info.Location)
		assert.Nil(t, info.RoomColor,
			"a room without color must propagate nil so the frontend falls back to OTHER_ROOM blue")
	})
}

// =============================================================================
// ResolveStudentLocationWithTime Tests
// =============================================================================

func TestStudentLocationSnapshot_ResolveStudentLocationWithTime_NilSnapshot(t *testing.T) {
	t.Parallel()

	var snapshot *common.StudentLocationSnapshot = nil

	info := snapshot.ResolveStudentLocationWithTime(123, true)

	assert.Equal(t, "Abwesend", info.Location)
	assert.Nil(t, info.Since)
}

func TestStudentLocationSnapshot_ResolveStudentLocationWithTime_CheckedOut_FullAccess(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
				GroupID:   ptrtest.Ptr(int64(789)),
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
	t.Parallel()

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
	t.Parallel()

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
				GroupID: ptrtest.Ptr(int64(100)), RoomID: 1, StartTime: startTime,
				Room: &facilities.Room{Name: "Cafeteria"},
			},
		},
	}

	tests := []struct {
		studentID     int64
		hasFullAccess bool
		expectedLoc   string
		expectSince   bool
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
	t.Parallel()

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
	t.Parallel()

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

// =============================================================================
// Binary mode + tri-state attendance tests
// =============================================================================

func TestBinaryMode_CheckedIn_ReturnsAnwesend(t *testing.T) {
	t.Parallel()

	checkinTime := time.Now().Add(-2 * time.Hour)
	snapshot := &common.StudentLocationSnapshot{
		Mode: common.PresenceModeBinary,
		Attendances: map[int64]*activeService.AttendanceStatus{
			42: {
				StudentID:   42,
				Status:      "checked_in",
				CheckInTime: &checkinTime,
			},
		},
	}

	info := snapshot.ResolveStudentLocationWithTime(42, true)

	assert.Equal(t, "Anwesend", info.Location)
	require.NotNil(t, info.Since)
	assert.Equal(t, checkinTime, *info.Since)
}

func TestBinaryMode_CheckedIn_NoFullAccess_OmitsTimestamp(t *testing.T) {
	t.Parallel()

	checkinTime := time.Now().Add(-2 * time.Hour)
	snapshot := &common.StudentLocationSnapshot{
		Mode: common.PresenceModeBinary,
		Attendances: map[int64]*activeService.AttendanceStatus{
			42: {
				StudentID:   42,
				Status:      "checked_in",
				CheckInTime: &checkinTime,
			},
		},
	}

	info := snapshot.ResolveStudentLocationWithTime(42, false)

	assert.Equal(t, "Anwesend", info.Location)
	assert.Nil(t, info.Since, "non-full-access viewers should not receive timestamps")
}

func TestBinaryMode_OnYard_ReturnsSchulhof(t *testing.T) {
	t.Parallel()

	checkinTime := time.Now().Add(-3 * time.Hour)
	yardSince := time.Now().Add(-15 * time.Minute)
	snapshot := &common.StudentLocationSnapshot{
		Mode: common.PresenceModeBinary,
		Attendances: map[int64]*activeService.AttendanceStatus{
			42: {
				StudentID:   42,
				Status:      "on_yard",
				CheckInTime: &checkinTime,
				YardSince:   &yardSince,
			},
		},
	}

	info := snapshot.ResolveStudentLocationWithTime(42, true)

	assert.Equal(t, "Schulhof", info.Location)
	require.NotNil(t, info.Since)
	assert.Equal(t, yardSince, *info.Since, "Since should be the yard transition timestamp, not the earlier check-in")
}

func TestBinaryMode_CheckedOut_ReturnsAbwesend(t *testing.T) {
	t.Parallel()

	checkoutTime := time.Now().Add(-10 * time.Minute)
	snapshot := &common.StudentLocationSnapshot{
		Mode: common.PresenceModeBinary,
		Attendances: map[int64]*activeService.AttendanceStatus{
			42: {
				StudentID:    42,
				Status:       "checked_out",
				CheckOutTime: &checkoutTime,
			},
		},
	}

	info := snapshot.ResolveStudentLocationWithTime(42, true)

	assert.Equal(t, "Abwesend", info.Location)
	require.NotNil(t, info.Since)
	assert.Equal(t, checkoutTime, *info.Since)
}

func TestBinaryMode_NotCheckedIn_ReturnsAbwesend(t *testing.T) {
	t.Parallel()

	snapshot := &common.StudentLocationSnapshot{
		Mode: common.PresenceModeBinary,
		Attendances: map[int64]*activeService.AttendanceStatus{
			42: {StudentID: 42, Status: "not_checked_in"},
		},
	}

	info := snapshot.ResolveStudentLocationWithTime(42, true)

	assert.Equal(t, "Abwesend", info.Location)
	assert.Nil(t, info.Since)
}

func TestBinaryMode_IgnoresVisitsAndGroups(t *testing.T) {
	t.Parallel()

	// Even if visit and group data somehow leak into a binary snapshot, the
	// resolver must not consult them — binary semantics are attendance-only.
	checkinTime := time.Now().Add(-1 * time.Hour)
	entryTime := time.Now().Add(-30 * time.Minute)
	snapshot := &common.StudentLocationSnapshot{
		Mode: common.PresenceModeBinary,
		Attendances: map[int64]*activeService.AttendanceStatus{
			42: {
				StudentID:   42,
				Status:      "checked_in",
				CheckInTime: &checkinTime,
			},
		},
		Visits: map[int64]*activeModels.Visit{
			42: {StudentID: 42, ActiveGroupID: 99, EntryTime: entryTime},
		},
		Groups: map[int64]*activeModels.Group{
			99: {RoomID: 1, Room: &facilities.Room{Name: "Art Room"}},
		},
	}

	info := snapshot.ResolveStudentLocationWithTime(42, true)

	assert.Equal(t, "Anwesend", info.Location, "binary mode must not render room names from stray visit data")
}

func TestDetailedMode_OnYardStatusFallsThroughToAbwesend(t *testing.T) {
	t.Parallel()

	// In detailed mode yard_since is never written (yard is binary-only), but
	// if the status somehow derives to "on_yard", the resolver treats it as
	// not-checked-in — detailed mode has no Schulhof label path.
	yardSince := time.Now().Add(-15 * time.Minute)
	snapshot := &common.StudentLocationSnapshot{
		Mode: common.PresenceModeDetailed,
		Attendances: map[int64]*activeService.AttendanceStatus{
			42: {StudentID: 42, Status: "on_yard", YardSince: &yardSince},
		},
	}

	info := snapshot.ResolveStudentLocationWithTime(42, true)

	assert.Equal(t, "Abwesend", info.Location, "detailed mode ignores yard state — only checked_in/checked_out drive the label")
}

func TestDefaultMode_EmptyModeBehavesAsDetailed(t *testing.T) {
	t.Parallel()

	// Old test fixtures construct a snapshot without setting Mode. The
	// resolver must keep treating those as detailed-mode for backwards
	// compatibility.
	checkinTime := time.Now().Add(-1 * time.Hour)
	snapshot := &common.StudentLocationSnapshot{
		Attendances: map[int64]*activeService.AttendanceStatus{
			42: {StudentID: 42, Status: "checked_in", CheckInTime: &checkinTime},
		},
		Visits: make(map[int64]*activeModels.Visit),
		Groups: make(map[int64]*activeModels.Group),
	}

	info := snapshot.ResolveStudentLocationWithTime(42, true)

	assert.Equal(t, "Unterwegs", info.Location, "empty Mode must default to detailed-mode rendering")
}
