package presence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type AttendanceCleanup interface {
	ListAttendance(context.Context, studentpresence.AttendanceFilter) ([]studentpresence.Attendance, error)
	CloseStaleAttendance(context.Context, int64, time.Time, time.Time) (int64, error)
}

type PresenceRetention interface {
	AttendanceCleanup
	studentpresence.VisitRetentionQuery
	DeleteCompletedVisitsBefore(context.Context, int64, studentpresence.VisitRetentionPolicy, time.Time) (int64, error)
}

// VisitRetentionSettings supplies the tenant's visit retention window for
// children without an accepted consent: gdpr.privacy_consent_retention_days,
// tenant override or registry default (#3835).
type VisitRetentionSettings interface {
	VisitHistoryRetention(context.Context) (bool, int, error)
}

// cleanupService implements the CleanupService interface
type cleanupService struct {
	presence         PresenceRetention
	supervisorRepo   ports.GroupSupervisorRepository
	dataDeletionRepo DeletionAudit
	settings         VisitRetentionSettings
	txHandler        *tenant.TransactionRunner
	batchSize        int
	today            func() calendar.Date
}

func (s *cleanupService) todayDate() calendar.Date {
	if s.today != nil {
		return s.today()
	}
	return calendar.TodayDate()
}

// NewCleanupService creates a new cleanup service instance
func NewCleanupService(
	presence PresenceRetention,
	supervisorRepo ports.GroupSupervisorRepository,
	dataDeletionRepo DeletionAudit,
	settings VisitRetentionSettings,
	today ...func() calendar.Date,
) CleanupService {
	service := &cleanupService{
		presence:         presence,
		supervisorRepo:   supervisorRepo,
		dataDeletionRepo: dataDeletionRepo,
		settings:         settings,
		txHandler:        tenant.NewTransactionRunner(),
		batchSize:        100, // Process 100 students at a time
	}
	if len(today) > 0 {
		service.today = today[0]
	}
	return service
}

func (s *cleanupService) retentionPolicy(ctx context.Context) (studentpresence.VisitRetentionPolicy, error) {
	if s.settings == nil {
		return studentpresence.VisitRetentionPolicy{}, errors.New("visit retention settings are not configured")
	}
	enabled, days, err := s.settings.VisitHistoryRetention(ctx)
	if err != nil {
		return studentpresence.VisitRetentionPolicy{}, fmt.Errorf("resolve attendance log: %w", err)
	}
	if !enabled {
		cutoff := s.todayDate().BerlinMidnight()
		return studentpresence.VisitRetentionPolicy{CompletedBefore: &cutoff}, nil
	}
	if days < 1 {
		return studentpresence.VisitRetentionPolicy{}, fmt.Errorf("invalid visit retention days: %d", days)
	}
	return studentpresence.VisitRetentionPolicy{DefaultDays: days}, nil
}

// ApplyAttendanceLogSetting removes older completed visits in the setting's
// transaction. Failure of any deletion or audit aborts the setting change.
func (s *cleanupService) ApplyAttendanceLogSetting(ctx context.Context, enabled bool) error {
	if enabled {
		return nil
	}
	cutoff := s.todayDate().BerlinMidnight()
	policy := studentpresence.VisitRetentionPolicy{CompletedBefore: &cutoff}
	if err := tenant.RequireTransaction(ctx); err != nil {
		return err
	}
	rows, err := s.presence.ListVisitRetentionCounts(ctx, policy)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if _, err := s.processStudent(ctx, row); err != nil {
			return err
		}
	}
	return nil
}

// CleanupExpiredVisits deletes the completed visits of every student whose
// retention window has passed, with one audit row per student.
func (s *cleanupService) CleanupExpiredVisits(ctx context.Context) (*CleanupResult, error) {
	result := &CleanupResult{
		StartedAt: time.Now(),
		Errors:    make([]CleanupError, 0),
		Success:   true,
	}

	var students []studentpresence.VisitRetentionCount
	err := s.txHandler.RunInTx(ctx, func(txCtx context.Context) error {
		policy, err := s.retentionPolicy(txCtx)
		if err != nil {
			return err
		}
		students, err = s.presence.ListVisitRetentionCounts(txCtx, policy)
		if err != nil {
			return fmt.Errorf("failed to get students with expired visits: %w", err)
		}
		return nil
	})
	if err != nil {
		result.Success = false
		result.CompletedAt = time.Now()
		return result, err
	}

	// Process students in batches
	for i := 0; i < len(students); i += s.batchSize {
		end := i + s.batchSize
		if end > len(students) {
			end = len(students)
		}

		batch := students[i:end]
		batchResult := s.processBatch(ctx, batch)

		// Aggregate results
		result.StudentsProcessed += batchResult.processed
		result.RecordsDeleted += batchResult.deleted
		result.Errors = append(result.Errors, batchResult.errors...)

		if len(batchResult.errors) > 0 {
			result.Success = false
		}
	}

	result.CompletedAt = time.Now()
	return result, nil
}

// GetRetentionStatistics gets statistics about data that will be deleted
func (s *cleanupService) GetRetentionStatistics(ctx context.Context) (*RetentionStats, error) {
	policy, err := s.retentionPolicy(ctx)
	if err != nil {
		return nil, err
	}
	stats := &RetentionStats{
		ExpiredVisitsByMonth: make(map[string]int64),
	}

	// Get total expired visits count
	count, err := s.presence.CountExpiredVisits(ctx, policy)
	if err != nil {
		return nil, fmt.Errorf("failed to count expired visits: %w", err)
	}
	stats.TotalExpiredVisits = count

	// Get per-student statistics
	studentStats, err := s.presence.ListVisitRetentionCounts(ctx, policy)
	if err != nil {
		return nil, fmt.Errorf("failed to get visit retention stats: %w", err)
	}
	stats.StudentsAffected = len(studentStats)

	// Get oldest expired visit
	oldest, err := s.presence.OldestExpiredVisitDate(ctx, policy)
	if err != nil {
		return nil, fmt.Errorf("failed to find oldest expired visit: %w", err)
	}
	if oldest != nil {
		stats.OldestExpiredVisit = oldest

		// Get monthly breakdown
		monthly, err := s.presence.ListExpiredVisitMonths(ctx, policy)
		if err != nil {
			return nil, fmt.Errorf("failed to get monthly expired visits: %w", err)
		}
		for _, row := range monthly {
			stats.ExpiredVisitsByMonth[row.Month] = row.Count
		}
	}

	return stats, nil
}

// PreviewCleanup shows what would be deleted without actually deleting
func (s *cleanupService) PreviewCleanup(ctx context.Context) (*CleanupPreview, error) {
	policy, err := s.retentionPolicy(ctx)
	if err != nil {
		return nil, err
	}
	preview := &CleanupPreview{
		StudentVisitCounts: make(map[int64]int),
	}

	// Get per-student statistics
	studentStats, err := s.presence.ListVisitRetentionCounts(ctx, policy)
	if err != nil {
		return nil, fmt.Errorf("failed to get visit retention stats: %w", err)
	}

	for _, row := range studentStats {
		preview.StudentVisitCounts[row.StudentID] = row.Count
	}

	// Calculate total
	var total int64
	for _, count := range studentStats {
		total += int64(count.Count)
	}
	preview.TotalVisits = total

	// Get oldest visit that would be deleted
	oldest, err := s.presence.OldestExpiredVisitDate(ctx, policy)
	if err != nil {
		return nil, fmt.Errorf("failed to find oldest expired visit: %w", err)
	}
	preview.OldestVisit = oldest

	return preview, nil
}

// Helper methods

type batchResult struct {
	processed int
	deleted   int64
	errors    []CleanupError
}

func (s *cleanupService) processBatch(ctx context.Context, students []studentpresence.VisitRetentionCount) batchResult {
	result := batchResult{
		errors: make([]CleanupError, 0),
	}

	for _, student := range students {
		result.processed++

		// Process each student
		deleted, err := s.processStudent(ctx, student)
		if err != nil {
			result.errors = append(result.errors, CleanupError{
				StudentID: student.StudentID,
				Error:     err.Error(),
				Timestamp: time.Now(),
			})
		} else {
			result.deleted += deleted
		}
	}

	return result
}

// Phase 3 deviation: RunInTx retained because processStudent runs from the scheduler's forEachTenant loop,
// which already injects tenant context per-iteration. Handler-level WithTenantTx is not applicable
// because there is no HTTP handler or JWT involved in scheduled batch cleanup.
func (s *cleanupService) processStudent(ctx context.Context, student studentpresence.VisitRetentionCount) (int64, error) {
	var deletedCount int64

	err := s.txHandler.RunInTx(ctx, func(ctx context.Context) error {
		// Delete expired visits
		policy := studentpresence.VisitRetentionPolicy{DefaultDays: student.RetentionDays, CompletedBefore: student.CompletedBefore}
		count, err := s.presence.DeleteCompletedVisitsBefore(ctx, student.StudentID, policy, time.Now().AddDate(0, 0, -student.RetentionDays))
		if err != nil {
			return err
		}
		deletedCount = count

		if deletedCount > 0 {
			// Create audit record
			deletion := &DeletionEvent{
				TenantID: tenant.FromContext(ctx), StudentID: &student.StudentID,
				DeletionType: "visit_retention", RecordsDeleted: int(deletedCount),
				DeletedBy: "system", DeletedAt: time.Now(),
				DeletionReason: fmt.Sprintf("Automated retention policy: %d days", student.RetentionDays),
				Metadata:       map[string]interface{}{"retention_days": student.RetentionDays, "batch_cleanup": true},
			}

			if student.CompletedBefore != nil {
				deletion.DeletionReason = "Attendance log disabled: completed visits before today's Berlin midnight"
				deletion.Metadata = map[string]interface{}{"attendance_log_enabled": false, "completed_before": student.CompletedBefore.Format(time.RFC3339), "batch_cleanup": true}
			}
			if err := s.dataDeletionRepo.Create(ctx, deletion); err != nil {
				return err
			}
		}

		return nil
	})

	return deletedCount, err
}

// staleAttendanceCheckOut returns the record's date and the check-out time
// that closes it: normally 23:59:59 of the record's date, but check_in_time
// plus one second when the check-in lies after that (a data integrity issue),
// so the check-in-before-check-out constraint holds.
func staleAttendanceCheckOut(record studentpresence.Attendance) (calendar.Date, time.Time, error) {
	date, err := calendar.ParseDate(record.Date)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("invalid attendance date for record %d: %w", record.ID, err)
	}
	endOfDay := date.EndOfDay()
	if record.CheckInTime.After(endOfDay) {
		return date, record.CheckInTime.Add(time.Second), nil
	}
	return date, endOfDay, nil
}

// closeStaleRecord closes one stale record and reports whether it closed a
// row. A failure is recorded in the result; the cleanup goes on with the next
// record.
func (s *cleanupService) closeStaleRecord(ctx context.Context, record studentpresence.Attendance, checkOutTime time.Time, result *AttendanceCleanupResult) bool {
	closed, err := s.presence.CloseStaleAttendance(ctx, record.ID, checkOutTime, time.Now())
	if err != nil {
		errMsg := fmt.Sprintf("Failed to close attendance record %d: %v", record.ID, err)
		result.Errors = append(result.Errors, errMsg)
		result.Success = false
		return false
	}
	return closed != 0
}

// CleanupStaleAttendance closes attendance records from previous days that lack check-out times
func (s *cleanupService) CleanupStaleAttendance(ctx context.Context) (*AttendanceCleanupResult, error) {
	result := &AttendanceCleanupResult{
		StartedAt: time.Now(),
		Success:   true,
		Errors:    make([]string, 0),
	}

	today := s.todayDate()

	// Find all attendance records from before today that don't have check-out times
	staleRecords, err := s.presence.ListAttendance(ctx, studentpresence.AttendanceFilter{BeforeDate: today.String(), OpenOnly: true})
	if err != nil {
		result.Success = false
		result.CompletedAt = time.Now()
		return result, fmt.Errorf("failed to find stale attendance records: %w", err)
	}

	if len(staleRecords) == 0 {
		result.CompletedAt = time.Now()
		return result, nil
	}

	// Track statistics
	studentsAffected := make(map[int64]bool)
	var oldestRecord *calendar.Date

	// Close each stale record by setting check-out time
	for _, record := range staleRecords {
		date, checkOutTime, err := staleAttendanceCheckOut(record)
		if err != nil {
			result.Success = false
			result.CompletedAt = time.Now()
			return result, err
		}

		if !s.closeStaleRecord(ctx, record, checkOutTime, result) {
			continue
		}
		result.RecordsClosed++
		studentsAffected[record.StudentID] = true

		// Track oldest record
		if oldestRecord == nil || date.Before(*oldestRecord) {
			oldestRecord = &date
		}
	}

	result.StudentsAffected = len(studentsAffected)
	result.OldestRecordDate = oldestRecord
	result.CompletedAt = time.Now()

	return result, nil
}

// PreviewAttendanceCleanup shows what attendance records would be cleaned
func (s *cleanupService) PreviewAttendanceCleanup(ctx context.Context) (*AttendanceCleanupPreview, error) {
	preview := &AttendanceCleanupPreview{
		StudentRecords: make(map[int64]int),
		RecordsByDate:  make(map[string]int),
	}

	today := s.todayDate()

	// Find all stale attendance records
	staleRecords, err := s.presence.ListAttendance(ctx, studentpresence.AttendanceFilter{BeforeDate: today.String(), OpenOnly: true})
	if err != nil {
		return nil, fmt.Errorf("failed to preview stale attendance records: %w", err)
	}

	preview.TotalRecords = len(staleRecords)

	// Build statistics
	for _, record := range staleRecords {
		// Track per-student counts
		preview.StudentRecords[record.StudentID]++

		// Track per-date counts
		date, err := calendar.ParseDate(record.Date)
		if err != nil {
			return nil, fmt.Errorf("invalid attendance date for record %d: %w", record.ID, err)
		}
		preview.RecordsByDate[record.Date]++

		// Track oldest record
		if preview.OldestRecord == nil || date.Before(*preview.OldestRecord) {
			preview.OldestRecord = &date
		}
	}

	return preview, nil
}

// CleanupStaleSupervisors closes supervisor records from previous days that lack end_date
func (s *cleanupService) CleanupStaleSupervisors(ctx context.Context) (*SupervisorCleanupResult, error) {
	result := &SupervisorCleanupResult{
		StartedAt: time.Now(),
		Success:   true,
		Errors:    make([]string, 0),
	}

	// Today as a Berlin calendar day; binds as a DATE literal.
	today := s.todayDate()

	// Find all supervisor records from before today that don't have end_date
	staleRecords, err := s.supervisorRepo.FindStaleOpen(ctx, today)
	if err != nil {
		result.Success = false
		result.CompletedAt = time.Now()
		return result, fmt.Errorf("failed to find stale supervisor records: %w", err)
	}

	if len(staleRecords) == 0 {
		result.CompletedAt = time.Now()
		return result, nil
	}

	// Track statistics
	staffAffected := make(map[int64]bool)
	var oldestRecord *calendar.Date

	// Close each stale record by setting end_date to start_date
	for _, record := range staleRecords {
		endDate := record.StartDate

		// Update the record
		record.EndDate = &endDate
		record.UpdatedAt = time.Now()
		if _, err := s.supervisorRepo.SetEndDate(ctx, record); err != nil {
			errMsg := fmt.Sprintf("Failed to close supervisor record %d: %v", record.ID, err)
			result.Errors = append(result.Errors, errMsg)
			result.Success = false
			continue
		}

		result.RecordsClosed++
		staffAffected[record.StaffID] = true

		// Track oldest record
		if oldestRecord == nil || record.StartDate.Before(*oldestRecord) {
			oldestRecord = &record.StartDate
		}
	}

	result.StaffAffected = len(staffAffected)
	result.OldestRecordDate = oldestRecord
	result.CompletedAt = time.Now()

	return result, nil
}

// PreviewSupervisorCleanup shows what supervisor records would be cleaned
func (s *cleanupService) PreviewSupervisorCleanup(ctx context.Context) (*SupervisorCleanupPreview, error) {
	preview := &SupervisorCleanupPreview{
		StaffRecords:  make(map[int64]int),
		RecordsByDate: make(map[string]int),
	}

	// Today as a Berlin calendar day; binds as a DATE literal.
	today := s.todayDate()

	// Find all stale supervisor records
	staleRecords, err := s.supervisorRepo.FindStaleOpen(ctx, today)
	if err != nil {
		return nil, fmt.Errorf("failed to preview stale supervisor records: %w", err)
	}

	preview.TotalRecords = len(staleRecords)

	// Build statistics
	for _, record := range staleRecords {
		// Track per-staff counts
		preview.StaffRecords[record.StaffID]++

		// Track per-date counts
		dateStr := record.StartDate.String()
		preview.RecordsByDate[dateStr]++

		// Track oldest record
		if preview.OldestRecord == nil || record.StartDate.Before(*preview.OldestRecord) {
			preview.OldestRecord = &record.StartDate
		}
	}

	return preview, nil
}
