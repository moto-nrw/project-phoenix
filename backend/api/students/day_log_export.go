package students

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/listexport"
)

const (
	dayLogColumnName    listexport.ColumnID = "name"
	dayLogColumnClass   listexport.ColumnID = "school_class"
	dayLogColumnStatus  listexport.ColumnID = "day_status"
	dayLogColumnFrom    listexport.ColumnID = "present_from"
	dayLogColumnUntil   listexport.ColumnID = "present_until"
	dayLogColumnDetails listexport.ColumnID = "details"
)

// exportStudentsDayLog handles GET /students/day-log/export?date=&group_id=&format=.
// Same gate, scope, cap, and audit rules as the JSON endpoint; renders one
// section per group via the shared listexport pipeline.
func (rs *Resource) exportStudentsDayLog(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := rs.dayLogLogger()

	if !configService.ResolveBoolOrDefault(ctx, rs.SettingsService, configModel.KeyAttendanceLogEnabled, false, logger) {
		renderError(w, r, common.ErrorForbidden(errors.New("feature_disabled")))
		return
	}
	if rs.ListExportService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("day log export service is not configured")))
		return
	}

	format, err := parseDayLogExportFormat(r)
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	date, err := parseDayLogDate(r)
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	groups, err := rs.resolveDayLogGroups(r, date, logger)
	if err != nil {
		renderDayLogGroupError(w, r, err, logger)
		return
	}

	data, err := rs.loadDayLogData(ctx, groups, date)
	if err != nil {
		renderError(w, r, common.ErrorInternalServerWrap("failed to load day log", err))
		return
	}
	log := buildDayLogResponse(date, groups, data)

	file, err := rs.ListExportService.Render(buildDayLogExportDocument(log, date), format, "tagesauswertung-"+date.String())
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	if err := rs.writeDayLogAudit(r, date, groups, logger); err != nil {
		renderError(w, r, common.ErrorInternalServerWrap("failed to record audit trail", err))
		return
	}

	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.Filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(file.Data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Data)
}

func parseDayLogExportFormat(r *http.Request) (listexport.Format, error) {
	format := listexport.Format(strings.TrimSpace(r.URL.Query().Get("format")))
	if format == "" {
		format = listexport.FormatPDF
	}
	if format != listexport.FormatPDF && format != listexport.FormatDOCX && format != listexport.FormatXLSX {
		return format, fmt.Errorf("unsupported export format %q", format)
	}
	return format, nil
}

func buildDayLogExportDocument(log dayLogResponse, date timezone.Date) listexport.Document {
	doc := listexport.Document{
		Title:       "Tagesauswertung",
		Subtitle:    "Anwesenheit am " + date.Format("02.01.2006"),
		GeneratedAt: time.Now(),
		Columns: []listexport.Column{
			{ID: dayLogColumnName, Label: "Name"},
			{ID: dayLogColumnClass, Label: "Klasse"},
			{ID: dayLogColumnStatus, Label: "Status"},
			{ID: dayLogColumnFrom, Label: "Anwesend ab"},
			{ID: dayLogColumnUntil, Label: "Anwesend bis"},
			{ID: dayLogColumnDetails, Label: "Hinweis"},
		},
		Footer: "Vertrauliche Anwesenheitsdaten",
	}
	grouped := len(log.Groups) > 1
	for _, group := range log.Groups {
		doc.Filters = append(doc.Filters, fmt.Sprintf(
			"%s: %d Anwesend · %d Krank · %d Entschuldigt · %d Klassenfahrt · %d Abwesend",
			group.Name, group.Counters.Present, group.Counters.Sick,
			group.Counters.Excused, group.Counters.ClassTrip, group.Counters.Absent,
		))
		// Group sections use the marker-row convention of the listexport
		// renderers: a Values-less row carrying only GroupTitle opens the
		// section, the data rows that follow carry no GroupTitle.
		if grouped {
			doc.Rows = append(doc.Rows, listexport.Row{GroupTitle: group.Name})
		}
		for _, student := range group.Students {
			doc.Rows = append(doc.Rows, listexport.Row{Values: map[listexport.ColumnID]string{
				dayLogColumnName:    student.LastName + ", " + student.FirstName,
				dayLogColumnClass:   student.SchoolClass,
				dayLogColumnStatus:  student.Label,
				dayLogColumnFrom:    exportOptionalTime(student.CheckInTime),
				dayLogColumnUntil:   dayLogExportUntil(student),
				dayLogColumnDetails: dayLogExportDetails(student),
			}})
		}
	}
	return doc
}

// dayLogExportUntil renders the departure cell: an open session on a present
// child reads as "anwesend" instead of an empty dash.
func dayLogExportUntil(student dayLogStudent) string {
	if student.Status == dayLogStatusPresent && student.CheckOutTime == nil {
		return "anwesend"
	}
	return exportOptionalTime(student.CheckOutTime)
}

func dayLogExportDetails(student dayLogStudent) string {
	if student.Hint != "" {
		return student.Hint
	}
	if student.ReportedAt != nil {
		return "gemeldet " + student.ReportedAt.In(timezone.Berlin).Format("02.01. 15:04") + dayLogSourceSuffix(student.Source)
	}
	if student.Source == dayLogSourceCancelledCareDay {
		return "Abmeldung im Betreuungsplan"
	}
	return ""
}

func dayLogSourceSuffix(source string) string {
	if source == "parent" {
		return " (Eltern)"
	}
	return ""
}
