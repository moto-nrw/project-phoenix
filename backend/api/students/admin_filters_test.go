package students

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
)

func TestApplyAdministrativeFilters(t *testing.T) {
	t.Parallel()

	planningDate := timezone.NewDate(2026, time.June, 1)
	consentYes := true
	consentNo := false
	students := []StudentResponse{
		{
			ID:                      301,
			Bus:                     true,
			PhotoConsentGiven:       &consentYes,
			PickupStatus:            "Geht alleine nach Hause",
			AllowedDepartureModes:   users.AllowedDepartureModes{users.PickupDayMonday: {users.DepartureAlone}},
			DepartureRuleConfigured: true,
			HasFullAccess:           true,
		},
		{
			ID:                      302,
			Bus:                     false,
			PhotoConsentGiven:       &consentNo,
			PickupStatus:            "Wird abgeholt",
			AllowedDepartureModes:   users.AllowedDepartureModes{users.PickupDayMonday: {users.DeparturePickup}},
			DepartureRuleConfigured: true,
			HasFullAccess:           true,
		},
		{
			// Redacted student: never matches an administrative filter (GDPR).
			ID:                      303,
			Bus:                     true,
			PickupStatus:            "Wird abgeholt",
			AllowedDepartureModes:   users.AllowedDepartureModes{users.PickupDayMonday: {users.DeparturePickup}},
			DepartureRuleConfigured: true,
			HasFullAccess:           false,
		},
		{
			// Full access, no pickup rule recorded → kind "none".
			ID:            304,
			Bus:           false,
			HasFullAccess: true,
		},
		{
			// Redacted student that would otherwise match pickup "self":
			// confirms GDPR redaction also gates the pickup filter.
			ID:                      305,
			PickupStatus:            "Geht alleine nach Hause",
			AllowedDepartureModes:   users.AllowedDepartureModes{users.PickupDayMonday: {users.DepartureAlone}},
			DepartureRuleConfigured: true,
			HasFullAccess:           false,
		},
		{
			ID:                      306,
			PickupStatus:            "Taxi mit Begleitperson",
			DepartureRuleConfigured: true,
			HasFullAccess:           true,
		},
	}

	tests := []struct {
		name                        string
		bus, photoConsent, pickupSt string
		wantIDs                     []int64
	}{
		{
			name:    "no filters returns all unchanged",
			wantIDs: []int64{301, 302, 303, 304, 305, 306},
		},
		{
			name:    "all sentinel is treated as off",
			bus:     "all",
			wantIDs: []int64{301, 302, 303, 304, 305, 306},
		},
		{
			name:    "bus yes excludes redacted students",
			bus:     "yes",
			wantIDs: []int64{301},
		},
		{
			name:         "photo_consent no",
			photoConsent: "no",
			wantIDs:      []int64{302},
		},
		{
			name:     "pickup self excludes redacted students",
			pickupSt: "self",
			wantIDs:  []int64{301},
		},
		{
			name:     "pickup none matches students without a recorded rule",
			pickupSt: "none",
			wantIDs:  []int64{304},
		},
		{
			name:     "legacy other rule does not match pickup self or none",
			pickupSt: "other",
			wantIDs:  []int64{306},
		},
		{
			name:         "combined bus + photo_consent + pickup",
			bus:          "no",
			photoConsent: "no",
			pickupSt:     "pickedUp",
			wantIDs:      []int64{302},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyAdministrativeFilters(students, tt.bus, tt.photoConsent, tt.pickupSt, planningDate)
			gotIDs := make([]int64, 0, len(got))
			for _, student := range got {
				gotIDs = append(gotIDs, student.ID)
			}
			assert.Equal(t, tt.wantIDs, gotIDs)
		})
	}
}

func TestParseStudentListParamsReadsAdministrativeFilters(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/students?bus=yes&photo_consent=no&pickup_status=self", nil)
	params := parseStudentListParams(req)

	assert.Equal(t, "yes", params.bus)
	assert.Equal(t, "no", params.photoConsent)
	assert.Equal(t, "self", params.pickupStatus)
	assert.True(t, params.hasAdministrativeFilters())
	// Administrative filters force the in-memory pipeline so pagination/counts
	// reflect the filtered set.
	assert.True(t, params.hasInMemoryFilters())
}

func TestParseStudentListParamsAdministrativeFiltersDefaultOff(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/students?bus=all", nil)
	params := parseStudentListParams(req)

	assert.False(t, params.hasAdministrativeFilters())
	assert.False(t, params.hasInMemoryFilters())
}
