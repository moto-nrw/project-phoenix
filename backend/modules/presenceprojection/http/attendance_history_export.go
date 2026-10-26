package presenceprojectionhttp

import (
	context "context"
	errors "errors"
	fmt "fmt"
	http "net/http"
	sort "sort"
	strconv "strconv"
	strings "strings"
	time "time"

	common "github.com/moto-nrw/project-phoenix/api/common"
	lists "github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	studentpresence "github.com/moto-nrw/project-phoenix/modules/studentpresence"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type attendanceExportOptions struct {
	Format lists.Format
	From   calendar.Date
	To     calendar.Date
}

const (
	attendanceExportDateLayout                = "02.01.2006"
	attendanceColumnDate       lists.ColumnID = "date"
	attendanceColumnOffering   lists.ColumnID = "care_offering"
	attendanceColumnWindow     lists.ColumnID = "time_window"
	attendanceColumnStatus     lists.ColumnID = "slot_status"
	attendanceColumnCheckIn    lists.ColumnID = "checked_in_at"
	attendanceColumnCheckOut   lists.ColumnID = "checked_out_at"
	attendanceColumnAssignment lists.ColumnID = "assignment"
	attendanceColumnNote       lists.ColumnID = "check_out_note"
	attendanceColumnCheckedIn  lists.ColumnID = "checked_in_by"
	attendanceColumnCheckedOut lists.ColumnID = "checked_out_by"
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

	visibleDays, err := rs.SettingsService.ResolveInt(r.Context(), settingAttendanceVisibleDays)
	if err != nil {
		renderError(w, r, common.ErrorInternalServerWrap("failed to resolve attendance settings", err))
		return
	}
	options, err := parseAttendanceExportOptions(r, visibleDays, rs.todayDate())
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

func (rs *Resource) checkAttendanceExportAccess(w http.ResponseWriter, r *http.Request, student *peopleModule.StudentState) bool {
	enabled, err := rs.SettingsService.ResolveBool(r.Context(), settingAttendanceLogEnabled)
	if err != nil {
		renderError(w, r, common.ErrorInternalServerWrap("failed to resolve attendance settings", err))
		return false
	}
	if !enabled {
		renderError(w, r, common.ErrorForbiddenWithCode(errors.New("feature_disabled"), common.CodeAttendanceLogDisabled))
		return false
	}
	if !rs.attendanceHistoryScopeAllows(r, student) {
		renderError(w, r, common.ErrorForbidden(errors.New("not_group_supervisor")))
		return false
	}
	if rs.ListExportService == nil || rs.StudentHistoryService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("attendance export service is not configured")))
		return false
	}
	return true
}

func parseAttendanceExportOptions(r *http.Request, visibleDays int, today calendar.Date) (attendanceExportOptions, error) {
	options := attendanceExportOptions{Format: lists.Format(strings.TrimSpace(r.URL.Query().Get("format")))}
	if options.Format == "" {
		options.Format = lists.FormatPDF
	}
	if options.Format != lists.FormatPDF && options.Format != lists.FormatDOCX && options.Format != lists.FormatXLSX {
		return options, fmt.Errorf("unsupported export format %q", options.Format)
	}
	options.To = today
	options.From = options.To.AddDays(-(visibleDays - 1))
	var err error
	if raw := strings.TrimSpace(r.URL.Query().Get("from")); raw != "" {
		options.From, err = calendar.ParseDate(raw)
		if err != nil {
			return options, errors.New("invalid from date format, expected YYYY-MM-DD")
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("to")); raw != "" {
		options.To, err = calendar.ParseDate(raw)
		if err != nil {
			return options, errors.New("invalid to date format, expected YYYY-MM-DD")
		}
	}
	if options.From.After(today) || options.To.After(today) {
		return options, errors.New("attendance exports cannot include future dates")
	}
	if options.To.Before(options.From) {
		return options, errors.New("to date must not be before from date")
	}
	if options.From.DaysUntil(options.To) >= visibleDays {
		return options, fmt.Errorf("date range cannot exceed %d days", visibleDays)
	}
	return options, nil
}

func (rs *Resource) buildAttendanceExportDocument(
	ctx context.Context, studentID int64, from, to calendar.Date,
) (lists.Document, error) {
	slots, err := rs.StudentHistoryService.GetSlotAttendanceByStudentAndDateRange(ctx, studentID, from, to)
	if err != nil {
		return lists.Document{}, err
	}
	attendance, err := rs.StudentHistoryService.GetAttendanceByStudentAndDateRange(ctx, studentID, from, to)
	if err != nil {
		return lists.Document{}, err
	}
	// Without a care plan there is nothing to report assignments against: the
	// document drops the offering/assignment columns and lists the observed
	// sessions plainly instead of labelling every row "Ohne Zuordnung". The
	// expectation is resolved tenant-wide (resolveSlotExpectation), so a
	// student without bookings at a plan-keeping school keeps the full layout.
	expectSlots, err := rs.resolveSlotExpectation(ctx, slots, from, to)
	if err != nil {
		return lists.Document{}, err
	}
	recorders, err := rs.StudentHistoryService.AttendanceRecorders(ctx, attendance)
	if err != nil {
		return lists.Document{}, err
	}
	title, columns := "Anwesenheit je Betreuungsangebot", attendanceExportColumns()
	if !expectSlots {
		title, columns = "Anwesenheit", attendanceSessionExportColumns()
	}
	return lists.Document{
		Title:       title,
		Subtitle:    fmt.Sprintf("Kind-ID %d · %s bis %s", studentID, from.Format(attendanceExportDateLayout), to.Format(attendanceExportDateLayout)),
		GeneratedAt: time.Now(), Columns: columns,
		Rows: attendanceExportRows(slots, attendance, recorders), Footer: "Vertrauliche Anwesenheitsdaten",
	}, nil
}

func attendanceExportColumns() []lists.Column {
	return []lists.Column{
		{ID: attendanceColumnDate, Label: "Datum"}, {ID: attendanceColumnOffering, Label: "Betreuungsangebot"},
		{ID: attendanceColumnWindow, Label: "Zeitslot"}, {ID: attendanceColumnStatus, Label: "Status"},
		{ID: attendanceColumnCheckIn, Label: "Anwesend ab"}, {ID: attendanceColumnCheckOut, Label: "Anwesend bis"},
		{ID: attendanceColumnCheckedIn, Label: "Angemeldet von"}, {ID: attendanceColumnCheckedOut, Label: "Abgemeldet von"},
		{ID: attendanceColumnAssignment, Label: "Zuordnung"},
		{ID: attendanceColumnNote, Label: "Früher gegangen"},
	}
}

// attendanceSessionExportColumns is the plan-free variant: the rows still carry
// the offering/assignment values, they are simply not rendered.
func attendanceSessionExportColumns() []lists.Column {
	return []lists.Column{
		{ID: attendanceColumnDate, Label: "Datum"},
		{ID: attendanceColumnWindow, Label: "Zeitraum"}, {ID: attendanceColumnStatus, Label: "Status"},
		{ID: attendanceColumnCheckIn, Label: "Anwesend ab"}, {ID: attendanceColumnCheckOut, Label: "Anwesend bis"},
		{ID: attendanceColumnCheckedIn, Label: "Angemeldet von"}, {ID: attendanceColumnCheckedOut, Label: "Abgemeldet von"},
		{ID: attendanceColumnNote, Label: "Früher gegangen"},
	}
}

// attendanceExportRows merges slot rows and unassigned observed sessions into
// one chronologically sorted list (date, then start clock time) so multi-day
// exports read in order regardless of which source a row came from.
func attendanceExportRows(slots []*studentpresence.HistorySlot, attendanceRows []*studentpresence.Attendance, recorders studentpresence.AttendanceRecorders) []lists.Row {
	type sortableExportRow struct {
		date  calendar.Date
		clock string // HH:MM:SS in Berlin, orders rows within a day
		row   lists.Row
	}
	entries := make([]sortableExportRow, 0, len(slots)+len(attendanceRows))
	notes := checkOutNotesByInstant(attendanceRows)
	coverageByDate := make(map[calendar.Date][]slotCoverage, len(slots))
	for _, row := range slots {
		if row == nil || row.Instance == nil || row.Attendance == nil {
			continue
		}
		coverage := slotCoverage{
			present:    row.Attendance.Status == studentpresence.SessionAttendancePresent,
			startClock: row.Instance.StartTime.Format("15:04"),
			endClock:   row.Instance.EndTime.Format("15:04"),
		}
		if row.Attendance.CheckedInAt != nil {
			coverage.checkInNano = row.Attendance.CheckedInAt.UnixNano()
			coverage.hasCheckIn = true
		}
		instanceDate := calendar.Date(row.Instance.Date)
		coverageByDate[instanceDate] = append(coverageByDate[instanceDate], coverage)
		entries = append(entries, sortableExportRow{
			date:  instanceDate,
			clock: row.Instance.StartTime.Format("15:04:05"),
			row:   withAttendanceActors(slotExportRow(row, notes), stayAt(attendanceRows, row.Attendance.CheckedInAt), recorders),
		})
	}
	for _, attendance := range attendanceRows {
		if sessionCoveredBySlots(coverageByDate[calendar.Date(attendance.Date)], attendance.CheckInTime) {
			continue
		}
		entries = append(entries, sortableExportRow{
			date:  calendar.Date(attendance.Date),
			clock: attendance.CheckInTime.In(calendar.Berlin).Format("15:04:05"),
			row:   withAttendanceActors(unassignedExportRow(attendance), attendance, recorders),
		})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].date != entries[j].date {
			return entries[i].date.Before(entries[j].date)
		}
		return entries[i].clock < entries[j].clock
	})
	rows := make([]lists.Row, len(entries))
	for i, entry := range entries {
		rows[i] = entry.row
	}
	return rows
}

// checkOutNotesByInstant indexes the early-checkout notes (#3324) by their
// checkout instant. A checkout mirrors into the slot it closes with the same
// timestamp, so a slot row finds the note of the stay that ended it.
func checkOutNotesByInstant(rows []*studentpresence.Attendance) map[int64]string {
	notes := make(map[int64]string)
	for _, row := range rows {
		if row.CheckOutTime != nil && row.CheckOutNote != nil && *row.CheckOutNote != "" {
			notes[row.CheckOutTime.UnixNano()] = *row.CheckOutNote
		}
	}
	return notes
}

func slotCheckOutNote(row *studentpresence.HistorySlot, notes map[int64]string) string {
	if row.Attendance.CheckedOutAt == nil {
		return ""
	}
	return notes[row.Attendance.CheckedOutAt.UnixNano()]
}

func slotExportRow(row *studentpresence.HistorySlot, notes map[int64]string) lists.Row {
	assignment := "Gebucht"
	if row.Attendance.IsUnplanned {
		assignment = "Ungeplant, ohne Buchung"
	}
	return lists.Row{Values: map[lists.ColumnID]string{
		attendanceColumnDate: row.Instance.Date.Format(attendanceExportDateLayout), attendanceColumnOffering: row.Instance.Title,
		attendanceColumnWindow:  row.Instance.StartTime.Format("15:04") + "–" + row.Instance.EndTime.Format("15:04"),
		attendanceColumnStatus:  attendanceSlotStatusLabel(row.Attendance.Status, row.Attendance.Substatus),
		attendanceColumnCheckIn: exportOptionalTime(row.Attendance.CheckedInAt), attendanceColumnCheckOut: exportOptionalTime(row.Attendance.CheckedOutAt),
		attendanceColumnAssignment: assignment,
		attendanceColumnNote:       slotCheckOutNote(row, notes),
	}}
}

// unassignedExportRow renders an observed session no slot could claim. The
// assignment is deliberately neutral: whether a booking existed is unknown
// here (zero or several candidate slots) — "Ungeplant, ohne Buchung" is
// reserved for persisted walk-in slot rows (is_unplanned).
func unassignedExportRow(attendance *studentpresence.Attendance) lists.Row {
	return lists.Row{Values: map[lists.ColumnID]string{
		attendanceColumnDate: calendar.Date(attendance.Date).Format(attendanceExportDateLayout), attendanceColumnOffering: "Ohne Zuordnung",
		attendanceColumnWindow: exportOptionalTime(&attendance.CheckInTime) + "–" + exportOptionalTime(attendance.CheckOutTime),
		attendanceColumnStatus: "Anwesend", attendanceColumnCheckIn: exportOptionalTime(&attendance.CheckInTime),
		attendanceColumnCheckOut: exportOptionalTime(attendance.CheckOutTime), attendanceColumnAssignment: "Nicht zugeordnet",
		attendanceColumnNote: optionalString(attendance.CheckOutNote),
	}}
}

func exportOptionalTime(value *time.Time) string {
	if value == nil {
		return "–"
	}
	return value.In(calendar.Berlin).Format("15:04")
}

func attendanceSlotStatusLabel(status string, substatus *string) string {
	if substatus != nil {
		switch *substatus {
		case "sick":
			return "Krank"
		case "excused":
			return "Entschuldigt"
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

func optionalString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
