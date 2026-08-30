package active

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// ConsentRetentionResolver resolves the data-retention window for a privacy
// consent, honouring the per-tenant settings default (issue #586, Rule 12: the
// retention default no longer lives on the PrivacyConsent model).
type ConsentRetentionResolver interface {
	ResolveDataRetentionDays(ctx context.Context, consent *userModels.PrivacyConsent) int
}

// cleanupService implements the CleanupService interface
type cleanupService struct {
	visitRepo          active.VisitRepository
	attendanceRepo     active.AttendanceRepository
	supervisorRepo     active.GroupSupervisorRepository
	privacyConsentRepo userModels.PrivacyConsentRepository
	dataDeletionRepo   audit.DataDeletionRepository
	consentRetention   ConsentRetentionResolver
	txHandler          *base.TxHandler
	batchSize          int
	today              func() timezone.Date
}

func (s *cleanupService) todayDate() timezone.Date {
	if s.today != nil {
		return s.today()
	}
	return timezone.TodayDate()
}

// NewCleanupService creates a new cleanup service instance
func NewCleanupService(
	visitRepo active.VisitRepository,
	attendanceRepo active.AttendanceRepository,
	supervisorRepo active.GroupSupervisorRepository,
	privacyConsentRepo userModels.PrivacyConsentRepository,
	dataDeletionRepo audit.DataDeletionRepository,
	consentRetention ConsentRetentionResolver,
	db *bun.DB,
	today ...func() timezone.Date,
) CleanupService {
	service := &cleanupService{
		visitRepo:          visitRepo,
		attendanceRepo:     attendanceRepo,
		supervisorRepo:     supervisorRepo,
		privacyConsentRepo: privacyConsentRepo,
		dataDeletionRepo:   dataDeletionRepo,
		consentRetention:   consentRetention,
		txHandler:          base.NewTxHandler(db),
		batchSize:          100, // Process 100 students at a time
	}
	if len(today) > 0 {
		service.today = today[0]
	}
	return service
}

// CleanupExpiredVisits runs the cleanup process for all students
func (s *cleanupService) CleanupExpiredVisits(ctx context.Context) (*CleanupResult, error) {
	result := &CleanupResult{
		StartedAt: time.Now(),
		Errors:    make([]CleanupError, 0),
		Success:   true,
	}

	// Get all students with privacy consents
	students, err := s.privacyConsentRepo.ListAcceptedRetentionSettings(ctx)
	if err != nil {
		result.Success = false
		result.CompletedAt = time.Now()
		return result, fmt.Errorf("failed to get students with retention settings: %w", err)
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
	stats := &RetentionStats{
		ExpiredVisitsByMonth: make(map[string]int64),
	}

	// Get total expired visits count
	count, err := s.visitRepo.CountExpiredVisits(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count expired visits: %w", err)
	}
	stats.TotalExpiredVisits = count

	// Get per-student statistics
	studentStats, err := s.visitRepo.GetVisitRetentionStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get visit retention stats: %w", err)
	}
	stats.StudentsAffected = len(studentStats)

	// Get oldest expired visit
	if oldest, err := s.visitRepo.OldestExpiredVisitDate(ctx); err == nil && oldest != nil {
		stats.OldestExpiredVisit = oldest

		// Get monthly breakdown
		if monthly, err := s.visitRepo.ExpiredVisitMonthlyCounts(ctx); err == nil {
			stats.ExpiredVisitsByMonth = monthly
		}
	}

	return stats, nil
}

// PreviewCleanup shows what would be deleted without actually deleting
func (s *cleanupService) PreviewCleanup(ctx context.Context) (*CleanupPreview, error) {
	preview := &CleanupPreview{
		StudentVisitCounts: make(map[int64]int),
	}

	// Get per-student statistics
	studentStats, err := s.visitRepo.GetVisitRetentionStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get visit retention stats: %w", err)
	}

	preview.StudentVisitCounts = studentStats

	// Calculate total
	var total int64
	for _, count := range studentStats {
		total += int64(count)
	}
	preview.TotalVisits = total

	// Get oldest visit that would be deleted
	if oldest, err := s.visitRepo.OldestExpiredVisitDate(ctx); err == nil && oldest != nil {
		preview.OldestVisit = oldest
	}

	return preview, nil
}

// Helper methods

type batchResult struct {
	processed int
	deleted   int64
	errors    []CleanupError
}

func (s *cleanupService) processBatch(ctx context.Context, students []userModels.StudentRetentionSetting) batchResult {
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
func (s *cleanupService) processStudent(ctx context.Context, student userModels.StudentRetentionSetting) (int64, error) {
	var deletedCount int64

	err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		// Delete expired visits
		count, err := s.visitRepo.DeleteExpiredVisits(ctx, student.StudentID, student.DataRetentionDays)
		if err != nil {
			return err
		}
		deletedCount = count

		if deletedCount > 0 {
			// Create audit record
			deletion := audit.NewDataDeletion(
				student.StudentID,
				audit.DeletionTypeVisitRetention,
				int(deletedCount),
				"system",
			)
			deletion.SetTenantID(tenant.FromContext(ctx))
			deletion.DeletionReason = fmt.Sprintf("Automated retention policy: %d days", student.DataRetentionDays)
			deletion.SetMetadata("retention_days", student.DataRetentionDays)
			deletion.SetMetadata("batch_cleanup", true)

			if err := s.dataDeletionRepo.Create(ctx, deletion); err != nil {
				return err
			}
		}

		return nil
	})

	return deletedCount, err
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
	staleRecords, err := s.attendanceRepo.FindStaleOpen(ctx, today)
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
	var oldestRecord *timezone.Date

	// Close each stale record by setting check-out time
	for _, record := range staleRecords {
		// Calculate appropriate check-out time:
		// - Normally use 23:59:59 of the record's date
		// - But if check_in_time is after that (data integrity issue), use check_in_time + 1 second
		endOfDay := record.Date.EndOfDay()
		checkOutTime := endOfDay
		if record.CheckInTime.After(endOfDay) {
			// check_in_time is after end of day - this is a data integrity issue
			// Use check_in_time + 1 second to satisfy the constraint
			checkOutTime = record.CheckInTime.Add(time.Second)
		}

		// Update the record
		record.CheckOutTime = &checkOutTime
		record.UpdatedAt = time.Now()
		if _, err := s.attendanceRepo.UpdateColumns(ctx, record, "check_out_time", "updated_at"); err != nil {
			errMsg := fmt.Sprintf("Failed to close attendance record %d: %v", record.ID, err)
			result.Errors = append(result.Errors, errMsg)
			result.Success = false
			continue
		}

		result.RecordsClosed++
		studentsAffected[record.StudentID] = true

		// Track oldest record
		if oldestRecord == nil || record.Date.Before(*oldestRecord) {
			oldestRecord = &record.Date
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
	staleRecords, err := s.attendanceRepo.FindStaleOpen(ctx, today)
	if err != nil {
		return nil, fmt.Errorf("failed to preview stale attendance records: %w", err)
	}

	preview.TotalRecords = len(staleRecords)

	// Build statistics
	for _, record := range staleRecords {
		// Track per-student counts
		preview.StudentRecords[record.StudentID]++

		// Track per-date counts
		dateStr := record.Date.String()
		preview.RecordsByDate[dateStr]++

		// Track oldest record
		if preview.OldestRecord == nil || record.Date.Before(*preview.OldestRecord) {
			preview.OldestRecord = &record.Date
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
	var oldestRecord *timezone.Date

	// Close each stale record by setting end_date to start_date
	for _, record := range staleRecords {
		endDate := record.StartDate

		// Update the record
		record.EndDate = &endDate
		record.UpdatedAt = time.Now()
		if _, err := s.supervisorRepo.UpdateColumns(ctx, record, "end_date", "updated_at"); err != nil {
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
