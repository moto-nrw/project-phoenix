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
	visitRepo            active.VisitRepository
	privacyConsentRepo   userModels.PrivacyConsentRepository
	dataDeletionRepo     audit.DataDeletionRepository
	db                   *bun.DB
	txHandler            *base.TxHandler
	batchSize            int
}

// NewCleanupService creates a new cleanup service instance
func NewCleanupService(
	visitRepo active.VisitRepository,
	privacyConsentRepo userModels.PrivacyConsentRepository,
	dataDeletionRepo audit.DataDeletionRepository,
	db *bun.DB,
) CleanupService {
	return &cleanupService{
		visitRepo:          visitRepo,
		privacyConsentRepo: privacyConsentRepo,
		dataDeletionRepo:   dataDeletionRepo,
		db:                 db,
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

	// Get oldest expired visit
	var oldestVisit struct {
		CreatedAt time.Time `bun:"created_at"`
	}
	err = s.db.NewRaw(`
		SELECT MIN(v.created_at) as created_at
		FROM active.visits v
		INNER JOIN users.privacy_consents pc ON pc.student_id = v.student_id
		WHERE v.exit_time IS NOT NULL
			AND v.created_at < NOW() - (pc.data_retention_days || ' days')::INTERVAL
	`).Scan(ctx, &oldestVisit)
	
	if err == nil && !oldestVisit.CreatedAt.IsZero() {
		stats.OldestExpiredVisit = &oldestVisit.CreatedAt
		
		// Get monthly breakdown
		var monthlyStats []struct {
			Month string `bun:"month"`
			Count int64  `bun:"count"`
		}
		err = s.db.NewRaw(`
			SELECT 
				TO_CHAR(v.created_at, 'YYYY-MM') as month,
				COUNT(*) as count
			FROM active.visits v
			INNER JOIN users.privacy_consents pc ON pc.student_id = v.student_id
			WHERE v.exit_time IS NOT NULL
				AND v.created_at < NOW() - (pc.data_retention_days || ' days')::INTERVAL
			GROUP BY TO_CHAR(v.created_at, 'YYYY-MM')
			ORDER BY month
		`).Scan(ctx, &monthlyStats)
		
		if err == nil {
			for _, ms := range monthlyStats {
				stats.ExpiredVisitsByMonth[ms.Month] = ms.Count
			}
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
	var oldestVisit struct {
		CreatedAt time.Time `bun:"created_at"`
	}
	err = s.db.NewRaw(`
		SELECT MIN(v.created_at) as created_at
		FROM active.visits v
		INNER JOIN users.privacy_consents pc ON pc.student_id = v.student_id
		WHERE v.exit_time IS NOT NULL
			AND v.created_at < NOW() - (pc.data_retention_days || ' days')::INTERVAL
	`).Scan(ctx, &oldestVisit)
	
	if err == nil && !oldestVisit.CreatedAt.IsZero() {
		preview.OldestVisit = &oldestVisit.CreatedAt
	}

	return preview, nil
}

// Helper methods

type studentWithConsent struct {
	StudentID         int64
	DataRetentionDays int
}

func (s *cleanupService) getStudentsWithRetentionSettings(ctx context.Context) ([]studentWithConsent, error) {
	var students []studentWithConsent
	
	err := s.db.NewRaw(`
		SELECT DISTINCT 
			pc.student_id,
			pc.data_retention_days
		FROM users.privacy_consents pc
		WHERE pc.accepted = true
		ORDER BY pc.student_id
	`).Scan(ctx, &students)
	
	if err != nil {
		return nil, err
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