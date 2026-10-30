package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
	"github.com/uptrace/bun"
)

// payrollDispatchRow mirrors active.payroll_dispatches (#4805).
type payrollDispatchRow struct {
	bun.BaseModel     `bun:"table:active.payroll_dispatches,alias:payroll_dispatch"`
	ID                int64      `bun:"id,pk,autoincrement"`
	TenantID          int64      `bun:"tenant_id,notnull"`
	Year              int        `bun:"year,notnull"`
	Month             int        `bun:"month,notnull"`
	Sequence          int        `bun:"sequence,notnull"`
	Correction        int        `bun:"correction,notnull"`
	Channel           string     `bun:"channel,notnull"`
	ExportFormat      string     `bun:"export_format,notnull"`
	RecipientEmail    string     `bun:"recipient_email,notnull"`
	ActorAccountID    int64      `bun:"actor_account_id,notnull"`
	ActorName         string     `bun:"actor_name,notnull"`
	State             string     `bun:"state,notnull"`
	Attempts          int        `bun:"attempts,notnull"`
	NextAttemptAt     time.Time  `bun:"next_attempt_at,notnull"`
	LeaseToken        *string    `bun:"lease_token"`
	LeaseExpiresAt    *time.Time `bun:"lease_expires_at"`
	Reason            string     `bun:"reason,notnull"`
	Filename          string     `bun:"filename,notnull"`
	ContentType       string     `bun:"content_type,notnull"`
	ByteSize          int64      `bun:"byte_size,notnull"`
	FileData          []byte     `bun:"file_data"`
	DownloadTokenHash *string    `bun:"download_token_hash"`
	DownloadExpiresAt *time.Time `bun:"download_expires_at"`
	DownloadedAt      *time.Time `bun:"downloaded_at"`
	MailOutboxID      *int64     `bun:"mail_outbox_id"`
	FinishedAt        *time.Time `bun:"finished_at"`
	CreatedAt         time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt         time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

func (r payrollDispatchRow) toDomain() domain.PayrollDispatch {
	value := domain.PayrollDispatch{
		ID: r.ID, Year: r.Year, Month: r.Month, Sequence: r.Sequence, Correction: r.Correction, Channel: r.Channel, Format: r.ExportFormat,
		Recipient: r.RecipientEmail, ActorAccountID: r.ActorAccountID, ActorName: r.ActorName, State: r.State,
		Attempts: r.Attempts, NextAttemptAt: r.NextAttemptAt, Reason: r.Reason, Filename: r.Filename,
		ContentType: r.ContentType, ByteSize: r.ByteSize, FileData: r.FileData,
		DownloadExpiresAt: r.DownloadExpiresAt, DownloadedAt: r.DownloadedAt, MailOutboxID: r.MailOutboxID,
		FinishedAt: r.FinishedAt, CreatedAt: r.CreatedAt,
	}
	if r.LeaseToken != nil {
		value.LeaseToken = *r.LeaseToken
	}
	if r.DownloadTokenHash != nil {
		value.DownloadTokenHash = *r.DownloadTokenHash
	}
	return value
}

func nullableText(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func (s *Store) payrollDatabase(ctx context.Context) (bun.IDB, int64, error) {
	db, scope, err := s.database(ctx)
	if err == nil && scope.TenantID <= 0 {
		err = errors.New("payroll dispatches: tenant is required")
	}
	return db, scope.TenantID, err
}

// NextPayrollDispatchSequence serializes the month's dispatches with a
// transaction lock, so two closes of the same month cannot both take the
// same number.
func (s *Store) NextPayrollDispatchSequence(ctx context.Context, year, month int) (int, int, error) {
	db, tenantID, err := s.payrollDatabase(ctx)
	if err != nil {
		return 0, 0, err
	}
	if _, err := db.NewRaw(`SELECT pg_advisory_xact_lock(hashtextextended(?, 0))`,
		fmt.Sprintf("payroll_dispatch:%d:%04d-%02d", tenantID, year, month)).Exec(ctx); err != nil {
		return 0, 0, fmt.Errorf("lock payroll dispatches of the month: %w", err)
	}
	var next struct {
		Sequence   int `bun:"sequence"`
		Correction int `bun:"correction"`
	}
	err = db.NewRaw(`SELECT COALESCE(max(sequence) + 1, 0) AS sequence,
		count(*) FILTER (WHERE state = ?) AS correction
		FROM active.payroll_dispatches WHERE tenant_id = ? AND year = ? AND month = ?`,
		workforce.PayrollDispatchSent,
		tenantID, year, month).Scan(ctx, &next)
	return next.Sequence, next.Correction, err
}

func (s *Store) InsertPayrollDispatch(ctx context.Context, value domain.PayrollDispatch) error {
	db, tenantID, err := s.payrollDatabase(ctx)
	if err != nil {
		return err
	}
	row := payrollDispatchRow{
		TenantID: tenantID, Year: value.Year, Month: value.Month, Sequence: value.Sequence, Correction: value.Correction,
		Channel: value.Channel, ExportFormat: value.Format, RecipientEmail: value.Recipient,
		ActorAccountID: value.ActorAccountID, ActorName: value.ActorName, State: value.State,
		NextAttemptAt: value.NextAttemptAt, Reason: value.Reason, FinishedAt: value.FinishedAt,
	}
	if _, err := db.NewInsert().Model(&row).ModelTableExpr("active.payroll_dispatches").
		ExcludeColumn("id", "created_at", "updated_at").Exec(ctx); err != nil {
		return fmt.Errorf("insert payroll dispatch: %w", err)
	}
	return nil
}

func (s *Store) LatestPayrollDispatch(ctx context.Context) (*domain.PayrollDispatch, error) {
	db, tenantID, err := s.payrollDatabase(ctx)
	if err != nil {
		return nil, err
	}
	row := new(payrollDispatchRow)
	err = db.NewSelect().Model(row).ModelTableExpr(`active.payroll_dispatches AS "payroll_dispatch"`).
		ExcludeColumn("file_data").
		Where(`"payroll_dispatch".tenant_id = ?`, tenantID).
		OrderExpr(`"payroll_dispatch".created_at DESC, "payroll_dispatch".id DESC`).Limit(1).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read latest payroll dispatch: %w", err)
	}
	value := row.toDomain()
	return &value, nil
}

// ClaimPayrollDispatches leases due rows with a fresh token. A row whose
// lease expired is due again: the pass that held it died.
func (s *Store) ClaimPayrollDispatches(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]domain.PayrollDispatch, error) {
	db, tenantID, err := s.payrollDatabase(ctx)
	if err != nil {
		return nil, err
	}
	var rows []payrollDispatchRow
	err = db.NewRaw(`WITH due AS (
		SELECT id FROM active.payroll_dispatches
		WHERE tenant_id = ? AND state IN ('pending', 'mailing') AND next_attempt_at <= ?
		AND (lease_expires_at IS NULL OR lease_expires_at <= ?)
		ORDER BY next_attempt_at, id LIMIT ? FOR UPDATE SKIP LOCKED
	) UPDATE active.payroll_dispatches AS d
	SET lease_token = gen_random_uuid()::text, lease_expires_at = ?, updated_at = ?
	FROM due WHERE d.tenant_id = ? AND d.id = due.id
	RETURNING d.id, d.tenant_id, d.year, d.month, d.sequence, d.correction, d.channel, d.export_format, d.recipient_email,
		d.actor_account_id, d.actor_name, d.state, d.attempts, d.next_attempt_at, d.lease_token, d.lease_expires_at,
		d.reason, d.filename, d.content_type, d.byte_size, d.download_token_hash, d.download_expires_at,
		d.downloaded_at, d.mail_outbox_id, d.finished_at, d.created_at, d.updated_at`,
		tenantID, now, now, limit, now.Add(lease), now, tenantID).Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("claim payroll dispatches: %w", err)
	}
	result := make([]domain.PayrollDispatch, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.toDomain())
	}
	return result, nil
}

// FinishPayrollDispatch writes everything a pass may change and releases its
// lease. A mailing row keeps its file; any other row whose file is set keeps
// it only when the pass handed it in.
func (s *Store) FinishPayrollDispatch(ctx context.Context, value domain.PayrollDispatch, now time.Time) (bool, error) {
	db, tenantID, err := s.payrollDatabase(ctx)
	if err != nil {
		return false, err
	}
	keepFile := value.State == workforce.PayrollDispatchMailing || value.State == workforce.PayrollDispatchSent
	result, err := db.NewRaw(`UPDATE active.payroll_dispatches SET
		state = ?, attempts = ?, next_attempt_at = ?, reason = ?, filename = ?, content_type = ?, byte_size = ?,
		file_data = CASE WHEN ?::bytea IS NOT NULL THEN ?::bytea WHEN ? THEN file_data ELSE NULL END,
		download_token_hash = COALESCE(?, download_token_hash), download_expires_at = COALESCE(?, download_expires_at),
		mail_outbox_id = COALESCE(?, mail_outbox_id), finished_at = ?,
		lease_token = NULL, lease_expires_at = NULL, updated_at = ?
		WHERE tenant_id = ? AND id = ? AND lease_token = ? AND lease_expires_at > ?`,
		value.State, value.Attempts, value.NextAttemptAt, value.Reason, value.Filename, value.ContentType, value.ByteSize,
		value.FileData, value.FileData, keepFile,
		nullableText(value.DownloadTokenHash), value.DownloadExpiresAt, value.MailOutboxID, value.FinishedAt,
		now, tenantID, value.ID, value.LeaseToken, now).Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("finish payroll dispatch: %w", err)
	}
	affected, err := result.RowsAffected()
	return affected == 1, err
}

// OpenPayrollDownload spends a live link in one statement: only the first
// caller gets the file, every later one finds downloaded_at set.
func (s *Store) OpenPayrollDownload(ctx context.Context, tokenHash string, now time.Time) (*domain.PayrollDispatch, error) {
	db, tenantID, err := s.payrollDatabase(ctx)
	if err != nil {
		return nil, err
	}
	var rows []payrollDispatchRow
	err = db.NewRaw(`WITH spent AS (
		SELECT id, file_data FROM active.payroll_dispatches
		WHERE tenant_id = ? AND download_token_hash = ? AND downloaded_at IS NULL
		AND download_expires_at > ? AND file_data IS NOT NULL
		FOR UPDATE
	) UPDATE active.payroll_dispatches AS d SET downloaded_at = ?, file_data = NULL, updated_at = ?
	FROM spent WHERE d.tenant_id = ? AND d.id = spent.id
	RETURNING d.id, d.year, d.month, d.sequence, d.correction, d.filename, d.content_type, d.byte_size, spent.file_data AS file_data`,
		tenantID, tokenHash, now, now, now, tenantID).Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("open payroll download: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	value := rows[0].toDomain()
	return &value, nil
}

// PurgePayrollDispatches keeps payroll data only as long as needed: the file
// of an unopened link goes when the link expires, the row 400 days after the
// dispatch finished.
func (s *Store) PurgePayrollDispatches(ctx context.Context, now, finishedBefore time.Time) error {
	db, tenantID, err := s.payrollDatabase(ctx)
	if err != nil {
		return err
	}
	if _, err := db.NewRaw(`UPDATE active.payroll_dispatches SET file_data = NULL, updated_at = ?
		WHERE tenant_id = ? AND file_data IS NOT NULL AND download_expires_at <= ?`, now, tenantID, now).Exec(ctx); err != nil {
		return fmt.Errorf("clear expired payroll files: %w", err)
	}
	if _, err := db.NewRaw(`DELETE FROM active.payroll_dispatches
		WHERE tenant_id = ? AND finished_at IS NOT NULL AND finished_at < ?`, tenantID, finishedBefore).Exec(ctx); err != nil {
		return fmt.Errorf("delete old payroll dispatches: %w", err)
	}
	return nil
}
