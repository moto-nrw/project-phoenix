package timetracking

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// German weekday names for export
var germanWeekdays = [7]string{"Sonntag", "Montag", "Dienstag", "Mittwoch", "Donnerstag", "Freitag", "Samstag"}

// German absence type labels for export
var germanAbsenceTypeLabels = map[string]string{
	AbsenceTypeSick:     "Krank",
	AbsenceTypeVacation: "Urlaub",
	AbsenceTypeTraining: "Fortbildung",
	AbsenceTypeOther:    "Sonstige",
	AbsenceTypeCompTime: "Freizeitausgleich",
}

// exportRow represents a single row in the export (either a work session or an absence day)
type exportRow struct {
	Date calendar.Date
	// Start orders multiple blocks of the same day chronologically (#2402).
	// Absence rows carry the zero value and sort before the day's sessions.
	Start time.Time
	Row   []string
}

// timeTrackingConfidentialityNote matches the wording of the other
// printed exports (see slotlists.confidentialityNote).
const timeTrackingConfidentialityNote = "Vertraulich, nur für berechtigte Personen. Nach Gebrauch sicher vernichten."

// timeTrackingColumns is the single-staff export's column set; the labels
// double as the CSV/XLSX header row.
func timeTrackingColumns() []TimeTrackingColumn {
	return []TimeTrackingColumn{
		{ID: "tt_date", Label: "Datum"},
		{ID: "tt_weekday", Label: "Wochentag"},
		{ID: "tt_start", Label: "Start"},
		{ID: "tt_end", Label: "Ende"},
		{ID: "tt_break", Label: "Pause (Min)"},
		{ID: "tt_net", Label: "Netto (Std)"},
		{ID: "tt_status", Label: "Status"},
		{ID: "tt_source", Label: "Quelle"},
		{ID: "tt_notes", Label: "Bemerkungen"},
	}
}

func timeTrackingHeaders() []string {
	columns := timeTrackingColumns()
	headers := make([]string, len(columns))
	for i, column := range columns {
		headers[i] = column.Label
	}
	return headers
}

func (s *workSessionService) ExportSessions(ctx context.Context, staffID int64, from, to calendar.Date, format string) (*ExportFile, error) {
	// Deliberately the stored-date read, not the intersecting one: an export
	// row is a whole block, and a night block that reaches into the next month
	// must be listed once, under the month it started in. Reading by interval
	// would put it into both months' files and double it in any sum drawn from
	// them. The absences below are clamped to the same range for that reason.
	historyResp, err := s.GetHistory(ctx, staffID, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to get sessions for export: %w", err)
	}

	absences, err := s.exportAbsences(ctx, staffID, from, to)
	if err != nil {
		return nil, err
	}

	// Build merged rows sorted by date
	rows := s.buildExportRows(historyResp.Sessions, clampAbsencesToRange(absences, from, to))

	fromStr := from.String()
	toStr := to.String()

	cells := make([][]string, 0, len(rows))
	for _, er := range rows {
		cells = append(cells, er.Row)
	}

	switch format {
	case "pdf":
		return s.renderSessionPDF(ctx, staffID, rows, from, to, fromStr, toStr)
	case "xlsx":
		data, err := s.renderWorkbook("Zeiterfassung", timeTrackingHeaders(), stringsRowsToAny(cells), nil)
		if err != nil {
			return nil, err
		}
		return &ExportFile{
			Data:        data,
			Filename:    fmt.Sprintf("zeiterfassung_%s_%s.xlsx", fromStr, toStr),
			ContentType: contentTypeXLSX,
		}, nil
	default:
		// Bemerkungen (column 8) carries untrusted free text — formula-escape
		// it like the cross-staff CSV always has.
		data, err := writeExportCSV(timeTrackingHeaders(), cells, []int{8})
		if err != nil {
			return nil, err
		}
		return &ExportFile{
			Data:        data,
			Filename:    fmt.Sprintf("zeiterfassung_%s_%s.csv", fromStr, toStr),
			ContentType: contentTypeCSV,
		}, nil
	}
}

// exportAbsences loads the absences of the export's date range, labelled
// with the school's own Abwesenheitsarten so the export prints the wording
// the school entered, not the generic "Sonstige" (#2403).
func (s *workSessionService) exportAbsences(ctx context.Context, staffID int64, from, to calendar.Date) ([]*StaffAbsence, error) {
	if s.absenceRepo == nil {
		return nil, nil
	}
	absences, err := s.absenceRepo.GetByStaffAndDateRange(ctx, staffID, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to get absences for export: %w", err)
	}
	StampAbsenceTypeLabels(ctx, s.absenceTypes, absences, s.getLogger())
	return absences, nil
}

// renderSessionPDF applies the shared print layout to the prepared session rows.
func (s *workSessionService) renderSessionPDF(ctx context.Context, staffID int64, rows []exportRow, from, to calendar.Date, fromStr, toStr string) (*ExportFile, error) {
	doc, err := s.buildTimeTrackingDocument(ctx, staffID, rows, from, to)
	if err != nil {
		return nil, err
	}
	if s.renderPDF == nil {
		return nil, errors.New("time-tracking PDF renderer is not configured")
	}
	data, err := s.renderPDF(doc)
	if err != nil {
		return nil, err
	}
	return &ExportFile{
		Data:        data,
		Filename:    fmt.Sprintf("zeiterfassung_%s_%s.pdf", fromStr, toStr),
		ContentType: "application/pdf",
	}, nil
}

// buildTimeTrackingDocument shapes the merged export rows into the shared
// listexport document: title block with the staff member's name, the
// requested period as a filter pill, and the confidentiality footer.
func (s *workSessionService) buildTimeTrackingDocument(ctx context.Context, staffID int64, rows []exportRow, from, to calendar.Date) (TimeTrackingDocument, error) {
	subtitle := "Arbeitszeiten und Abwesenheiten"
	if s.staffRepo != nil {
		staffMap, err := s.staffRepo.StaffNames(ctx, []int64{staffID})
		if err != nil {
			return TimeTrackingDocument{}, fmt.Errorf("failed to load staff for export: %w", err)
		}
		if staff, ok := staffMap[staffID]; ok {
			subtitle = staff.FirstName + " " + staff.LastName
		}
	}

	columns := timeTrackingColumns()
	docRows := make([]TimeTrackingRow, 0, len(rows))
	for _, er := range rows {
		values := make(map[string]string, len(columns))
		for i, column := range columns {
			if i < len(er.Row) {
				values[column.ID] = er.Row[i]
			}
		}
		docRows = append(docRows, TimeTrackingRow{Values: values})
	}

	return TimeTrackingDocument{
		Title:       "Zeiterfassung",
		Subtitle:    subtitle,
		GeneratedAt: s.now(),
		Filters:     []string{"Zeitraum: " + from.Format("02.01.2006") + " bis " + to.Format("02.01.2006")},
		Columns:     columns,
		Rows:        docRows,
		Footer:      timeTrackingConfidentialityNote,
	}, nil
}

// clampAbsencesToRange returns shallow copies whose expanded day ranges stay
// inside the requested export period. Repository range lookups deliberately
// return overlapping absences, including records that start before `from` or
// end after `to`.
func clampAbsencesToRange(absences []*StaffAbsence, from, to calendar.Date) []*StaffAbsence {
	clamped := make([]*StaffAbsence, 0, len(absences))
	for _, absence := range absences {
		if absence == nil || absence.DateEnd.Before(from) || to.Before(absence.DateStart) {
			continue
		}
		copy := *absence
		if copy.DateStart.Before(from) {
			copy.DateStart = from
		}
		if to.Before(copy.DateEnd) {
			copy.DateEnd = to
		}
		clamped = append(clamped, &copy)
	}
	return clamped
}

// buildExportRows merges session rows and absence rows, sorted by date
func (s *workSessionService) buildExportRows(sessions []*SessionResponse, absences []*StaffAbsence) []exportRow {
	var rows []exportRow

	// Add session rows (a day can carry several blocks since #2402)
	for _, sr := range sessions {
		rows = append(rows, exportRow{
			Date:  sr.Date,
			Start: sr.CheckInTime,
			Row:   s.sessionToRow(sr),
		})
	}

	// Add absence rows (one row per day in the absence range)
	for _, absence := range absences {
		if absence.Status != AbsenceStatusReported &&
			absence.Status != AbsenceStatusApproved {
			continue
		}
		// The school's own wording wins when the absence carries one (#2403);
		// the standard German label is the fallback, the raw type the last
		// resort for a value this map has not caught up with.
		label := absence.AbsenceTypeLabel
		if label == "" {
			label = germanAbsenceTypeLabels[absence.AbsenceType]
		}
		if label == "" {
			label = absence.AbsenceType
		}

		d := absence.DateStart
		for !d.After(absence.DateEnd) {
			datum := d.Format("02.01.2006")
			wochentag := germanWeekdays[d.Weekday()]
			// Column layout must match the export header (9 cells):
			// Datum, Wochentag, Start, Ende, Pause, Netto, Status, Quelle, Bemerkungen.
			// Absences have no Quelle (the staff member did not stamp at all),
			// so that cell stays empty rather than borrowing the "-" sentinel
			// already used by quelleLabel for legacy work_sessions of unknown
			// channel. Those are two different facts and must read distinctly.
			rows = append(rows, exportRow{
				Date: d,
				Row:  []string{datum, wochentag, "--", "--", "--", "--", label, "", absence.Note},
			})
			d = d.AddDays(1)
		}
	}

	// Sort by date, blocks of the same day by check-in time
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Date != rows[j].Date {
			return rows[i].Date.Before(rows[j].Date)
		}
		return rows[i].Start.Before(rows[j].Start)
	})

	return rows
}

func (s *workSessionService) sessionToRow(sr *SessionResponse) []string {
	sess := sr.WorkSession

	datum := sess.Date.Format("02.01.2006")
	wochentag := germanWeekdays[sess.Date.Weekday()]

	start := sess.CheckInTime.Format("15:04")

	ende := ""
	if sess.CheckOutTime != nil {
		ende = sess.CheckOutTime.Format("15:04")
	}

	// Read the pause from the response-level total, not the model cache: for an
	// open session with a running break the cache (sess.BreakMinutes) still holds
	// only ENDED breaks and would print "Pause 0" next to a Netto that already
	// deducts the running break (sr.NetMinutes). sr.BreakMinutes is the live
	// total that pairs with that Netto, so the export row stays internally
	// consistent (#1842).
	pauseMin := strconv.Itoa(sr.BreakMinutes)

	// Net as "Xh YYmin"
	netMins := sr.NetMinutes
	h := netMins / 60
	m := netMins % 60
	netto := fmt.Sprintf("%dh %02dmin", h, m)

	// Status carries the underlying work mode and is never overwritten by
	// system events. The OGS-Leitung must always be able to see whether
	// the row was Vor Ort or Homeoffice, even if the session was later
	// auto-closed or manually corrected.
	status := "Vor Ort"
	if sess.Status == WorkSessionStatusHomeOffice {
		status = "Homeoffice"
	}

	// Quelle records the originating channel, with system-event overlays:
	// a manual correction is the most informative signal and wins; an
	// auto-checkout still tells the reader the session was closed by the
	// scheduler rather than the staff member; otherwise the persisted
	// source (App / NFC) is shown verbatim.
	quelle := quelleLabel(sess.Source)
	if sess.AutoCheckedOut {
		quelle = "Auto-Checkout"
	}
	if sr.EditCount > 0 {
		quelle = "Manuell korrigiert"
	}

	return []string{datum, wochentag, start, ende, pauseMin, netto, status, quelle, sess.Notes}
}

// quelleLabel renders the export "Quelle" cell from the persisted source.
// 'unknown' marks rows that pre-date migration 1.15.54. No channel was ever
// recorded, so we render "-" rather than guess. Anything else falls through
// to App, matching the DB column default for in-flight rows that may exist
// briefly between migration and new-server boot.
func quelleLabel(source string) string {
	switch source {
	case WorkSessionSourceNFC:
		return "NFC"
	case WorkSessionSourceUnknown:
		return "-"
	default:
		return "App"
	}
}
