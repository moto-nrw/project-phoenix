package active

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

// ============================================================================
// Helper to create test service with absence repo
// ============================================================================

type wsMockStaffRepository struct {
	userModels.StaffRepository
	findWithPersonByIDsFunc func(context.Context, []int64) (map[int64]*userModels.Staff, error)
}

func (m *wsMockStaffRepository) FindWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*userModels.Staff, error) {
	return m.findWithPersonByIDsFunc(ctx, ids)
}

func wsCreateTestServiceWithAbsenceRepo() (*workSessionService, *wsMockWorkSessionRepository, *wsMockWorkSessionBreakRepository, *wsMockWorkSessionEditRepository, *wsMockStaffAbsenceRepository, *wsMockGroupSupervisorRepository) {
	sessionRepo := &wsMockWorkSessionRepository{}
	breakRepo := &wsMockWorkSessionBreakRepository{}
	auditRepo := &wsMockWorkSessionEditRepository{}
	absenceRepo := &wsMockStaffAbsenceRepository{}
	supervisorRepo := &wsMockGroupSupervisorRepository{}

	service := &workSessionService{
		repo:           sessionRepo,
		breakRepo:      breakRepo,
		auditRepo:      auditRepo,
		absenceRepo:    absenceRepo,
		supervisorRepo: supervisorRepo,
	}

	return service, sessionRepo, breakRepo, auditRepo, absenceRepo, supervisorRepo
}

// ============================================================================
// ExportSessions Tests
// ============================================================================

func TestWSExportSessions_CSV_Success(t *testing.T) {
	t.Parallel()

	svc, sessionRepo, breakRepo, auditRepo, absenceRepo, _ := wsCreateTestServiceWithAbsenceRepo()
	ctx := context.Background()
	staffID := int64(100)
	from := timezone.NewDate(2024, 1, 1)
	to := timezone.NewDate(2024, 1, 7)

	date1 := timezone.NewDate(2024, 1, 2)
	checkIn := time.Date(2024, 1, 2, 8, 0, 0, 0, time.UTC)
	checkOut := time.Date(2024, 1, 2, 16, 30, 0, 0, time.UTC)

	sessionRepo.getHistoryByStaffIDFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.WorkSession, error) {
		return []*activeModels.WorkSession{
			{
				Model:        base.Model{ID: 1},
				StaffID:      staffID,
				Date:         date1,
				Status:       activeModels.WorkSessionStatusPresent,
				CheckInTime:  checkIn,
				CheckOutTime: &checkOut,
				BreakMinutes: 30,
				Notes:        "Test note",
			},
		}, nil
	}

	auditRepo.countBySessionIDsFunc = func(_ context.Context, _ []int64) (map[int64]int, error) {
		return map[int64]int{1: 0}, nil
	}

	breakRepo.getBySessionIDFunc = func(_ context.Context, _ int64) ([]*activeModels.WorkSessionBreak, error) {
		return []*activeModels.WorkSessionBreak{}, nil
	}

	absenceRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.StaffAbsence, error) {
		return nil, nil
	}

	file, err := svc.ExportSessions(ctx, staffID, from, to, "csv")
	require.NoError(t, err)
	require.NotNil(t, file)
	assert.NotEmpty(t, file.Data)
	assert.Contains(t, file.Filename, ".csv")
	assert.Contains(t, file.Filename, "2024-01-01")
	assert.Contains(t, file.Filename, "2024-01-07")
	assert.Contains(t, file.ContentType, "text/csv")

	// Verify UTF-8 BOM
	assert.Equal(t, byte(0xEF), file.Data[0])
	assert.Equal(t, byte(0xBB), file.Data[1])
	assert.Equal(t, byte(0xBF), file.Data[2])

	// Verify semicolon separator
	content := string(file.Data[3:])
	assert.Contains(t, content, "Datum;Wochentag;Start;Ende")
}

func TestWSExportSessions_XLSX_Success(t *testing.T) {
	t.Parallel()

	svc, sessionRepo, breakRepo, auditRepo, absenceRepo, _ := wsCreateTestServiceWithAbsenceRepo()
	ctx := context.Background()
	staffID := int64(100)
	from := timezone.NewDate(2024, 1, 1)
	to := timezone.NewDate(2024, 1, 7)

	sessionRepo.getHistoryByStaffIDFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.WorkSession, error) {
		return []*activeModels.WorkSession{}, nil
	}

	auditRepo.countBySessionIDsFunc = func(_ context.Context, _ []int64) (map[int64]int, error) {
		return map[int64]int{}, nil
	}

	breakRepo.getBySessionIDFunc = func(_ context.Context, _ int64) ([]*activeModels.WorkSessionBreak, error) {
		return []*activeModels.WorkSessionBreak{}, nil
	}

	absenceRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.StaffAbsence, error) {
		return nil, nil
	}

	file, err := svc.ExportSessions(ctx, staffID, from, to, "xlsx")
	require.NoError(t, err)
	require.NotNil(t, file)
	assert.NotEmpty(t, file.Data)
	assert.Contains(t, file.Filename, ".xlsx")
	assert.Contains(t, file.ContentType, "spreadsheetml")

	// Verify ZIP magic bytes (XLSX is a ZIP file)
	assert.Equal(t, byte(0x50), file.Data[0]) // 'P'
	assert.Equal(t, byte(0x4B), file.Data[1]) // 'K'
}

func TestWSExportSessions_GetHistoryError(t *testing.T) {
	t.Parallel()

	svc, sessionRepo, _, _, _, _ := wsCreateTestServiceWithAbsenceRepo()

	sessionRepo.getHistoryByStaffIDFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.WorkSession, error) {
		return nil, errors.New("database error")
	}

	file, err := svc.ExportSessions(context.Background(), 100, timezone.TodayDate(), timezone.TodayDate(), "csv")
	require.Error(t, err)
	assert.Nil(t, file)
	assert.Contains(t, err.Error(), "failed to get sessions for export")
}

func TestWSExportSessions_GetAbsencesError(t *testing.T) {
	t.Parallel()

	svc, sessionRepo, breakRepo, auditRepo, absenceRepo, _ := wsCreateTestServiceWithAbsenceRepo()

	sessionRepo.getHistoryByStaffIDFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.WorkSession, error) {
		return []*activeModels.WorkSession{}, nil
	}

	auditRepo.countBySessionIDsFunc = func(_ context.Context, _ []int64) (map[int64]int, error) {
		return map[int64]int{}, nil
	}

	breakRepo.getBySessionIDFunc = func(_ context.Context, _ int64) ([]*activeModels.WorkSessionBreak, error) {
		return []*activeModels.WorkSessionBreak{}, nil
	}

	absenceRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.StaffAbsence, error) {
		return nil, errors.New("absence repo error")
	}

	file, err := svc.ExportSessions(context.Background(), 100, timezone.TodayDate(), timezone.TodayDate(), "csv")
	require.Error(t, err)
	assert.Nil(t, file)
	assert.Contains(t, err.Error(), "failed to get absences for export")
}

// ============================================================================
// buildExportRows Tests
// ============================================================================

func TestWSBuildExportRows_SessionsOnly(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _, _ := wsCreateTestServiceWithAbsenceRepo()

	date1 := timezone.NewDate(2024, 1, 2)
	date2 := timezone.NewDate(2024, 1, 3)
	checkIn1 := time.Date(2024, 1, 2, 8, 0, 0, 0, time.UTC)
	checkOut1 := time.Date(2024, 1, 2, 16, 0, 0, 0, time.UTC)
	checkIn2 := time.Date(2024, 1, 3, 9, 0, 0, 0, time.UTC)

	sessions := []*SessionResponse{
		{
			WorkSession: &activeModels.WorkSession{
				Model:        base.Model{ID: 1},
				Date:         date1,
				CheckInTime:  checkIn1,
				CheckOutTime: &checkOut1,
				BreakMinutes: 30,
				Status:       activeModels.WorkSessionStatusPresent,
			},
			NetMinutes: 450,
		},
		{
			WorkSession: &activeModels.WorkSession{
				Model:        base.Model{ID: 2},
				Date:         date2,
				CheckInTime:  checkIn2,
				CheckOutTime: nil,
				BreakMinutes: 0,
				Status:       activeModels.WorkSessionStatusHomeOffice,
			},
			NetMinutes: 0,
		},
	}

	rows := svc.buildExportRows(sessions, nil)

	require.Len(t, rows, 2)
	assert.Equal(t, date1, rows[0].Date)
	assert.Equal(t, date2, rows[1].Date)
}

func TestWSBuildExportRows_AbsencesOnly(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _, _ := wsCreateTestServiceWithAbsenceRepo()

	dateStart := timezone.NewDate(2024, 1, 2)
	dateEnd := timezone.NewDate(2024, 1, 4)

	absences := []*activeModels.StaffAbsence{
		{
			Model:       base.Model{ID: 1},
			StaffID:     100,
			AbsenceType: activeModels.AbsenceTypeSick,
			DateStart:   dateStart,
			DateEnd:     dateEnd,
			Note:        "Flu",
			Status:      "approved",
		},
	}

	rows := svc.buildExportRows(nil, absences)

	require.Len(t, rows, 3) // 3 days: Jan 2, 3, 4
	assert.Equal(t, dateStart, rows[0].Date)
	// Header is 9 cells after the Status/Quelle split, pin the length so a
	// future regression that drops a cell (and silently shifts Note into the
	// Quelle column) fails here instead of on the user's CSV.
	require.Len(t, rows[0].Row, 9, "absence rows must match the 9-column export header")
	assert.Contains(t, rows[0].Row[6], "Krank")                     // Status
	assert.Equal(t, "", rows[0].Row[7], "absences carry no Quelle") // Quelle
	assert.Contains(t, rows[0].Row[8], "Flu")                       // Bemerkungen
}

func TestClampAbsencesToRange_BoundsOverlappingAbsence(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _, _ := wsCreateTestServiceWithAbsenceRepo()
	from := timezone.NewDate(2024, 1, 2)
	to := timezone.NewDate(2024, 1, 3)
	absence := &activeModels.StaffAbsence{
		AbsenceType: activeModels.AbsenceTypeSick,
		DateStart:   from.AddDays(-1),
		DateEnd:     to.AddDays(1),
		Status:      activeModels.AbsenceStatusApproved,
	}

	clamped := clampAbsencesToRange([]*activeModels.StaffAbsence{absence}, from, to)

	require.Len(t, clamped, 1)
	assert.Equal(t, from, clamped[0].DateStart)
	assert.Equal(t, to, clamped[0].DateEnd)
	rows := svc.buildExportRows(nil, clamped)
	require.Len(t, rows, 2)
	assert.Equal(t, from, rows[0].Date)
	assert.Equal(t, to, rows[1].Date)
	assert.Equal(t, from.AddDays(-1), absence.DateStart, "the repository model must not be mutated")
	assert.Equal(t, to.AddDays(1), absence.DateEnd, "the repository model must not be mutated")
}

func TestDayExportRowsByStaffIDs_BatchesAllRepositoryLoads(t *testing.T) {
	t.Parallel()

	svc, sessionRepo, breakRepo, auditRepo, absenceRepo, _ := wsCreateTestServiceWithAbsenceRepo()
	ctx := context.Background()
	staffIDs := []int64{100, 200}
	date := timezone.NewDate(2024, 1, 2)
	checkIn := time.Date(2024, 1, 2, 8, 0, 0, 0, time.UTC)
	checkOut := time.Date(2024, 1, 2, 16, 0, 0, 0, time.UTC)

	var sessionLoads, absenceLoads, breakLoads, manualAuditLoads int
	sessionRepo.getHistoryByStaffIDsFunc = func(_ context.Context, gotStaffIDs []int64, _, _ timezone.Date) (map[int64][]*activeModels.WorkSession, error) {
		sessionLoads++
		assert.Equal(t, staffIDs, gotStaffIDs)
		return map[int64][]*activeModels.WorkSession{
			staffIDs[0]: {{
				Model:        base.Model{ID: 1},
				StaffID:      staffIDs[0],
				Date:         date,
				CheckInTime:  checkIn,
				CheckOutTime: &checkOut,
				Status:       activeModels.WorkSessionStatusPresent,
			}},
			staffIDs[1]: {{
				Model:        base.Model{ID: 2},
				StaffID:      staffIDs[1],
				Date:         date,
				CheckInTime:  checkIn,
				CheckOutTime: &checkOut,
				Status:       activeModels.WorkSessionStatusPresent,
			}},
		}, nil
	}
	sessionRepo.getHistoryByStaffIDFunc = func(context.Context, int64, timezone.Date, timezone.Date) ([]*activeModels.WorkSession, error) {
		return nil, errors.New("single-staff session query must not run")
	}
	absenceRepo.getByStaffIDsAndDateRangeFunc = func(_ context.Context, gotStaffIDs []int64, _, _ timezone.Date) (map[int64][]*activeModels.StaffAbsence, error) {
		absenceLoads++
		assert.Equal(t, staffIDs, gotStaffIDs)
		return map[int64][]*activeModels.StaffAbsence{}, nil
	}
	absenceRepo.getByStaffAndDateRangeFunc = func(context.Context, int64, timezone.Date, timezone.Date) ([]*activeModels.StaffAbsence, error) {
		return nil, errors.New("single-staff absence query must not run")
	}
	breakRepo.listFunc = func(context.Context, *base.QueryOptions) ([]*activeModels.WorkSessionBreak, error) {
		breakLoads++
		return nil, nil
	}
	breakRepo.getBySessionIDFunc = func(context.Context, int64) ([]*activeModels.WorkSessionBreak, error) {
		return nil, errors.New("per-session break query must not run")
	}
	auditRepo.countManualBySessionIDsFunc = func(context.Context, []int64) (map[int64]int, error) {
		manualAuditLoads++
		return map[int64]int{}, nil
	}
	auditRepo.countBySessionIDsFunc = func(context.Context, []int64) (map[int64]int, error) {
		return nil, errors.New("unused audit-count query must not run")
	}

	rowsByStaff, err := svc.DayExportRowsByStaffIDs(ctx, staffIDs, date, date)
	require.NoError(t, err)
	require.Len(t, rowsByStaff[staffIDs[0]], 1)
	require.Len(t, rowsByStaff[staffIDs[1]], 1)
	assert.Equal(t, 1, sessionLoads)
	assert.Equal(t, 1, absenceLoads)
	assert.Equal(t, 1, breakLoads)
	assert.Equal(t, 1, manualAuditLoads)
}

func TestWSBuildExportRows_CompTimeUsesGermanLabel(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _, _ := wsCreateTestServiceWithAbsenceRepo()
	date := timezone.NewDate(2026, time.July, 24)

	rows := svc.buildExportRows(nil, []*activeModels.StaffAbsence{{
		StaffID:     100,
		AbsenceType: activeModels.AbsenceTypeCompTime,
		DateStart:   date,
		DateEnd:     date,
		Status:      activeModels.AbsenceStatusApproved,
	}})

	require.Len(t, rows, 1)
	assert.Contains(t, rows[0].Row[6], "Freizeitausgleich")
}

func TestWSBuildExportRows_Mixed(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _, _ := wsCreateTestServiceWithAbsenceRepo()

	date1 := timezone.NewDate(2024, 1, 2)
	date2 := timezone.NewDate(2024, 1, 3)
	checkIn := time.Date(2024, 1, 2, 8, 0, 0, 0, time.UTC)
	checkOut := time.Date(2024, 1, 2, 16, 0, 0, 0, time.UTC)

	sessions := []*SessionResponse{
		{
			WorkSession: &activeModels.WorkSession{
				Model:        base.Model{ID: 1},
				Date:         date1,
				CheckInTime:  checkIn,
				CheckOutTime: &checkOut,
				BreakMinutes: 30,
				Status:       activeModels.WorkSessionStatusPresent,
			},
			NetMinutes: 450,
		},
	}

	absences := []*activeModels.StaffAbsence{
		{
			Model:       base.Model{ID: 1},
			StaffID:     100,
			AbsenceType: activeModels.AbsenceTypeVacation,
			DateStart:   date2,
			DateEnd:     date2,
			Note:        "Holiday",
			Status:      "approved",
		},
	}

	rows := svc.buildExportRows(sessions, absences)

	require.Len(t, rows, 2)
	assert.Equal(t, date1, rows[0].Date)
	assert.Equal(t, date2, rows[1].Date)
	assert.Contains(t, rows[1].Row[6], "Urlaub")
}

func TestWSBuildExportRows_SortsByDate(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _, _ := wsCreateTestServiceWithAbsenceRepo()

	date1 := timezone.NewDate(2024, 1, 5)
	date2 := timezone.NewDate(2024, 1, 2)
	checkIn := time.Date(2024, 1, 5, 8, 0, 0, 0, time.UTC)

	sessions := []*SessionResponse{
		{
			WorkSession: &activeModels.WorkSession{
				Model:       base.Model{ID: 1},
				Date:        date1,
				CheckInTime: checkIn,
				Status:      activeModels.WorkSessionStatusPresent,
			},
			NetMinutes: 0,
		},
	}

	absences := []*activeModels.StaffAbsence{
		{
			Model:       base.Model{ID: 1},
			AbsenceType: activeModels.AbsenceTypeSick,
			DateStart:   date2,
			DateEnd:     date2,
			Status:      "approved",
		},
	}

	rows := svc.buildExportRows(sessions, absences)

	require.Len(t, rows, 2)
	assert.Equal(t, date2, rows[0].Date) // Earlier date first
	assert.Equal(t, date1, rows[1].Date)
}

func TestCrossStaffCSV_SanitizesUntrustedTextOnly(t *testing.T) {
	t.Parallel()

	data, err := writeMonthCSV([]MonthExportRow{{
		LastName:       "=1+1",
		FirstName:      "+SUM(A1:A2)",
		EmploymentType: "@legacy",
		BalanceMinutes: -90,
	}}, ExportTimeDecimal)
	require.NoError(t, err)

	reader := csv.NewReader(strings.NewReader(string(bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF}))))
	reader.Comma = ';'
	records, err := reader.ReadAll()
	require.NoError(t, err)
	require.Len(t, records, 2)
	assert.Equal(t, "'=1+1", records[1][0])
	assert.Equal(t, "'+SUM(A1:A2)", records[1][1])
	assert.Equal(t, "'@legacy", records[1][2])
	// Located by header, not by a fixed index: the column set grows (#2132
	// inserted "Eröffnungssaldo" ahead of it) and a hardcoded offset turns a
	// column addition into an unrelated sanitizer failure.
	balanceColumn := slices.Index(records[0], "Saldo Monat")
	require.GreaterOrEqual(t, balanceColumn, 0, "month export must carry a Saldo Monat column")
	assert.Equal(t, "-1,50", records[1][balanceColumn], "trusted negative durations must remain numeric CSV values")

	dayData, err := writeDayCSV([]dayExportBlockRow{{
		LastName:  "-danger",
		FirstName: "Safe",
		Cells:     []string{"02.01.2024", "Dienstag", "--", "--", "--", "--", "Krank", "", "\t=1+1"},
	}})
	require.NoError(t, err)
	reader = csv.NewReader(strings.NewReader(string(bytes.TrimPrefix(dayData, []byte{0xEF, 0xBB, 0xBF}))))
	reader.Comma = ';'
	records, err = reader.ReadAll()
	require.NoError(t, err)
	assert.Equal(t, "'-danger", records[1][0])
	assert.Equal(t, "'\t=1+1", records[1][10])
}

func TestWriteMonthXLSX_DecimalDurationsAreNumeric(t *testing.T) {
	t.Parallel()

	data, err := writeMonthXLSX([]MonthExportRow{{
		CarryInMinutes: 750,
		BalanceMinutes: -90,
	}}, ExportTimeDecimal)
	require.NoError(t, err)

	book, err := excelize.OpenReader(bytes.NewReader(data))
	require.NoError(t, err)
	t.Cleanup(func() { _ = book.Close() })

	cellType, err := book.GetCellType("Zeiterfassung", "F2")
	require.NoError(t, err)
	assert.NotEqual(t, excelize.CellTypeSharedString, cellType)
	assert.NotEqual(t, excelize.CellTypeInlineString, cellType)
	value, err := book.GetCellValue("Zeiterfassung", "F2")
	require.NoError(t, err)
	assert.Equal(t, "12.50", value)
	styleID, err := book.GetCellStyle("Zeiterfassung", "F2")
	require.NoError(t, err)
	style, err := book.GetStyle(styleID)
	require.NoError(t, err)
	require.NotNil(t, style.CustomNumFmt)
	assert.Equal(t, "0.00", *style.CustomNumFmt)
}

// ============================================================================
// sessionToRow Tests
// ============================================================================

func TestWSSessionToRow_Complete(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _, _ := wsCreateTestServiceWithAbsenceRepo()

	date := timezone.NewDate(2024, 1, 15) // Monday
	checkIn := time.Date(2024, 1, 15, 8, 30, 0, 0, time.UTC)
	checkOut := time.Date(2024, 1, 15, 17, 15, 0, 0, time.UTC)

	sr := &SessionResponse{
		WorkSession: &activeModels.WorkSession{
			Model:        base.Model{ID: 1},
			Date:         date,
			CheckInTime:  checkIn,
			CheckOutTime: &checkOut,
			BreakMinutes: 45,
			Status:       activeModels.WorkSessionStatusPresent,
			Source:       activeModels.WorkSessionSourceApp,
			Notes:        "Regular day",
		},
		// Response-level total, as GetHistory populates it (totalBreakMinutes).
		// For a closed session it equals the model cache; sessionToRow reads
		// THIS field so the Pause column pairs with the live Netto (#1842).
		BreakMinutes: 45,
		NetMinutes:   480, // 8h 0min
	}

	row := svc.sessionToRow(sr)

	require.Len(t, row, 9)
	assert.Equal(t, "15.01.2024", row[0])  // Datum
	assert.Equal(t, "Montag", row[1])      // Wochentag
	assert.Equal(t, "08:30", row[2])       // Start
	assert.Equal(t, "17:15", row[3])       // Ende
	assert.Equal(t, "45", row[4])          // Pause
	assert.Equal(t, "8h 00min", row[5])    // Netto
	assert.Equal(t, "Vor Ort", row[6])     // Status (Issue #1368)
	assert.Equal(t, "App", row[7])         // Quelle (Issue #1368)
	assert.Equal(t, "Regular day", row[8]) // Bemerkungen
}

// An open session with a running break: the model cache (WorkSession.BreakMinutes)
// still holds only ENDED breaks (0 here), while GetHistory has already put the
// live total (35) on the response and deducted it from Netto. The export must
// print the live total, not the stale cache — otherwise the row reads "Pause 0"
// next to a Netto that visibly excludes those 35 minutes (#1842).
func TestWSSessionToRow_RunningBreakUsesLiveTotal(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _, _ := wsCreateTestServiceWithAbsenceRepo()

	date := timezone.NewDate(2024, 1, 15)
	checkIn := time.Date(2024, 1, 15, 8, 0, 0, 0, time.UTC)

	sr := &SessionResponse{
		WorkSession: &activeModels.WorkSession{
			Model:        base.Model{ID: 1},
			Date:         date,
			CheckInTime:  checkIn,
			CheckOutTime: nil, // still open
			BreakMinutes: 0,   // cache: no ENDED breaks yet
			Status:       activeModels.WorkSessionStatusPresent,
			Source:       activeModels.WorkSessionSourceApp,
		},
		BreakMinutes: 35,  // live total incl. the running break
		NetMinutes:   385, // already deducts the 35 min
	}

	row := svc.sessionToRow(sr)

	assert.Equal(t, "35", row[4], "Pause must show the live break total, not the ended-break cache")
	assert.Equal(t, "6h 25min", row[5])
}

func TestWSSessionToRow_NFC(t *testing.T) {
	t.Parallel()

	// Issue #1368: an NFC-stamped session must be distinguishable from an
	// App-stamped one in the export. Status and Quelle are now in separate
	// columns (Status row[6], Quelle row[7]) so the channel survives even
	// when system events overlay the Quelle cell.
	svc, _, _, _, _, _ := wsCreateTestServiceWithAbsenceRepo()

	date := timezone.NewDate(2024, 1, 15)
	checkIn := time.Date(2024, 1, 15, 8, 0, 0, 0, time.UTC)

	sr := &SessionResponse{
		WorkSession: &activeModels.WorkSession{
			Model:       base.Model{ID: 1},
			Date:        date,
			CheckInTime: checkIn,
			Status:      activeModels.WorkSessionStatusPresent,
			Source:      activeModels.WorkSessionSourceNFC,
		},
		NetMinutes: 0,
	}

	row := svc.sessionToRow(sr)
	assert.Equal(t, "Vor Ort", row[6])
	assert.Equal(t, "NFC", row[7])
}

// TestWSSessionToRow_LegacyUnknownSource verifies that pre-migration rows
// (source = 'unknown') render Quelle as "-" rather than guessing App or NFC.
// Issue #1368: the audit trail must distinguish "we know it was App" from
// "we never recorded the channel".
func TestWSSessionToRow_LegacyUnknownSource(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _, _ := wsCreateTestServiceWithAbsenceRepo()

	date := timezone.NewDate(2024, 1, 15)
	checkIn := time.Date(2024, 1, 15, 8, 0, 0, 0, time.UTC)

	sr := &SessionResponse{
		WorkSession: &activeModels.WorkSession{
			Model:       base.Model{ID: 1},
			Date:        date,
			CheckInTime: checkIn,
			Status:      activeModels.WorkSessionStatusPresent,
			Source:      activeModels.WorkSessionSourceUnknown,
		},
		NetMinutes: 0,
	}

	row := svc.sessionToRow(sr)
	assert.Equal(t, "Vor Ort", row[6])
	assert.Equal(t, "-", row[7], "legacy 'unknown' source must render as hyphen, not guessed App/NFC")
}

func TestWSSessionToRow_NoCheckOut(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _, _ := wsCreateTestServiceWithAbsenceRepo()

	date := timezone.NewDate(2024, 1, 15)
	checkIn := time.Date(2024, 1, 15, 8, 0, 0, 0, time.UTC)

	sr := &SessionResponse{
		WorkSession: &activeModels.WorkSession{
			Model:        base.Model{ID: 1},
			Date:         date,
			CheckInTime:  checkIn,
			CheckOutTime: nil,
			BreakMinutes: 0,
			Status:       activeModels.WorkSessionStatusPresent,
		},
		NetMinutes: 0,
	}

	row := svc.sessionToRow(sr)

	assert.Equal(t, "", row[3]) // Ende should be empty
}

func TestWSSessionToRow_HomeOffice(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _, _ := wsCreateTestServiceWithAbsenceRepo()

	date := timezone.NewDate(2024, 1, 15)
	checkIn := time.Date(2024, 1, 15, 8, 0, 0, 0, time.UTC)

	sr := &SessionResponse{
		WorkSession: &activeModels.WorkSession{
			Model:       base.Model{ID: 1},
			Date:        date,
			CheckInTime: checkIn,
			Status:      activeModels.WorkSessionStatusHomeOffice,
			Source:      activeModels.WorkSessionSourceApp,
		},
		NetMinutes: 0,
	}

	row := svc.sessionToRow(sr)

	assert.Equal(t, "Homeoffice", row[6]) // Status (Issue #1368)
	assert.Equal(t, "App", row[7])        // Quelle (Issue #1368)
}

func TestWSSessionToRow_Quelle_AutoCheckedOut(t *testing.T) {
	t.Parallel()

	// Issue #1368: Auto-Checkout overlays the Quelle cell so the
	// OGS-Leitung sees the session was closed by the scheduler. Status
	// remains the underlying work mode. Vor Ort is preserved here even
	// though Quelle is "Auto-Checkout".
	svc, _, _, _, _, _ := wsCreateTestServiceWithAbsenceRepo()

	date := timezone.NewDate(2024, 1, 15)
	checkIn := time.Date(2024, 1, 15, 8, 0, 0, 0, time.UTC)
	checkOut := time.Date(2024, 1, 15, 23, 59, 0, 0, time.UTC)

	sr := &SessionResponse{
		WorkSession: &activeModels.WorkSession{
			Model:          base.Model{ID: 1},
			Date:           date,
			CheckInTime:    checkIn,
			CheckOutTime:   &checkOut,
			Status:         activeModels.WorkSessionStatusPresent,
			AutoCheckedOut: true,
		},
		NetMinutes: 0,
		EditCount:  0,
	}

	row := svc.sessionToRow(sr)
	assert.Equal(t, "Vor Ort", row[6], "Status survives Auto-Checkout overlay")
	assert.Equal(t, "Auto-Checkout", row[7])
}

func TestWSSessionToRow_Quelle_ManuelCorrected(t *testing.T) {
	t.Parallel()

	// Issue #1368: a manual correction overlays Quelle, but Status still
	// shows the underlying work mode (Homeoffice), so the OGS-Leitung can
	// see both the corrected-state signal AND what the staff actually did.
	svc, _, _, _, _, _ := wsCreateTestServiceWithAbsenceRepo()

	date := timezone.NewDate(2024, 1, 15)
	checkIn := time.Date(2024, 1, 15, 8, 0, 0, 0, time.UTC)

	sr := &SessionResponse{
		WorkSession: &activeModels.WorkSession{
			Model:          base.Model{ID: 1},
			Date:           date,
			CheckInTime:    checkIn,
			Status:         activeModels.WorkSessionStatusHomeOffice,
			AutoCheckedOut: true,
		},
		NetMinutes: 0,
		EditCount:  2,
	}

	row := svc.sessionToRow(sr)
	assert.Equal(t, "Homeoffice", row[6], "Status survives Manuell-korrigiert overlay")
	assert.Equal(t, "Manuell korrigiert", row[7])
}

func TestWSSessionToRow_NetMinutesFormatting(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _, _ := wsCreateTestServiceWithAbsenceRepo()

	tests := []struct {
		netMinutes int
		expected   string
	}{
		{0, "0h 00min"},
		{59, "0h 59min"},
		{60, "1h 00min"},
		{125, "2h 05min"},
		{480, "8h 00min"},
		{525, "8h 45min"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			date := timezone.NewDate(2024, 1, 15)
			checkIn := time.Date(2024, 1, 15, 8, 0, 0, 0, time.UTC)

			sr := &SessionResponse{
				WorkSession: &activeModels.WorkSession{
					Model:       base.Model{ID: 1},
					Date:        date,
					CheckInTime: checkIn,
					Status:      activeModels.WorkSessionStatusPresent,
				},
				NetMinutes: tt.netMinutes,
			}

			row := svc.sessionToRow(sr)
			assert.Equal(t, tt.expected, row[5])
		})
	}
}

func TestWSSessionToRow_GermanWeekdays(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _, _ := wsCreateTestServiceWithAbsenceRepo()

	weekdays := []struct {
		date     time.Time
		expected string
	}{
		{time.Date(2024, 1, 14, 0, 0, 0, 0, time.UTC), "Sonntag"},    // Sunday
		{time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC), "Montag"},     // Monday
		{time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC), "Dienstag"},   // Tuesday
		{time.Date(2024, 1, 17, 0, 0, 0, 0, time.UTC), "Mittwoch"},   // Wednesday
		{time.Date(2024, 1, 18, 0, 0, 0, 0, time.UTC), "Donnerstag"}, // Thursday
		{time.Date(2024, 1, 19, 0, 0, 0, 0, time.UTC), "Freitag"},    // Friday
		{time.Date(2024, 1, 20, 0, 0, 0, 0, time.UTC), "Samstag"},    // Saturday
	}

	for _, wd := range weekdays {
		t.Run(wd.expected, func(t *testing.T) {
			checkIn := wd.date.Add(8 * time.Hour)

			sr := &SessionResponse{
				WorkSession: &activeModels.WorkSession{
					Model:       base.Model{ID: 1},
					Date:        timezone.DateFromTime(wd.date),
					CheckInTime: checkIn,
					Status:      activeModels.WorkSessionStatusPresent,
				},
				NetMinutes: 0,
			}

			row := svc.sessionToRow(sr)
			assert.Equal(t, wd.expected, row[1])
		})
	}
}

// ============================================================================
// Single-staff CSV serialization Tests
// ============================================================================

func TestWSExportCSV_Headers(t *testing.T) {
	t.Parallel()

	data, err := writeExportCSV(timeTrackingHeaders(), nil, []int{8})

	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Skip BOM and parse CSV
	reader := csv.NewReader(bytes.NewReader(data[3:]))
	reader.Comma = ';'

	records, err := reader.ReadAll()
	require.NoError(t, err)
	require.Len(t, records, 1) // Only header

	header := records[0]
	require.Len(t, header, 9) // Issue #1368: split "Ort" into Status + Quelle
	assert.Equal(t, "Datum", header[0])
	assert.Equal(t, "Wochentag", header[1])
	assert.Equal(t, "Start", header[2])
	assert.Equal(t, "Ende", header[3])
	assert.Equal(t, "Pause (Min)", header[4])
	assert.Equal(t, "Netto (Std)", header[5])
	assert.Equal(t, "Status", header[6]) // Issue #1368: was part of "Ort"
	assert.Equal(t, "Quelle", header[7]) // Issue #1368: split out
	assert.Equal(t, "Bemerkungen", header[8])
}

func TestWSExportCSV_UTF8BOM(t *testing.T) {
	t.Parallel()

	data, err := writeExportCSV(timeTrackingHeaders(), nil, []int{8})
	require.NoError(t, err)

	assert.Equal(t, byte(0xEF), data[0])
	assert.Equal(t, byte(0xBB), data[1])
	assert.Equal(t, byte(0xBF), data[2])
}

func TestWSExportCSV_SemicolonSeparator(t *testing.T) {
	t.Parallel()

	rows := [][]string{
		{"15.01.2024", "Montag", "08:00", "16:00", "30", "7h 30min", "In der OGS", "App", "Test"},
	}

	data, err := writeExportCSV(timeTrackingHeaders(), rows, []int{8})
	require.NoError(t, err)

	content := string(data[3:])
	lines := strings.Split(content, "\n")
	assert.True(t, strings.Contains(lines[1], ";"))
	assert.False(t, strings.Contains(lines[1], ","))
}

// TestWSExportCSV_SanitizesNotes: the Bemerkungen column carries untrusted
// free text — a note starting with a formula trigger must be escaped so
// spreadsheet software cannot evaluate it (#1568 migration hardening; the
// cross-staff CSV always did this, the single-staff CSV now shares the path).
func TestWSExportCSV_SanitizesNotes(t *testing.T) {
	t.Parallel()

	rows := [][]string{
		{"15.01.2024", "Montag", "08:00", "16:00", "30", "7h 30min", "In der OGS", "App", "=1+1"},
	}

	data, err := writeExportCSV(timeTrackingHeaders(), rows, []int{8})
	require.NoError(t, err)

	reader := csv.NewReader(bytes.NewReader(data[3:]))
	reader.Comma = ';'
	records, err := reader.ReadAll()
	require.NoError(t, err)
	assert.Equal(t, "'=1+1", records[1][8])
	// Absence sentinel cells ("--") sit outside the untrusted column list
	// and must stay verbatim.
	assert.Equal(t, "15.01.2024", records[1][0])
}

// ============================================================================
// Single-staff XLSX/PDF document Tests
// ============================================================================

func TestWSBuildTimeTrackingDocument_ShapesRows(t *testing.T) {
	t.Parallel()

	service, _, _, _, _, _ := wsCreateTestServiceWithAbsenceRepo()

	date := timezone.NewDate(2024, 1, 15)
	rows := []exportRow{
		{
			Date: date,
			Row:  []string{"15.01.2024", "Montag", "08:00", "16:00", "30", "7h 30min", "In der OGS", "App", "Test"},
		},
	}

	doc, err := service.buildTimeTrackingDocument(context.Background(), 100, rows, date, date)
	require.NoError(t, err)
	assert.Equal(t, "Zeiterfassung", doc.Title)
	assert.NotEmpty(t, doc.Footer)
	require.Len(t, doc.Rows, 1)
	require.Len(t, doc.Columns, 9)
	assert.Equal(t, "15.01.2024", doc.Rows[0].Values[doc.Columns[0].ID])
	assert.Equal(t, "Test", doc.Rows[0].Values[doc.Columns[8].ID])
	require.Len(t, doc.Filters, 1)
	assert.Contains(t, doc.Filters[0], "15.01.2024")
}

func TestWSExportSessions_PDF_Success(t *testing.T) {
	t.Parallel()

	svc, sessionRepo, breakRepo, auditRepo, absenceRepo, _ := wsCreateTestServiceWithAbsenceRepo()

	sessionRepo.getHistoryByStaffIDFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.WorkSession, error) {
		return []*activeModels.WorkSession{}, nil
	}
	auditRepo.countBySessionIDsFunc = func(_ context.Context, _ []int64) (map[int64]int, error) {
		return map[int64]int{}, nil
	}
	breakRepo.getBySessionIDFunc = func(_ context.Context, _ int64) ([]*activeModels.WorkSessionBreak, error) {
		return []*activeModels.WorkSessionBreak{}, nil
	}
	absenceRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.StaffAbsence, error) {
		return nil, nil
	}

	file, err := svc.ExportSessions(context.Background(), 100, timezone.NewDate(2024, 1, 1), timezone.NewDate(2024, 1, 7), "pdf")
	require.NoError(t, err)
	require.NotNil(t, file)
	assert.Contains(t, file.Filename, ".pdf")
	assert.Equal(t, "application/pdf", file.ContentType)
	assert.True(t, bytes.HasPrefix(file.Data, []byte("%PDF")))
}

func TestWSExportSessions_PDF_StaffLookupError(t *testing.T) {
	t.Parallel()

	svc, sessionRepo, breakRepo, auditRepo, absenceRepo, _ := wsCreateTestServiceWithAbsenceRepo()

	sessionRepo.getHistoryByStaffIDFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.WorkSession, error) {
		return nil, nil
	}
	auditRepo.countBySessionIDsFunc = func(_ context.Context, _ []int64) (map[int64]int, error) {
		return map[int64]int{}, nil
	}
	breakRepo.getBySessionIDFunc = func(_ context.Context, _ int64) ([]*activeModels.WorkSessionBreak, error) {
		return nil, nil
	}
	absenceRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.StaffAbsence, error) {
		return nil, nil
	}
	svc.staffRepo = &wsMockStaffRepository{
		findWithPersonByIDsFunc: func(_ context.Context, _ []int64) (map[int64]*userModels.Staff, error) {
			return nil, errors.New("database error")
		},
	}

	file, err := svc.ExportSessions(context.Background(), 100, timezone.NewDate(2024, 1, 1), timezone.NewDate(2024, 1, 7), "pdf")
	require.Error(t, err)
	assert.Nil(t, file)
	assert.ErrorContains(t, err, "failed to load staff for export")
}

// ============================================================================
// UpdateSession Additional Tests
// ============================================================================

func TestWSUpdateSession_StatusChange(t *testing.T) {
	t.Parallel()

	// Issue #1368: status changes require a non-empty reason in notes. The
	// happy path now sends notes alongside the new status.
	svc, sessionRepo, _, auditRepo, _ := wsCreateTestService()
	staffID := int64(100)
	sessionID := int64(100)

	sessionRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:       base.Model{ID: sessionID},
			StaffID:     staffID,
			CheckInTime: time.Now().Add(-8 * time.Hour),
			Status:      activeModels.WorkSessionStatusPresent,
			Date:        timezone.TodayDate(),
			CreatedBy:   staffID,
		}, nil
	}

	sessionRepo.updateFunc = func(_ context.Context, _ *activeModels.WorkSession) error {
		return nil
	}

	auditRepo.createBatchFunc = func(_ context.Context, edits []*auditModels.WorkSessionEdit) error {
		// Both status and notes change → two audit entries, both annotated
		// with the same reason via uc.notes.
		assert.Len(t, edits, 2)
		fields := []string{edits[0].FieldName, edits[1].FieldName}
		assert.Contains(t, fields, auditModels.FieldStatus)
		assert.Contains(t, fields, auditModels.FieldNotes)
		return nil
	}

	newStatus := activeModels.WorkSessionStatusHomeOffice
	reason := "Mittags ins Homeoffice gewechselt"
	updates := SessionUpdateRequest{
		Status: &newStatus,
		Notes:  &reason,
	}

	session, err := svc.UpdateSession(context.Background(), staffID, sessionID, updates)
	require.NoError(t, err)
	require.NotNil(t, session)
}

func TestWSUpdateSession_StatusChangeRequiresNotes(t *testing.T) {
	t.Parallel()

	// Issue #1368: a status change without a reason must be rejected so the
	// audit trail can never silently lose the "why" of a Vor Ort ↔ Homeoffice
	// switch. Empty/whitespace-only notes count as missing.
	svc, sessionRepo, _, auditRepo, _ := wsCreateTestService()
	staffID := int64(100)
	sessionID := int64(100)

	sessionRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:       base.Model{ID: sessionID},
			StaffID:     staffID,
			CheckInTime: time.Now().Add(-8 * time.Hour),
			Status:      activeModels.WorkSessionStatusPresent,
			Date:        timezone.TodayDate(),
			CreatedBy:   staffID,
		}, nil
	}
	sessionRepo.updateFunc = func(_ context.Context, _ *activeModels.WorkSession) error {
		t.Fatal("repo.Update must not be called when status change is rejected")
		return nil
	}
	auditRepo.createBatchFunc = func(_ context.Context, _ []*auditModels.WorkSessionEdit) error {
		t.Fatal("audit.CreateBatch must not be called when status change is rejected")
		return nil
	}

	newStatus := activeModels.WorkSessionStatusHomeOffice
	emptyNotes := ""
	whitespaceNotes := "   "
	cases := []struct {
		name  string
		notes *string
	}{
		{name: "nil notes", notes: nil},
		{name: "empty notes", notes: &emptyNotes},
		{name: "whitespace-only notes", notes: &whitespaceNotes},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			updates := SessionUpdateRequest{
				Status: &newStatus,
				Notes:  tc.notes,
			}
			session, err := svc.UpdateSession(context.Background(), staffID, sessionID, updates)
			require.Error(t, err)
			assert.Nil(t, session)
			assert.Contains(t, err.Error(), "notes required")
		})
	}
}

func TestWSUpdateSession_NotesOnlyDoesNotRequireReason(t *testing.T) {
	t.Parallel()

	// Issue #1368: the gate is specifically for status changes. Editing only
	// notes (e.g., adding a comment) must still work without a reason field.
	svc, sessionRepo, _, auditRepo, _ := wsCreateTestService()
	staffID := int64(100)
	sessionID := int64(100)

	sessionRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:       base.Model{ID: sessionID},
			StaffID:     staffID,
			CheckInTime: time.Now().Add(-8 * time.Hour),
			Status:      activeModels.WorkSessionStatusHomeOffice,
			Date:        timezone.TodayDate(),
			CreatedBy:   staffID,
			Notes:       "old",
		}, nil
	}
	sessionRepo.updateFunc = func(_ context.Context, _ *activeModels.WorkSession) error {
		return nil
	}
	auditRepo.createBatchFunc = func(_ context.Context, _ []*auditModels.WorkSessionEdit) error {
		return nil
	}

	newNotes := "added comment"
	session, err := svc.UpdateSession(context.Background(), staffID, sessionID, SessionUpdateRequest{
		Notes: &newNotes,
	})
	require.NoError(t, err)
	require.NotNil(t, session)
}

func TestWSUpdateSession_NotesChange(t *testing.T) {
	t.Parallel()

	svc, sessionRepo, _, auditRepo, _ := wsCreateTestService()
	staffID := int64(100)
	sessionID := int64(100)

	sessionRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:       base.Model{ID: sessionID},
			StaffID:     staffID,
			CheckInTime: time.Now().Add(-8 * time.Hour),
			Status:      activeModels.WorkSessionStatusPresent,
			Date:        timezone.TodayDate(),
			CreatedBy:   staffID,
			Notes:       "Old note",
		}, nil
	}

	sessionRepo.updateFunc = func(_ context.Context, _ *activeModels.WorkSession) error {
		return nil
	}

	auditRepo.createBatchFunc = func(_ context.Context, edits []*auditModels.WorkSessionEdit) error {
		assert.Len(t, edits, 1)
		assert.Equal(t, auditModels.FieldNotes, edits[0].FieldName)
		return nil
	}

	newNotes := "New note"
	updates := SessionUpdateRequest{
		Notes: &newNotes,
	}

	session, err := svc.UpdateSession(context.Background(), staffID, sessionID, updates)
	require.NoError(t, err)
	require.NotNil(t, session)
}

func TestWSUpdateSession_BreakMinutesWithoutIndividualBreaks(t *testing.T) {
	t.Parallel()

	svc, sessionRepo, _, auditRepo, _ := wsCreateTestService()
	staffID := int64(100)
	sessionID := int64(100)

	sessionRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:        base.Model{ID: sessionID},
			StaffID:      staffID,
			CheckInTime:  time.Now().Add(-8 * time.Hour),
			Status:       activeModels.WorkSessionStatusPresent,
			Date:         timezone.TodayDate(),
			CreatedBy:    staffID,
			BreakMinutes: 30,
		}, nil
	}

	sessionRepo.updateFunc = func(_ context.Context, _ *activeModels.WorkSession) error {
		return nil
	}

	auditRepo.createBatchFunc = func(_ context.Context, edits []*auditModels.WorkSessionEdit) error {
		assert.Len(t, edits, 1)
		assert.Equal(t, auditModels.FieldBreakMinutes, edits[0].FieldName)
		return nil
	}

	newBreakMinutes := 45
	updates := SessionUpdateRequest{
		BreakMinutes: &newBreakMinutes,
	}

	session, err := svc.UpdateSession(context.Background(), staffID, sessionID, updates)
	require.NoError(t, err)
	require.NotNil(t, session)
}

func TestWSUpdateSession_CheckOutTimeChange(t *testing.T) {
	t.Parallel()

	svc, sessionRepo, _, auditRepo, _ := wsCreateTestService()
	staffID := int64(100)
	sessionID := int64(100)

	oldCheckOut := time.Now().Add(-1 * time.Hour)
	newCheckOut := time.Now()

	sessionRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:        base.Model{ID: sessionID},
			StaffID:      staffID,
			CheckInTime:  time.Now().Add(-8 * time.Hour),
			CheckOutTime: &oldCheckOut,
			Status:       activeModels.WorkSessionStatusPresent,
			Date:         timezone.TodayDate(),
			CreatedBy:    staffID,
		}, nil
	}

	sessionRepo.updateFunc = func(_ context.Context, _ *activeModels.WorkSession) error {
		return nil
	}

	auditRepo.createBatchFunc = func(_ context.Context, edits []*auditModels.WorkSessionEdit) error {
		assert.Len(t, edits, 1)
		assert.Equal(t, auditModels.FieldCheckOutTime, edits[0].FieldName)
		return nil
	}

	updates := SessionUpdateRequest{
		CheckOutTime: &newCheckOut,
	}

	session, err := svc.UpdateSession(context.Background(), staffID, sessionID, updates)
	require.NoError(t, err)
	require.NotNil(t, session)
}

func TestWSUpdateSession_NoChanges(t *testing.T) {
	t.Parallel()

	svc, sessionRepo, _, auditRepo, _ := wsCreateTestService()
	staffID := int64(100)
	sessionID := int64(100)

	sessionRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:       base.Model{ID: sessionID},
			StaffID:     staffID,
			CheckInTime: time.Now().Add(-8 * time.Hour),
			Status:      activeModels.WorkSessionStatusPresent,
			Date:        timezone.TodayDate(),
			CreatedBy:   staffID,
		}, nil
	}

	sessionRepo.updateFunc = func(_ context.Context, _ *activeModels.WorkSession) error {
		return nil
	}

	// Should not be called when no changes
	auditRepo.createBatchFunc = func(_ context.Context, edits []*auditModels.WorkSessionEdit) error {
		t.Fatal("createBatch should not be called with no changes")
		return nil
	}

	updates := SessionUpdateRequest{} // No changes

	session, err := svc.UpdateSession(context.Background(), staffID, sessionID, updates)
	require.NoError(t, err)
	require.NotNil(t, session)
}

func TestWSUpdateSession_AuditRepoError(t *testing.T) {
	t.Parallel()

	svc, sessionRepo, _, auditRepo, _ := wsCreateTestService()
	staffID := int64(100)
	sessionID := int64(100)

	sessionRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:       base.Model{ID: sessionID},
			StaffID:     staffID,
			CheckInTime: time.Now().Add(-8 * time.Hour),
			Status:      activeModels.WorkSessionStatusPresent,
			Date:        timezone.TodayDate(),
			CreatedBy:   staffID,
			Notes:       "Old",
		}, nil
	}

	sessionRepo.updateFunc = func(_ context.Context, _ *activeModels.WorkSession) error {
		return nil
	}

	auditRepo.createBatchFunc = func(_ context.Context, _ []*auditModels.WorkSessionEdit) error {
		return errors.New("audit error")
	}

	newNotes := "New"
	updates := SessionUpdateRequest{
		Notes: &newNotes,
	}

	session, err := svc.UpdateSession(context.Background(), staffID, sessionID, updates)
	require.Error(t, err)
	assert.Nil(t, session)
	assert.Contains(t, err.Error(), "failed to create audit entries")
}

// ============================================================================
// Additional Error Path Tests
// ============================================================================

func TestWSCleanupOpenSessions_RepoError(t *testing.T) {
	t.Parallel()

	svc, sessionRepo, _, _, _ := wsCreateTestService()

	sessionRepo.getOpenSessionsFunc = func(_ context.Context, _ timezone.Date) ([]*activeModels.WorkSession, error) {
		return nil, errors.New("database error")
	}

	count, err := svc.CleanupOpenSessions(context.Background())
	require.Error(t, err)
	assert.Equal(t, 0, count)
	assert.Contains(t, err.Error(), "failed to get open sessions")
}

func TestWSCleanupOpenSessions_CloseSessionError(t *testing.T) {
	t.Parallel()

	svc, sessionRepo, _, _, _ := wsCreateTestService()
	yesterday := timezone.TodayDate().AddDays(-1)

	sessionRepo.getOpenSessionsFunc = func(_ context.Context, _ timezone.Date) ([]*activeModels.WorkSession, error) {
		return []*activeModels.WorkSession{
			{Model: base.Model{ID: 1}, Date: yesterday},
		}, nil
	}

	sessionRepo.closeSessionFunc = func(_ context.Context, _ int64, _ time.Time, _ bool) (bool, error) {
		return false, errors.New("close error")
	}

	count, err := svc.CleanupOpenSessions(context.Background())
	require.Error(t, err)
	assert.Equal(t, 0, count)
	assert.Contains(t, err.Error(), "failed to close session")
}

func TestWSGetHistory_AuditCountError(t *testing.T) {
	t.Parallel()

	svc, sessionRepo, _, auditRepo, _ := wsCreateTestService()

	sessionRepo.getHistoryByStaffIDFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.WorkSession, error) {
		return []*activeModels.WorkSession{
			{Model: base.Model{ID: 1}},
		}, nil
	}

	auditRepo.countBySessionIDsFunc = func(_ context.Context, _ []int64) (map[int64]int, error) {
		return nil, errors.New("audit count error")
	}

	responses, err := svc.GetHistory(context.Background(), 100, timezone.TodayDate(), timezone.TodayDate())
	require.Error(t, err)
	assert.Nil(t, responses)
	assert.Contains(t, err.Error(), "failed to get edit counts")
}

func TestWSGetHistory_BreaksError(t *testing.T) {
	t.Parallel()

	svc, sessionRepo, breakRepo, auditRepo, _ := wsCreateTestService()

	sessionRepo.getHistoryByStaffIDFunc = func(_ context.Context, _ int64, _, _ timezone.Date) ([]*activeModels.WorkSession, error) {
		return []*activeModels.WorkSession{
			{Model: base.Model{ID: 1}},
		}, nil
	}

	auditRepo.countBySessionIDsFunc = func(_ context.Context, _ []int64) (map[int64]int, error) {
		return map[int64]int{1: 0}, nil
	}

	breakRepo.getBySessionIDFunc = func(_ context.Context, _ int64) ([]*activeModels.WorkSessionBreak, error) {
		return nil, errors.New("breaks error")
	}

	responses, err := svc.GetHistory(context.Background(), 100, timezone.TodayDate(), timezone.TodayDate())
	require.Error(t, err)
	assert.Nil(t, responses)
	assert.Contains(t, err.Error(), "failed to get breaks")
}

func TestWSGetTodayPresenceMap_RepoError(t *testing.T) {
	t.Parallel()

	svc, sessionRepo, _, _, _ := wsCreateTestService()

	sessionRepo.getTodayPresenceMapFunc = func(_ context.Context) (map[int64]string, error) {
		return nil, errors.New("database error")
	}

	presenceMap, err := svc.GetTodayPresenceMap(context.Background())
	require.Error(t, err)
	assert.Nil(t, presenceMap)
}

func TestWSGetSessionEdits_RepoError(t *testing.T) {
	t.Parallel()

	svc, sessionRepo, _, auditRepo, _, _ := wsCreateTestServiceWithAbsenceRepo()
	staffID := int64(100)
	sessionID := int64(500)

	// Mock FindByID to return session owned by staffID
	sessionRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:   base.Model{ID: sessionID},
			StaffID: staffID,
		}, nil
	}

	auditRepo.getBySessionIDFunc = func(_ context.Context, _ int64) ([]*auditModels.WorkSessionEdit, error) {
		return nil, errors.New("database error")
	}

	edits, err := svc.GetSessionEdits(context.Background(), staffID, sessionID)
	require.Error(t, err)
	assert.Nil(t, edits)
}

func TestWSGetSessionBreaks_RepoError(t *testing.T) {
	t.Parallel()

	svc, sessionRepo, breakRepo, _, _, _ := wsCreateTestServiceWithAbsenceRepo()
	staffID := int64(100)
	sessionID := int64(501)

	// Mock FindByID to return session owned by staffID
	sessionRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:   base.Model{ID: sessionID},
			StaffID: staffID,
		}, nil
	}

	breakRepo.getBySessionIDFunc = func(_ context.Context, _ int64) ([]*activeModels.WorkSessionBreak, error) {
		return nil, errors.New("database error")
	}

	breaks, err := svc.GetSessionBreaks(context.Background(), staffID, sessionID)
	require.Error(t, err)
	assert.Nil(t, breaks)
}

func TestWSCheckOut_BreakCheckError(t *testing.T) {
	t.Parallel()

	svc, sessionRepo, breakRepo, _, _ := wsCreateTestService()

	sessionRepo.getCurrentByStaffIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:       base.Model{ID: 1},
			StaffID:     100,
			CheckInTime: time.Now().Add(-4 * time.Hour),
		}, nil
	}

	breakRepo.getActiveBySessionIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSessionBreak, error) {
		return nil, errors.New("break check error")
	}

	session, err := svc.CheckOut(context.Background(), 100, "")
	require.Error(t, err)
	assert.Nil(t, session)
	assert.Contains(t, err.Error(), "failed to check active break")
}

func TestWSCheckOut_CloseSessionError(t *testing.T) {
	t.Parallel()

	svc, sessionRepo, breakRepo, _, _ := wsCreateTestService()

	sessionRepo.getCurrentByStaffIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:       base.Model{ID: 1},
			StaffID:     100,
			CheckInTime: time.Now().Add(-4 * time.Hour),
		}, nil
	}

	breakRepo.getActiveBySessionIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSessionBreak, error) {
		return nil, nil
	}

	sessionRepo.closeSessionFunc = func(_ context.Context, _ int64, _ time.Time, _ bool) (bool, error) {
		return false, errors.New("close error")
	}

	session, err := svc.CheckOut(context.Background(), 100, "")
	require.Error(t, err)
	assert.Nil(t, session)
	assert.Contains(t, err.Error(), "failed to close session")
}

func TestWSCheckOut_FindByIDError(t *testing.T) {
	t.Parallel()

	svc, sessionRepo, breakRepo, _, supervisorRepo := wsCreateTestService()

	sessionRepo.getCurrentByStaffIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:       base.Model{ID: 1},
			StaffID:     100,
			CheckInTime: time.Now().Add(-4 * time.Hour),
		}, nil
	}

	breakRepo.getActiveBySessionIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSessionBreak, error) {
		return nil, nil
	}

	sessionRepo.closeSessionFunc = func(_ context.Context, _ int64, _ time.Time, _ bool) (bool, error) {
		return true, nil
	}

	supervisorRepo.endAllActiveByStaffIDFunc = func(_ context.Context, _ int64) (int, error) {
		return 0, nil
	}

	sessionRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.WorkSession, error) {
		return nil, errors.New("find error")
	}

	session, err := svc.CheckOut(context.Background(), 100, "")
	require.Error(t, err)
	assert.Nil(t, session)
	assert.Contains(t, err.Error(), "failed to retrieve updated session")
}

func TestWSStartBreak_NilSession(t *testing.T) {
	t.Parallel()

	svc, sessionRepo, _, _, _ := wsCreateTestService()

	sessionRepo.getCurrentByStaffIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
		return nil, nil
	}

	brk, err := svc.StartBreak(context.Background(), 100, nil)
	require.Error(t, err)
	assert.Nil(t, brk)
	assert.Contains(t, err.Error(), "no active session found")
}

func TestWSEndBreak_EndBreakError(t *testing.T) {
	t.Parallel()

	svc, sessionRepo, breakRepo, _, _ := wsCreateTestService()

	sessionRepo.getCurrentByStaffIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:       base.Model{ID: 1},
			StaffID:     100,
			CheckInTime: time.Now().Add(-2 * time.Hour),
		}, nil
	}

	breakRepo.getActiveBySessionIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSessionBreak, error) {
		return &activeModels.WorkSessionBreak{
			Model:     base.Model{ID: 1},
			SessionID: 1,
			StartedAt: time.Now().Add(-30 * time.Minute),
		}, nil
	}

	breakRepo.endBreakFunc = func(_ context.Context, _ int64, _ time.Time, _ int) error {
		return errors.New("end break error")
	}

	session, err := svc.EndBreak(context.Background(), 100)
	require.Error(t, err)
	assert.Nil(t, session)
	assert.Contains(t, err.Error(), "failed to end break")
}

func TestWSEndBreak_RecalcBreakMinutesError(t *testing.T) {
	t.Parallel()

	svc, sessionRepo, breakRepo, _, _ := wsCreateTestService()

	sessionRepo.getCurrentByStaffIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:       base.Model{ID: 1},
			StaffID:     100,
			CheckInTime: time.Now().Add(-2 * time.Hour),
		}, nil
	}

	breakRepo.getActiveBySessionIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSessionBreak, error) {
		return &activeModels.WorkSessionBreak{
			Model:     base.Model{ID: 1},
			SessionID: 1,
			StartedAt: time.Now().Add(-30 * time.Minute),
		}, nil
	}

	breakRepo.endBreakFunc = func(_ context.Context, _ int64, _ time.Time, _ int) error {
		return nil
	}

	breakRepo.getBySessionIDFunc = func(_ context.Context, _ int64) ([]*activeModels.WorkSessionBreak, error) {
		return nil, errors.New("get breaks error")
	}

	session, err := svc.EndBreak(context.Background(), 100)
	require.Error(t, err)
	assert.Nil(t, session)
	assert.Contains(t, err.Error(), "failed to update break minutes")
}

func TestWSEndBreak_FindByIDError(t *testing.T) {
	t.Parallel()

	svc, sessionRepo, breakRepo, _, _ := wsCreateTestService()

	sessionRepo.getCurrentByStaffIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:       base.Model{ID: 1},
			StaffID:     100,
			CheckInTime: time.Now().Add(-2 * time.Hour),
		}, nil
	}

	breakRepo.getActiveBySessionIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSessionBreak, error) {
		return &activeModels.WorkSessionBreak{
			Model:     base.Model{ID: 1},
			SessionID: 1,
			StartedAt: time.Now().Add(-30 * time.Minute),
		}, nil
	}

	breakRepo.endBreakFunc = func(_ context.Context, _ int64, _ time.Time, _ int) error {
		return nil
	}

	breakRepo.getBySessionIDFunc = func(_ context.Context, _ int64) ([]*activeModels.WorkSessionBreak, error) {
		return []*activeModels.WorkSessionBreak{}, nil
	}

	sessionRepo.updateBreakMinutesFunc = func(_ context.Context, _ int64, _ int) error {
		return nil
	}

	sessionRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.WorkSession, error) {
		return nil, errors.New("find error")
	}

	session, err := svc.EndBreak(context.Background(), 100)
	require.Error(t, err)
	assert.Nil(t, session)
	assert.Contains(t, err.Error(), "failed to retrieve updated session")
}

func TestWSCheckIn_CreateError(t *testing.T) {
	t.Parallel()

	svc, sessionRepo, _, _, _ := wsCreateTestService()

	sessionRepo.listByStaffAndDateFunc = func(_ context.Context, _ int64, _ timezone.Date) ([]*activeModels.WorkSession, error) {
		return nil, nil
	}

	sessionRepo.createFunc = func(_ context.Context, _ *activeModels.WorkSession) error {
		return errors.New("create error")
	}

	session, err := svc.CheckIn(context.Background(), 100, activeModels.WorkSessionStatusPresent, activeModels.WorkSessionSourceApp, "")
	require.Error(t, err)
	assert.Nil(t, session)
	assert.Contains(t, err.Error(), "failed to create work session")
}
