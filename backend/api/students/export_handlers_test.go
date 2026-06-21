package students

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExportRequestToListParamsPreservesRoomFilter(t *testing.T) {
	params := exportRequestToListParams(studentExportRequest{
		Filters: studentExportFilters{
			Search:  "  mila  ",
			GroupID: "17",
			RoomID:  "42",
		},
	})

	assert.Equal(t, "mila", params.search)
	assert.Equal(t, int64(17), params.groupID)
	assert.Equal(t, int64(42), params.roomID)
	assert.Equal(t, studentExportPageSize, params.pageSize)
	assert.True(t, params.includePickupTimes)
	assert.True(t, params.includeArrivalTimes)
}

func TestApplyExportFiltersAdministrativeFilters(t *testing.T) {
	consentYes := true
	consentNo := false
	students := []StudentResponse{
		{
			ID:                101,
			SchoolClass:       "Klasse 1a",
			Bus:               true,
			PhotoConsentGiven: &consentYes,
			PickupStatus:      "Geht alleine nach Hause",
			HasFullAccess:     true,
		},
		{
			ID:                102,
			SchoolClass:       "Klasse 2a",
			Bus:               false,
			PhotoConsentGiven: &consentNo,
			PickupStatus:      "Wird abgeholt",
			HasFullAccess:     true,
		},
		{
			ID:            103,
			SchoolClass:   "Klasse 3a",
			Bus:           true,
			PickupStatus:  "Wird abgeholt",
			HasFullAccess: false,
		},
	}

	tests := []struct {
		name    string
		filters studentExportFilters
		wantIDs []int64
	}{
		{
			name:    "bus yes excludes redacted students",
			filters: studentExportFilters{Bus: "yes"},
			wantIDs: []int64{101},
		},
		{
			name:    "photo consent no",
			filters: studentExportFilters{PhotoConsent: "no"},
			wantIDs: []int64{102},
		},
		{
			name:    "pickup self",
			filters: studentExportFilters{PickupStatus: "self"},
			wantIDs: []int64{101},
		},
		{
			name:    "combined filters",
			filters: studentExportFilters{Bus: "no", PhotoConsent: "no", PickupStatus: "pickedUp"},
			wantIDs: []int64{102},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyExportFilters(students, tt.filters)
			gotIDs := make([]int64, 0, len(got))
			for _, student := range got {
				gotIDs = append(gotIDs, student.ID)
			}
			assert.Equal(t, tt.wantIDs, gotIDs)
		})
	}
}

func TestApplyExportFiltersClassTripStatus(t *testing.T) {
	students := []StudentResponse{
		{ID: 101, ClassTrip: true},
		{ID: 102, Excused: true},
		{ID: 103, Sick: true},
		{ID: 104, Location: "Zuhause"},
	}

	got := applyExportFilters(students, studentExportFilters{Status: "klassenfahrt"})

	require.Len(t, got, 1)
	assert.Equal(t, int64(101), got[0].ID)
}

func TestPopulateExportPhotoConsentFilterDataSupportsFeatureOffResponses(t *testing.T) {
	now := time.Now()
	responses := []StudentResponse{
		{ID: 101, HasFullAccess: true},
		{ID: 102, HasFullAccess: true},
	}
	students := []*users.Student{
		{Model: base.Model{ID: 101}, PhotoConsentGivenAt: &now},
		{Model: base.Model{ID: 102}},
	}

	populateExportPhotoConsentFilterData(responses, students)

	require.NotNil(t, responses[0].PhotoConsentGiven)
	assert.True(t, *responses[0].PhotoConsentGiven)
	require.NotNil(t, responses[1].PhotoConsentGiven)
	assert.False(t, *responses[1].PhotoConsentGiven)

	yes := applyExportFilters(responses, studentExportFilters{PhotoConsent: "yes"})
	require.Len(t, yes, 1)
	assert.Equal(t, int64(101), yes[0].ID)

	no := applyExportFilters(responses, studentExportFilters{PhotoConsent: "no"})
	require.Len(t, no, 1)
	assert.Equal(t, int64(102), no[0].ID)
}

func TestApplyExportFiltersCombinedWithDayStatus(t *testing.T) {
	consentYes := true
	consentNo := false
	// 201/204 come today; 202/203 are planned absent (krank/entschuldigt → not_coming_today).
	students := []StudentResponse{
		{
			ID:                201,
			Bus:               true,
			PhotoConsentGiven: &consentYes,
			PickupStatus:      "Geht alleine nach Hause",
			DayPlanningStatus: DayPlanningStatusComesToday,
			HasFullAccess:     true,
		},
		{
			ID:                202,
			Bus:               false,
			PhotoConsentGiven: &consentNo,
			PickupStatus:      "Wird abgeholt",
			DayPlanningStatus: DayPlanningStatusNotComingToday,
			HasFullAccess:     true,
		},
		{
			ID:                203,
			Bus:               true,
			PhotoConsentGiven: &consentYes,
			PickupStatus:      "Wird abgeholt",
			DayPlanningStatus: DayPlanningStatusNotComingToday,
			HasFullAccess:     true,
		},
		{
			ID:                204,
			Bus:               true,
			PhotoConsentGiven: &consentYes,
			PickupStatus:      "Geht alleine nach Hause",
			DayPlanningStatus: DayPlanningStatusComesToday,
			HasFullAccess:     true,
		},
	}

	tests := []struct {
		name    string
		filters studentExportFilters
		wantIDs []int64
	}{
		{
			name:    "day_status comes_today only",
			filters: studentExportFilters{DayStatus: DayPlanningStatusComesToday},
			wantIDs: []int64{201, 204},
		},
		{
			name:    "day_status not_coming_today keeps planned krank/entschuldigt",
			filters: studentExportFilters{DayStatus: DayPlanningStatusNotComingToday},
			wantIDs: []int64{202, 203},
		},
		{
			name:    "day_status comes_today AND bus yes",
			filters: studentExportFilters{DayStatus: DayPlanningStatusComesToday, Bus: "yes"},
			wantIDs: []int64{201, 204},
		},
		{
			name:    "day_status not_coming_today AND photo_consent yes",
			filters: studentExportFilters{DayStatus: DayPlanningStatusNotComingToday, PhotoConsent: "yes"},
			wantIDs: []int64{203},
		},
		{
			name:    "day_status comes_today AND pickup_status self",
			filters: studentExportFilters{DayStatus: DayPlanningStatusComesToday, PickupStatus: "self"},
			wantIDs: []int64{201, 204},
		},
		{
			name:    "day_status all keeps every student",
			filters: studentExportFilters{DayStatus: DayPlanningStatusAll},
			wantIDs: []int64{201, 202, 203, 204},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyExportFilters(students, tt.filters)
			gotIDs := make([]int64, 0, len(got))
			for _, student := range got {
				gotIDs = append(gotIDs, student.ID)
			}
			assert.Equal(t, tt.wantIDs, gotIDs)
		})
	}
}

func TestExportRequestToListParamsParsesDayStatus(t *testing.T) {
	params := exportRequestToListParams(studentExportRequest{
		Filters: studentExportFilters{DayStatus: "not_coming_today"},
	})
	assert.Equal(t, "not_coming_today", params.dayStatus)

	// Administrative filters are applied client-side, so they must NOT leak
	// into the backend list query params.
	adminOnly := exportRequestToListParams(studentExportRequest{
		Filters: studentExportFilters{Bus: "yes", PhotoConsent: "no", PickupStatus: "self"},
	})
	assert.Equal(t, DayPlanningStatusAll, adminOnly.dayStatus)
}

func TestExportFilterLabelsCombinesDayStatusAndAdministrative(t *testing.T) {
	labels := exportFilterLabels(studentExportFilters{
		Bus:          "yes",
		PhotoConsent: "no",
		PickupStatus: "self",
		Status:       "klassenfahrt",
		DayStatus:    DayPlanningStatusNotComingToday,
	})

	assert.Contains(t, labels, "Buskind")
	assert.Contains(t, labels, "Keine Fotoerlaubnis")
	assert.Contains(t, labels, "Abholregelung: Geht alleine nach Hause")
	assert.Contains(t, labels, "Momentaufnahme: Klassenfahrt")
	assert.Contains(t, labels, "Tagesplanung: Kommt heute nicht")
}

func TestWeeklyCellUsesExplicitLabels(t *testing.T) {
	tests := []struct {
		name string
		plan weeklySchedule
		want string
	}{
		{
			name: "arrival and pickup",
			plan: weeklySchedule{
				ArrivalByWeekday: map[int]string{schedule.WeekdayMonday: "08:00"},
				PickupByWeekday:  map[int]string{schedule.WeekdayMonday: "16:00"},
			},
			want: "Ankunft: 08:00, Abholung: 16:00",
		},
		{
			name: "pickup only",
			plan: weeklySchedule{
				ArrivalByWeekday: map[int]string{},
				PickupByWeekday:  map[int]string{schedule.WeekdayMonday: "16:00"},
			},
			want: "Abholung: 16:00",
		},
		{
			name: "arrival only",
			plan: weeklySchedule{
				ArrivalByWeekday: map[int]string{schedule.WeekdayMonday: "08:00"},
				PickupByWeekday:  map[int]string{},
			},
			want: "Ankunft: 08:00",
		},
		{
			name: "no plan",
			plan: weeklySchedule{
				ArrivalByWeekday: map[int]string{},
				PickupByWeekday:  map[int]string{},
			},
			want: "nein",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := weeklyCell(tt.plan, schedule.WeekdayMonday); got != tt.want {
				t.Fatalf("weeklyCell() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestDepartureExportCell pins the accompanied companion-note rendering in the
// student export (#1694): the "mit wem" note is appended only when the resolved
// plan actually allows the accompanied ("Mit anderem Kind") mode.
func TestDepartureExportCell(t *testing.T) {
	t.Run("appends companion note for an accompanied day", func(t *testing.T) {
		got := departureExportCell(StudentResponse{
			AllowedDepartureModes: users.AllowedDepartureModes{
				users.PickupDayMonday: []users.DepartureMode{users.DepartureAccompanied},
			},
			DepartureCompanionNote: "Geschwisterkind Lena",
		})
		require.Equal(t, "Mo: Mit anderem Kind (mit: Geschwisterkind Lena)", got)
	})

	t.Run("ignores a stray note when no day is accompanied", func(t *testing.T) {
		got := departureExportCell(StudentResponse{
			AllowedDepartureModes: users.AllowedDepartureModes{
				users.PickupDayMonday: []users.DepartureMode{users.DepartureBus},
			},
			DepartureCompanionNote: "Geschwisterkind Lena",
		})
		require.Equal(t, "Mo: Bus", got)
	})

	t.Run("falls back to the legacy departure days for the accompanied check", func(t *testing.T) {
		got := departureExportCell(StudentResponse{
			DepartureDays: users.DepartureDays{
				users.PickupDayTuesday: users.DepartureAccompanied,
			},
			DepartureCompanionNote: "Nachbarskind Tom",
		})
		require.Equal(t, "Di: Mit anderem Kind (mit: Nachbarskind Tom)", got)
	})

	t.Run("no note renders the plain summary", func(t *testing.T) {
		got := departureExportCell(StudentResponse{
			AllowedDepartureModes: users.AllowedDepartureModes{
				users.PickupDayMonday: []users.DepartureMode{users.DepartureAccompanied},
			},
		})
		require.Equal(t, "Mo: Mit anderem Kind", got)
	})
}

func TestDepartureSummary(t *testing.T) {
	tests := []struct {
		name    string
		allowed users.AllowedDepartureModes
		days    users.DepartureDays
		want    string
	}{
		{
			name: "mixed modes in week order, alone days skipped",
			days: users.DepartureDays{
				users.PickupDayMonday:    users.DepartureBus,
				users.PickupDayWednesday: users.DeparturePickup,
				users.PickupDayThursday:  users.DepartureAlone,
			},
			want: "Mo: Bus, Mi: Abholung",
		},
		{
			name: "allowed modes render multiple values per day",
			allowed: users.AllowedDepartureModes{
				users.PickupDayMonday:  []users.DepartureMode{users.DepartureBus, users.DeparturePickup},
				users.PickupDayTuesday: []users.DepartureMode{users.DepartureAlone},
			},
			want: "Mo: Bus, Abholung, Di: zu Fuß",
		},
		{name: "empty plan", days: users.DepartureDays{}, want: "Geht alleine"},
		{name: "nil plan", days: nil, want: "Geht alleine"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := departureSummary(tt.allowed, tt.days); got != tt.want {
				t.Fatalf("departureSummary() = %q, want %q", got, tt.want)
			}
		})
	}
}
