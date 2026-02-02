package active

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"strings"
	"testing"
	"time"

	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Helper to create test service with absence repo
// ============================================================================

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
	svc, sessionRepo, breakRepo, auditRepo, absenceRepo, _ := wsCreateTestServiceWithAbsenceRepo()
	ctx := context.Background()
	staffID := int64(100)
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 7, 23, 59, 59, 0, time.UTC)

	date1 := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	checkIn := time.Date(2024, 1, 2, 8, 0, 0, 0, time.UTC)
	checkOut := time.Date(2024, 1, 2, 16, 30, 0, 0, time.UTC)

	sessionRepo.getHistoryByStaffIDFunc = func(_ context.Context, _ int64, _, _ time.Time) ([]*activeModels.WorkSession, error) {
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

	absenceRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ time.Time) ([]*activeModels.StaffAbsence, error) {
		return nil, nil
	}

	data, filename, err := svc.ExportSessions(ctx, staffID, from, to, "csv")
	require.NoError(t, err)
	assert.NotEmpty(t, data)
	assert.Contains(t, filename, ".csv")
	assert.Contains(t, filename, "2024-01-01")
	assert.Contains(t, filename, "2024-01-07")

	// Verify UTF-8 BOM
	assert.Equal(t, byte(0xEF), data[0])
	assert.Equal(t, byte(0xBB), data[1])
	assert.Equal(t, byte(0xBF), data[2])

	// Verify semicolon separator
	content := string(data[3:])
	assert.Contains(t, content, "Datum;Wochentag;Start;Ende")
}

func TestWSExportSessions_XLSX_Success(t *testing.T) {
	svc, sessionRepo, breakRepo, auditRepo, absenceRepo, _ := wsCreateTestServiceWithAbsenceRepo()
	ctx := context.Background()
	staffID := int64(100)
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 7, 23, 59, 59, 0, time.UTC)

	sessionRepo.getHistoryByStaffIDFunc = func(_ context.Context, _ int64, _, _ time.Time) ([]*activeModels.WorkSession, error) {
		return []*activeModels.WorkSession{}, nil
	}

	auditRepo.countBySessionIDsFunc = func(_ context.Context, _ []int64) (map[int64]int, error) {
		return map[int64]int{}, nil
	}

	breakRepo.getBySessionIDFunc = func(_ context.Context, _ int64) ([]*activeModels.WorkSessionBreak, error) {
		return []*activeModels.WorkSessionBreak{}, nil
	}

	absenceRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ time.Time) ([]*activeModels.StaffAbsence, error) {
		return nil, nil
	}

	data, filename, err := svc.ExportSessions(ctx, staffID, from, to, "xlsx")
	require.NoError(t, err)
	assert.NotEmpty(t, data)
	assert.Contains(t, filename, ".xlsx")

	// Verify ZIP magic bytes (XLSX is a ZIP file)
	assert.Equal(t, byte(0x50), data[0]) // 'P'
	assert.Equal(t, byte(0x4B), data[1]) // 'K'
}

func TestWSExportSessions_GetHistoryError(t *testing.T) {
	svc, sessionRepo, _, _, _, _ := wsCreateTestServiceWithAbsenceRepo()

	sessionRepo.getHistoryByStaffIDFunc = func(_ context.Context, _ int64, _, _ time.Time) ([]*activeModels.WorkSession, error) {
		return nil, errors.New("database error")
	}

	data, filename, err := svc.ExportSessions(context.Background(), 100, time.Now(), time.Now(), "csv")
	require.Error(t, err)
	assert.Nil(t, data)
	assert.Empty(t, filename)
	assert.Contains(t, err.Error(), "failed to get sessions for export")
}

func TestWSExportSessions_GetAbsencesError(t *testing.T) {
	svc, sessionRepo, breakRepo, auditRepo, absenceRepo, _ := wsCreateTestServiceWithAbsenceRepo()

	sessionRepo.getHistoryByStaffIDFunc = func(_ context.Context, _ int64, _, _ time.Time) ([]*activeModels.WorkSession, error) {
		return []*activeModels.WorkSession{}, nil
	}

	auditRepo.countBySessionIDsFunc = func(_ context.Context, _ []int64) (map[int64]int, error) {
		return map[int64]int{}, nil
	}

	breakRepo.getBySessionIDFunc = func(_ context.Context, _ int64) ([]*activeModels.WorkSessionBreak, error) {
		return []*activeModels.WorkSessionBreak{}, nil
	}

	absenceRepo.getByStaffAndDateRangeFunc = func(_ context.Context, _ int64, _, _ time.Time) ([]*activeModels.StaffAbsence, error) {
		return nil, errors.New("absence repo error")
	}

	data, filename, err := svc.ExportSessions(context.Background(), 100, time.Now(), time.Now(), "csv")
	require.Error(t, err)
	assert.Nil(t, data)
	assert.Empty(t, filename)
	assert.Contains(t, err.Error(), "failed to get absences for export")
}

// ============================================================================
// buildExportRows Tests
// ============================================================================

func TestWSBuildExportRows_SessionsOnly(t *testing.T) {
	svc, _, _, _, _, _ := wsCreateTestServiceWithAbsenceRepo()

	date1 := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	date2 := time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)
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
	svc, _, _, _, _, _ := wsCreateTestServiceWithAbsenceRepo()

	dateStart := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	dateEnd := time.Date(2024, 1, 4, 0, 0, 0, 0, time.UTC)

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
	assert.Contains(t, rows[0].Row[6], "Krank")
	assert.Contains(t, rows[0].Row[7], "Flu")
}

func TestWSBuildExportRows_Mixed(t *testing.T) {
	svc, _, _, _, _, _ := wsCreateTestServiceWithAbsenceRepo()

	date1 := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	date2 := time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)
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
	svc, _, _, _, _, _ := wsCreateTestServiceWithAbsenceRepo()

	date1 := time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC)
	date2 := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
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

// ============================================================================
// sessionToRow Tests
// ============================================================================

func TestWSSessionToRow_Complete(t *testing.T) {
	svc, _, _, _, _, _ := wsCreateTestServiceWithAbsenceRepo()

	date := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC) // Monday
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
			Notes:        "Regular day",
		},
		NetMinutes: 480, // 8h 0min
	}

	row := svc.sessionToRow(sr)

	require.Len(t, row, 8)
	assert.Equal(t, "15.01.2024", row[0])  // Datum
	assert.Equal(t, "Montag", row[1])      // Wochentag
	assert.Equal(t, "08:30", row[2])       // Start
	assert.Equal(t, "17:15", row[3])       // Ende
	assert.Equal(t, "45", row[4])          // Pause
	assert.Equal(t, "8h 00min", row[5])    // Netto
	assert.Equal(t, "In der OGS", row[6])  // Ort
	assert.Equal(t, "Regular day", row[7]) // Bemerkungen
}

func TestWSSessionToRow_NoCheckOut(t *testing.T) {
	svc, _, _, _, _, _ := wsCreateTestServiceWithAbsenceRepo()

	date := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
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
	svc, _, _, _, _, _ := wsCreateTestServiceWithAbsenceRepo()

	date := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	checkIn := time.Date(2024, 1, 15, 8, 0, 0, 0, time.UTC)

	sr := &SessionResponse{
		WorkSession: &activeModels.WorkSession{
			Model:       base.Model{ID: 1},
			Date:        date,
			CheckInTime: checkIn,
			Status:      activeModels.WorkSessionStatusHomeOffice,
		},
		NetMinutes: 0,
	}

	row := svc.sessionToRow(sr)

	assert.Equal(t, "Homeoffice", row[6]) // Ort
}

func TestWSSessionToRow_NetMinutesFormatting(t *testing.T) {
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
			date := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
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
					Date:        wd.date,
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
// exportCSV Tests
// ============================================================================

func TestWSExportCSV_Headers(t *testing.T) {
	svc, _, _, _, _, _ := wsCreateTestServiceWithAbsenceRepo()

	rows := []exportRow{}
	data, err := svc.exportCSV(rows)

	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Skip BOM and parse CSV
	reader := csv.NewReader(bytes.NewReader(data[3:]))
	reader.Comma = ';'

	records, err := reader.ReadAll()
	require.NoError(t, err)
	require.Len(t, records, 1) // Only header

	header := records[0]
	assert.Equal(t, "Datum", header[0])
	assert.Equal(t, "Wochentag", header[1])
	assert.Equal(t, "Start", header[2])
	assert.Equal(t, "Ende", header[3])
	assert.Equal(t, "Pause (Min)", header[4])
	assert.Equal(t, "Netto (Std)", header[5])
	assert.Equal(t, "Ort", header[6])
	assert.Equal(t, "Bemerkungen", header[7])
}

func TestWSExportCSV_UTF8BOM(t *testing.T) {
	svc, _, _, _, _, _ := wsCreateTestServiceWithAbsenceRepo()

	data, err := svc.exportCSV([]exportRow{})
	require.NoError(t, err)

	assert.Equal(t, byte(0xEF), data[0])
	assert.Equal(t, byte(0xBB), data[1])
	assert.Equal(t, byte(0xBF), data[2])
}

func TestWSExportCSV_SemicolonSeparator(t *testing.T) {
	svc, _, _, _, _, _ := wsCreateTestServiceWithAbsenceRepo()

	date := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	rows := []exportRow{
		{
			Date: date,
			Row:  []string{"15.01.2024", "Montag", "08:00", "16:00", "30", "7h 30min", "In der OGS", "Test"},
		},
	}

	data, err := svc.exportCSV(rows)
	require.NoError(t, err)

	content := string(data[3:])
	lines := strings.Split(content, "\n")
	assert.True(t, strings.Contains(lines[1], ";"))
	assert.False(t, strings.Contains(lines[1], ","))
}

// ============================================================================
// exportXLSX Tests
// ============================================================================

func TestWSExportXLSX_ValidZipFormat(t *testing.T) {
	svc, _, _, _, _, _ := wsCreateTestServiceWithAbsenceRepo()

	rows := []exportRow{}
	data, err := svc.exportXLSX(rows)

	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Verify ZIP magic bytes
	assert.Equal(t, byte(0x50), data[0]) // 'P'
	assert.Equal(t, byte(0x4B), data[1]) // 'K'
}

func TestWSExportXLSX_WithData(t *testing.T) {
	svc, _, _, _, _, _ := wsCreateTestServiceWithAbsenceRepo()

	date := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	rows := []exportRow{
		{
			Date: date,
			Row:  []string{"15.01.2024", "Montag", "08:00", "16:00", "30", "7h 30min", "In der OGS", "Test"},
		},
	}

	data, err := svc.exportXLSX(rows)
	require.NoError(t, err)
	assert.NotEmpty(t, data)
	assert.True(t, len(data) > 100) // Should be substantial file
}

// ============================================================================
// UpdateSession Additional Tests
// ============================================================================

func TestWSUpdateSession_StatusChange(t *testing.T) {
	svc, sessionRepo, _, auditRepo, _ := wsCreateTestService()
	staffID := int64(100)
	sessionID := int64(100)

	sessionRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:       base.Model{ID: sessionID},
			StaffID:     staffID,
			CheckInTime: time.Now().Add(-8 * time.Hour),
			Status:      activeModels.WorkSessionStatusPresent,
			Date:        time.Now().Truncate(24 * time.Hour),
			CreatedBy:   staffID,
		}, nil
	}

	sessionRepo.updateFunc = func(_ context.Context, _ *activeModels.WorkSession) error {
		return nil
	}

	auditRepo.createBatchFunc = func(_ context.Context, edits []*auditModels.WorkSessionEdit) error {
		assert.Len(t, edits, 1)
		assert.Equal(t, auditModels.FieldStatus, edits[0].FieldName)
		return nil
	}

	newStatus := activeModels.WorkSessionStatusHomeOffice
	updates := SessionUpdateRequest{
		Status: &newStatus,
	}

	session, err := svc.UpdateSession(context.Background(), staffID, sessionID, updates)
	require.NoError(t, err)
	require.NotNil(t, session)
}

func TestWSUpdateSession_NotesChange(t *testing.T) {
	svc, sessionRepo, _, auditRepo, _ := wsCreateTestService()
	staffID := int64(100)
	sessionID := int64(100)

	sessionRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:       base.Model{ID: sessionID},
			StaffID:     staffID,
			CheckInTime: time.Now().Add(-8 * time.Hour),
			Status:      activeModels.WorkSessionStatusPresent,
			Date:        time.Now().Truncate(24 * time.Hour),
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
	svc, sessionRepo, _, auditRepo, _ := wsCreateTestService()
	staffID := int64(100)
	sessionID := int64(100)

	sessionRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:        base.Model{ID: sessionID},
			StaffID:      staffID,
			CheckInTime:  time.Now().Add(-8 * time.Hour),
			Status:       activeModels.WorkSessionStatusPresent,
			Date:         time.Now().Truncate(24 * time.Hour),
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
			Date:         time.Now().Truncate(24 * time.Hour),
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
	svc, sessionRepo, _, auditRepo, _ := wsCreateTestService()
	staffID := int64(100)
	sessionID := int64(100)

	sessionRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:       base.Model{ID: sessionID},
			StaffID:     staffID,
			CheckInTime: time.Now().Add(-8 * time.Hour),
			Status:      activeModels.WorkSessionStatusPresent,
			Date:        time.Now().Truncate(24 * time.Hour),
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
	svc, sessionRepo, _, auditRepo, _ := wsCreateTestService()
	staffID := int64(100)
	sessionID := int64(100)

	sessionRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.WorkSession, error) {
		return &activeModels.WorkSession{
			Model:       base.Model{ID: sessionID},
			StaffID:     staffID,
			CheckInTime: time.Now().Add(-8 * time.Hour),
			Status:      activeModels.WorkSessionStatusPresent,
			Date:        time.Now().Truncate(24 * time.Hour),
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
	svc, sessionRepo, _, _, _ := wsCreateTestService()

	sessionRepo.getOpenSessionsFunc = func(_ context.Context, _ time.Time) ([]*activeModels.WorkSession, error) {
		return nil, errors.New("database error")
	}

	count, err := svc.CleanupOpenSessions(context.Background())
	require.Error(t, err)
	assert.Equal(t, 0, count)
	assert.Contains(t, err.Error(), "failed to get open sessions")
}

func TestWSCleanupOpenSessions_CloseSessionError(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()
	yesterday := time.Now().AddDate(0, 0, -1).Truncate(24 * time.Hour)

	sessionRepo.getOpenSessionsFunc = func(_ context.Context, _ time.Time) ([]*activeModels.WorkSession, error) {
		return []*activeModels.WorkSession{
			{Model: base.Model{ID: 1}, Date: yesterday},
		}, nil
	}

	sessionRepo.closeSessionFunc = func(_ context.Context, _ int64, _ time.Time, _ bool) error {
		return errors.New("close error")
	}

	count, err := svc.CleanupOpenSessions(context.Background())
	require.Error(t, err)
	assert.Equal(t, 0, count)
	assert.Contains(t, err.Error(), "failed to close session")
}

func TestWSGetHistory_AuditCountError(t *testing.T) {
	svc, sessionRepo, _, auditRepo, _ := wsCreateTestService()

	sessionRepo.getHistoryByStaffIDFunc = func(_ context.Context, _ int64, _, _ time.Time) ([]*activeModels.WorkSession, error) {
		return []*activeModels.WorkSession{
			{Model: base.Model{ID: 1}},
		}, nil
	}

	auditRepo.countBySessionIDsFunc = func(_ context.Context, _ []int64) (map[int64]int, error) {
		return nil, errors.New("audit count error")
	}

	responses, err := svc.GetHistory(context.Background(), 100, time.Now(), time.Now())
	require.Error(t, err)
	assert.Nil(t, responses)
	assert.Contains(t, err.Error(), "failed to get edit counts")
}

func TestWSGetHistory_BreaksError(t *testing.T) {
	svc, sessionRepo, breakRepo, auditRepo, _ := wsCreateTestService()

	sessionRepo.getHistoryByStaffIDFunc = func(_ context.Context, _ int64, _, _ time.Time) ([]*activeModels.WorkSession, error) {
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

	responses, err := svc.GetHistory(context.Background(), 100, time.Now(), time.Now())
	require.Error(t, err)
	assert.Nil(t, responses)
	assert.Contains(t, err.Error(), "failed to get breaks")
}

func TestWSGetTodayPresenceMap_RepoError(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()

	sessionRepo.getTodayPresenceMapFunc = func(_ context.Context) (map[int64]string, error) {
		return nil, errors.New("database error")
	}

	presenceMap, err := svc.GetTodayPresenceMap(context.Background())
	require.Error(t, err)
	assert.Nil(t, presenceMap)
}

func TestWSGetSessionEdits_RepoError(t *testing.T) {
	svc, _, _, auditRepo, _ := wsCreateTestService()

	auditRepo.getBySessionIDFunc = func(_ context.Context, _ int64) ([]*auditModels.WorkSessionEdit, error) {
		return nil, errors.New("database error")
	}

	edits, err := svc.GetSessionEdits(context.Background(), 1)
	require.Error(t, err)
	assert.Nil(t, edits)
}

func TestWSGetSessionBreaks_RepoError(t *testing.T) {
	svc, _, breakRepo, _, _ := wsCreateTestService()

	breakRepo.getBySessionIDFunc = func(_ context.Context, _ int64) ([]*activeModels.WorkSessionBreak, error) {
		return nil, errors.New("database error")
	}

	breaks, err := svc.GetSessionBreaks(context.Background(), 1)
	require.Error(t, err)
	assert.Nil(t, breaks)
}

func TestWSCheckOut_BreakCheckError(t *testing.T) {
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

	session, err := svc.CheckOut(context.Background(), 100)
	require.Error(t, err)
	assert.Nil(t, session)
	assert.Contains(t, err.Error(), "failed to check active break")
}

func TestWSCheckOut_CloseSessionError(t *testing.T) {
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

	sessionRepo.closeSessionFunc = func(_ context.Context, _ int64, _ time.Time, _ bool) error {
		return errors.New("close error")
	}

	session, err := svc.CheckOut(context.Background(), 100)
	require.Error(t, err)
	assert.Nil(t, session)
	assert.Contains(t, err.Error(), "failed to close session")
}

func TestWSCheckOut_FindByIDError(t *testing.T) {
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

	sessionRepo.closeSessionFunc = func(_ context.Context, _ int64, _ time.Time, _ bool) error {
		return nil
	}

	supervisorRepo.endAllActiveByStaffIDFunc = func(_ context.Context, _ int64) (int, error) {
		return 0, nil
	}

	sessionRepo.findByIDFunc = func(_ context.Context, _ any) (*activeModels.WorkSession, error) {
		return nil, errors.New("find error")
	}

	session, err := svc.CheckOut(context.Background(), 100)
	require.Error(t, err)
	assert.Nil(t, session)
	assert.Contains(t, err.Error(), "failed to retrieve updated session")
}

func TestWSStartBreak_NilSession(t *testing.T) {
	svc, sessionRepo, _, _, _ := wsCreateTestService()

	sessionRepo.getCurrentByStaffIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
		return nil, nil
	}

	brk, err := svc.StartBreak(context.Background(), 100)
	require.Error(t, err)
	assert.Nil(t, brk)
	assert.Contains(t, err.Error(), "no active session found")
}

func TestWSEndBreak_EndBreakError(t *testing.T) {
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
	svc, sessionRepo, _, _, _ := wsCreateTestService()

	sessionRepo.getByStaffAndDateFunc = func(_ context.Context, _ int64, _ time.Time) (*activeModels.WorkSession, error) {
		return nil, sql.ErrNoRows
	}

	sessionRepo.createFunc = func(_ context.Context, _ *activeModels.WorkSession) error {
		return errors.New("create error")
	}

	session, err := svc.CheckIn(context.Background(), 100, activeModels.WorkSessionStatusPresent)
	require.Error(t, err)
	assert.Nil(t, session)
	assert.Contains(t, err.Error(), "failed to create work session")
}
