package presenceprojectionhttp

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/stretchr/testify/assert"
)

// Place before status (#6371): the export's status filter puts a checked-in
// child under its place, never under its sick, excused or class-trip status.
// Every child lands in exactly one status bucket; a child without a check-in
// keeps its status.
func TestExportStatus_PlaceBeforeStatus(t *testing.T) {
	t.Parallel()

	statuses := map[string]func(*StudentResponse){
		"krank":        func(s *StudentResponse) { s.Sick = true },
		"entschuldigt": func(s *StudentResponse) { s.Excused = true },
		"klassenfahrt": func(s *StudentResponse) { s.ClassTrip = true },
	}
	places := map[string]string{
		"Anwesend - Raum 101":                 "anwesend",
		"Anwesend":                            "anwesend",
		studentpresence.YardLocationLabel:     "schulhof",
		"Unterwegs":                           "unterwegs",
		studentpresence.AbsentLocationLabel:   "",
		studentpresence.AtSchoolLocationLabel: "",
		studentpresence.InClassLocationLabel:  "",
		"":                                    "",
	}
	filters := []string{"krank", "entschuldigt", "klassenfahrt", "entschuldigt-klassenfahrt", "abwesend", "schule", "unterricht", "unterwegs", "schulhof", "anwesend"}

	for status, mark := range statuses {
		for location, placeBucket := range places {
			student := StudentResponse{Location: location}
			mark(&student)
			want := placeBucket
			if want == "" {
				want = status
			}
			assert.Equal(t, want, exportStatus(student), "status %s at %q", status, location)

			matched := 0
			for _, filter := range filters {
				if filter != "entschuldigt-klassenfahrt" && matchesExportStatusFilter(student, filter) {
					matched++
				}
			}
			assert.Equal(t, 1, matched, "status %s at %q matches exactly one bucket", status, location)
			assert.Equal(t, placeBucket == "", matchesExportStatusFilter(student, status),
				"the %s filter lists the child only without a check-in (at %q)", status, location)
		}
	}
}

// Data minimization (#6371): without a day plan the daily-status cell falls
// back to the status. It names no health status for a checked-in child; a
// child without a check-in keeps it.
func TestDailyStatusExportCell_NamesNoStatusOfACheckedInChild(t *testing.T) {
	t.Parallel()

	assert.Empty(t, dailyStatusExportCell(StudentResponse{Location: "Anwesend - Raum 101", Sick: true}, true))
	assert.Empty(t, dailyStatusExportCell(StudentResponse{Location: studentpresence.YardLocationLabel, Excused: true}, true))
	assert.Equal(t, "Krank", dailyStatusExportCell(StudentResponse{Location: studentpresence.AbsentLocationLabel, Sick: true}, true))
	assert.Equal(t, "Klassenfahrt", dailyStatusExportCell(StudentResponse{ClassTrip: true, Excused: true}, false))
	assert.Equal(t, "Entschuldigt", dailyStatusExportCell(StudentResponse{Location: studentpresence.AtSchoolLocationLabel, Excused: true}, true))
}
