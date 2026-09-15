// Package audience implements the tenant-safe staff-announcement audience
// projection across Communication, Identity & Access, and Organization data.
package audience

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication/internal/domain"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

type Database func(context.Context) (bun.IDB, error)

type Projection struct{ database Database }

type announcementRow struct {
	bun.BaseModel   `bun:"table:platform.announcements,alias:announcement"`
	ID              int64      `bun:"id,pk"`
	Title           string     `bun:"title"`
	Content         string     `bun:"content"`
	Type            string     `bun:"type"`
	Severity        string     `bun:"severity"`
	Version         *string    `bun:"version"`
	Active          bool       `bun:"active"`
	PublishedAt     *time.Time `bun:"published_at"`
	ExpiresAt       *time.Time `bun:"expires_at"`
	TargetRoles     []string   `bun:"target_roles,array"`
	TargetOrgIDs    []int64    `bun:"target_org_ids,array"`
	TargetTenantIDs []int64    `bun:"target_tenant_ids,array"`
	CreatedBy       int64      `bun:"created_by"`
	CreatedAt       time.Time  `bun:"created_at"`
	UpdatedAt       time.Time  `bun:"updated_at"`
}

func New(database Database) *Projection {
	if database == nil {
		panic("announcement audience projection: database runtime is required")
	}
	return &Projection{database: database}
}

func unreadArgs(userID int64, roles []string, tenantID, orgID int64) []any {
	if roles == nil {
		roles = []string{}
	}
	return []any{userID, tenantID, orgID, pgdialect.Array(roles)}
}

const unreadFrom = ` FROM platform.announcements a
	JOIN auth.accounts acc ON acc.id = ?
	JOIN auth.account_tenants at ON at.account_id = acc.id AND at.tenant_id = ? AND at.status = 'active'
	JOIN platform.schools s ON s.id = at.tenant_id AND s.organization_id = ? AND s.deleted_at IS NULL
	LEFT JOIN platform.announcement_views v ON v.announcement_id = a.id AND v.user_id = acc.id`

const unreadWhere = `
	WHERE a.active = true
		AND a.published_at IS NOT NULL
		AND a.published_at <= CURRENT_TIMESTAMP
		AND (a.expires_at IS NULL OR a.expires_at > CURRENT_TIMESTAMP)
		AND a.published_at >= GREATEST(s.created_at, COALESCE(at.invited_at, at.created_at), acc.created_at)
		AND v.seen_at IS NULL
		AND (a.target_roles = '{}'
			OR a.target_roles && ?::text[]
			OR EXISTS (
				SELECT 1 FROM auth.account_roles ar
				JOIN auth.roles r ON r.id = ar.role_id
				WHERE ar.account_id = acc.id AND ar.tenant_id = at.tenant_id
					AND r.base_role IS NOT NULL AND r.base_role = ANY(a.target_roles)
			))
		AND (
			(a.target_org_ids = '{}' AND a.target_tenant_ids = '{}')
			OR (a.target_tenant_ids != '{}' AND at.tenant_id = ANY(a.target_tenant_ids))
			OR (a.target_org_ids != '{}' AND s.organization_id = ANY(a.target_org_ids))
		)`

func (p *Projection) Unread(ctx context.Context, userID int64, roles []string, tenantID, orgID int64) ([]domain.Announcement, domain.OperationStats, error) {
	db, err := p.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []announcementRow
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewRaw(`SELECT a.*`+unreadFrom+unreadWhere+` ORDER BY a.published_at DESC`, unreadArgs(userID, roles, tenantID, orgID)...).Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("announcement audience projection: list unread: %w", err)
	}
	stats.Rows = int64(len(rows))
	result := make([]domain.Announcement, 0, len(rows))
	for _, row := range rows {
		result = append(result, toDomain(row))
	}
	return result, stats, nil
}

func (p *Projection) CountUnread(ctx context.Context, userID int64, roles []string, tenantID, orgID int64) (int, domain.OperationStats, error) {
	db, err := p.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	var count int
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewRaw(`SELECT COUNT(*)`+unreadFrom+unreadWhere, unreadArgs(userID, roles, tenantID, orgID)...).Scan(ctx, &count)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, fmt.Errorf("announcement audience projection: count unread: %w", err)
	}
	stats.Rows = 1
	return count, stats, nil
}

func (p *Projection) TargetCount(ctx context.Context, value domain.Announcement) (int, domain.OperationStats, error) {
	db, err := p.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	var count int
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewRaw(`
		SELECT COUNT(DISTINCT at.account_id)
		FROM auth.account_tenants at
		JOIN auth.accounts acc ON acc.id = at.account_id AND acc.active = true
		JOIN platform.schools s ON s.id = at.tenant_id AND s.deleted_at IS NULL
		WHERE at.status = 'active'
			AND (?::timestamptz IS NULL OR ? >= GREATEST(s.created_at, COALESCE(at.invited_at, at.created_at), acc.created_at))
			AND (
				COALESCE(array_length(?::text[], 1), 0) = 0
				OR EXISTS (
					SELECT 1
					FROM auth.account_roles ar
					JOIN auth.roles r ON r.id = ar.role_id
					WHERE ar.account_id = at.account_id
						AND ar.tenant_id = at.tenant_id
						AND (r.name = ANY(?::text[]) OR (r.base_role IS NOT NULL AND r.base_role = ANY(?::text[])))
				)
			)
			AND (
				(COALESCE(array_length(?::bigint[], 1), 0) = 0 AND COALESCE(array_length(?::bigint[], 1), 0) = 0)
				OR s.organization_id = ANY(?::bigint[])
				OR at.tenant_id = ANY(?::bigint[])
			)
	`, value.PublishedAt, value.PublishedAt,
		pgdialect.Array(value.TargetRoles), pgdialect.Array(value.TargetRoles), pgdialect.Array(value.TargetRoles),
		pgdialect.Array(value.TargetOrgIDs), pgdialect.Array(value.TargetTenantIDs),
		pgdialect.Array(value.TargetOrgIDs), pgdialect.Array(value.TargetTenantIDs)).Scan(ctx, &count)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, fmt.Errorf("announcement audience projection: count targets: %w", err)
	}
	stats.Rows = 1
	return count, stats, nil
}

func (p *Projection) ViewDetails(ctx context.Context, announcementID int64) ([]domain.AnnouncementViewDetail, domain.OperationStats, error) {
	db, err := p.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []struct {
		UserID       int64     `bun:"user_id"`
		AccountEmail string    `bun:"account_email"`
		UserName     string    `bun:"user_name"`
		SeenAt       time.Time `bun:"seen_at"`
		Dismissed    bool      `bun:"dismissed"`
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewRaw(`SELECT user_id, account_email, user_name, seen_at, dismissed
		FROM (
			SELECT DISTINCT ON (v.user_id) v.user_id, acc.email AS account_email,
				acc.email AS user_name, v.seen_at, v.dismissed
			FROM platform.announcement_views v
			JOIN auth.accounts acc ON acc.id = v.user_id
			WHERE v.announcement_id = ?
			ORDER BY v.user_id, v.seen_at DESC
		) AS latest_view ORDER BY seen_at DESC`, announcementID).Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("announcement audience projection: get view details: %w", err)
	}
	stats.Rows = int64(len(rows))
	result := make([]domain.AnnouncementViewDetail, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.AnnouncementViewDetail{
			UserID: row.UserID, AccountEmail: row.AccountEmail, UserName: row.UserName,
			SeenAt: row.SeenAt, Dismissed: row.Dismissed,
		})
	}
	return result, stats, nil
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
