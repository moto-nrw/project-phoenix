package active

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestWithYardRoomColor pins the binary-mode yard tinting introduced by
// #2405: the Schulhof room's colour reaches the badge, and only the yard
// state adopts it.
//
// Pure unit test — the resolver's own DB path is exercised through the
// snapshot integration tests; what needs pinning here is which of the three
// binary labels may carry a room colour.
func TestWithYardRoomColor(t *testing.T) {
	yard := "#A3D977"

	t.Run("stamps the color on the Schulhof label", func(t *testing.T) {
		out := withYardRoomColor(StudentLocationInfo{Location: YardLocationLabel}, &yard)
		if assert.NotNil(t, out.RoomColor) {
			assert.Equal(t, yard, *out.RoomColor)
		}
	})

	t.Run("leaves Anwesend untouched", func(t *testing.T) {
		// Binary mode has no room behind "Anwesend"; tinting it would colour
		// an unrelated badge with the yard's hex.
		out := withYardRoomColor(StudentLocationInfo{Location: "Anwesend"}, &yard)
		assert.Nil(t, out.RoomColor)
	})

	t.Run("leaves Abwesend untouched", func(t *testing.T) {
		out := withYardRoomColor(StudentLocationInfo{Location: "Abwesend"}, &yard)
		assert.Nil(t, out.RoomColor)
	})

	t.Run("passes through when no color is configured", func(t *testing.T) {
		// nil means "no colour set" — the frontend then renders the orange
		// Schulhof default.
		out := withYardRoomColor(StudentLocationInfo{Location: YardLocationLabel}, nil)
		assert.Nil(t, out.RoomColor)
	})
}

// TestResolveYardRoomColorWithoutCapability guards the optional-interface
// wiring: a Service implementation that predates YardRoomColorResolver must
// degrade to "no colour", never panic.
func TestResolveYardRoomColorWithoutCapability(t *testing.T) {
	assert.Nil(t, ResolveYardRoomColor(t.Context(), nil))
}

// TestSnapshotBinaryYardColor covers the snapshot resolver end of the wiring
// without touching the database.
func TestSnapshotBinaryYardColor(t *testing.T) {
	yard := "#A3D977"
	const studentID int64 = 42

	snapshot := &StudentLocationSnapshot{
		Mode: PresenceModeBinary,
		Attendances: map[int64]*AttendanceStatus{
			studentID: {Status: "on_yard"},
		},
		YardRoomColor: &yard,
	}

	info := snapshot.ResolveStudentLocationWithTime(studentID, true)
	assert.Equal(t, YardLocationLabel, info.Location)
	if assert.NotNil(t, info.RoomColor) {
		assert.Equal(t, yard, *info.RoomColor)
	}

	// Same snapshot, a checked-in student: no room, no colour.
	snapshot.Attendances[studentID] = &AttendanceStatus{Status: "checked_in"}
	assert.Nil(t, snapshot.ResolveStudentLocationWithTime(studentID, true).RoomColor)
}
