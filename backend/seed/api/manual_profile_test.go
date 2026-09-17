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

	err := verifyManualStudentRows(rows, SeedExpectedState{PresentStudents: 1, CheckedOutStudents: 1})

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

	err := verifyManualStudentRows(rows, SeedExpectedState{})

	assert.ErrorContains(t, err, `room-tracking location "Raum 101"`)
}
