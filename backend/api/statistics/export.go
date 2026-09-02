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

const (
	columnCourse       listexport.ColumnID = "course"
	columnCategory     listexport.ColumnID = "category"
	columnHeld         listexport.ColumnID = "held_instances"
	columnCancelled    listexport.ColumnID = "cancelled_instances"
	columnParticipated listexport.ColumnID = "present_slots"
	columnMissed       listexport.ColumnID = "absent_slots"
	columnOpen         listexport.ColumnID = "open_slots"
	columnOccupancy    listexport.ColumnID = "occupancy"
	columnSeats        listexport.ColumnID = "seats"
)

const exportConfidentialityNote = "Vertrauliche Anwesenheitsdaten"

// buildSectionDocument picks the document for the requested export section
// and the filename stem that goes with it. Every section has its own column
// grid — mixing them under shared headers produced unreadable PDFs (#2606).
func buildSectionDocument(report *statisticsService.Report, section string) (listexport.Document, string) {
	switch section {
	case sectionRooms:
		return buildRoomExportDocument(report), "raumauslastung"
	case sectionCourses:
		return buildCourseExportDocument(report), "kurse"
	case sectionCourseStudents:
		return buildCourseStudentExportDocument(report), "kurse-je-kind"
	default:
		return buildExportDocument(report), "statistik"
	}
}

// courseExportFilters states the definitions both course documents rest on,
// in the same words the screen uses.
func courseExportFilters(report *statisticsService.Report) []string {
	// The window is already in the subtitle; repeating it here would just cost
	// a line.
	return []string{
		"Quote = Teilnahmetage geteilt durch entschiedene Termine (Teilnahme + Fehltage)",
		"Abgesagte Termine zählen nicht. Offene Termine sind noch nicht abgeschlossen und zählen nicht in die Quote.",
		fmt.Sprintf("Termine werden höchstens %d Tage aufbewahrt (ab %s).", report.CourseDataDays, report.CourseDataFrom.Format("02.01.2006")),
	}
}

// buildCourseExportDocument renders one row per course.
func buildCourseExportDocument(report *statisticsService.Report) listexport.Document {
	doc := listexport.Document{
		Title:       "Kursteilnahme",
		Subtitle:    fmt.Sprintf("Teilnahme je Kurs vom %s bis %s", report.From.Format("02.01.2006"), report.To.Format("02.01.2006")),
		GeneratedAt: time.Now(),
		Columns: []listexport.Column{
			{ID: columnCourse, Label: "Kurs"},
			{ID: columnCategory, Label: "Kategorie"},
			{ID: columnHeld, Label: "Termine"},
			{ID: columnCancelled, Label: "Abgesagt"},
			{ID: columnStudents, Label: "Kinder"},
			{ID: columnSeats, Label: "Plätze"},
			{ID: columnParticipated, Label: "Teilnahme"},
			{ID: columnMissed, Label: "Fehltage"},
			{ID: columnOpen, Label: "Offen"},
			{ID: columnRate, Label: "Quote"},
			{ID: columnOccupancy, Label: "Belegung"},
		},
		Filters: courseExportFilters(report),
		Footer:  exportConfidentialityNote,
	}
	for _, course := range report.Courses {
		doc.Rows = append(doc.Rows, listexport.Row{Values: map[listexport.ColumnID]string{
			columnCourse:       course.Name,
			columnCategory:     course.CategoryName,
			columnHeld:         strconv.Itoa(course.HeldInstances),
			columnCancelled:    strconv.Itoa(course.CancelledInstances),
			columnStudents:     strconv.Itoa(course.StudentCount),
			columnSeats:        formatSeats(course.MaxParticipants),
			columnParticipated: strconv.Itoa(course.PresentDays),
			columnMissed:       strconv.Itoa(course.AbsentDays),
			columnOpen:         strconv.Itoa(course.OpenDays),
			columnRate:         formatRate(course.ParticipationRate),
			columnOccupancy:    formatRate(course.OccupancyPercent),
		}})
	}
	return doc
}

// buildCourseStudentExportDocument renders one row per child and course,
// grouped by child so a family sees its own block in one place.
func buildCourseStudentExportDocument(report *statisticsService.Report) listexport.Document {
	doc := listexport.Document{
		Title:       "Kursteilnahme je Kind",
		Subtitle:    fmt.Sprintf("Teilnahme je Kind vom %s bis %s", report.From.Format("02.01.2006"), report.To.Format("02.01.2006")),
		GeneratedAt: time.Now(),
		Columns: []listexport.Column{
			{ID: listexport.ColumnName, Label: "Name"},
			{ID: listexport.ColumnSchoolClass, Label: "Klasse"},
			{ID: columnCourse, Label: "Kurs"},
			{ID: columnParticipated, Label: "Teilnahme"},
			{ID: columnMissed, Label: "Fehltage"},
			{ID: columnOpen, Label: "Offen"},
			{ID: columnRate, Label: "Quote"},
		},
		Filters: courseExportFilters(report),
		Footer:  exportConfidentialityNote,
	}
	for _, row := range report.CourseStudents {
		doc.Rows = append(doc.Rows, listexport.Row{Values: map[listexport.ColumnID]string{
			listexport.ColumnName:        row.LastName + ", " + row.FirstName,
			listexport.ColumnSchoolClass: row.SchoolClass,
			columnCourse:                 row.CourseName,
			columnParticipated:           strconv.Itoa(row.PresentDays),
			columnMissed:                 strconv.Itoa(row.AbsentDays),
			columnOpen:                   strconv.Itoa(row.OpenDays),
			columnRate:                   formatRate(row.ParticipationRate),
		}})
	}
	return doc
}

// formatSeats renders the Teilnehmergrenze; 0 means the course has none.
func formatSeats(maxParticipants int) string {
	if maxParticipants <= 0 {
		return "unbegrenzt"
	}
	return strconv.Itoa(maxParticipants)
}

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
			fmt.Sprintf("Raumdaten können höchstens %d Tage zurückreichen (ab %s). Je Kind kann die Frist kürzer sein.", report.RoomDataDays, report.RoomDataFrom.Format("02.01.2006")),
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
