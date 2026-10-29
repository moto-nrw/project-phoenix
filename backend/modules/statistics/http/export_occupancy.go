package statisticshttp

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

const (
	columnSlot           string = "slot"
	columnPlanned        string = "planned"
	columnActual         string = "actual"
	columnPlannedAverage string = "planned_average"
	columnPlannedMax     string = "planned_max"
	columnActualAverage  string = "actual_average"
	columnActualMax      string = "actual_max"
)

var weekdayNames = map[int]string{
	1: "Montag", 2: "Dienstag", 3: "Mittwoch", 4: "Donnerstag", 5: "Freitag", 6: "Samstag", 7: "Sonntag",
}

// buildOccupancyExportDocument renders the occupancy by time of day (#4764):
// one block per weekday, one row per 30-minute slot. A single day shows its
// two head counts; a period shows average and highest value of both.
func buildOccupancyExportDocument(report *studentpresence.StatisticsReport) ExportDocument {
	singleDay := report.From == report.To
	doc := occupancyExportFrame(report, singleDay)
	occupancy := report.Occupancy
	if occupancy == nil {
		return doc
	}
	doc.Filters = occupancyExportFilters(occupancy)
	for _, weekday := range occupancy.Weekdays {
		title := fmt.Sprintf("%s · %d Betreuungstage", weekdayNames[weekday.Weekday], weekday.PlanDays)
		if singleDay {
			title = weekdayNames[weekday.Weekday]
		}
		doc.Rows = append(doc.Rows, ExportRow{GroupTitle: title})
		for _, slot := range weekday.Slots {
			doc.Rows = append(doc.Rows, ExportRow{Values: occupancyExportValues(slot, occupancy.SlotMinutes, singleDay)})
		}
	}
	return doc
}

// occupancyExportFrame is the title and the column grid.
func occupancyExportFrame(report *studentpresence.StatisticsReport, singleDay bool) ExportDocument {
	doc := ExportDocument{
		Title:       "Belegung nach Uhrzeit",
		GeneratedAt: time.Now(),
		Footer:      exportConfidentialityNote,
	}
	if singleDay {
		doc.Subtitle = "Kinder je halbe Stunde am " + report.From.Format("02.01.2006")
		doc.Columns = []ExportColumn{
			{ID: columnSlot, Label: "Uhrzeit"},
			{ID: columnPlanned, Label: "Geplant"},
			{ID: columnActual, Label: "Da"},
		}
		return doc
	}
	doc.Subtitle = fmt.Sprintf("Kinder je halbe Stunde vom %s bis %s", report.From.Format("02.01.2006"), report.To.Format("02.01.2006"))
	doc.Columns = []ExportColumn{
		{ID: columnSlot, Label: "Uhrzeit"},
		{ID: columnPlannedAverage, Label: "Geplant Ø"},
		{ID: columnPlannedMax, Label: "Geplant höchstens"},
		{ID: columnActualAverage, Label: "Da Ø"},
		{ID: columnActualMax, Label: "Da höchstens"},
	}
	return doc
}

// occupancyExportValues formats one slot; a missing actual value stays empty.
func occupancyExportValues(slot studentpresence.StatisticsOccupancySlot, width int, singleDay bool) map[string]string {
	values := map[string]string{columnSlot: formatSlotRange(slot.StartMinute, width)}
	if singleDay {
		values[columnPlanned] = strconv.Itoa(slot.PlannedMax)
		values[columnActual] = formatOptionalCount(slot.ActualMax)
		return values
	}
	values[columnPlannedAverage] = formatAverage(slot.PlannedAverage)
	values[columnPlannedMax] = strconv.Itoa(slot.PlannedMax)
	values[columnActualAverage] = ""
	if slot.ActualAverage != nil {
		values[columnActualAverage] = formatAverage(*slot.ActualAverage)
	}
	values[columnActualMax] = formatOptionalCount(slot.ActualMax)
	return values
}

// occupancyExportFilters states the counting rules in the words the screen uses.
func occupancyExportFilters(occupancy *studentpresence.StatisticsOccupancy) []string {
	lines := []string{
		"Ein Kind zählt in einer halben Stunde, wenn es in dieser Zeit da ist, auch nur kurz.",
		"Geplant: Betreuungszeiten und Änderungen für den Tag. Kranke, entschuldigte Kinder und Kinder mit „Kommt heute nicht“ zählen nicht.",
		"Da: Anmeldung bis Abmeldung.",
		fmt.Sprintf("Anwesenheiten werden %d Tage aufbewahrt (ab %s). Für ältere Tage fehlt die Spalte „Da“.", occupancy.ActualDataDays, occupancy.ActualDataFrom.Format("02.01.2006")),
	}
	if occupancy.UntimedPlanDays > 0 {
		lines = append(lines, fmt.Sprintf("%d-mal fehlte bei einem Kind die geplante Ankunft oder Abholung. Dann zählt das Kind nicht bei „Geplant“.", occupancy.UntimedPlanDays))
	}
	return lines
}

func formatSlotRange(start, width int) string {
	return formatSlotStart(start) + "–" + formatSlotStart(start+width)
}

func formatAverage(v float64) string {
	return strings.ReplaceAll(strconv.FormatFloat(v, 'f', 1, 64), ".", ",")
}

func formatOptionalCount(v *int) string {
	if v == nil {
		return ""
	}
	return strconv.Itoa(*v)
}
