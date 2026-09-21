package care

import (
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
)

// SickNoteEventBody renders the chat pill for a parent absence submission. The
// label MUST follow the submitted status: a "Krankmeldung" (sick) and an
// "Entschuldigte Abwesenheit" (excused — e.g. an appointment, #1735) are
// distinct in the data and to staff, so an excused absence must not be
// recorded as a sickness.
func SickNoteEventBody(status string, dates []timezone.Date) string {
	parts := make([]string, 0, len(dates))
	for _, d := range dates {
		parts = append(parts, d.Format("02.01."))
	}
	label := "Krankmeldung"
	if status == absencerecords.StudentStatusDayExcused {
		label = "Entschuldigte Abwesenheit"
	}
	return label + ": " + strings.Join(parts, ", ")
}

// CareExceptionEventBody renders the chat pill for a guardian care-time
// submission. The headline stays neutral ("Betreuungszeit") because either leg
// (arrival, pickup, or both) may have changed — labelling it as an "Abholung"
// change would record a pickup change that did not happen on an arrival-only
// submit. The appended value lines name the concrete leg(s) that were set.
func CareExceptionEventBody(date timezone.Date, pickupTime, arrivalTime *time.Time) string {
	parts := []string{"Betreuungszeit " + date.Format("02.01.") + " geändert"}
	if arrivalTime != nil {
		parts = append(parts, "Ankunft "+arrivalTime.Format("15:04"))
	}
	if pickupTime != nil {
		parts = append(parts, "Abholung "+pickupTime.Format("15:04"))
	}
	return strings.Join(parts, ": ")
}
