package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/uptrace/bun"
)

// requestUploadRow is a row of enrollment.request_uploads (#4783).
type requestUploadRow struct {
	bun.BaseModel   `bun:"table:enrollment.request_uploads,alias:upload"`
	ID              int64      `bun:"id,pk,autoincrement"`
	TenantID        int64      `bun:"tenant_id,notnull"`
	PhaseID         int64      `bun:"phase_id,notnull"`
	FieldKey        string     `bun:"field_key,notnull"`
	RequestID       *int64     `bun:"request_id"`
	TokenHash       *string    `bun:"token_hash"`
	FilenameDisplay string     `bun:"filename_display,notnull"`
	FilenameStored  string     `bun:"filename_stored,notnull"`
	SizeBytes       int64      `bun:"size_bytes,notnull"`
	ContentType     string     `bun:"content_type,notnull"`
	CreatedAt       time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	ClaimedAt       *time.Time `bun:"claimed_at"`
}

func (r requestUploadRow) value() *enrollment.RequestUpload {
	upload := &enrollment.RequestUpload{
		ID: r.ID, TenantID: r.TenantID, PhaseID: r.PhaseID, RequestID: r.RequestID, FieldKey: r.FieldKey,
		Filename: r.FilenameDisplay, StoredName: r.FilenameStored, SizeBytes: r.SizeBytes,
		ContentType: r.ContentType, CreatedAt: r.CreatedAt, ClaimedAt: r.ClaimedAt,
	}
	if r.TokenHash != nil {
		upload.TokenHash = *r.TokenHash
	}
	return upload
}

func requestUploadValues(rows []requestUploadRow) []*enrollment.RequestUpload {
	out := make([]*enrollment.RequestUpload, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.value())
	}
	return out
}

// InsertRequestUpload stores an unclaimed upload for the tenant in context.
func (r *Store) InsertRequestUpload(ctx context.Context, upload *enrollment.RequestUpload) error {
	db, tenantID, err := r.tenantDB(ctx)
	if err != nil {
		return err
	}
	tokenHash := upload.TokenHash
	row := requestUploadRow{
		TenantID: tenantID, PhaseID: upload.PhaseID, FieldKey: upload.FieldKey, TokenHash: &tokenHash,
		FilenameDisplay: upload.Filename, FilenameStored: upload.StoredName, SizeBytes: upload.SizeBytes, ContentType: upload.ContentType,
	}
	if _, err := db.NewInsert().Model(&row).Returning("id, created_at").Exec(ctx); err != nil {
		return fmt.Errorf("insert enrollment request upload: %w", err)
	}
	upload.ID, upload.TenantID, upload.CreatedAt = row.ID, tenantID, row.CreatedAt
	return nil
}

// DeleteRequestUpload deletes one upload; the trigger queues its object.
func (r *Store) DeleteRequestUpload(ctx context.Context, uploadID int64) error {
	db, tenantID, err := r.tenantDB(ctx)
	if err != nil {
		return err
	}
	if _, err := db.NewDelete().TableExpr("enrollment.request_uploads").
		Where("tenant_id = ? AND id = ?", tenantID, uploadID).Exec(ctx); err != nil {
		return fmt.Errorf("delete enrollment request upload: %w", err)
	}
	return nil
}

// UnclaimedRequestUploads locks the matching unclaimed uploads.
func (r *Store) UnclaimedRequestUploads(ctx context.Context, phaseID int64, fieldKey string, tokenHashes []string, createdAfter time.Time) ([]*enrollment.RequestUpload, error) {
	if len(tokenHashes) == 0 {
		return nil, nil
	}
	db, tenantID, err := r.tenantDB(ctx)
	if err != nil {
		return nil, err
	}
	var rows []requestUploadRow
	err = db.NewSelect().Model(&rows).
		Where("upload.tenant_id = ?", tenantID).
		Where("upload.phase_id = ?", phaseID).
		Where("upload.field_key = ?", fieldKey).
		Where("upload.request_id IS NULL").
		Where("upload.token_hash IN (?)", bun.List(tokenHashes)).
		Where("upload.created_at > ?", createdAfter).
		OrderExpr("upload.id ASC").
		For("UPDATE").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("load unclaimed enrollment uploads: %w", err)
	}
	return requestUploadValues(rows), nil
}

// ClaimRequestUploads binds unclaimed uploads to the request and clears
// their token hash.
func (r *Store) ClaimRequestUploads(ctx context.Context, uploadIDs []int64, requestID int64, claimedAt time.Time) error {
	if len(uploadIDs) == 0 {
		return nil
	}
	db, tenantID, err := r.tenantDB(ctx)
	if err != nil {
		return err
	}
	res, err := db.NewUpdate().TableExpr("enrollment.request_uploads").
		Set("request_id = ?", requestID).
		Set("claimed_at = ?", claimedAt).
		Set("token_hash = NULL").
		Where("tenant_id = ?", tenantID).
		Where("id IN (?)", bun.List(uploadIDs)).
		Where("request_id IS NULL").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("claim enrollment request uploads: %w", err)
	}
	if affected, err := res.RowsAffected(); err != nil || int(affected) != len(uploadIDs) {
		return fmt.Errorf("claim enrollment request uploads: claimed %d of %d", affected, len(uploadIDs))
	}
	return nil
}

// RequestUploads lists the uploads of a request, oldest first.
func (r *Store) RequestUploads(ctx context.Context, requestID int64) ([]*enrollment.RequestUpload, error) {
	db, tenantID, err := r.tenantDB(ctx)
	if err != nil {
		return nil, err
	}
	var rows []requestUploadRow
	if err := db.NewSelect().Model(&rows).
		Where("upload.tenant_id = ? AND upload.request_id = ?", tenantID, requestID).
		OrderExpr("upload.id ASC").Scan(ctx); err != nil {
		return nil, fmt.Errorf("list enrollment request uploads: %w", err)
	}
	return requestUploadValues(rows), nil
}

// RequestUpload loads one upload of a request; a missing or foreign one
// reports sql.ErrNoRows.
func (r *Store) RequestUpload(ctx context.Context, requestID, uploadID int64) (*enrollment.RequestUpload, error) {
	db, tenantID, err := r.tenantDB(ctx)
	if err != nil {
		return nil, err
	}
	var row requestUploadRow
	if err := db.NewSelect().Model(&row).
		Where("upload.tenant_id = ? AND upload.request_id = ? AND upload.id = ?", tenantID, requestID, uploadID).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("load enrollment request upload: %w", err)
	}
	return row.value(), nil
}

// DeleteRequestUploads deletes the listed uploads of a request.
func (r *Store) DeleteRequestUploads(ctx context.Context, requestID int64, uploadIDs []int64) error {
	if len(uploadIDs) == 0 {
		return nil
	}
	db, tenantID, err := r.tenantDB(ctx)
	if err != nil {
		return err
	}
	if _, err := db.NewDelete().TableExpr("enrollment.request_uploads").
		Where("tenant_id = ? AND request_id = ?", tenantID, requestID).
		Where("id IN (?)", bun.List(uploadIDs)).Exec(ctx); err != nil {
		return fmt.Errorf("delete enrollment request uploads: %w", err)
	}
	return nil
}

// DeleteExpiredRequestUploads deletes unclaimed uploads past their lifetime
// and the uploads of requests decided before decidedBefore: every child is
// approved, rejected or withdrawn, and the last of those decisions is older
// than the cutoff.
func (r *Store) DeleteExpiredRequestUploads(ctx context.Context, unclaimedBefore, decidedBefore time.Time) (int, error) {
	db, tenantID, err := r.tenantDB(ctx)
	if err != nil {
		return 0, err
	}
	res, err := db.NewDelete().TableExpr("enrollment.request_uploads AS upload").
		Where("upload.tenant_id = ?", tenantID).
		WhereGroup(" AND ", func(q *bun.DeleteQuery) *bun.DeleteQuery {
			return q.Where("upload.request_id IS NULL AND upload.created_at < ?", unclaimedBefore).
				WhereOr(`upload.request_id IN (
					SELECT rc.request_id FROM enrollment.request_children AS rc
					WHERE rc.tenant_id = ?
					GROUP BY rc.request_id
					HAVING BOOL_AND(rc.status IN (?, ?, ?))
					   AND MAX(COALESCE(rc.reviewed_at, rc.updated_at)) < ?)`,
					tenantID, enrollment.ChildStatusApproved, enrollment.ChildStatusRejected, enrollment.ChildStatusWithdrawn, decidedBefore)
		}).
		Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("delete expired enrollment uploads: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("delete expired enrollment uploads: rows affected: %w", err)
	}
	return int(affected), nil
}

type requestUploadCleanupRow struct {
	bun.BaseModel  `bun:"table:enrollment.request_upload_cleanup,alias:intent"`
	ID             int64  `bun:"id,pk,autoincrement"`
	TenantID       int64  `bun:"tenant_id,notnull"`
	FilenameStored string `bun:"filename_stored,notnull"`
}

// PendingUploadCleanups locks a batch of queued object removals. CALLER
// CONTRACT: remove the objects and complete the intents in the same
// transaction.
func (r *Store) PendingUploadCleanups(ctx context.Context, limit int) ([]enrollment.UploadCleanupIntent, error) {
	db, tenantID, err := r.tenantDB(ctx)
	if err != nil {
		return nil, err
	}
	var rows []requestUploadCleanupRow
	if err := db.NewSelect().Model(&rows).
		Where("intent.tenant_id = ? AND intent.cleaned_at IS NULL", tenantID).
		OrderExpr("intent.id ASC").Limit(int64(limit)).
		For("UPDATE SKIP LOCKED").Scan(ctx); err != nil {
		return nil, fmt.Errorf("list enrollment upload cleanups: %w", err)
	}
	out := make([]enrollment.UploadCleanupIntent, 0, len(rows))
	for _, row := range rows {
		out = append(out, enrollment.UploadCleanupIntent{ID: row.ID, TenantID: row.TenantID, StoredName: row.FilenameStored})
	}
	return out, nil
}

// CompleteUploadCleanup settles one intent.
func (r *Store) CompleteUploadCleanup(ctx context.Context, intentID int64, at time.Time) error {
	db, tenantID, err := r.tenantDB(ctx)
	if err != nil {
		return err
	}
	if _, err := db.NewUpdate().TableExpr("enrollment.request_upload_cleanup").
		Set("cleaned_at = ?", at).
		Where("tenant_id = ? AND id = ?", tenantID, intentID).Exec(ctx); err != nil {
		return fmt.Errorf("complete enrollment upload cleanup: %w", err)
	}
	return nil
}

// DeleteFinishedUploadCleanups deletes the settled intents of the tenant
// whose object went before the cutoff.
func (r *Store) DeleteFinishedUploadCleanups(ctx context.Context, cleanedBefore time.Time) (int, error) {
	db, tenantID, err := r.tenantDB(ctx)
	if err != nil {
		return 0, err
	}
	res, err := db.NewDelete().TableExpr("enrollment.request_upload_cleanup").
		Where("tenant_id = ? AND cleaned_at IS NOT NULL AND cleaned_at < ?", tenantID, cleanedBefore).Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("delete finished enrollment upload cleanups: %w", err)
	}
	affected, err := res.RowsAffected()
	return int(affected), err
}

func (r *Store) tenantDB(ctx context.Context) (bun.IDB, int64, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, 0, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, 0, err
	}
	return db, tenantID, nil
}
