package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerifyManualStudentRowsAcceptsAtSchoolBeforeCheckIn(t *testing.T) {
	t.Parallel()
	pickup := "2026-09-17T15:00:00+02:00"
	raw := []byte(`[
		{"id": 1, "school_class": "1a", "group_id": 7, "current_location": "Anwesend"},
		{"id": 2, "school_class": "1a", "group_id": 7, "current_location": "Abwesend", "actual_pickup_time": "` + pickup + `"},
		{"id": 3, "school_class": "1a", "group_id": 7, "current_location": "Schule"},
		{"id": 4, "school_class": "1a", "group_id": 7, "current_location": "Abwesend"}
	]`)
	var rows []struct {
		ID             int64   `json:"id"`
		SchoolClass    string  `json:"school_class"`
		GroupID        int64   `json:"group_id"`
		Location       string  `json:"current_location"`
		ActualPickupAt *string `json:"actual_pickup_time"`
	}
	require.NoError(t, json.Unmarshal(raw, &rows))

	err := verifyManualStudentRows(rows, SeedExpectedState{PresentStudents: 1, CheckedOutStudents: 1}, false)

	assert.NoError(t, err)
}

func TestVerifyManualStudentRowsRejectsRoomLocation(t *testing.T) {
	t.Parallel()
	raw := []byte(`[{"id": 5, "school_class": "1a", "group_id": 7, "current_location": "Raum 101"}]`)
	var rows []struct {
		ID             int64   `json:"id"`
		SchoolClass    string  `json:"school_class"`
		GroupID        int64   `json:"group_id"`
		Location       string  `json:"current_location"`
		ActualPickupAt *string `json:"actual_pickup_time"`
	}
	require.NoError(t, json.Unmarshal(raw, &rows))

	err := verifyManualStudentRows(rows, SeedExpectedState{}, false)

	assert.ErrorContains(t, err, `room-tracking location "Raum 101"`)
}

// With room tracking (marketing profile, presence mode "detailed") a child in
// a running supervision counts as present; a binary profile still rejects it.
func TestVerifyManualStudentRowsCountsRoomsOnlyWithRoomTracking(t *testing.T) {
	t.Parallel()
	raw := []byte(`[{"id": 6, "school_class": "1a", "group_id": 7, "current_location": "Anwesend - Bauraum"}]`)
	var rows []struct {
		ID             int64   `json:"id"`
		SchoolClass    string  `json:"school_class"`
		GroupID        int64   `json:"group_id"`
		Location       string  `json:"current_location"`
		ActualPickupAt *string `json:"actual_pickup_time"`
	}
	require.NoError(t, json.Unmarshal(raw, &rows))

	require.NoError(t, verifyManualStudentRows(rows, SeedExpectedState{PresentStudents: 1}, true))
	assert.ErrorContains(t, verifyManualStudentRows(rows, SeedExpectedState{PresentStudents: 1}, false), `room-tracking location "Anwesend - Bauraum"`)
}

func TestVerifyManualStudentRowsCountsTransitOnlyWithRoomTracking(t *testing.T) {
	t.Parallel()
	raw := []byte(`[{"id": 7, "school_class": "1a", "group_id": 7, "current_location": "Unterwegs"}]`)
	var rows []struct {
		ID             int64   `json:"id"`
		SchoolClass    string  `json:"school_class"`
		GroupID        int64   `json:"group_id"`
		Location       string  `json:"current_location"`
		ActualPickupAt *string `json:"actual_pickup_time"`
	}
	require.NoError(t, json.Unmarshal(raw, &rows))

	require.NoError(t, verifyManualStudentRows(rows, SeedExpectedState{PresentStudents: 1}, true))
	assert.Error(t, verifyManualStudentRows(rows, SeedExpectedState{PresentStudents: 1}, false))
}
