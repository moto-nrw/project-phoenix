package students

import (
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// fullyPopulatedStudentResponse sets every field of the full projection to a
// non-zero value, so `omitempty` cannot hide a field the slim projection
// wrongly keeps (or wrongly drops).
func fullyPopulatedStudentResponse() StudentResponse {
	now := time.Now()
	str := func(s string) *string { return &s }
	id := int64(4711)
	yes := true

	return StudentResponse{
		ID:                       id,
		PersonID:                 id + 1,
		FirstName:                "Emma",
		LastName:                 "Meyer",
		TagID:                    "TAG-1",
		Birthday:                 "2019-02-02",
		SchoolClass:              "1a",
		Location:                 "Gruppenraum",
		LocationSince:            &now,
		RoomColor:                str("#83CD2D"),
		GuardianName:             "Erika Mustermann",
		GuardianContact:          "0221 1234567",
		GuardianEmail:            "erika@example.com",
		GuardianPhone:            "+49 170 1234567",
		GroupID:                  id + 2,
		GroupName:                "Sternengruppe",
		AddressStreet:            "Musterstrasse 12a",
		AddressCity:              "Koeln",
		AddressPostalCode:        "50667",
		ExtraInfo:                "Geschwisterkind",
		HealthInfo:               "Laktoseintoleranz",
		SupervisorNotes:          "Braucht mehr Zeit",
		PickupStatus:             "Wird abgeholt",
		PickupDays:               users.PickupDays{"mon": true},
		PickupTime:               str("15:30"),
		PickupIsException:        true,
		PickupNotes:              "Oma holt ab",
		ArrivalTime:              str("08:00"),
		ArrivalIsException:       true,
		ArrivalNotes:             "Arzttermin",
		ActualArrivalTime:        str("08:05"),
		ActualPickupTime:         str("15:35"),
		DepartureDays:            users.DepartureDays{"mon": "pickup"},
		AllowedDepartureModes:    users.AllowedDepartureModes{"mon": {"pickup"}},
		DepartureRuleConfigured:  true,
		DepartureCompanionNote:   "geht mit Nachbarskind",
		CompanionStudentIDs:      []int64{id + 3},
		CompanionsChanged:        &yes,
		Bus:                      true,
		BusDays:                  users.BusDays{"mon": true},
		Sick:                     true,
		SickSince:                &now,
		Excused:                  true,
		ExcusedSince:             &now,
		ClassTrip:                true,
		ClassTripSince:           &now,
		DayPlanningStatus:        "comes_today",
		DayPlanningReason:        "care_day",
		DayPlanningLabel:         "kommt heute",
		PendingExcusedNote:       str("Arzttermin"),
		PhotoURL:                 "/api/students/4711/photo",
		PhotoConsentGiven:        &yes,
		PhotoConsentGivenAt:      &now,
		PhotoConsentGivenBy:      &id,
		AGBAcceptedAt:            &now,
		DataProcessingAcceptedAt: &now,
		EmailContactAcceptedAt:   &now,
		HasFullAccess:            true,
		CreatedAt:                now,
		UpdatedAt:                now,
	}
}

// TestSlimStudentResponseWireContract pins the exact key set the Kindersuche
// projection puts on the wire (#2097). It fails both ways: a field silently
// re-added (payload creep) and a field the page reads silently dropped.
func TestSlimStudentResponseWireContract(t *testing.T) {
	t.Parallel()

	slim := slimStudentResponses([]StudentResponse{fullyPopulatedStudentResponse()}, timezone.NewDate(2026, time.June, 1))
	require.Len(t, slim, 1)

	encoded, err := json.Marshal(slim[0])
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(encoded, &decoded))

	keys := make([]string, 0, len(decoded))
	for key := range decoded {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	want := []string{
		"actual_arrival_time",
		"actual_pickup_time",
		"arrival_is_exception",
		"arrival_notes",
		"arrival_time",
		"class_trip",
		"class_trip_since",
		"companion_student_ids",
		"current_location",
		"current_room_color",
		"day_planning_label",
		"day_planning_status",
		"departure_modes",
		"excused",
		"excused_since",
		"first_name",
		"group_id",
		"group_name",
		"has_full_access",
		"id",
		"last_name",
		"location_since",
		"pending_excused_note",
		"photo_consent_given",
		"photo_url",
		"pickup_is_exception",
		"pickup_notes",
		"pickup_time",
		"school_class",
		"sick",
		"sick_since",
	}
	assert.Equal(t, want, keys,
		"the Kindersuche wire contract changed — update the page and this list together (#2097)")
}

// TestSlimStudentResponseCopiesValues guards against a copy-paste slip in the
// projection: every kept field must carry the full response's value, not a
// zero value or a neighbouring field.
func TestSlimStudentResponseCopiesValues(t *testing.T) {
	t.Parallel()

	full := fullyPopulatedStudentResponse()
	slim := slimStudentResponses([]StudentResponse{full}, timezone.NewDate(2026, time.June, 1))
	require.Len(t, slim, 1)
	got := slim[0]

	assert.Equal(t, full.ID, got.ID)
	assert.Equal(t, full.FirstName, got.FirstName)
	assert.Equal(t, full.LastName, got.LastName)
	assert.Equal(t, full.SchoolClass, got.SchoolClass)
	assert.Equal(t, full.Location, got.Location)
	assert.Equal(t, full.LocationSince, got.LocationSince)
	assert.Equal(t, full.RoomColor, got.RoomColor)
	assert.Equal(t, full.GroupID, got.GroupID)
	assert.Equal(t, full.GroupName, got.GroupName)
	assert.Equal(t, full.Sick, got.Sick)
	assert.Equal(t, full.SickSince, got.SickSince)
	assert.Equal(t, full.Excused, got.Excused)
	assert.Equal(t, full.ExcusedSince, got.ExcusedSince)
	assert.Equal(t, full.ClassTrip, got.ClassTrip)
	assert.Equal(t, full.ClassTripSince, got.ClassTripSince)
	assert.Equal(t, full.DayPlanningStatus, got.DayPlanningStatus)
	assert.Equal(t, full.DayPlanningLabel, got.DayPlanningLabel)
	assert.Equal(t, full.PendingExcusedNote, got.PendingExcusedNote)
	assert.Equal(t, []users.DepartureMode{users.DeparturePickup}, got.DepartureModes)
	assert.Equal(t, full.PickupTime, got.PickupTime)
	assert.Equal(t, full.PickupIsException, got.PickupIsException)
	assert.Equal(t, full.PickupNotes, got.PickupNotes)
	assert.Equal(t, full.ArrivalTime, got.ArrivalTime)
	assert.Equal(t, full.ArrivalIsException, got.ArrivalIsException)
	assert.Equal(t, full.ArrivalNotes, got.ArrivalNotes)
	assert.Equal(t, full.ActualArrivalTime, got.ActualArrivalTime)
	assert.Equal(t, full.ActualPickupTime, got.ActualPickupTime)
	assert.Equal(t, full.CompanionStudentIDs, got.CompanionStudentIDs)
	assert.Equal(t, full.PhotoURL, got.PhotoURL)
	assert.Equal(t, full.PhotoConsentGiven, got.PhotoConsentGiven)
	assert.Equal(t, full.HasFullAccess, got.HasFullAccess)
}

func TestSlimStudentResponsePreservesUnknownLegacyDepartureLabel(t *testing.T) {
	t.Parallel()

	full := StudentResponse{
		ID:                      7,
		PickupStatus:            "Taxi mit Begleitperson",
		DepartureRuleConfigured: true,
		HasFullAccess:           true,
	}

	slim := slimStudentResponses([]StudentResponse{full}, timezone.NewDate(2026, time.June, 1))
	require.Len(t, slim, 1)
	assert.Empty(t, slim[0].DepartureModes)
	assert.Equal(t, "Taxi mit Begleitperson", slim[0].DepartureLabel)
}

// TestParseStudentListView covers the query-parameter contract.
func TestParseStudentListView(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		value   string
		want    bool
		wantErr bool
	}{
		{value: "", want: false},
		{value: StudentListViewFull, want: false},
		{value: StudentListViewSlim, want: true},
		{value: "compact", wantErr: true},
		{value: "SLIM", wantErr: true},
	} {
		got, err := parseStudentListView(tc.value)
		if tc.wantErr {
			assert.Error(t, err, "view %q must be rejected", tc.value)
			continue
		}
		require.NoError(t, err, "view %q", tc.value)
		assert.Equal(t, tc.want, got, "view %q", tc.value)
	}
}
