package statistics

import (
	"fmt"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/services/listexport"
	statisticsService "github.com/moto-nrw/project-phoenix/services/statistics"
)

// Column IDs: catalog IDs where the PDF has a layout weight for them
// (name, class, group), ad-hoc IDs for the numeric columns.
const (
	columnPresent     listexport.ColumnID = "present_days"
	columnSick        listexport.ColumnID = "sick_days"
	columnExcused     listexport.ColumnID = "excused_days"
	columnUnexplained listexport.ColumnID = "unexplained_days"
	columnRate        listexport.ColumnID = "attendance_rate"
)

const exportConfidentialityNote = "Vertrauliche Anwesenheitsdaten"

// buildExportDocument renders the child table grouped by education group
// (GroupTitle marker rows), followed by one "Räume" section carrying the
// room utilization in the same column grid.
func buildExportDocument(report *statisticsService.Report) listexport.Document {
	doc := listexport.Document{
		Title:       "Statistik",
		Subtitle:    fmt.Sprintf("Anwesenheit vom %s bis %s", report.From.Format("02.01.2006"), report.To.Format("02.01.2006")),
		GeneratedAt: time.Now(),
		Columns: []listexport.Column{
			{ID: listexport.ColumnName, Label: "Name"},
			{ID: listexport.ColumnSchoolClass, Label: "Klasse"},
			{ID: columnPresent, Label: "Anwesend"},
			{ID: columnSick, Label: "Krank"},
			{ID: columnExcused, Label: "Entschuldigt"},
			{ID: columnUnexplained, Label: "Ohne Meldung"},
			{ID: columnRate, Label: "Quote"},
		},
		Footer: exportConfidentialityNote,
	}
	doc.Filters = append(doc.Filters,
		fmt.Sprintf("%d Betreuungstage", report.CareDays),
		fmt.Sprintf("%d Tage abgezogen (Feiertage, Schließtage, Ferien)", report.ExcludedDays.Total),
		"Quote = Tage mit Anmeldung geteilt durch Betreuungstage",
	)
	if report.Totals.AttendanceRate != nil {
		doc.Filters = append(doc.Filters, fmt.Sprintf("Gesamt: %s Anwesenheit", formatRate(report.Totals.AttendanceRate)))
	}

	byGroup := map[int64][]statisticsService.StudentRow{}
	for _, st := range report.Students {
		id := int64(0)
		if st.GroupID != nil {
			id = *st.GroupID
		}
		byGroup[id] = append(byGroup[id], st)
	}
	for _, group := range report.Groups {
		title := fmt.Sprintf("%s · %d Kinder", group.Name, group.StudentCount)
		if group.AttendanceRate != nil {
			title += " · " + formatRate(group.AttendanceRate)
		}
		doc.Rows = append(doc.Rows, listexport.Row{GroupTitle: title})
		for _, st := range byGroup[group.GroupID] {
			doc.Rows = append(doc.Rows, listexport.Row{Values: map[listexport.ColumnID]string{
				listexport.ColumnName:        st.LastName + ", " + st.FirstName,
				listexport.ColumnSchoolClass: st.SchoolClass,
				columnPresent:                strconv.Itoa(st.PresentDays),
				columnSick:                   strconv.Itoa(st.SickDays),
				columnExcused:                strconv.Itoa(st.ExcusedDays),
				columnUnexplained:            strconv.Itoa(st.UnexplainedDays),
				columnRate:                   formatRate(st.AttendanceRate),
			}})
		}
	}

	// Room section reuses the grid: name / capacity / days used / children /
	// hours / peak / utilization. The column labels stay those of the child
	// table, so the section title spells out the mapping.
	doc.Rows = append(doc.Rows, listexport.Row{GroupTitle: fmt.Sprintf(
		"Räume (Daten der letzten %d Tage): Name · Plätze · Tage genutzt · Kinder · Stunden · Spitze · Auslastung",
		report.RoomDataDays,
	)})
	for _, room := range report.Rooms {
		capacity := ""
		if room.Capacity != nil {
			capacity = strconv.Itoa(*room.Capacity)
		}
		doc.Rows = append(doc.Rows, listexport.Row{Values: map[listexport.ColumnID]string{
			listexport.ColumnName:        room.Name,
			listexport.ColumnSchoolClass: capacity,
			columnPresent:                strconv.Itoa(room.DaysUsed),
			columnSick:                   strconv.Itoa(room.DistinctStudents),
			columnExcused:                formatHours(room.StudentMinutes),
			columnUnexplained:            strconv.Itoa(room.PeakOccupancy),
			columnRate:                   formatRate(room.PeakUtilizationPercent),
		}})
	}
	return doc
}

func formatRate(v *float64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatFloat(*v, 'f', 1, 64) + " %"
}

func formatHours(minutes int) string {
	return strconv.FormatFloat(float64(minutes)/60, 'f', 1, 64)
}
