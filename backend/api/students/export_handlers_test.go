package students

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testExportDate is the fixed "today" row building is evaluated against, so
// age cells stay deterministic instead of drifting with the wall clock.
var testExportDate = timezone.NewDate(2026, time.July, 15)

func TestExportRequestToListParamsPreservesRoomFilter(t *testing.T) {
	t.Parallel()

	params := exportRequestToListParams(studentExportRequest{
		Filters: studentExportFilters{
			Search:      "  mila  ",
			GroupID:     "17",
			RoomID:      "42",
			SchoolClass: "3a",
		},
	})

	assert.Equal(t, "mila", params.search)
	assert.Equal(t, []int64{17}, params.groupIDs)
	assert.Equal(t, int64(42), params.roomID)
	assert.Equal(t, []string{"3a"}, params.schoolClasses)
	assert.Equal(t, studentExportPageSize, params.pageSize)
	assert.True(t, params.includePickupTimes)
	assert.True(t, params.includeArrivalTimes)
}

func TestExportSelectionTooLarge(t *testing.T) {
	t.Parallel()

	// The cap is inclusive: a full page still exports, one over does not.
	assert.False(t, exportSelectionTooLarge(0))
	assert.False(t, exportSelectionTooLarge(studentExportPageSize-1))
	assert.False(t, exportSelectionTooLarge(studentExportPageSize))
	assert.True(t, exportSelectionTooLarge(studentExportPageSize+1))

	// The message names the actual count and the cap so a school knows how far
	// over it is and what to do about it.
	msg := errExportSelectionTooLarge(studentExportPageSize + 1).Error()
	assert.Contains(t, msg, "5001")
	assert.Contains(t, msg, "eingrenzen")
}

// The export must fetch every SQL-matching row even when no in-memory filter is
// active (the whole-school birthday list): the month filter runs after the
// fetch, so a paginated page would silently drop matching children past the
// boundary. buildQueryOptions must therefore leave pagination off.
func TestExportRequestToListParamsFetchesAllRows(t *testing.T) {
	t.Parallel()

	params := exportRequestToListParams(studentExportRequest{
		Preset: listexport.PresetBirthdayList,
		// No search / group / class: without fetchAll this would paginate.
		Filters: studentExportFilters{Months: []string{"09"}},
	})

	assert.True(t, params.fetchAll)
	assert.False(t, params.hasInMemoryFilters(),
		"a month-only birthday export has no in-memory list filter, so only fetchAll keeps pagination off")
	assert.Nil(t, params.buildQueryOptions().Pagination,
		"the export query must be unpaginated so the in-memory month filter sees every child")
}

// The cap is decided on the filtered result, so exportSelectionCapError returns
// nil for anything that fits and a ready-to-render error only once the document
// would exceed the row cap.
func TestExportSelectionCapError(t *testing.T) {
	t.Parallel()

	assert.Nil(t, exportSelectionCapError(0))
	assert.Nil(t, exportSelectionCapError(studentExportPageSize))
	require.NotNil(t, exportSelectionCapError(studentExportPageSize+1))
}

func TestApplyExportFiltersAdministrativeFilters(t *testing.T) {
	t.Parallel()

	consentYes := true
	consentNo := false
	students := []StudentResponse{
		{
			ID:                      101,
			SchoolClass:             "Klasse 1a",
			Bus:                     true,
			PhotoConsentGiven:       &consentYes,
			PickupStatus:            "Geht alleine nach Hause",
			AllowedDepartureModes:   users.AllowedDepartureModes{users.PickupDayWednesday: {users.DepartureAlone}},
			DepartureRuleConfigured: true,
			HasFullAccess:           true,
		},
		{
			ID:                      102,
			SchoolClass:             "Klasse 2a",
			Bus:                     false,
			PhotoConsentGiven:       &consentNo,
			PickupStatus:            "Wird abgeholt",
			AllowedDepartureModes:   users.AllowedDepartureModes{users.PickupDayWednesday: {users.DeparturePickup}},
			DepartureRuleConfigured: true,
			HasFullAccess:           true,
		},
		{
			ID:                      103,
			SchoolClass:             "Klasse 3a",
			Bus:                     true,
			PickupStatus:            "Wird abgeholt",
			AllowedDepartureModes:   users.AllowedDepartureModes{users.PickupDayWednesday: {users.DeparturePickup}},
			DepartureRuleConfigured: true,
			HasFullAccess:           false,
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
			got := applyExportFilters(students, tt.filters, listexport.PresetOGSWeekly, testExportDate)
			gotIDs := make([]int64, 0, len(got))
			for _, student := range got {
				gotIDs = append(gotIDs, student.ID)
			}
			assert.Equal(t, tt.wantIDs, gotIDs)
		})
	}
}

func TestApplyExportFiltersClassTripStatus(t *testing.T) {
	t.Parallel()

	students := []StudentResponse{
		{ID: 101, ClassTrip: true},
		{ID: 102, Excused: true},
		{ID: 103, Sick: true},
		{ID: 104, Location: "Zuhause"},
	}

	got := applyExportFilters(students, studentExportFilters{Status: "klassenfahrt"}, listexport.PresetOGSWeekly, testExportDate)

	require.Len(t, got, 1)
	assert.Equal(t, int64(101), got[0].ID)
}

func TestPopulateExportPhotoConsentFilterDataSupportsFeatureOffResponses(t *testing.T) {
	t.Parallel()

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

	yes := applyExportFilters(responses, studentExportFilters{PhotoConsent: "yes"}, listexport.PresetOGSWeekly, testExportDate)
	require.Len(t, yes, 1)
	assert.Equal(t, int64(101), yes[0].ID)

	no := applyExportFilters(responses, studentExportFilters{PhotoConsent: "no"}, listexport.PresetOGSWeekly, testExportDate)
	require.Len(t, no, 1)
	assert.Equal(t, int64(102), no[0].ID)
}

func TestApplyExportFiltersCombinedWithDayStatus(t *testing.T) {
	t.Parallel()

	consentYes := true
	consentNo := false
	// 201/204 come today; 202/203 are planned absent (krank/entschuldigt → not_coming_today).
	students := []StudentResponse{
		{
			ID:                      201,
			Bus:                     true,
			PhotoConsentGiven:       &consentYes,
			PickupStatus:            "Geht alleine nach Hause",
			AllowedDepartureModes:   users.AllowedDepartureModes{users.PickupDayWednesday: {users.DepartureAlone}},
			DepartureRuleConfigured: true,
			DayPlanningStatus:       DayPlanningStatusComesToday,
			HasFullAccess:           true,
		},
		{
			ID:                      202,
			Bus:                     false,
			PhotoConsentGiven:       &consentNo,
			PickupStatus:            "Wird abgeholt",
			AllowedDepartureModes:   users.AllowedDepartureModes{users.PickupDayWednesday: {users.DeparturePickup}},
			DepartureRuleConfigured: true,
			DayPlanningStatus:       DayPlanningStatusNotComingToday,
			HasFullAccess:           true,
		},
		{
			ID:                      203,
			Bus:                     true,
			PhotoConsentGiven:       &consentYes,
			PickupStatus:            "Wird abgeholt",
			AllowedDepartureModes:   users.AllowedDepartureModes{users.PickupDayWednesday: {users.DeparturePickup}},
			DepartureRuleConfigured: true,
			DayPlanningStatus:       DayPlanningStatusNotComingToday,
			HasFullAccess:           true,
		},
		{
			ID:                      204,
			Bus:                     true,
			PhotoConsentGiven:       &consentYes,
			PickupStatus:            "Geht alleine nach Hause",
			AllowedDepartureModes:   users.AllowedDepartureModes{users.PickupDayWednesday: {users.DepartureAlone}},
			DepartureRuleConfigured: true,
			DayPlanningStatus:       DayPlanningStatusComesToday,
			HasFullAccess:           true,
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
			got := applyExportFilters(students, tt.filters, listexport.PresetOGSWeekly, testExportDate)
			gotIDs := make([]int64, 0, len(got))
			for _, student := range got {
				gotIDs = append(gotIDs, student.ID)
			}
			assert.Equal(t, tt.wantIDs, gotIDs)
		})
	}
}

func TestExportRequestToListParamsParsesDayStatus(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	labels := exportFilterLabels(studentExportFilters{
		Bus:          "yes",
		PhotoConsent: "no",
		PickupStatus: "self",
		Status:       "klassenfahrt",
		DayStatus:    DayPlanningStatusNotComingToday,
		SchoolClass:  "3a",
	})

	assert.Contains(t, labels, "Klasse: 3a")
	assert.Contains(t, labels, "Buskind")
	assert.Contains(t, labels, "Keine Fotoerlaubnis")
	assert.Contains(t, labels, "Abholregelung: Geht alleine nach Hause")
	assert.Contains(t, labels, "Momentaufnahme: Klassenfahrt")
	assert.Contains(t, labels, "Tagesplanung: Kommt heute nicht")
}

// #2218: an export started from the Kindersuche inherits the page's filters,
// so a multi-class / multi-group selection must survive the trip into the list
// query instead of collapsing to its first value.
func TestExportRequestToListParamsAcceptsMultipleClassesAndGroups(t *testing.T) {
	t.Parallel()

	params := exportRequestToListParams(studentExportRequest{
		Filters: studentExportFilters{
			SchoolClass: "3a, 4b",
			GroupID:     "17,19",
		},
	})

	assert.Equal(t, []string{"3a", "4b"}, params.schoolClasses)
	assert.Equal(t, []int64{17, 19}, params.groupIDs)
}

// The school-year filter runs in memory over the fetched rows, so it needs the
// same multi-value semantics — and the printed header must name every selected
// year rather than only the first (#2218).
func TestApplyExportFiltersMultipleSchoolYears(t *testing.T) {
	t.Parallel()

	students := []StudentResponse{
		{ID: 201, SchoolClass: "2a"},
		{ID: 202, SchoolClass: "3a"},
		{ID: 203, SchoolClass: "Klasse 4b"},
	}

	got := applyExportFilters(students, studentExportFilters{Year: "3,4"}, listexport.PresetOGSWeekly, testExportDate)

	gotIDs := make([]int64, 0, len(got))
	for _, student := range got {
		gotIDs = append(gotIDs, student.ID)
	}
	assert.Equal(t, []int64{202, 203}, gotIDs)

	labels := exportFilterLabels(studentExportFilters{Year: "3,4", SchoolClass: "3a,4b"})
	assert.Contains(t, labels, "Stufe: 3, 4")
	assert.Contains(t, labels, "Klasse: 3a, 4b")
}

func TestWeeklyCellUsesExplicitLabels(t *testing.T) {
	t.Parallel()

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

func TestBuildExportRowsIncludesDailyStatus(t *testing.T) {
	t.Parallel()

	students := []StudentResponse{
		{
			ID:                101,
			FirstName:         "Mila",
			LastName:          "Muster",
			DayPlanningStatus: DayPlanningStatusComesToday,
			DayPlanningReason: dayPlanningReasonUnplanned,
			Sick:              true,
		},
		{
			ID:                102,
			FirstName:         "Tom",
			LastName:          "Krank",
			DayPlanningStatus: DayPlanningStatusNotComingToday,
			DayPlanningReason: dayPlanningReasonSick,
		},
		{
			ID:                103,
			FirstName:         "Ella",
			LastName:          "Entschuldigt",
			DayPlanningStatus: DayPlanningStatusNotComingToday,
			DayPlanningReason: dayPlanningReasonExcused,
		},
		{
			ID:                104,
			FirstName:         "Klara",
			LastName:          "Fahrt",
			DayPlanningStatus: DayPlanningStatusNotComingToday,
			DayPlanningReason: dayPlanningReasonClassTrip,
		},
		{
			ID:                105,
			FirstName:         "Noah",
			LastName:          "Ausnahme",
			DayPlanningStatus: DayPlanningStatusNotComingToday,
			DayPlanningReason: dayPlanningReasonArrivalException,
			DayPlanningLabel:  "arzttermin",
		},
	}

	rows := buildExportRows(students, map[int64]weeklySchedule{}, map[int64]string{
		101: "Angemeldet: OGS",
	}, testExportDate, true)

	require.Len(t, rows, len(students))
	assert.Equal(t, "Angemeldet: OGS", rows[0].Values[listexport.ColumnEnrollmentSummary])
	assert.Equal(t, "Kommt heute", rows[0].Values[listexport.ColumnDailyStatus])
	assert.Equal(t, "Krank", rows[1].Values[listexport.ColumnDailyStatus])
	assert.Equal(t, "Entschuldigt", rows[2].Values[listexport.ColumnDailyStatus])
	assert.Equal(t, "Klassenfahrt", rows[3].Values[listexport.ColumnDailyStatus])
	assert.Equal(t, "Arzttermin", rows[4].Values[listexport.ColumnDailyStatus])
}

// TestDepartureExportCell pins the accompanied companion-note rendering in the
// student export (#1694): the "mit wem" note is appended only when the resolved
// plan actually allows the accompanied ("Mit anderem Kind") mode.
func TestDepartureExportCell(t *testing.T) {
	t.Parallel()

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

	t.Run("names the structured companion links", func(t *testing.T) {
		got := departureExportCell(StudentResponse{
			AllowedDepartureModes: users.AllowedDepartureModes{
				users.PickupDayMonday: []users.DepartureMode{users.DepartureAccompanied},
			},
			DepartureCompanions: []users.CompanionLink{
				{CompanionStudentID: 7, FirstName: "Mia", LastName: "Schulz", Weekdays: []string{users.PickupDayMonday}},
			},
		})
		// A child whose "mit wem" is answered by a link has no note at all —
		// without the links the paper list would only say "Mit anderem Kind".
		require.Equal(t, "Mo: Mit anderem Kind (mit: Mia Schulz (Mo))", got)
	})

	t.Run("renders links and note together", func(t *testing.T) {
		got := departureExportCell(StudentResponse{
			AllowedDepartureModes: users.AllowedDepartureModes{
				users.PickupDayMonday:  []users.DepartureMode{users.DepartureAccompanied},
				users.PickupDayTuesday: []users.DepartureMode{users.DepartureAccompanied},
			},
			DepartureCompanions: []users.CompanionLink{
				{CompanionStudentID: 7, FirstName: "Mia", LastName: "Schulz", Weekdays: []string{users.PickupDayMonday}},
			},
			DepartureCompanionNote: "Dienstags mit Nachbarskind Tom",
		})
		require.Equal(t, "Mo: Mit anderem Kind, Di: Mit anderem Kind (mit: Mia Schulz (Mo); Dienstags mit Nachbarskind Tom)", got)
	})

	t.Run("ignores links when no day is accompanied", func(t *testing.T) {
		got := departureExportCell(StudentResponse{
			AllowedDepartureModes: users.AllowedDepartureModes{
				users.PickupDayMonday: []users.DepartureMode{users.DepartureBus},
			},
			DepartureCompanions: []users.CompanionLink{
				{CompanionStudentID: 7, FirstName: "Mia", LastName: "Schulz", Weekdays: []string{users.PickupDayMonday}},
			},
		})
		require.Equal(t, "Mo: Bus", got)
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
	t.Parallel()

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

func TestSortExportResponsesGermanNameOrder(t *testing.T) {
	t.Parallel()

	responses := []StudentResponse{
		{FirstName: "Jan", LastName: "Zimmermann"},
		{FirstName: "Emre", LastName: "Özdemir"},
		{FirstName: "Lena", LastName: "Ärmel"},
		{FirstName: "Ben", LastName: "Müller"},
		{FirstName: "Anna", LastName: "Müller"},
		{FirstName: "Tim", LastName: "Mueller"},
		{FirstName: "Ida", LastName: "von Berg"},
		{FirstName: "Ali", LastName: "anders"},
	}

	sortExportResponses(responses, "")

	got := make([]string, 0, len(responses))
	for _, r := range responses {
		got = append(got, r.LastName+", "+r.FirstName)
	}
	want := []string{
		"anders, Ali",
		"Ärmel, Lena",
		"Mueller, Tim",
		"Müller, Anna",
		"Müller, Ben",
		"Özdemir, Emre",
		"von Berg, Ida",
		"Zimmermann, Jan",
	}
	assert.Equal(t, want, got)
}

func TestSortExportResponsesPickupStaysTimeOnly(t *testing.T) {
	t.Parallel()

	early := "12:00"
	late := "16:00"
	responses := []StudentResponse{
		{FirstName: "Jan", LastName: "Zimmermann", PickupTime: &late},
		{FirstName: "Lena", LastName: "Ärmel", PickupTime: &late},
		{FirstName: "Anna", LastName: "Müller", PickupTime: &early},
	}

	sortExportResponses(responses, "pickup")

	// Times decide; equal times keep incoming order (stable sort, no name tiebreak).
	assert.Equal(t, "Müller", responses[0].LastName)
	assert.Equal(t, "Zimmermann", responses[1].LastName)
	assert.Equal(t, "Ärmel", responses[2].LastName)
}
