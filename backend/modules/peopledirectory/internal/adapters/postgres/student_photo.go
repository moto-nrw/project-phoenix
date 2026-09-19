package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

// studentPhotoFeatureLockClass is the pg_advisory_xact_lock class id that
// serializes photo writes against a feature-disable purge. It must stay equal
// to the value the retained repository used, or the two would take different
// locks during the cutover.
const studentPhotoFeatureLockClass int32 = 0x70686F74

func (s *StudentStore) FindPhoto(ctx context.Context, studentID int64) (domain.StudentPhoto, bool, domain.OperationStats, error) {
	return s.readPhoto(ctx, studentID, "")
}

func (s *StudentStore) LockPhoto(ctx context.Context, studentID int64) (domain.StudentPhoto, bool, domain.OperationStats, error) {
	return s.readPhoto(ctx, studentID, "UPDATE")
}

func (s *StudentStore) readPhoto(ctx context.Context, studentID int64, lock string) (domain.StudentPhoto, bool, domain.OperationStats, error) {
	record, found, stats, err := s.FindRecord(ctx, studentID, lock)
	return domain.StudentPhoto{
		StudentID: record.ID, Status: record.Status, PhotoPath: record.PhotoPath,
		PhotoConsentGivenAt: record.PhotoConsentGivenAt, PhotoConsentGivenBy: record.PhotoConsentGivenBy,
	}, found, stats, err
}

func (s *StudentStore) LockPhotoFeature(ctx context.Context) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	if tenantID <= 0 {
		return domain.OperationStats{}, errors.New("people directory postgres: tenant is required to lock the photo feature")
	}
	if tenantID > 0x7fffffff {
		return domain.OperationStats{}, fmt.Errorf(
			"people directory postgres: tenant %d exceeds the advisory-lock object id range", tenantID)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	_, err = db.NewRaw("SELECT pg_advisory_xact_lock(?, ?)", studentPhotoFeatureLockClass, int32(tenantID)).Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("people directory postgres: lock photo feature: %w", err)
	}
	return stats, nil
}

func (s *StudentStore) SavePhoto(ctx context.Context, photo domain.StudentPhoto) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	if tenantID <= 0 {
		return domain.OperationStats{}, errors.New("people directory postgres: tenant is required to write a student photo")
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := db.NewUpdate().TableExpr(`users.student_profiles AS "student"`).
		Set("photo_path = ?", photo.PhotoPath).
		Set("photo_consent_given_at = ?", photo.PhotoConsentGivenAt).
		Set("photo_consent_given_by = ?", photo.PhotoConsentGivenBy).
		Set("updated_at = NOW()").
		Where(`"student".id = ?`, photo.StudentID).
		Where(`"student".tenant_id = ?`, tenantID).
		Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("people directory postgres: save student photo: %w", err)
	}
	affected, err := result.RowsAffected()
	if err == nil {
		stats.Rows = affected
	}
	if affected == 0 {
		return stats, domain.ErrStudentNotFound
	}
	return stats, nil
}

// PurgePhotos detaches every stored photo of the tenant. The rows are locked
// inside the statement so a concurrent photo write waits for this purge rather
// than reattaching a file it is about to lose.
func (s *StudentStore) PurgePhotos(ctx context.Context) ([]string, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	if tenantID <= 0 {
		return nil, domain.OperationStats{}, errors.New("people directory postgres: tenant is required to purge student photos")
	}
	const purgePhotos = `
		WITH locked AS (
			SELECT id, photo_path
			FROM users.student_profiles
			WHERE photo_path IS NOT NULL AND tenant_id = ?
			FOR UPDATE
		)
		UPDATE users.student_profiles AS student
		SET photo_path = NULL, updated_at = NOW()
		FROM locked
		WHERE student.id = locked.id
		RETURNING locked.photo_path
	`
	var rows []struct {
		PhotoPath string `bun:"photo_path"`
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewRaw(purgePhotos, tenantID).Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("people directory postgres: purge student photos: %w", err)
	}
	stats.Rows = int64(len(rows))
	urls := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.PhotoPath != "" {
			urls = append(urls, row.PhotoPath)
		}
	}
	return urls, stats, nil
}
