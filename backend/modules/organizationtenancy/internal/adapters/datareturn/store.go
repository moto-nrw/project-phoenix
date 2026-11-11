package datareturn

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/uptrace/bun"
)

// grantRow is one row of platform.data_return_grants as the retrieval reads it.
type grantRow struct {
	bun.BaseModel  `bun:"table:platform.data_return_grants"`
	ID             int64      `bun:"id,pk,autoincrement"`
	Package        string     `bun:"package"`
	SubjectID      int64      `bun:"subject_id"`
	ArchiveName    string     `bun:"archive_name"`
	ArchiveBytes   int64      `bun:"archive_bytes"`
	ArchiveSHA256  string     `bun:"archive_sha256"`
	ManifestSHA256 string     `bun:"manifest_sha256"`
	FormatVersion  int        `bun:"format_version"`
	KeyCheck       string     `bun:"key_check"`
	Recipient      string     `bun:"recipient"`
	WrappedKey     []byte     `bun:"wrapped_key"`
	ExpiresAt      time.Time  `bun:"expires_at"`
	KeepUntil      time.Time  `bun:"keep_until"`
	RevokedAt      *time.Time `bun:"revoked_at"`
	PurgedAt       *time.Time `bun:"purged_at"`
	MaxDownloads   int        `bun:"max_downloads"`
	Downloads      int        `bun:"downloads"`
}

func (g grantRow) usable(now time.Time) bool {
	return g.RevokedAt == nil && g.PurgedAt == nil && len(g.WrappedKey) > 0 && now.Before(g.ExpiresAt) && g.Downloads < g.MaxDownloads
}

func (g grantRow) toPackage() organizationtenancy.DataReturnPackage {
	return organizationtenancy.DataReturnPackage{Package: g.Package, FormatVersion: g.FormatVersion, ManifestSHA256: g.ManifestSHA256,
		ArchiveSHA256: g.ArchiveSHA256, ArchiveBytes: g.ArchiveBytes, Recipient: g.Recipient, ExpiresAt: g.ExpiresAt.UTC(),
		DownloadsLeft: g.MaxDownloads - g.Downloads}
}

// retrievalStore is the HTTP side. It runs as the application role, which
// may only read grants, count a download and write download rows.
type retrievalStore struct{ db bun.IDB }

func (s retrievalStore) byLookup(ctx context.Context, lookup string) (grantRow, error) {
	var row grantRow
	err := s.db.NewSelect().Model(&row).Where("token_lookup = ?", lookup).Limit(1).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return row, organizationtenancy.ErrDataReturnDenied
	}
	return row, err
}

// stillOpen reports whether a grant is neither revoked nor purged; an
// in-flight download asks it between chunks.
func (s retrievalStore) stillOpen(ctx context.Context, id int64) (bool, error) {
	var open bool
	err := s.db.NewRaw(`SELECT revoked_at IS NULL AND purged_at IS NULL FROM platform.data_return_grants WHERE id = ?`, id).Scan(ctx, &open)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return open, err
}

// countDownload spends one download of a usable grant in one statement and
// answers the download row; no row means the grant is no longer usable.
func (s retrievalStore) countDownload(ctx context.Context, id int64, now time.Time) (int64, int, error) {
	var downloads int
	err := s.db.NewRaw(`UPDATE platform.data_return_grants SET downloads = downloads + 1
		WHERE id = ? AND downloads < max_downloads AND revoked_at IS NULL AND purged_at IS NULL
		AND wrapped_key IS NOT NULL AND expires_at > ? RETURNING downloads`, id, now).Scan(ctx, &downloads)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, organizationtenancy.ErrDataReturnDenied
	}
	if err != nil {
		return 0, 0, err
	}
	var download int64
	err = s.db.NewRaw(`INSERT INTO platform.data_return_downloads (grant_id, started_at) VALUES (?, ?) RETURNING id`, id, now).Scan(ctx, &download)
	return download, downloads, err
}

func (s retrievalStore) finishDownload(ctx context.Context, id int64, now time.Time, completed bool, bytes int64) error {
	outcome := "failed"
	if completed {
		outcome = "completed"
	}
	_, err := s.db.NewRaw(`UPDATE platform.data_return_downloads SET finished_at = ?, outcome = ?, bytes = ? WHERE id = ?`,
		now, outcome, bytes, id).Exec(ctx)
	return err
}

// releaseStore is the CLI side. Every call runs in its own session as the
// NOLOGIN group phoenix_data_return, which may only execute the functions of
// migration 1.15.910; they check the operator and write the operator audit
// in the same transaction. The session is discarded afterwards, so the role
// never returns to the pool.
type releaseStore struct{ db *bun.DB }

func (s releaseStore) call(ctx context.Context, fn func(bun.Conn) error) error {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return errors.New("data return: database session unavailable")
	}
	defer func() {
		_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		_ = conn.Close()
	}()
	if _, err := conn.ExecContext(ctx, `SET ROLE phoenix_data_return`); err != nil {
		return errors.New("data return: role phoenix_data_return unavailable to this login")
	}
	if err := fn(conn); err != nil {
		// SQL errors may carry values; the CLI shows the class only.
		return fmt.Errorf("data return: database step refused: %w", errDatabaseStep)
	}
	return nil
}

var errDatabaseStep = errors.New("operator inactive, archive not live or a constraint failed")

// issueRow is what data_return_issue stores.
type issueRow struct {
	Action, Package, ArchiveName, ArchiveSHA256, ManifestSHA256, KeyCheck, Recipient, RecipientName, TokenLookup string
	SubjectID, ArchiveBytes, OperatorID                                                                          int64
	FormatVersion, MaxDownloads                                                                                  int
	WrappedKey                                                                                                   []byte
	ExpiresAt, KeepUntil                                                                                         time.Time
}

func (s releaseStore) issue(ctx context.Context, r issueRow) (int64, error) {
	var id int64
	err := s.call(ctx, func(conn bun.Conn) error {
		return conn.NewRaw(`SELECT platform.data_return_issue(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			r.OperatorID, r.Action, r.Package, r.SubjectID, r.ArchiveName, r.ArchiveBytes, r.ArchiveSHA256, r.ManifestSHA256,
			r.FormatVersion, r.KeyCheck, r.Recipient, r.RecipientName, r.TokenLookup, r.WrappedKey, r.ExpiresAt, r.KeepUntil,
			r.MaxDownloads).Scan(ctx, &id)
	})
	return id, err
}

// archiveHead is what a further grant needs of a live archive.
type archiveHead struct {
	Package        string    `json:"package"`
	SubjectID      int64     `json:"subject_id"`
	ArchiveName    string    `json:"archive_name"`
	ArchiveBytes   int64     `json:"archive_bytes"`
	ArchiveSHA256  string    `json:"archive_sha256"`
	ManifestSHA256 string    `json:"manifest_sha256"`
	FormatVersion  int       `json:"format_version"`
	KeyCheck       string    `json:"key_check"`
	KeepUntil      time.Time `json:"keep_until"`
}

func (s releaseStore) archive(ctx context.Context, operatorID int64, name string) (archiveHead, bool, error) {
	var raw sql.NullString
	err := s.call(ctx, func(conn bun.Conn) error {
		return conn.NewRaw(`SELECT platform.data_return_archive(?, ?)::text`, operatorID, name).Scan(ctx, &raw)
	})
	var head archiveHead
	if err != nil || !raw.Valid {
		return head, false, err
	}
	return head, true, json.Unmarshal([]byte(raw.String), &head)
}

func (s releaseStore) requireOperator(ctx context.Context, operatorID int64) error {
	return s.call(ctx, func(conn bun.Conn) error {
		_, err := conn.NewRaw(`SELECT platform.data_return_list(?)`, operatorID).Exec(ctx)
		return err
	})
}

// revoke takes the selectors as the function does: exactly one of grant,
// archive and all is set, the others are NULL.
func (s releaseStore) revoke(ctx context.Context, operatorID int64, grant *int64, archive *string, all bool) (int, error) {
	var count int
	err := s.call(ctx, func(conn bun.Conn) error {
		return conn.NewRaw(`SELECT platform.data_return_revoke(?, ?, ?, ?)`, operatorID, grant, archive, all).Scan(ctx, &count)
	})
	return count, err
}

func (s releaseStore) names(ctx context.Context, function string, operatorID int64) ([]string, error) {
	var names []string
	err := s.call(ctx, func(conn bun.Conn) error {
		return conn.NewRaw(`SELECT * FROM platform.`+function+`(?)`, operatorID).Scan(ctx, &names)
	})
	return names, err
}

func (s releaseStore) purgeArchive(ctx context.Context, operatorID int64, name string) (bool, error) {
	var purged bool
	err := s.call(ctx, func(conn bun.Conn) error {
		return conn.NewRaw(`SELECT platform.data_return_purge_archive(?, ?)`, operatorID, name).Scan(ctx, &purged)
	})
	return purged, err
}

func (s releaseStore) dropEvidence(ctx context.Context, operatorID int64) (int, error) {
	var count int
	err := s.call(ctx, func(conn bun.Conn) error {
		return conn.NewRaw(`SELECT platform.data_return_drop_evidence(?)`, operatorID).Scan(ctx, &count)
	})
	return count, err
}

func (s releaseStore) list(ctx context.Context, operatorID int64) ([]organizationtenancy.DataReturnGrantSummary, error) {
	var raw string
	err := s.call(ctx, func(conn bun.Conn) error {
		return conn.NewRaw(`SELECT platform.data_return_list(?)::text`, operatorID).Scan(ctx, &raw)
	})
	if err != nil {
		return nil, err
	}
	var grants []organizationtenancy.DataReturnGrantSummary
	return grants, json.Unmarshal([]byte(raw), &grants)
}
