package statistics

import (
	"fmt"
	"strconv"
	"strings"
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

const (
	columnCapacity    listexport.ColumnID = "capacity"
	columnDaysUsed    listexport.ColumnID = "days_used"
	columnStudents    listexport.ColumnID = "distinct_students"
	columnHours       listexport.ColumnID = "student_hours"
	columnPeak        listexport.ColumnID = "peak_occupancy"
	columnUtilization listexport.ColumnID = "peak_utilization"
)

const exportConfidentialityNote = "Vertrauliche Anwesenheitsdaten"

// buildRoomExportDocument renders the room utilization table.
func buildRoomExportDocument(report *statisticsService.Report) listexport.Document {
	doc := listexport.Document{
		Title:       "Raumauslastung",
		Subtitle:    fmt.Sprintf("Raumnutzung vom %s bis %s", report.From.Format("02.01.2006"), report.To.Format("02.01.2006")),
		GeneratedAt: time.Now(),
		Columns: []listexport.Column{
			{ID: listexport.ColumnRoomName, Label: "Raum"},
			{ID: columnCapacity, Label: "Plätze"},
			{ID: columnDaysUsed, Label: "Tage genutzt"},
			{ID: columnStudents, Label: "Kinder"},
			{ID: columnHours, Label: "Stunden"},
			{ID: columnPeak, Label: "Spitze"},
			{ID: columnUtilization, Label: "Auslastung"},
		},
		Filters: []string{
			fmt.Sprintf("Raumdaten liegen nur für die letzten %d Tage vor (ab %s)", report.RoomDataDays, report.RoomDataFrom.Format("02.01.2006")),
			"Spitze = die meisten Kinder gleichzeitig im Raum",
			"Auslastung = Spitze im Verhältnis zu den Plätzen",
		},
		Footer: exportConfidentialityNote,
	}
	for _, room := range report.Rooms {
		capacity := ""
		if room.Capacity != nil {
			capacity = strconv.Itoa(*room.Capacity)
		}
		doc.Rows = append(doc.Rows, listexport.Row{Values: map[listexport.ColumnID]string{
			listexport.ColumnRoomName: room.Name,
			columnCapacity:            capacity,
			columnDaysUsed:            strconv.Itoa(room.DaysUsed),
			columnStudents:            strconv.Itoa(room.DistinctStudents),
			columnHours:               formatHours(room.StudentMinutes),
			columnPeak:                strconv.Itoa(room.PeakOccupancy),
			columnUtilization:         formatRate(room.PeakUtilizationPercent),
		}})
	}
	return doc
}

// buildExportDocument renders the child table grouped by education group
// (GroupTitle marker rows). Rooms have their own column grid and therefore
// their own document (buildRoomExportDocument, section=rooms).
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

	return doc
}

func formatRate(v *float64) string {
	if v == nil {
		return ""
	}
	// German decimal comma, as on the screen.
	return strings.ReplaceAll(strconv.FormatFloat(*v, 'f', 1, 64), ".", ",") + " %"
}

func formatHours(minutes int) string {
	return strings.ReplaceAll(strconv.FormatFloat(float64(minutes)/60, 'f', 1, 64), ".", ",")
}
