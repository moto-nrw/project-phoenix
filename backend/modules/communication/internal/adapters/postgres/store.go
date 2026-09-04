package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication/internal/domain"
	"github.com/uptrace/bun"
)

const announcementTable = `platform.announcements AS "announcement"`

type Database func(context.Context) (bun.IDB, error)

type Store struct{ database Database }

type announcementRow struct {
	bun.BaseModel   `bun:"table:platform.announcements,alias:announcement"`
	ID              int64      `bun:"id,pk,autoincrement"`
	Title           string     `bun:"title,notnull"`
	Content         string     `bun:"content,notnull"`
	Type            string     `bun:"type,notnull"`
	Severity        string     `bun:"severity,notnull"`
	Version         *string    `bun:"version"`
	Active          bool       `bun:"active,notnull"`
	PublishedAt     *time.Time `bun:"published_at"`
	ExpiresAt       *time.Time `bun:"expires_at"`
	TargetRoles     []string   `bun:"target_roles,array"`
	TargetOrgIDs    []int64    `bun:"target_org_ids,array"`
	TargetTenantIDs []int64    `bun:"target_tenant_ids,array"`
	CreatedBy       int64      `bun:"created_by,notnull"`
	CreatedAt       time.Time  `bun:"created_at"`
	UpdatedAt       time.Time  `bun:"updated_at"`
}

type viewRow struct {
	bun.BaseModel  `bun:"table:platform.announcement_views,alias:view"`
	UserID         int64     `bun:"user_id,pk"`
	AnnouncementID int64     `bun:"announcement_id,pk"`
	SeenAt         time.Time `bun:"seen_at,notnull"`
	Dismissed      bool      `bun:"dismissed,notnull"`
}

func New(database Database) *Store {
	if database == nil {
		panic("communication postgres: database runtime is required")
	}
	return &Store{database: database}
}

func (s *Store) Create(ctx context.Context, value domain.Announcement) (domain.Announcement, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.Announcement{}, domain.OperationStats{}, err
	}
	row := fromDomain(value)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr("platform.announcements").Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.Announcement{}, stats, fmt.Errorf("communication postgres: create announcement: %w", err)
	}
	stats.Rows = 1
	return toDomain(row), stats, nil
}

func (s *Store) Get(ctx context.Context, id int64) (domain.Announcement, domain.OperationStats, error) {
	return s.get(ctx, id, false)
}

func (s *Store) GetForMutation(ctx context.Context, id int64) (domain.Announcement, domain.OperationStats, error) {
	return s.get(ctx, id, true)
}

func (s *Store) get(ctx context.Context, id int64, forUpdate bool) (domain.Announcement, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.Announcement{}, domain.OperationStats{}, err
	}
	var row announcementRow
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	query := db.NewSelect().Model(&row).ModelTableExpr(announcementTable).
		Where(`"announcement".id = ?`, id)
	if forUpdate {
		query = query.For("UPDATE")
	}
	err = query.Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Announcement{}, stats, domain.ErrAnnouncementNotFound
	}
	if err != nil {
		return domain.Announcement{}, stats, fmt.Errorf("communication postgres: get announcement: %w", err)
	}
	stats.Rows = 1
	return toDomain(row), stats, nil
}

func (s *Store) Update(ctx context.Context, value domain.Announcement) (domain.Announcement, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.Announcement{}, domain.OperationStats{}, err
	}
	row := fromDomain(value)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewUpdate().Model(&row).ModelTableExpr(announcementTable).
		WherePK().Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Announcement{}, stats, domain.ErrAnnouncementNotFound
	}
	if err != nil {
		return domain.Announcement{}, stats, fmt.Errorf("communication postgres: update announcement: %w", err)
	}
	stats.Rows = 1
	return toDomain(row), stats, nil
}

func (s *Store) Delete(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execute(ctx, db.NewDelete().Model((*announcementRow)(nil)).ModelTableExpr(announcementTable).
		Where(`"announcement".id = ?`, id), "delete announcement")
}

func (s *Store) List(ctx context.Context, includeInactive bool) ([]domain.Announcement, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []announcementRow
	query := db.NewSelect().Model(&rows).ModelTableExpr(announcementTable)
	if !includeInactive {
		query = query.Where(`"announcement".active = TRUE`)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.OrderExpr(`"announcement".created_at DESC`).Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("communication postgres: list announcements: %w", err)
	}
	stats.Rows = int64(len(rows))
	return toDomainList(rows), stats, nil
}

func (s *Store) Publish(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execute(ctx, db.NewUpdate().Model((*announcementRow)(nil)).ModelTableExpr(announcementTable).
		Set("published_at = CURRENT_TIMESTAMP").Where(`"announcement".id = ?`, id), "publish announcement")
}

func (s *Store) Unpublish(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execute(ctx, db.NewUpdate().Model((*announcementRow)(nil)).ModelTableExpr(announcementTable).
		Set("published_at = NULL").Where(`"announcement".id = ?`, id), "unpublish announcement")
}

func (s *Store) MarkSeen(ctx context.Context, userID, announcementID int64) (domain.OperationStats, error) {
	return s.mark(ctx, userID, announcementID, false)
}

func (s *Store) MarkDismissed(ctx context.Context, userID, announcementID int64) (domain.OperationStats, error) {
	return s.mark(ctx, userID, announcementID, true)
}

func (s *Store) mark(ctx context.Context, userID, announcementID int64, dismissed bool) (domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	var conflicted bool
	row := viewRow{UserID: userID, AnnouncementID: announcementID, SeenAt: time.Now(), Dismissed: dismissed}
	query := db.NewInsert().Model(&row).ModelTableExpr("platform.announcement_views").
		On("CONFLICT (user_id, announcement_id) DO UPDATE").Set("seen_at = EXCLUDED.seen_at")
	if dismissed {
		query = query.Set("dismissed = TRUE")
	}
	err = query.Returning("(xmax <> 0) AS conflicted").Scan(ctx, &conflicted)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("communication postgres: mark announcement: %w", err)
	}
	stats.Rows = 1
	if conflicted {
		stats.DuplicatePreventionConflicts = 1
	}
	return stats, nil
}

func (s *Store) ViewStats(ctx context.Context, announcementID int64) (domain.AnnouncementStats, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.AnnouncementStats{}, domain.OperationStats{}, err
	}
	result := domain.AnnouncementStats{AnnouncementID: announcementID}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewRaw(`SELECT
		COALESCE(SUM(CASE WHEN seen_at IS NOT NULL THEN 1 ELSE 0 END), 0) AS seen_count,
		COALESCE(SUM(CASE WHEN dismissed = true THEN 1 ELSE 0 END), 0) AS dismissed_count
		FROM platform.announcement_views WHERE announcement_id = ?`, announcementID).
		Scan(ctx, &result.SeenCount, &result.DismissedCount)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return result, stats, fmt.Errorf("communication postgres: count announcement views: %w", err)
	}
	stats.Rows = 1
	return result, stats, nil
}

type executable interface {
	Exec(context.Context, ...any) (sql.Result, error)
}

func execute(ctx context.Context, query executable, operation string) (domain.OperationStats, error) {
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("communication postgres: %s: %w", operation, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return stats, fmt.Errorf("communication postgres: count %s rows: %w", operation, err)
	}
	stats.Rows = rows
	return stats, nil
}

func fromDomain(value domain.Announcement) announcementRow {
	return announcementRow{
		ID: value.ID, Title: value.Title, Content: value.Content, Type: value.Type,
		Severity: value.Severity, Version: value.Version, Active: value.Active,
		PublishedAt: value.PublishedAt, ExpiresAt: value.ExpiresAt,
		TargetRoles: value.TargetRoles, TargetOrgIDs: value.TargetOrgIDs,
		TargetTenantIDs: value.TargetTenantIDs, CreatedBy: value.CreatedBy,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func toDomain(row announcementRow) domain.Announcement {
	return domain.Announcement{
		ID: row.ID, Title: row.Title, Content: row.Content, Type: row.Type,
		Severity: row.Severity, Version: row.Version, Active: row.Active,
		PublishedAt: row.PublishedAt, ExpiresAt: row.ExpiresAt,
		TargetRoles: row.TargetRoles, TargetOrgIDs: row.TargetOrgIDs,
		TargetTenantIDs: row.TargetTenantIDs, CreatedBy: row.CreatedBy,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func toDomainList(rows []announcementRow) []domain.Announcement {
	result := make([]domain.Announcement, 0, len(rows))
	for _, row := range rows {
		result = append(result, toDomain(row))
	}
	return result
}
