package active

import (
	"context"
	"errors"
	"fmt"
	"strings"

	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
)

// DATEV writers (#1417 2b, final part). Both serialize the UNCHANGED
// MonthExportRow values — one line per staff member and configured Lohnart,
// value copied 1:1 from exactly one row field. No arithmetic happens here
// beyond rendering minutes as decimal hours.
//
// Format research (sources in the PR description):
//   - LODAS ASCII is self-describing: [Allgemein] header with Berater-/
//     MandantenNr, [Satzbeschreibung] declaring the record layout against
//     DATEV's table fields, then [Bewegungsdaten]. The Schnittstellenhandbuch
//     requires ANSI encoding and CR/LF line ends.
//   - Lohn und Gehalt ASCII has no header block: the tax consultant creates
//     the Importbeschreibung themselves in the ASCII-Importassistent. We write
//     the widespread 11-field Bewegungsdaten layout (Personalnummer;
//     Kalendertag;Ausfallschlüssel;Lohnartennummer;Stundenanzahl;Tagesanzahl;
//     Wert;Abw. Faktor;Abw. Lohnveränderung;Kostenstelle;Kostenträger) with
//     one header row the import description can skip.
//
// Encoding for both: Windows-1252 ("ANSI"), CR/LF, no BOM.

// ErrPayrollConfigIncomplete marks a DATEV export refused because the payroll
// configuration cannot produce a correct file (missing LODAS header, no
// configured Lohnart). Maintained on /payroll — HTTP 400 with the reason.
var ErrPayrollConfigIncomplete = errors.New("payroll configuration incomplete")

// DatevSkippedStaff is one staff member left out of the file.
type DatevSkippedStaff struct {
	LastName  string `json:"last_name"`
	FirstName string `json:"first_name"`
	Reason    string `json:"reason"`
}

// DatevExportReport is the visible companion of a DATEV file: the dialog
// shows it before the download, and a file is never produced without the
// caller being able to see it ("silently incomplete" must not exist).
type DatevExportReport struct {
	Format string `json:"format"`
	Year   int    `json:"year"`
	Month  int    `json:"month"`
	// LineCount is the number of Bewegungsdaten lines the file carries.
	LineCount int `json:"line_count"`
	// StaffExported / StaffSkipped partition the active staff.
	StaffExported int                 `json:"staff_exported"`
	StaffSkipped  []DatevSkippedStaff `json:"staff_skipped"`
	// UnconfiguredCategories lists Lohnart categories without a number —
	// they export no line (decided in #2008).
	UnconfiguredCategories []string `json:"unconfigured_categories"`
	// OpenMonth is true when at least one exported month row is not closed:
	// the values are current, not final.
	OpenMonth bool `json:"open_month"`
}

// datevLine is one Bewegungsdaten booking before serialization.
type datevLine struct {
	personnelNumber string
	lohnart         string
	// exactly one of hoursMinutes / days is set, mirroring the configured unit
	hoursMinutes *int
	days         *float64
}

// datevCategoryValue picks the ONE MonthExportRow field a category exports.
// Returned minutes/days follow the category's configured unit; ok=false means
// no line (zero value). Payout and comp time are stored as negative balance
// deltas; the booked amount is the sign-inverted delta, so a reversal booking
// shows up negative instead of disappearing.
func datevCategoryValue(category configSvc.PayrollCategoryStatus, row MonthExportRow) (minutes int, days float64, useDays bool, ok bool) {
	switch category.ID {
	case "regelarbeit":
		minutes = row.ActualMinutes
	case "plus_stunden":
		// Only a positive month balance is a Plus-Stunden booking; a negative
		// month is an account matter, not a payroll line.
		if row.BalanceMinutes > 0 {
			minutes = row.BalanceMinutes
		}
	case "auszahlung":
		minutes = -row.PayoutMinutes
	case "freizeitausgleich":
		minutes = -row.CompTimeMinutes
	case "krank":
		minutes, days = row.CreditedSickMinutes, row.SickDays
	case "urlaub":
		minutes, days = row.CreditedVacationMinutes, row.VacationDays
	case "fortbildung":
		minutes, days = row.CreditedTrainingMinutes, row.TrainingDays
	default:
		return 0, 0, false, false
	}
	useDays = category.UnitRequired && category.Unit == "tage"
	if useDays {
		return 0, days, true, days != 0
	}
	return minutes, 0, false, minutes != 0
}

// buildDatevLines maps the export rows onto Bewegungsdaten lines and collects
// the report. Categories without a Lohnartnummer export no line; staff
// without a Personalnummer are skipped and reported.
func buildDatevLines(rows []MonthExportRow, status *configSvc.PayrollStatus) ([]datevLine, *DatevExportReport) {
	report := &DatevExportReport{
		StaffSkipped:           []DatevSkippedStaff{},
		UnconfiguredCategories: []string{},
	}
	var configured []configSvc.PayrollCategoryStatus
	for _, category := range status.Categories {
		if category.Number == "" || (category.UnitRequired && category.Unit == "") {
			report.UnconfiguredCategories = append(report.UnconfiguredCategories, category.Label)
			continue
		}
		configured = append(configured, category)
	}

	var lines []datevLine
	exported := make(map[int64]bool)
	skipped := make(map[int64]bool)
	for _, row := range rows {
		if row.PersonnelNumber == "" {
			if !skipped[row.StaffID] {
				skipped[row.StaffID] = true
				report.StaffSkipped = append(report.StaffSkipped, DatevSkippedStaff{
					LastName:  row.LastName,
					FirstName: row.FirstName,
					Reason:    "keine Personalnummer",
				})
			}
			continue
		}
		exported[row.StaffID] = true
		if !row.IsClosed {
			report.OpenMonth = true
		}
		for _, category := range configured {
			minutes, days, useDays, ok := datevCategoryValue(category, row)
			if !ok {
				continue
			}
			line := datevLine{personnelNumber: row.PersonnelNumber, lohnart: category.Number}
			if useDays {
				d := days
				line.days = &d
			} else {
				m := minutes
				line.hoursMinutes = &m
			}
			lines = append(lines, line)
		}
	}
	report.LineCount = len(lines)
	report.StaffExported = len(exported)
	return lines, report
}

// datevHours renders minutes as decimal hours with a German comma ("160,00").
func datevHours(minutes int) string {
	return formatExportMinutes(minutes, ExportTimeDecimal)
}

// datevDays renders a day count with a German comma ("1,5").
func datevDays(days float64) string {
	return formatExportDays(days)
}

// --- LODAS ------------------------------------------------------------------

// LODAS record types: 1 = hour bookings, 2 = day bookings. The field order of
// the data lines is fixed by this [Satzbeschreibung]; both tables are from the
// Schnittstellenhandbuch (u_lod_bwd_buchung_stunden / u_lod_bwd_buchung_tage).
const (
	lodasRecordHours = "1"
	lodasRecordDays  = "2"
)

func writeDatevLodas(lines []datevLine, status *configSvc.PayrollStatus, year, month int) []byte {
	period := fmt.Sprintf("01.%02d.%04d", month, year)
	var b strings.Builder
	b.WriteString("[Allgemein]\n")
	b.WriteString("Ziel=LODAS\n")
	b.WriteString("Version_SST=1.0\n")
	b.WriteString("BeraterNr=" + status.Beraternummer + "\n")
	b.WriteString("MandantenNr=" + status.Mandantennummer + "\n")
	b.WriteString("Feldtrennzeichen=;\n")
	b.WriteString("Zahlenkomma=,\n")
	b.WriteString("Datumsformat=TT.MM.JJJJ\n")
	b.WriteString("\n[Satzbeschreibung]\n")
	b.WriteString(lodasRecordHours + ";u_lod_bwd_buchung_stunden;pnr#bwd;abrechnung_zeitraum#bwd;stunden#bwd;la_eigene#bwd;\n")
	b.WriteString(lodasRecordDays + ";u_lod_bwd_buchung_tage;pnr#bwd;abrechnung_zeitraum#bwd;tage#bwd;la_eigene#bwd;\n")
	b.WriteString("\n[Bewegungsdaten]\n")
	for _, line := range lines {
		if line.days != nil {
			b.WriteString(strings.Join([]string{lodasRecordDays, line.personnelNumber, period, datevDays(*line.days), line.lohnart}, ";") + ";\n")
			continue
		}
		b.WriteString(strings.Join([]string{lodasRecordHours, line.personnelNumber, period, datevHours(*line.hoursMinutes), line.lohnart}, ";") + ";\n")
	}
	return encodeDatevANSI(b.String())
}

// --- Lohn und Gehalt --------------------------------------------------------

// The 11-field Bewegungsdaten layout. Kalendertag stays empty: the bookings
// are month aggregates, the Abrechnungszeitraum is chosen at import time.
// Ausfallschlüssel, Wert, factors and cost centers stay empty — we book
// amounts per Lohnart, nothing else.
var datevLugHeader = []string{
	"Personalnummer", "Kalendertag", "Ausfallschluessel", "Lohnartennummer",
	"Stundenanzahl", "Tagesanzahl", "Wert",
	"Abw. Faktor", "Abw. Lohnveraenderung", "Kostenstellennummer", "Kostentraeger",
}

func writeDatevLug(lines []datevLine) []byte {
	var b strings.Builder
	b.WriteString(strings.Join(datevLugHeader, ";") + "\n")
	for _, line := range lines {
		hours, days := "", ""
		if line.days != nil {
			days = datevDays(*line.days)
		} else {
			hours = datevHours(*line.hoursMinutes)
		}
		fields := []string{line.personnelNumber, "", "", line.lohnart, hours, days, "", "", "", "", ""}
		b.WriteString(strings.Join(fields, ";") + "\n")
	}
	return encodeDatevANSI(b.String())
}

// --- encoding ---------------------------------------------------------------

// encodeDatevANSI converts to Windows-1252 with CR/LF line ends — the
// encoding the LODAS Schnittstellenhandbuch prescribes ("ANSI-Code", CR/LF
// record ends) and the Lohn und Gehalt import default. Characters outside
// CP1252 are substitution-encoded rather than failing the whole file.
func encodeDatevANSI(s string) []byte {
	s = strings.ReplaceAll(s, "\n", "\r\n")
	encoder := encoding.ReplaceUnsupported(charmap.Windows1252.NewEncoder())
	out, err := encoder.Bytes([]byte(s))
	if err != nil {
		// ReplaceUnsupported never errors; keep the fallback total anyway.
		return []byte(s)
	}
	return out
}

// --- service entry points ---------------------------------------------------

// validateDatev enforces the format decisions: single month only (payroll is
// monthly; both formats book per Abrechnungszeitraum), and a payroll
// configuration that can produce a correct file. The open month stays allowed
// — the report says so.
func (s *staffTimeExportService) validateDatevRequest(req TimeExportRequest) error {
	if req.Month == 0 {
		return fmt.Errorf("%w: DATEV export requires a single month", ErrTimeExportInvalid)
	}
	if req.Granularity == ExportGranularityDay {
		return fmt.Errorf("%w: DATEV export is month-granular", ErrTimeExportInvalid)
	}
	return nil
}

func (s *staffTimeExportService) payrollStatusForDatev(ctx context.Context, format string) (*configSvc.PayrollStatus, error) {
	if s.payrollStatus == nil {
		return nil, errors.New("datev export: payroll status service not configured")
	}
	status, err := s.payrollStatus.GetPayrollStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load payroll status: %w", err)
	}
	if format == ExportFormatDatevLodas && !status.LodasHeaderComplete {
		return nil, fmt.Errorf("%w: Berater- und Mandantennummer fehlen", ErrPayrollConfigIncomplete)
	}
	if status.ConfiguredCategories == 0 {
		return nil, fmt.Errorf("%w: keine Lohnart konfiguriert", ErrPayrollConfigIncomplete)
	}
	return status, nil
}

// buildDatev computes lines + report for both the download and the report
// endpoint — one code path, so the report can never disagree with the file.
func (s *staffTimeExportService) buildDatev(ctx context.Context, req TimeExportRequest) ([]datevLine, *DatevExportReport, *configSvc.PayrollStatus, error) {
	if err := s.validateDatevRequest(req); err != nil {
		return nil, nil, nil, err
	}
	status, err := s.payrollStatusForDatev(ctx, req.Format)
	if err != nil {
		return nil, nil, nil, err
	}
	rows, err := s.overview.GetMonthExportRows(ctx, req.Year, req.Month)
	if err != nil {
		return nil, nil, nil, err
	}
	lines, report := buildDatevLines(rows, status)
	report.Format = req.Format
	report.Year = req.Year
	report.Month = req.Month
	return lines, report, status, nil
}

// DatevReport is the preflight the export dialog shows before the download.
// It runs the exact same build as the file and writes no audit row — nothing
// leaves the system.
func (s *staffTimeExportService) DatevReport(ctx context.Context, req TimeExportRequest) (*DatevExportReport, error) {
	if err := s.validate(&req); err != nil {
		return nil, err
	}
	_, report, _, err := s.buildDatev(ctx, req)
	return report, err
}

// exportDatev renders the requested DATEV file. Same audit contract as the
// other formats: no file without the audit row.
func (s *staffTimeExportService) exportDatev(ctx context.Context, req TimeExportRequest) (*ExportFile, *DatevExportReport, error) {
	lines, report, status, err := s.buildDatev(ctx, req)
	if err != nil {
		return nil, nil, err
	}

	file := ExportFile{ContentType: "text/plain; charset=windows-1252"}
	if req.Format == ExportFormatDatevLodas {
		file.Data = writeDatevLodas(lines, status, req.Year, req.Month)
	} else {
		file.Data = writeDatevLug(lines)
	}
	file.Filename = fmt.Sprintf("%s_%04d-%02d.txt", req.Format, req.Year, req.Month)
	return &file, report, nil
}
