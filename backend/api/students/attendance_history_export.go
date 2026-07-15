package students

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/listexport"
)

type attendanceExportOptions struct {
	Format listexport.Format
	From   timezone.Date
	To     timezone.Date
}

const (
	attendanceColumnDate       listexport.ColumnID = "date"
	attendanceColumnOffering   listexport.ColumnID = "care_offering"
	attendanceColumnWindow     listexport.ColumnID = "time_window"
	attendanceColumnStatus     listexport.ColumnID = "slot_status"
	attendanceColumnCheckIn    listexport.ColumnID = "checked_in_at"
	attendanceColumnCheckOut   listexport.ColumnID = "checked_out_at"
	attendanceColumnAssignment listexport.ColumnID = "assignment"
)

func (rs *Resource) exportStudentAttendanceHistory(w http.ResponseWriter, r *http.Request) {
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}
	logger := rs.attendanceHistoryLogger()
	if !rs.checkAttendanceExportAccess(w, r, student) {
		return
	}

	visibleDays := config.ResolveIntOrDefault(r.Context(), rs.SettingsService, configModel.KeyAttendanceVisibleDays, 30, logger)
	options, err := parseAttendanceExportOptions(r, visibleDays)
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	doc, err := rs.buildAttendanceExportDocument(r.Context(), student.ID, options.From, options.To)
	if err != nil {
		renderError(w, r, common.ErrorInternalServerWrap("failed to build attendance export", err))
		return
	}
	file, err := rs.ListExportService.Render(doc, options.Format, fmt.Sprintf("anwesenheit-kind-%d", student.ID))
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if err := rs.writeAttendanceHistoryAudit(r, student.ID, options.From.BerlinMidnight(), options.To.EndOfDay(), logger); err != nil {
		renderError(w, r, common.ErrorInternalServerWrap("failed to record audit trail", err))
		return
	}
	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.Filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(file.Data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Data)
}

func (rs *Resource) checkAttendanceExportAccess(w http.ResponseWriter, r *http.Request, student *usersModel.Student) bool {
	logger := rs.attendanceHistoryLogger()
	if !config.ResolveBoolOrDefault(r.Context(), rs.SettingsService, configModel.KeyAttendanceLogEnabled, false, logger) {
		renderError(w, r, common.ErrorForbidden(errors.New("feature_disabled")))
		return false
	}
	scope := config.ResolveStringOrDefault(r.Context(), rs.SettingsService, configModel.KeyAttendanceLogScope, configModel.AttendanceLogScopeGroupSupervisorsOnly, logger)
	if !rs.attendanceHistoryScopeAllows(r, student, scope) {
		renderError(w, r, common.ErrorForbidden(errors.New("not_group_supervisor")))
		return false
	}
	if rs.ListExportService == nil || rs.StudentHistoryService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("attendance export service is not configured")))
		return false
	}
	return true
}

func parseAttendanceExportOptions(r *http.Request, visibleDays int) (attendanceExportOptions, error) {
	options := attendanceExportOptions{Format: listexport.Format(strings.TrimSpace(r.URL.Query().Get("format")))}
	if options.Format == "" {
		options.Format = listexport.FormatPDF
	}
	if options.Format != listexport.FormatPDF && options.Format != listexport.FormatDOCX && options.Format != listexport.FormatXLSX {
		return options, fmt.Errorf("unsupported export format %q", options.Format)
	}
	options.To = timezone.TodayDate()
	options.From = options.To.AddDays(-(visibleDays - 1))
	var err error
	if raw := strings.TrimSpace(r.URL.Query().Get("from")); raw != "" {
		options.From, err = timezone.ParseDate(raw)
		if err != nil {
			return options, errors.New("invalid from date format, expected YYYY-MM-DD")
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("to")); raw != "" {
		options.To, err = timezone.ParseDate(raw)
		if err != nil {
			return options, errors.New("invalid to date format, expected YYYY-MM-DD")
		}
	}
	if options.To.Before(options.From) || options.From.DaysUntil(options.To) >= visibleDays {
		return options, fmt.Errorf("date range cannot exceed %d days", visibleDays)
	}
	return options, nil
}

func (rs *Resource) buildAttendanceExportDocument(
	ctx context.Context, studentID int64, from, to timezone.Date,
) (listexport.Document, error) {
	slots, err := rs.StudentHistoryService.GetSlotAttendanceByStudentAndDateRange(ctx, studentID, from, to)
	if err != nil {
		return listexport.Document{}, err
	}
	attendance, err := rs.StudentHistoryService.GetAttendanceByStudentAndDateRange(ctx, studentID, from, to)
	if err != nil {
		return listexport.Document{}, err
	}
	return listexport.Document{
		Title:       "Anwesenheit je Betreuungsangebot",
		Subtitle:    fmt.Sprintf("Kind-ID %d · %s bis %s", studentID, from.Format("02.01.2006"), to.Format("02.01.2006")),
		GeneratedAt: time.Now(), Columns: attendanceExportColumns(),
		Rows: attendanceExportRows(slots, attendance), Footer: "Vertrauliche Anwesenheitsdaten",
	}, nil
}

func attendanceExportColumns() []listexport.Column {
	return []listexport.Column{
		{ID: attendanceColumnDate, Label: "Datum"}, {ID: attendanceColumnOffering, Label: "Betreuungsangebot"},
		{ID: attendanceColumnWindow, Label: "Zeitslot"}, {ID: attendanceColumnStatus, Label: "Status"},
		{ID: attendanceColumnCheckIn, Label: "Anwesend ab"}, {ID: attendanceColumnCheckOut, Label: "Anwesend bis"},
		{ID: attendanceColumnAssignment, Label: "Zuordnung"},
	}
}

func attendanceExportRows(slots []*scheduleModel.ScheduledInstanceRow, attendanceRows []*activeModel.Attendance) []listexport.Row {
	rows := make([]listexport.Row, 0, len(slots)+len(attendanceRows))
	assigned := make(map[int64]struct{}, len(slots))
	for _, row := range slots {
		if row == nil || row.Instance == nil || row.Attendance == nil {
			continue
		}
		if row.Attendance.CheckedInAt != nil {
			assigned[row.Attendance.CheckedInAt.UnixNano()] = struct{}{}
		}
		rows = append(rows, slotExportRow(row))
	}
	for _, attendance := range attendanceRows {
		if _, ok := assigned[attendance.CheckInTime.UnixNano()]; !ok {
			rows = append(rows, unassignedExportRow(attendance))
		}
	}
	return rows
}

func slotExportRow(row *scheduleModel.ScheduledInstanceRow) listexport.Row {
	assignment := "Gebucht"
	if row.Attendance.IsUnplanned {
		assignment = "Ungeplant, ohne Buchung"
	}
	return listexport.Row{Values: map[listexport.ColumnID]string{
		attendanceColumnDate: row.Instance.Date.Format("02.01.2006"), attendanceColumnOffering: row.Instance.Title,
		attendanceColumnWindow:  row.Instance.StartTime.Format("15:04") + "–" + row.Instance.EndTime.Format("15:04"),
		attendanceColumnStatus:  attendanceSlotStatusLabel(row.Attendance.Status, row.Attendance.Substatus),
		attendanceColumnCheckIn: exportOptionalTime(row.Attendance.CheckedInAt), attendanceColumnCheckOut: exportOptionalTime(row.Attendance.CheckedOutAt),
		attendanceColumnAssignment: assignment,
	}}
}

func unassignedExportRow(attendance *activeModel.Attendance) listexport.Row {
	return listexport.Row{Values: map[listexport.ColumnID]string{
		attendanceColumnDate: attendance.Date.Format("02.01.2006"), attendanceColumnOffering: "Ohne Zuordnung",
		attendanceColumnWindow: exportOptionalTime(&attendance.CheckInTime) + "–" + exportOptionalTime(attendance.CheckOutTime),
		attendanceColumnStatus: "Anwesend", attendanceColumnCheckIn: exportOptionalTime(&attendance.CheckInTime),
		attendanceColumnCheckOut: exportOptionalTime(attendance.CheckOutTime), attendanceColumnAssignment: "Ungeplant, ohne Buchung",
	}}
}

func exportOptionalTime(value *time.Time) string {
	if value == nil {
		return "–"
	}
	return value.In(timezone.Berlin).Format("15:04")
}

func attendanceSlotStatusLabel(status string, substatus *string) string {
	if substatus != nil {
		switch *substatus {
		case "sick":
			return "Krank"
		case "excused":
			return "Entschuldigt abwesend"
		case "field_trip":
			return "Klassenfahrt"
		}
	}
	switch status {
	case "present":
		return "Anwesend"
	case "absent":
		return "Abwesend"
	default:
		return "Erwartet"
	}
}
