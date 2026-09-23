package compose_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDayReportKeepsTheWireShape is the output golden of the cutover
// (#2701): the school portal serves the projection's DayReport where it
// served the enrollment report before, so every field keeps the JSON name
// and omission the enrollment report gave it. The class-day HTTP tests
// compare the served report with the retained report end to end (#3444).
func TestDayReportKeepsTheWireShape(t *testing.T) {
	t.Parallel()

	reportedAt := time.Date(2026, 8, 5, 5, 0, 0, 0, time.UTC)
	report := classday.DayReport{
		SchoolClass:     "4a",
		Date:            classday.Date("2026-08-05"),
		Weekday:         "wed",
		SchoolDay:       true,
		PhaseName:       "Schuljahr 2026/27",
		EnrollmentKnown: true,
		Totals:          classday.DayTotals{Students: 3, Staying: 1, Leaving: 1, Absent: 1, ListEntries: 1},
		Rows: []classday.DayRow{
			{
				StudentID: 11, FirstName: "Klara", LastName: "Klassentag", GroupName: "Delfine", Registered: true,
				StaysToday: true, Offerings: []string{"Lernzeit"}, Arrival: "11:45", Pickup: "15:00", Departure: "Abholung",
				PickupChanged: true, PickupRegular: "16:00", ReportedAt: &reportedAt,
			},
			{
				StudentID: 12, FirstName: "Nico", LastName: "Krank", Registered: true, Offerings: []string{},
				Departure: "Keine Angabe", Status: "sick", ReportedAt: &reportedAt,
			},
			{
				FirstName: "Lisa", LastName: "NurListe", ListEntry: true, ListEntryID: 9007199254740993, Offerings: []string{},
			},
		},
		ClassArrivalException: &classday.DayArrivalException{ArrivalTime: "12:45", Reason: "Unterricht fällt aus", Origin: "school"},
	}

	got, err := json.Marshal(report)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"school_class": "4a", "date": "2026-08-05", "weekday": "wed", "school_day": true,
		"phase_name": "Schuljahr 2026/27", "enrollment_known": true,
		"totals": {"students": 3, "staying": 1, "leaving": 1, "absent": 1, "list_entries": 1},
		"rows": [
			{"student_id": 11, "first_name": "Klara", "last_name": "Klassentag", "group_name": "Delfine",
			 "registered": true, "stays_today": true, "offerings": ["Lernzeit"], "arrival": "11:45",
			 "pickup": "15:00", "departure": "Abholung", "pickup_changed": true, "pickup_regular": "16:00",
			 "reported_at": "2026-08-05T05:00:00Z"},
			{"student_id": 12, "first_name": "Nico", "last_name": "Krank", "registered": true, "stays_today": false,
			 "offerings": [], "departure": "Keine Angabe", "status": "sick", "reported_at": "2026-08-05T05:00:00Z"},
			{"student_id": 0, "first_name": "Lisa", "last_name": "NurListe", "list_entry": true,
			 "list_entry_id": "9007199254740993", "registered": false, "stays_today": false, "offerings": []}
		],
		"class_arrival_exception": {"arrival_time": "12:45", "reason": "Unterricht fällt aus", "origin": "school"}
	}`, string(got))

	// The optionals stay omitted.
	bare, err := json.Marshal(classday.DayReport{SchoolClass: "1b", Date: classday.Date("2026-08-08"), Rows: []classday.DayRow{}})
	require.NoError(t, err)
	assert.NotContains(t, string(bare), "class_arrival_exception")
	assert.NotContains(t, string(bare), "phase_name")
}

// TestArrivalExceptionErrorsKeepTheirMessages pins the error contract of the
// class-day write seam: the projection's sentinels carry the messages of the
// Care Plan errors the retained service returns under its own names. The
// class-day HTTP tests drive a wrapped service error through the real seam
// (#3444).
func TestArrivalExceptionErrorsKeepTheirMessages(t *testing.T) {
	t.Parallel()

	pairs := map[error]error{
		classday.ErrArrivalExceptionPastDate:      careplan.ErrClassArrivalExceptionPastDate,
		classday.ErrArrivalExceptionWeekend:       careplan.ErrClassArrivalExceptionWeekend,
		classday.ErrArrivalExceptionClassNotFound: careplan.ErrClassArrivalExceptionClassNotFound,
		classday.ErrArrivalExceptionNotFound:      careplan.ErrClassArrivalExceptionNotFound,
	}
	for sentinel, legacy := range pairs {
		assert.Equal(t, legacy.Error(), sentinel.Error())
	}
}
