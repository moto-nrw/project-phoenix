package datareturn

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/uptrace/bun"
)

// grantRow is one row of platform.data_return_grants.
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
	RecipientName  string     `bun:"recipient_name"`
	TokenLookup    string     `bun:"token_lookup"`
	WrappedKey     []byte     `bun:"wrapped_key"`
	IssuedBy       int64      `bun:"issued_by"`
	IssuedAt       time.Time  `bun:"issued_at"`
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

func (g grantRow) summary() organizationtenancy.DataReturnGrantSummary {
	return organizationtenancy.DataReturnGrantSummary{GrantID: g.ID, Package: g.Package, SubjectID: g.SubjectID, ArchiveName: g.ArchiveName,
		Recipient: g.Recipient, RecipientName: g.RecipientName, IssuedAt: g.IssuedAt, ExpiresAt: g.ExpiresAt, KeepUntil: g.KeepUntil,
		RevokedAt: g.RevokedAt, PurgedAt: g.PurgedAt, Downloads: g.Downloads, MaxDownloads: g.MaxDownloads}
}

// store is the persistence of the grants. The HTTP side runs as the
// application role, which may only read grants, count a download and write
// download rows; the CLI uses the maintenance connection.
type store struct{ db bun.IDB }

func (s store) byLookup(ctx context.Context, lookup string) (grantRow, error) {
	var row grantRow
	err := s.db.NewSelect().Model(&row).Where("token_lookup = ?", lookup).Limit(1).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return row, organizationtenancy.ErrDataReturnDenied
	}
	return row, err
}

func (s store) byArchive(ctx context.Context, archive string) (grantRow, error) {
	var row grantRow
	err := s.db.NewSelect().Model(&row).Where("archive_name = ?", archive).Where("purged_at IS NULL").OrderExpr("id").Limit(1).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return row, errors.New("data return: no released archive with this name")
	}
	return row, err
}

func (s store) insert(ctx context.Context, row *grantRow) error {
	_, err := s.db.NewInsert().Model(row).ExcludeColumn("issued_at", "revoked_at", "purged_at", "downloads").Returning("id, issued_at").Exec(ctx)
	return err
}

func (s store) remove(ctx context.Context, id int64) error {
	_, err := s.db.NewDelete().Model((*grantRow)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}

// countDownload spends one download of a usable grant in one statement and
// answers the download row; no row means the grant is no longer usable.
func (s store) countDownload(ctx context.Context, id int64, now time.Time) (int64, int, error) {
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

func (s store) finishDownload(ctx context.Context, id int64, now time.Time, completed bool, bytes int64) error {
	outcome := "failed"
	if completed {
		outcome = "completed"
	}
	_, err := s.db.NewRaw(`UPDATE platform.data_return_downloads SET finished_at = ?, outcome = ?, bytes = ? WHERE id = ?`,
		now, outcome, bytes, id).Exec(ctx)
	return err
}

// revoke ends one grant or every grant of an archive and drops their keys.
func (s store) revoke(ctx context.Context, grantID int64, archive string, now time.Time) ([]grantRow, error) {
	var rows []grantRow
	query := s.db.NewUpdate().Model(&rows).Set("revoked_at = ?", now).Set("wrapped_key = NULL").
		Where("revoked_at IS NULL").Where("purged_at IS NULL")
	if grantID > 0 {
		query = query.Where("id = ?", grantID)
	} else {
		query = query.Where("archive_name = ?", archive)
	}
	_, err := query.Returning("id, package, subject_id, archive_name").Exec(ctx)
	return rows, err
}

// purgeable answers the archives whose retrieval period ended or whose
// grants are all revoked.
func (s store) purgeable(ctx context.Context, now time.Time) ([]string, error) {
	var names []string
	err := s.db.NewRaw(`SELECT archive_name FROM platform.data_return_grants WHERE purged_at IS NULL
		GROUP BY archive_name HAVING max(keep_until) <= ? OR bool_and(revoked_at IS NOT NULL) ORDER BY archive_name`, now).Scan(ctx, &names)
	return names, err
}

func (s store) markPurged(ctx context.Context, archive string, now time.Time) error {
	_, err := s.db.NewRaw(`UPDATE platform.data_return_grants SET purged_at = ?, wrapped_key = NULL
		WHERE archive_name = ? AND purged_at IS NULL`, now, archive).Exec(ctx)
	return err
}

// dropEvidence deletes grants (and their downloads) whose archive is gone
// and whose retrieval period ended the given years ago.
func (s store) dropEvidence(ctx context.Context, before time.Time) (int, error) {
	result, err := s.db.NewRaw(`DELETE FROM platform.data_return_grants WHERE purged_at IS NOT NULL AND keep_until < ?`, before).Exec(ctx)
	if err != nil {
		return 0, err
	}
	n, err := result.RowsAffected()
	return int(n), err
}

func (s store) list(ctx context.Context) ([]grantRow, error) {
	var rows []grantRow
	err := s.db.NewSelect().Model(&rows).OrderExpr("id").Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("data return: list grants: %w", err)
	}
	return rows, nil
}
