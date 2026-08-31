package platform

import (
	"context"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

// Table and query constants
const (
	tablePlatformAnnouncementViews      = "platform.announcement_views"
	tablePlatformAnnouncementViewsAlias = `platform.announcement_views AS "view"`
)

// AnnouncementViewRepository implements platform.AnnouncementViewRepository interface
type AnnouncementViewRepository struct {
	db *bun.DB
}

// NewAnnouncementViewRepository creates a new AnnouncementViewRepository
func NewAnnouncementViewRepository(db *bun.DB) platform.AnnouncementViewRepository {
	return &AnnouncementViewRepository{db: db}
}

// MarkSeen marks an announcement as seen by a user
func (r *AnnouncementViewRepository) MarkSeen(ctx context.Context, userID, announcementID int64) error {
	view := &platform.AnnouncementView{
		UserID:         userID,
		AnnouncementID: announcementID,
		SeenAt:         time.Now(),
		Dismissed:      false,
	}

	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(view).
		ModelTableExpr(tablePlatformAnnouncementViews).
		On("CONFLICT (user_id, announcement_id) DO UPDATE").
		Set("seen_at = EXCLUDED.seen_at").
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "mark announcement seen",
			Err: base.TranslateNotFound(err),
		}
	}

	return nil
}

// MarkDismissed marks an announcement as dismissed by a user
func (r *AnnouncementViewRepository) MarkDismissed(ctx context.Context, userID, announcementID int64) error {
	view := &platform.AnnouncementView{
		UserID:         userID,
		AnnouncementID: announcementID,
		SeenAt:         time.Now(),
		Dismissed:      true,
	}

	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(view).
		ModelTableExpr(tablePlatformAnnouncementViews).
		On("CONFLICT (user_id, announcement_id) DO UPDATE").
		Set("seen_at = EXCLUDED.seen_at").
		Set("dismissed = true").
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "mark announcement dismissed",
			Err: base.TranslateNotFound(err),
		}
	}

	return nil
}

// buildUnreadArgs returns the fixed-size positional args shared by the FROM + WHERE
// in the unread announcement query.
// Arg order: userID (account JOIN), tenantID (membership JOIN), orgID (school JOIN),
// now, now, pgArray(userRoles) — 6 args.
func buildUnreadArgs(userID int64, userRoles []string, tenantID int64, orgID int64) []any {
	now := time.Now()
	if userRoles == nil {
		userRoles = []string{}
	}
	return []any{userID, tenantID, orgID, now, now, pgdialect.Array(userRoles)}
}

// unreadWhereClause is the shared SQL fragment used by both GetUnreadForUser
// and CountUnread. Arg positions are fixed (always 6 — see buildUnreadArgs).
//
// Role matching: a user matches if their JWT role names overlap with target_roles
// (direct match) OR if any of their assigned roles in the current session tenant
// has a base_role that appears in target_roles (base-role match via EXISTS).
// The tenant scoping on the EXISTS prevents cross-tenant role leakage: a user's
// custom role in Tenant B must not influence announcement delivery in Tenant A.
// This mirrors the strategy used by GetStats to keep delivery and stats consistent.
const unreadWhereClause = `
	WHERE a.active = true
		AND a.published_at IS NOT NULL
		AND a.published_at <= ?
		AND (a.expires_at IS NULL OR a.expires_at > ?)
		AND a.published_at >= GREATEST(
			s.created_at,
			COALESCE(at.invited_at, at.created_at),
			acc.created_at
		)
		AND v.seen_at IS NULL
		AND (a.target_roles = '{}'
			OR a.target_roles && ?::text[]
			OR EXISTS (
				SELECT 1 FROM auth.account_roles ar
				JOIN auth.roles r ON r.id = ar.role_id
				WHERE ar.account_id = acc.id
				AND ar.tenant_id = at.tenant_id
				AND r.base_role IS NOT NULL
				AND r.base_role = ANY(a.target_roles)
			))
		AND (
			(a.target_org_ids = '{}' AND a.target_tenant_ids = '{}')
			OR (a.target_tenant_ids != '{}' AND at.tenant_id = ANY(a.target_tenant_ids))
			OR (a.target_org_ids != '{}' AND s.organization_id = ANY(a.target_org_ids))
		)`

const unreadFromClause = ` FROM platform.announcements a
		JOIN auth.accounts acc
			ON acc.id = ?
		JOIN auth.account_tenants at
			ON at.account_id = acc.id
			AND at.tenant_id = ?
			AND at.status = 'active'
		JOIN platform.schools s
			ON s.id = at.tenant_id
			AND s.organization_id = ?
			AND s.deleted_at IS NULL
		LEFT JOIN platform.announcement_views v
			ON v.announcement_id = a.id AND v.user_id = acc.id`

// GetUnreadForUser retrieves all unread active announcements for a user scoped to the current session tenant/org.
// Targeting logic uses OR-union: global / org match / tenant match.
func (r *AnnouncementViewRepository) GetUnreadForUser(ctx context.Context, userID int64, userRoles []string, tenantID int64, orgID int64) ([]*platform.Announcement, error) {
	var announcements []*platform.Announcement
	args := buildUnreadArgs(userID, userRoles, tenantID, orgID)

	query := `SELECT a.*` +
		unreadFromClause +
		unreadWhereClause +
		` ORDER BY a.published_at DESC`

	err := base.GetDB(ctx, r.db).NewRaw(query, args...).Scan(ctx, &announcements)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get unread announcements for user",
			Err: base.TranslateNotFound(err),
		}
	}

	return announcements, nil
}

// CountUnread counts unread announcements for a user scoped to the current session tenant/org.
func (r *AnnouncementViewRepository) CountUnread(ctx context.Context, userID int64, userRoles []string, tenantID int64, orgID int64) (int, error) {
	args := buildUnreadArgs(userID, userRoles, tenantID, orgID)

	query := `SELECT COUNT(*)` +
		unreadFromClause +
		unreadWhereClause

	var count int
	err := base.GetDB(ctx, r.db).NewRaw(query, args...).Scan(ctx, &count)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "count unread announcements",
			Err: base.TranslateNotFound(err),
		}
	}

	return count, nil
}

// GetStats retrieves view statistics for an announcement.
// Target count is scoped by role, org, and tenant targeting.
func (r *AnnouncementViewRepository) GetStats(ctx context.Context, announcementID int64) (*platform.AnnouncementStats, error) {
	stats := &platform.AnnouncementStats{
		AnnouncementID: announcementID,
	}

	// Get the targeting criteria for this announcement.
	// Use a typed struct so bun/pgdriver correctly deserializes bigint[] columns
	// (positional Scan with []int64 fails because database/sql sees them as text).
	var targeting struct {
		TargetRoles     []string `bun:"target_roles,array"`
		TargetOrgIDs    []int64  `bun:"target_org_ids,array"`
		TargetTenantIDs []int64  `bun:"target_tenant_ids,array"`
		PublishedAt     *time.Time
	}
	err := base.GetDB(ctx, r.db).NewRaw(`
		SELECT
			COALESCE(target_roles, '{}'::text[]) AS target_roles,
			COALESCE(target_org_ids, '{}'::bigint[]) AS target_org_ids,
			COALESCE(target_tenant_ids, '{}'::bigint[]) AS target_tenant_ids,
			published_at
		FROM platform.announcements WHERE id = ?
	`, announcementID).Scan(ctx, &targeting)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get announcement targeting criteria",
			Err: base.TranslateNotFound(err),
		}
	}

	targetRoles := targeting.TargetRoles
	targetOrgIDs := targeting.TargetOrgIDs
	targetTenantIDs := targeting.TargetTenantIDs
	hasRoleFilter := len(targetRoles) > 0
	hasOrgFilter := len(targetOrgIDs) > 0
	hasTenantFilter := len(targetTenantIDs) > 0

	// Build the target count query dynamically instead of 2^N switch branches.
	// Base: count distinct accounts with active tenant membership.
	// Role filter: JOIN account_roles correlated with the SAME tenant row (ar.tenant_id = at.tenant_id)
	//              so a user who is admin in school A but teacher in school B is not overcounted.
	// Org/tenant filter: JOIN schools for org lookup, OR-union when both are present.
	var queryParts []string
	var queryArgs []any

	queryParts = append(queryParts, `SELECT COUNT(DISTINCT at.account_id) FROM auth.account_tenants at`)
	queryParts = append(queryParts, `JOIN auth.accounts acc ON acc.id = at.account_id AND acc.active = true`)

	// Always join schools to exclude accounts linked to soft-deleted tenants,
	// even for global announcements (consistent with targeted path).
	queryParts = append(queryParts, `JOIN platform.schools s ON s.id = at.tenant_id AND s.deleted_at IS NULL`)

	// Role filter: correlate role assignment with the same tenant
	if hasRoleFilter {
		queryParts = append(queryParts, `JOIN auth.account_roles ar ON ar.account_id = at.account_id AND ar.tenant_id = at.tenant_id`)
		queryParts = append(queryParts, `JOIN auth.roles r ON r.id = ar.role_id`)
	}

	queryParts = append(queryParts, `WHERE at.status = 'active'`)

	if targeting.PublishedAt != nil {
		queryParts = append(queryParts, `AND ? >= GREATEST(s.created_at, COALESCE(at.invited_at, at.created_at), acc.created_at)`)
		queryArgs = append(queryArgs, *targeting.PublishedAt)
	}

	if hasRoleFilter {
		queryParts = append(queryParts, `AND (r.name IN (?) OR (r.base_role IS NOT NULL AND r.base_role IN (?)))`)
		queryArgs = append(queryArgs, bun.List(targetRoles), bun.List(targetRoles))
	}

	if hasOrgFilter && hasTenantFilter {
		queryParts = append(queryParts, `AND (s.organization_id IN (?) OR at.tenant_id IN (?))`)
		queryArgs = append(queryArgs, bun.List(targetOrgIDs), bun.List(targetTenantIDs))
	} else if hasOrgFilter {
		queryParts = append(queryParts, `AND s.organization_id IN (?)`)
		queryArgs = append(queryArgs, bun.List(targetOrgIDs))
	} else if hasTenantFilter {
		queryParts = append(queryParts, `AND s.id IN (?)`)
		queryArgs = append(queryArgs, bun.List(targetTenantIDs))
	}

	rawSQL := strings.Join(queryParts, " ")
	err = base.GetDB(ctx, r.db).NewRaw(rawSQL, queryArgs...).Scan(ctx, &stats.TargetCount)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "count target users",
			Err: base.TranslateNotFound(err),
		}
	}

	// Count seen and dismissed
	err = base.GetDB(ctx, r.db).NewRaw(`
		SELECT
			COALESCE(SUM(CASE WHEN seen_at IS NOT NULL THEN 1 ELSE 0 END), 0) as seen_count,
			COALESCE(SUM(CASE WHEN dismissed = true THEN 1 ELSE 0 END), 0) as dismissed_count
		FROM platform.announcement_views
		WHERE announcement_id = ?
	`, announcementID).Scan(ctx, &stats.SeenCount, &stats.DismissedCount)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get announcement view stats",
			Err: base.TranslateNotFound(err),
		}
	}

	return stats, nil
}

// GetViewDetails returns detailed view information including user names
func (r *AnnouncementViewRepository) GetViewDetails(ctx context.Context, announcementID int64) ([]*platform.AnnouncementViewDetail, error) {
	var details []*platform.AnnouncementViewDetail

	// Join with auth.accounts and users.persons to get user names.
	// Use DISTINCT ON to avoid duplicate rows when an account has multiple persons
	// (e.g. staff assigned to multiple tenants). Pick the most recently updated person.
	err := base.GetDB(ctx, r.db).NewRaw(`
		SELECT user_id, user_name, seen_at, dismissed
		FROM (
			SELECT DISTINCT ON (v.user_id)
				v.user_id,
				COALESCE(
					NULLIF(CONCAT(p.first_name, ' ', p.last_name), ' '),
					acc.email
				) as user_name,
				v.seen_at,
				v.dismissed
			FROM platform.announcement_views v
			JOIN auth.accounts acc ON acc.id = v.user_id
			LEFT JOIN users.persons p ON p.account_id = acc.id
			WHERE v.announcement_id = ?
			ORDER BY v.user_id, p.updated_at DESC NULLS LAST
		) sub
		ORDER BY seen_at DESC
	`, announcementID).Scan(ctx, &details)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get announcement view details",
			Err: base.TranslateNotFound(err),
		}
	}

	return details, nil
}
