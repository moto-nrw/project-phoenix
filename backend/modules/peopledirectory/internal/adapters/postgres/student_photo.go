package postgres

import (
	"context"
	"database/sql"
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

type studentPhotoRow struct {
	ID                  int64      `bun:"id"`
	Status              string     `bun:"status"`
	PhotoPath           *string    `bun:"photo_path"`
	PhotoConsentGivenAt *time.Time `bun:"photo_consent_given_at"`
	PhotoConsentGivenBy *int64     `bun:"photo_consent_given_by"`
}

func (r studentPhotoRow) toDomain() domain.StudentPhoto {
	return domain.StudentPhoto{
		StudentID: r.ID, Status: r.Status, PhotoPath: r.PhotoPath,
		PhotoConsentGivenAt: r.PhotoConsentGivenAt, PhotoConsentGivenBy: r.PhotoConsentGivenBy,
	}
}

func (s *StudentStore) FindPhoto(ctx context.Context, studentID int64) (domain.StudentPhoto, bool, domain.OperationStats, error) {
	return s.readPhoto(ctx, studentID, "")
}

func (s *StudentStore) LockPhoto(ctx context.Context, studentID int64) (domain.StudentPhoto, bool, domain.OperationStats, error) {
	return s.readPhoto(ctx, studentID, "UPDATE")
}

func (s *StudentStore) readPhoto(ctx context.Context, studentID int64, lock string) (domain.StudentPhoto, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.StudentPhoto{}, false, domain.OperationStats{}, err
	}
	var row studentPhotoRow
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	query := db.NewSelect().TableExpr(`users.students AS "student"`).
		ColumnExpr(`"student".id, "student".status, "student".photo_path,
			"student".photo_consent_given_at, "student".photo_consent_given_by`).
		Where(`"student".id = ?`, studentID)
	if tenantID > 0 {
		query = query.Where(`"student".tenant_id = ?`, tenantID)
	}
	if lock != "" {
		query = query.For(lock)
	}
	err = query.Scan(ctx, &row.ID, &row.Status, &row.PhotoPath, &row.PhotoConsentGivenAt, &row.PhotoConsentGivenBy)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.StudentPhoto{}, false, stats, nil
	}
	if err != nil {
		return domain.StudentPhoto{}, false, stats, fmt.Errorf("people directory postgres: read student photo: %w", err)
	}
	stats.Rows = 1
	return row.toDomain(), true, stats, nil
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
	result, err := db.NewUpdate().TableExpr(`users.students AS "student"`).
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
			FROM users.students
			WHERE photo_path IS NOT NULL AND tenant_id = ?
			FOR UPDATE
		)
		UPDATE users.students AS student
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
