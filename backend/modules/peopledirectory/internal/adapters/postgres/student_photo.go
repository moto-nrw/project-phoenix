package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/peopledirectory/internal/domain"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/uptrace/bun"
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
	db, scope, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	if scope.TenantID <= 0 {
		return domain.OperationStats{}, errors.New("people directory postgres: tenant is required to lock the photo feature")
	}
	if scope.TenantID > 0x7fffffff {
		return domain.OperationStats{}, fmt.Errorf(
			"people directory postgres: tenant %d exceeds the advisory-lock object id range", scope.TenantID)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	_, err = db.NewRaw("SELECT pg_advisory_xact_lock(?, ?)", studentPhotoFeatureLockClass, int32(scope.TenantID)).Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("people directory postgres: lock photo feature: %w", err)
	}
	return stats, nil
}

func (s *StudentStore) SavePhoto(ctx context.Context, photo domain.StudentPhoto) (domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	if scope.TenantID <= 0 {
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
		Where(`"student".tenant_id = ?`, scope.TenantID).
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
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	if scope.TenantID <= 0 {
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
	err = db.NewRaw(purgePhotos, scope.TenantID).Scan(ctx, &rows)
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

// ListPhotosPastPurpose names, in ascending id order, the children of the
// tenant whose care is over on today (last care day before today, or
// graduated) and whose row still holds a photo, a photo consent or who
// recorded it (#6650). The candidates come from the database on every run,
// so a child a run deferred is found again by the next one.
func (s *StudentStore) ListPhotosPastPurpose(ctx context.Context, today calendar.Date) ([]int64, domain.OperationStats, error) {
	db, scope, err := s.reads.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	if scope.AcrossSchools || scope.TenantID <= 0 {
		return nil, domain.OperationStats{}, errors.New("people directory postgres: one tenant is required to list photos past their purpose")
	}
	var ids []int64
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = withStudentTenant(s.reads.studentRows(db, scope).
		ColumnExpr(`"student".id`).
		Where(`("student".status = ? OR "student".enrolled_until < ?)`, domain.StudentStatusAlumnus, today).
		Where(`("student".photo_path IS NOT NULL OR "student".photo_consent_given_at IS NOT NULL OR "student".photo_consent_given_by IS NOT NULL)`).
		OrderExpr(`"student".id ASC`), scope).
		Scan(ctx, &ids)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("people directory postgres: list photos past their purpose: %w", err)
	}
	stats.Rows = int64(len(ids))
	return ids, stats, nil
}

// TryLockPhotoRecords locks the projected rows in ID order, including their
// membership, so care resumption cannot race the eligibility check. SKIP LOCKED
// leaves held rows untouched without aborting the rest of the set.
func (s *StudentStore) TryLockPhotoRecords(ctx context.Context, ids []int64) (map[int64]domain.StudentRecord, map[int64]bool, domain.OperationStats, error) {
	db, scope, err := s.reads.database(ctx)
	if err != nil {
		return nil, nil, domain.OperationStats{}, err
	}
	if scope.AcrossSchools || scope.TenantID <= 0 {
		return nil, nil, domain.OperationStats{}, domain.ErrTenantRequired
	}
	presentQuery := withStudentTenant(s.reads.studentRows(db, scope).ColumnExpr(`"student".id`).Where(`"student".id IN (?)`, bun.List(ids)), scope)
	lockedQuery := withStudentTenant(s.reads.studentRows(db, scope).
		ColumnExpr(`"student".id, "student".status, "student".enrolled_until, "student".photo_path, "student".photo_consent_given_at, "student".photo_consent_given_by`).
		Where(`"student".id IN (?)`, bun.List(ids)).OrderExpr(`"student".id ASC`).For("UPDATE SKIP LOCKED"), scope)
	var rows []struct {
		ID                  int64          `bun:"id"`
		Locked              bool           `bun:"locked"`
		Status              string         `bun:"status"`
		EnrolledUntil       *calendar.Date `bun:"enrolled_until"`
		PhotoPath           *string        `bun:"photo_path"`
		PhotoConsentGivenAt *time.Time     `bun:"photo_consent_given_at"`
		PhotoConsentGivenBy *int64         `bun:"photo_consent_given_by"`
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewRaw(`WITH present AS MATERIALIZED (?), locked AS MATERIALIZED (?)
 SELECT present.id, locked.id IS NOT NULL AS locked, locked.status, locked.enrolled_until,
 locked.photo_path, locked.photo_consent_given_at, locked.photo_consent_given_by
 FROM present LEFT JOIN locked ON locked.id = present.id`, presentQuery, lockedQuery).Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, nil, stats, fmt.Errorf("people directory postgres: lock photos past purpose: %w", err)
	}
	records, present := make(map[int64]domain.StudentRecord, len(rows)), make(map[int64]bool, len(rows))
	for _, row := range rows {
		present[row.ID] = true
		if !row.Locked {
			continue
		}
		record := domain.StudentRecord{ID: row.ID, Status: row.Status, PhotoPath: row.PhotoPath, PhotoConsentGivenAt: row.PhotoConsentGivenAt, PhotoConsentGivenBy: row.PhotoConsentGivenBy}
		if row.EnrolledUntil != nil {
			record.EnrolledUntil = row.EnrolledUntil.String()
		}
		records[row.ID] = record
	}
	stats.Rows = int64(len(records))
	return records, present, stats, nil
}

func (s *StudentStore) ClearPhotos(ctx context.Context, ids []int64) (domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	if scope.TenantID <= 0 {
		return domain.OperationStats{}, domain.ErrTenantRequired
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := db.NewUpdate().TableExpr(`users.student_profiles AS "student"`).
		Set("photo_path = NULL").Set("photo_consent_given_at = NULL").Set("photo_consent_given_by = NULL").Set("updated_at = NOW()").
		Where(`"student".tenant_id = ?`, scope.TenantID).Where(`"student".id IN (?)`, bun.List(ids)).Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("people directory postgres: clear photos: %w", err)
	}
	stats.Rows, err = result.RowsAffected()
	if err == nil && stats.Rows != int64(len(ids)) {
		err = domain.ErrStudentNotFound
	}
	return stats, err
}
