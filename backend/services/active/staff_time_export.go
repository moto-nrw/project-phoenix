package active

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
)

// Cross-staff time-tracking export (#1417 Tranche 2b).
//
// The number determination is strictly separated from the serialization: the
// rows come out of the UNCHANGED month math (via the #1986 prefetch adapters),
// and each output format is a writer over those typed rows. The upcoming DATEV
// writers (LODAS ASCII, Lohn und Gehalt ASCII) will serialize the same
// MonthExportRow values — they must never rebuild the arithmetic.

// Accepted request values.
const (
	ExportGranularityMonth = "month"
	ExportGranularityDay   = "day"

	ExportFormatCSV  = "csv"
	ExportFormatXLSX = "xlsx"
	// DATEV formats (#1417 2b final part): month-granular Bewegungsdaten
	// files, one line per staff member and configured Lohnart.
	ExportFormatDatevLodas = "datev_lodas"
	ExportFormatDatevLug   = "datev_lug"

	ExportTimeHHMM    = "hhmm"
	ExportTimeDecimal = "decimal"
)

// ErrTimeExportInvalid marks a caller mistake in an export request — HTTP 400.
var ErrTimeExportInvalid = errors.New("invalid time export request")

// TimeExportRequest selects what to export. Month == 0 means the whole year
// (January through the current month for the running year).
type TimeExportRequest struct {
	Year        int
	Month       int
	Granularity string
	Format      string
	TimeFormat  string
}

// MonthExportRow is one staff member's month as the payroll file shows it.
// Every minute value is copied from the MonthSummary of the very same code
// path the Monatskarte uses — no arithmetic happens on the way here except
// splitting the adjustment sum by type (payout / comp_time / reset are the
// categories that later become Lohnarten).
type MonthExportRow struct {
	StaffID        int64
	FirstName      string
	LastName       string
	EmploymentType string
	// PersonnelNumber is the payroll system's Personalnummer ("" = none).
	// Only the DATEV writers consume it; CSV/XLSX stay name-based.
	PersonnelNumber string
	Year            int
	Month           int

	CarryInMinutes          int
	TargetMinutes           int
	ActualMinutes           int
	CreditedSickMinutes     int
	CreditedVacationMinutes int
	CreditedTrainingMinutes int
	CreditedOtherMinutes    int
	SickDays                float64
	VacationDays            float64
	TrainingDays            float64

	PayoutMinutes   int
	CompTimeMinutes int
	ResetMinutes    int

	BalanceMinutes int
	// ClosingBalanceMinutes is the AUTHORITATIVE month-end carry: the frozen
	// value for a closed month, the live value otherwise. This is the number
	// the following month really starts from, so it is the number the payroll
	// file must carry — the same rule the overview list applies (#1986).
	ClosingBalanceMinutes int
	// DriftMinutes is how far the live recomputation has moved away from the
	// frozen value since the close (0 for open months). Shown instead of
	// silently smoothing the gap — mirroring the Monatskarte (#1995).
	DriftMinutes int
	IsClosed     bool
	ClosedAt     *time.Time
}

// ExportFile is a rendered download.
type ExportFile struct {
	Data        []byte
	Filename    string
	ContentType string
}

// StaffTimeExportService renders the cross-staff export (#1417 2b). It writes
// a GDPR access-audit row for every export; if that write fails, no file is
// produced (same contract as the enrollment exports).
type StaffTimeExportService interface {
	Export(ctx context.Context, req TimeExportRequest, actorAccountID int64, actorRole string) (*ExportFile, error)
	// DatevReport is the preflight for the DATEV formats: the same build as
	// the file, returned as a visible report instead of a download.
	DatevReport(ctx context.Context, req TimeExportRequest) (*DatevExportReport, error)
}

type staffTimeExportService struct {
	overview      StaffOverviewService
	sessions      WorkSessionService
	staffRepo     userModels.StaffRepository
	auditRepo     auditModels.DataAccessLogRepository
	payrollStatus configSvc.PayrollStatusGetter
	logger        *slog.Logger

	// todayFunc is a test hook; production uses timezone.TodayDate.
	todayFunc func() timezone.Date
	// nowFunc is a test hook for the audit timestamp; production uses timezone.Now.
	nowFunc func() time.Time
}

func NewStaffTimeExportService(
	overview StaffOverviewService,
	sessions WorkSessionService,
	staffRepo userModels.StaffRepository,
	auditRepo auditModels.DataAccessLogRepository,
	payrollStatus configSvc.PayrollStatusGetter,
	logger *slog.Logger,
) *staffTimeExportService {
	return &staffTimeExportService{
		overview:      overview,
		sessions:      sessions,
		staffRepo:     staffRepo,
		auditRepo:     auditRepo,
		payrollStatus: payrollStatus,
		logger:        logger,
	}
}

func (s *staffTimeExportService) today() timezone.Date {
	if s.todayFunc != nil {
		return s.todayFunc()
	}
	return timezone.TodayDate()
}

func (s *staffTimeExportService) now() time.Time {
	if s.nowFunc != nil {
		return s.nowFunc()
	}
	return timezone.Now()
}

// validate normalizes defaults and rejects everything the writers would
// otherwise have to guess about.
func (s *staffTimeExportService) validate(req *TimeExportRequest) error {
	switch req.Granularity {
	case "":
		req.Granularity = ExportGranularityMonth
	case ExportGranularityMonth, ExportGranularityDay:
	default:
		return fmt.Errorf("%w: granularity must be %s or %s", ErrTimeExportInvalid, ExportGranularityMonth, ExportGranularityDay)
	}
	switch req.Format {
	case "":
		req.Format = ExportFormatCSV
	case ExportFormatCSV, ExportFormatXLSX, ExportFormatDatevLodas, ExportFormatDatevLug:
	default:
		return fmt.Errorf("%w: format must be %s, %s, %s or %s", ErrTimeExportInvalid,
			ExportFormatCSV, ExportFormatXLSX, ExportFormatDatevLodas, ExportFormatDatevLug)
	}
	switch req.TimeFormat {
	case "":
		req.TimeFormat = ExportTimeHHMM
	case ExportTimeHHMM, ExportTimeDecimal:
	default:
		return fmt.Errorf("%w: time_format must be %s or %s", ErrTimeExportInvalid, ExportTimeHHMM, ExportTimeDecimal)
	}
	if req.Year < 2000 || req.Year > 2100 {
		return fmt.Errorf("%w: year out of range", ErrTimeExportInvalid)
	}
	if req.Month < 0 || req.Month > 12 {
		return fmt.Errorf("%w: month out of range", ErrTimeExportInvalid)
	}
	today := s.today()
	if req.Year > today.Year {
		return fmt.Errorf("%w: year %d lies in the future", ErrTimeExportInvalid, req.Year)
	}
	// The running month is allowed (it exports as "offen"); a future month is
	// not — its Soll would be the full month against zero Ist, the same reason
	// the overview rejects it (#1988).
	if req.Month != 0 && monthOf(today).before(monthKey{Year: req.Year, Month: req.Month}) {
		return fmt.Errorf("%w: month %04d-%02d lies in the future", ErrTimeExportInvalid, req.Year, req.Month)
	}
	return nil
}

// periodBounds returns the calendar range the request covers. For a
// current-year export the range ends with the running month.
func (s *staffTimeExportService) periodBounds(req TimeExportRequest) (from, to timezone.Date) {
	if req.Month != 0 {
		key := monthKey{Year: req.Year, Month: req.Month}
		return key.firstDay(), key.lastDay()
	}
	last := monthKey{Year: req.Year, Month: 12}
	if today := s.today(); req.Year == today.Year {
		last = monthOf(today)
	}
	return timezone.NewDate(req.Year, time.January, 1), last.lastDay()
}

func (s *staffTimeExportService) Export(ctx context.Context, req TimeExportRequest, actorAccountID int64, actorRole string) (*ExportFile, error) {
	if err := s.validate(&req); err != nil {
		return nil, err
	}
	from, to := s.periodBounds(req)

	var (
		file      *ExportFile
		rowCount  int
		staffSeen int
		extra     map[string]interface{}
		err       error
	)
	switch {
	case req.Format == ExportFormatDatevLodas || req.Format == ExportFormatDatevLug:
		var report *DatevExportReport
		file, report, err = s.exportDatev(ctx, req)
		if report != nil {
			rowCount = report.LineCount
			staffSeen = report.StaffExported
			// The skipped staff are part of the audit trail: the file is
			// deliberately incomplete, and the record must say so.
			extra = map[string]interface{}{"skipped_staff_count": len(report.StaffSkipped)}
		}
	case req.Granularity == ExportGranularityDay:
		file, rowCount, staffSeen, err = s.exportDays(ctx, req, from, to)
	default:
		file, rowCount, staffSeen, err = s.exportMonths(ctx, req)
	}
	if err != nil {
		return nil, err
	}

	if err := s.writeAudit(ctx, req, from, to, actorAccountID, actorRole, rowCount, staffSeen, extra); err != nil {
		return nil, err
	}
	return file, nil
}

// --- month granularity -----------------------------------------------------

func (s *staffTimeExportService) exportMonths(ctx context.Context, req TimeExportRequest) (*ExportFile, int, int, error) {
	rows, err := s.overview.GetMonthExportRows(ctx, req.Year, req.Month)
	if err != nil {
		return nil, 0, 0, err
	}
	staffIDs := make(map[int64]bool, len(rows))
	for _, row := range rows {
		staffIDs[row.StaffID] = true
	}

	var file ExportFile
	if req.Format == ExportFormatXLSX {
		file.Data, err = writeMonthXLSX(rows, req.TimeFormat)
		file.ContentType = contentTypeXLSX
	} else {
		file.Data, err = writeMonthCSV(rows, req.TimeFormat)
		file.ContentType = contentTypeCSV
	}
	if err != nil {
		return nil, 0, 0, err
	}
	file.Filename = exportFilename("zeiterfassung_alle", req)
	return &file, len(rows), len(staffIDs), nil
}

// --- day granularity -------------------------------------------------------

// exportDays renders the ArbZG evidence view: the per-day rows of the existing
// single-staff export, one block per staff member, prefixed with the name.
func (s *staffTimeExportService) exportDays(ctx context.Context, req TimeExportRequest, from, to timezone.Date) (*ExportFile, int, int, error) {
	staffMembers, err := s.staffRepo.ListAllWithPerson(ctx)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to list staff for export: %w", err)
	}
	sortStaffByName(staffMembers)

	staffIDs := make([]int64, len(staffMembers))
	for i, staff := range staffMembers {
		staffIDs[i] = staff.ID
	}
	dayRowsByStaff, err := s.sessions.DayExportRowsByStaffIDs(ctx, staffIDs, from, to)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to build day export rows: %w", err)
	}

	var rows []dayExportBlockRow
	for _, staff := range staffMembers {
		firstName, lastName := staffNames(staff)
		for _, dayRow := range dayRowsByStaff[staff.ID] {
			rows = append(rows, dayExportBlockRow{
				LastName:  lastName,
				FirstName: firstName,
				Cells:     dayRow.Cells,
			})
		}
	}

	var file ExportFile
	if req.Format == ExportFormatXLSX {
		file.Data, err = writeDayXLSX(rows)
		file.ContentType = contentTypeXLSX
	} else {
		file.Data, err = writeDayCSV(rows)
		file.ContentType = contentTypeCSV
	}
	if err != nil {
		return nil, 0, 0, err
	}
	file.Filename = exportFilename("zeiterfassung_alle_tage", req)
	return &file, len(rows), len(staffMembers), nil
}

// --- audit -----------------------------------------------------------------

// writeAudit records the export in audit.data_access_logs (same table and
// contract as the enrollment exports): a cross-staff working-time file is a
// bulk personal-data export, and producing it without a trace would contradict
// the #2001 Änderungsprotokoll. No file leaves the server when this fails.
func (s *staffTimeExportService) writeAudit(
	ctx context.Context,
	req TimeExportRequest,
	from, to timezone.Date,
	actorAccountID int64,
	actorRole string,
	rowCount, staffCount int,
	extra map[string]interface{},
) error {
	if s.auditRepo == nil {
		return errors.New("time export: audit repository not configured")
	}
	if actorAccountID <= 0 {
		return errors.New("time export: actor account id required")
	}
	if strings.TrimSpace(actorRole) == "" {
		actorRole = "unknown"
	}
	entry := &auditModels.DataAccessLog{
		ActorAccountID: actorAccountID,
		ActorRole:      actorRole,
		ResourceType:   "time_tracking_export",
		RangeStart:     from.BerlinMidnight(),
		RangeEnd:       to.EndOfDay(),
		AccessedAt:     s.now(),
		Metadata: map[string]interface{}{
			"format":      req.Format,
			"granularity": req.Granularity,
			"time_format": req.TimeFormat,
			"year":        req.Year,
			"month":       req.Month,
			"row_count":   rowCount,
			"staff_count": staffCount,
		},
	}
	for key, value := range extra {
		entry.Metadata[key] = value
	}
	if err := s.auditRepo.Create(ctx, entry); err != nil {
		return fmt.Errorf("time export: audit write failed: %w", err)
	}
	return nil
}

// --- shared helpers --------------------------------------------------------

func exportFilename(base string, req TimeExportRequest) string {
	period := fmt.Sprintf("%04d", req.Year)
	if req.Month != 0 {
		period = fmt.Sprintf("%04d-%02d", req.Year, req.Month)
	}
	return fmt.Sprintf("%s_%s.%s", base, period, req.Format)
}

func staffNames(staff *userModels.Staff) (firstName, lastName string) {
	if staff.Person != nil {
		return staff.Person.FirstName, staff.Person.LastName
	}
	return "", ""
}

// sortStaffByName orders like the overview list: last name, first name, with
// the staff-ID tiebreak that keeps repeated exports byte-identical.
func sortStaffByName(staffMembers []*userModels.Staff) {
	slices.SortStableFunc(staffMembers, func(a, b *userModels.Staff) int {
		aFirst, aLast := staffNames(a)
		bFirst, bLast := staffNames(b)
		order := cmp.Compare(
			strings.ToLower(aLast+" "+aFirst),
			strings.ToLower(bLast+" "+bFirst),
		)
		return cmp.Or(order, cmp.Compare(a.ID, b.ID))
	})
}
