package active

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// cleanupService implements the CleanupService interface
type cleanupService struct {
	visitRepo          active.VisitRepository
	attendanceRepo     active.AttendanceRepository
	privacyConsentRepo userModels.PrivacyConsentRepository
	dataDeletionRepo   audit.DataDeletionRepository
	txHandler          *base.TxHandler
	batchSize          int
}

// NewCleanupService creates a new cleanup service instance
func NewCleanupService(
	visitRepo active.VisitRepository,
	attendanceRepo active.AttendanceRepository,
	privacyConsentRepo userModels.PrivacyConsentRepository,
	dataDeletionRepo audit.DataDeletionRepository,
	db *bun.DB,
) CleanupService {
	return &cleanupService{
		visitRepo:          visitRepo,
		attendanceRepo:     attendanceRepo,
		privacyConsentRepo: privacyConsentRepo,
		dataDeletionRepo:   dataDeletionRepo,
		txHandler:          base.NewTxHandler(db),
		batchSize:          100, // Process 100 students at a time
	}
}

// CleanupExpiredVisits runs the cleanup process for all students
func (s *cleanupService) CleanupExpiredVisits(ctx context.Context) (*CleanupResult, error) {
	result := &CleanupResult{
		StartedAt: time.Now(),
		Errors:    make([]CleanupError, 0),
		Success:   true,
	}

	// Get all students with privacy consents
	students, err := s.getStudentsWithRetentionSettings(ctx)
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

// CleanupVisitsForStudent runs cleanup for a specific student
func (s *cleanupService) CleanupVisitsForStudent(ctx context.Context, studentID int64) (int64, error) {
	// Get student's privacy consents
	consents, err := s.privacyConsentRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return 0, fmt.Errorf("failed to get privacy consent: %w", err)
	}

	var consent *userModels.PrivacyConsent
	// Get the most recent accepted consent
	for _, c := range consents {
		if c.Accepted && (consent == nil || c.CreatedAt.After(consent.CreatedAt)) {
			consent = c
		}
	}

	if consent == nil {
		// No consent found, use default 30 days
		consent = &userModels.PrivacyConsent{
			StudentID:         studentID,
			DataRetentionDays: 30,
		}
	}

	// Execute cleanup in transaction
	var deletedCount int64
	err = s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		// Delete expired visits
		count, err := s.visitRepo.DeleteExpiredVisits(ctx, studentID, consent.GetDataRetentionDays())
		if err != nil {
			return fmt.Errorf("failed to delete expired visits: %w", err)
		}
		deletedCount = count

		if deletedCount > 0 {
			// Create audit record
			deletion := audit.NewDataDeletion(
				studentID,
				audit.DeletionTypeVisitRetention,
				int(deletedCount),
				"system",
			)
			deletion.DeletionReason = fmt.Sprintf("Data retention policy: %d days", consent.GetDataRetentionDays())
			deletion.SetMetadata("retention_days", consent.GetDataRetentionDays())
			deletion.SetMetadata("consent_id", consent.ID)

			if err := s.dataDeletionRepo.Create(ctx, deletion); err != nil {
				return fmt.Errorf("failed to create audit record: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		return 0, err
	}

	return deletedCount, nil
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

	// Get oldest expired visit via repository
	oldestVisit, err := s.visitRepo.GetOldestExpiredVisit(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get oldest expired visit: %w", err)
	}
	stats.OldestExpiredVisit = oldestVisit

	// Get monthly breakdown via repository
	if oldestVisit != nil {
		monthlyStats, err := s.visitRepo.GetExpiredVisitsByMonth(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get expired visits by month: %w", err)
		}
		stats.ExpiredVisitsByMonth = monthlyStats
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

	// Get oldest visit that would be deleted via repository
	oldestVisit, err := s.visitRepo.GetOldestExpiredVisit(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get oldest expired visit: %w", err)
	}
	preview.OldestVisit = oldestVisit

	return preview, nil
}

// Helper methods

type studentWithConsent struct {
	StudentID         int64
	DataRetentionDays int
}

func (s *cleanupService) getStudentsWithRetentionSettings(ctx context.Context) ([]studentWithConsent, error) {
	// Use repository method to get students with retention settings
	settings, err := s.privacyConsentRepo.GetStudentsWithRetentionSettings(ctx)
	if err != nil {
		return nil, err
	}

	// Convert to local type (same structure, just different package)
	students := make([]studentWithConsent, len(settings))
	for i, setting := range settings {
		students[i] = studentWithConsent{
			StudentID:         setting.StudentID,
			DataRetentionDays: setting.DataRetentionDays,
		}
	}

	return students, nil
}

type batchResult struct {
	processed int
	deleted   int64
	errors    []CleanupError
}

func (s *cleanupService) processBatch(ctx context.Context, students []studentWithConsent) batchResult {
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

func (s *cleanupService) processStudent(ctx context.Context, student studentWithConsent) (int64, error) {
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

	// Get today's date at midnight (start of day) - use UTC to match database
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	// Find all attendance records from before today that don't have check-out times
	staleRecords, err := s.attendanceRepo.FindStaleRecords(ctx, today)
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
	var oldestRecord *time.Time

	// Close each stale record by setting check-out time
	for _, record := range staleRecords {
		// Set check-out time to 11:59:59 PM of the record's date
		endOfDay := time.Date(
			record.Date.Year(), record.Date.Month(), record.Date.Day(),
			23, 59, 59, 0, record.Date.Location(),
		)

		// Handle edge case: if check_in_time is after end of day (corrupted data),
		// set check_out_time to 1 second after check_in_time to satisfy the constraint
		checkOutTime := endOfDay
		if record.CheckInTime.After(endOfDay) {
			checkOutTime = record.CheckInTime.Add(time.Second)
		}

		// Update the record via repository
		if err := s.attendanceRepo.CloseStaleRecord(ctx, record.ID, checkOutTime); err != nil {
			errMsg := fmt.Sprintf("Failed to close attendance record %d: %v", record.ID, err)
			result.Errors = append(result.Errors, errMsg)
			result.Success = false
			continue
		}

		result.RecordsClosed++
		studentsAffected[record.StudentID] = true

		// Track oldest record
		if oldestRecord == nil || record.Date.Before(*oldestRecord) {
			recordDate := record.Date
			oldestRecord = &recordDate
		}
	}

	result.StudentsAffected = len(studentsAffected)
	result.OldestRecordDate = oldestRecord
	result.CompletedAt = time.Now()

	// Create audit log entry
	if result.RecordsClosed > 0 {
		deletion := audit.NewDataDeletion(
			0, // No specific student (affects multiple)
			"attendance_cleanup",
			result.RecordsClosed,
			"system",
		)
		deletion.DeletionReason = fmt.Sprintf("Automated cleanup of stale attendance records from before %s", today.Format("2006-01-02"))
		deletion.SetMetadata("students_affected", result.StudentsAffected)
		deletion.SetMetadata("oldest_record", oldestRecord)

		if err := s.dataDeletionRepo.Create(ctx, deletion); err != nil {
			// Log error but don't fail the cleanup
			errMsg := fmt.Sprintf("Failed to create audit record: %v", err)
			result.Errors = append(result.Errors, errMsg)
		}
	}

	return result, nil
}

// PreviewAttendanceCleanup shows what attendance records would be cleaned
func (s *cleanupService) PreviewAttendanceCleanup(ctx context.Context) (*AttendanceCleanupPreview, error) {
	preview := &AttendanceCleanupPreview{
		StudentRecords: make(map[int64]int),
		RecordsByDate:  make(map[string]int),
	}

	// Get today's date at midnight - use UTC to match database
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	// Find all stale attendance records via repository
	staleRecords, err := s.attendanceRepo.FindStaleRecords(ctx, today)
	if err != nil {
		return nil, fmt.Errorf("failed to preview stale attendance records: %w", err)
	}

	preview.TotalRecords = len(staleRecords)

	// Build statistics
	for _, record := range staleRecords {
		// Track per-student counts
		preview.StudentRecords[record.StudentID]++

		// Track per-date counts
		dateStr := record.Date.Format("2006-01-02")
		preview.RecordsByDate[dateStr]++

		// Track oldest record
		if preview.OldestRecord == nil || record.Date.Before(*preview.OldestRecord) {
			recordDate := record.Date
			preview.OldestRecord = &recordDate
		}
	}

	return preview, nil
}
