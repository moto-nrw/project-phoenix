package application

import (
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/classday"
)

// classDayReportedAt answers "since when is this known" for a deviating row,
// and only for one: a row that matches the regular plan has nothing to date.
// A reported day status outranks a changed pickup time — it is the stronger
// statement, and the badge the reader sees follows the same precedence.
func classDayReportedAt(facts classDayFacts, studentID int64, status string, pickupChanged bool) *time.Time {
	if status != "" {
		if stamp, ok := facts.statusReportedAt[studentID]; ok {
			return &stamp
		}
		return nil
	}
	if pickupChanged {
		if stamp, ok := facts.pickupChangedAt[studentID]; ok {
			return &stamp
		}
	}
	return nil
}

// buildClassDayReport projects full roster rows onto one calendar day: the
// weekday's offerings decide who stays, a reported day status wins over any
// enrollment, a not-scheduled care day (materialized plan says "an dem Tag
// nicht gebucht") overrides the offering, and everyone else goes home after
// lessons. Effective arrival/pickup times (from the live plans) replace the
// roster's form-answer values when available; the departure column comes
// exclusively from the per-day plan (or "Keine Angabe") on school days.
func buildClassDayReport(schoolClass string, date timezone.Date, phaseName string, rosterRows []DayRosterRow, facts classDayFacts) *classday.DayReport {
	weekday := classDayWeekdayKey(date)
	report := &classday.DayReport{
		SchoolClass: schoolClass,
		Date:        classday.Date(date.String()),
		Weekday:     weekday,
		SchoolDay:   weekday != "",
		PhaseName:   phaseName,
		Rows:        make([]classday.DayRow, 0, len(rosterRows)),
	}
	for _, row := range rosterRows {
		dayRow := classDayRow(row, weekday, facts)
		report.Rows = append(report.Rows, dayRow)
		report.Totals.Students++
		switch {
		case row.ListEntry:
			// A class-list-only entry neither stays nor leaves — "Keine
			// Betreuung" is its own neutral roster bucket, never a
			// "geht nach Hause" claim.
			report.Totals.ListEntries++
		case dayRow.Status != "":
			report.Totals.Absent++
		case dayRow.StaysToday:
			report.Totals.Staying++
		default:
			report.Totals.Leaving++
		}
	}
	if !report.SchoolDay {
		// Kein Schultag: there is no handoff, so "geht nach Hause" is not a
		// statement either — only the Klassenverband count stays honest.
		report.Totals.Staying = 0
		report.Totals.Leaving = 0
		report.Totals.Absent = 0
		report.Totals.ListEntries = 0
	}
	return report
}

// classDayRow projects one roster row onto the day.
func classDayRow(row DayRosterRow, weekday string, facts classDayFacts) classday.DayRow {
	offerings := []string{}
	arrival, pickup := "", ""
	pickupChanged, pickupRegular := false, ""
	if weekday != "" {
		offerings = append(offerings, row.OfferingsByDay[weekday]...)
		arrival = firstNonEmpty(facts.arrivals[row.StudentID], strings.TrimSpace(row.ArrivalByDay[weekday]))
		pickup = firstNonEmpty(facts.pickups[row.StudentID], strings.TrimSpace(row.PickupByDay[weekday]))
		pickupChanged = facts.pickupChanged[row.StudentID]
		pickupRegular = facts.pickupRegular[row.StudentID]
	}
	status := facts.statuses[row.StudentID]
	// A class-list-only entry has no care plan at all (#2382): it can
	// neither deviate from one nor carry a report time.
	if row.ListEntry {
		pickupChanged, pickupRegular = false, ""
	}
	return classday.DayRow{
		StudentID:   row.StudentID,
		FirstName:   row.FirstName,
		LastName:    row.LastName,
		ListEntry:   row.ListEntry,
		ListEntryID: row.ListEntryID,
		GroupName:   row.GroupName,
		Registered:  row.Registered,
		// The materialized care plan is the current truth: a weekday the
		// parents struck from the plan beats the approved offering — the
		// same source the effective times above already come from.
		StaysToday:    len(offerings) > 0 && status == "" && !facts.notScheduled[row.StudentID],
		Offerings:     offerings,
		Arrival:       arrival,
		Pickup:        pickup,
		Departure:     classDayRowDeparture(row, weekday, facts),
		Status:        status,
		PickupChanged: pickupChanged,
		PickupRegular: pickupRegular,
		ReportedAt:    classDayReportedAt(facts, row.StudentID, status, pickupChanged),
	}
}

// classDayRowDeparture is the departure column. The per-day plan is the ONLY
// departure source: the roster's per-day map is never empty — it floors
// every day at "geht alleine" — so falling back to it would fabricate an
// unaccompanied departure for a child without any plan. Missing data renders
// as explicit "Keine Angabe"; on a non-school day the column stays empty
// entirely (mirror of the zeroed totals) — a weekend request must not serve
// any departure instruction to non-UI consumers either. A class-list-only
// entry has no departure column at all: "Keine Betreuung" is the whole
// statement, and "Keine Angabe" would suggest a plan gap the office should
// fill.
func classDayRowDeparture(row DayRosterRow, weekday string, facts classDayFacts) string {
	if weekday == "" || row.ListEntry {
		return ""
	}
	if departure := facts.departures[row.StudentID]; departure != "" {
		return departure
	}
	return classDayDepartureUnknown
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
