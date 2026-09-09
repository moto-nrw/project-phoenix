package postgres

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

func (s *Store) EnqueueDocumentCleanup(ctx context.Context, staffID int64, now time.Time) error {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return err
	}
	_, err = db.NewRaw(`INSERT INTO users.staff_offboarding_cleanup (tenant_id, staff_id, next_retry_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?) ON CONFLICT (tenant_id, staff_id) DO NOTHING`, tenantID, staffID, now, now, now).Exec(ctx)
	return err
}

func (s *Store) ClaimDocumentCleanup(ctx context.Context, limit int, now time.Time) ([]domain.DocumentCleanupClaim, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	rows := []struct {
		StaffID    int64
		LeaseToken string
		Attempts   int
	}{}
	err = db.NewRaw(`WITH candidates AS (
		SELECT staff_id FROM users.staff_offboarding_cleanup
		WHERE tenant_id = ? AND completed_at IS NULL AND next_retry_at <= ?
		AND (lease_expires_at IS NULL OR lease_expires_at <= ?)
		ORDER BY staff_id LIMIT ? FOR UPDATE SKIP LOCKED
	) UPDATE users.staff_offboarding_cleanup AS job
	SET lease_token = gen_random_uuid()::text, lease_expires_at = ?, attempts = attempts + 1, updated_at = ?
	FROM candidates WHERE job.tenant_id = ? AND job.staff_id = candidates.staff_id
	RETURNING job.staff_id, job.lease_token, job.attempts`, tenantID, now, now, limit,
		now.Add(2*time.Minute), now, tenantID).Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}
	result := make([]domain.DocumentCleanupClaim, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.DocumentCleanupClaim{StaffID: row.StaffID, Token: row.LeaseToken, Attempts: row.Attempts})
	}
	return result, nil
}

func (s *Store) FinishDocumentCleanup(ctx context.Context, claim domain.DocumentCleanupClaim, success bool, now time.Time) (bool, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return false, err
	}
	var completedAt any
	if success {
		completedAt = now
	}
	result, err := db.NewRaw(`UPDATE users.staff_offboarding_cleanup
		SET completed_at = ?, next_retry_at = ?, lease_token = NULL, lease_expires_at = NULL, updated_at = ?
		WHERE tenant_id = ? AND staff_id = ? AND lease_token = ? AND lease_expires_at > ? AND completed_at IS NULL`,
		completedAt, now.Add(time.Minute), now, tenantID, claim.StaffID, claim.Token, now).Exec(ctx)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	return rows == 1, err
}

func (s *Store) DocumentCleanupBacklog(ctx context.Context, now time.Time) (domain.DocumentCleanupBacklog, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.DocumentCleanupBacklog{}, err
	}
	var result domain.DocumentCleanupBacklog
	err = db.NewRaw(`SELECT count(*) AS pending, COALESCE(EXTRACT(EPOCH FROM (?::timestamptz - min(created_at))), 0) AS oldest_age_seconds
		FROM users.staff_offboarding_cleanup WHERE tenant_id = ? AND completed_at IS NULL`, now, tenantID).Scan(ctx, &result)
	return result, err
}
