// Package operatordashboard implements the tenant-safe read projection behind
// the operator dashboard (#3253): organisation and school rows with their
// account counts, and the per-school PWA standalone-usage buckets.
//
// It is a read-only projection because every one of those answers joins
// Organisation & Tenancy's platform rows with Identity & Access account
// mappings and roles and with the Observability usage rows. It never writes.
// Every statement is a compile-time constant so the architecture evaluator
// can read which tables it touches.
package operatordashboard

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
	"github.com/uptrace/bun"
)

// Database resolves the caller's ambient administrative transaction.
type Database func(context.Context) (bun.IDB, error)

// guardianRoleName mirrors the Identity & Access base role that marks a
// parent account. It is a literal here because a projection reads another
// owner's rows without importing that owner's package.
const guardianRoleName = "guardian"

// Projection answers the operator dashboard reads.
type Projection struct{ database Database }

// New builds the projection over the ambient transaction runtime.
func New(database Database) *Projection {
	if database == nil {
		panic("organization tenancy operator dashboard: database runtime is required")
	}
	return &Projection{database: database}
}

// countsQuery counts an account active in several schools once.
const countsQuery = `
SELECT
	(SELECT COUNT(*) FROM platform.organizations WHERE deleted_at IS NULL) AS organizations,
	(SELECT COUNT(*) FROM platform.schools WHERE deleted_at IS NULL) AS schools,
	(
		SELECT COUNT(DISTINCT "at".account_id)
		FROM auth.account_tenants AS "at"
		INNER JOIN platform.schools AS "s" ON "s".id = "at".tenant_id
		WHERE "s".deleted_at IS NULL
			AND "at".status = 'active'
	) AS accounts
`

type countsRow struct {
	Organizations int `bun:"organizations"`
	Schools       int `bun:"schools"`
	Accounts      int `bun:"accounts"`
}

// Counts returns the platform-wide organisation, school and account counts.
func (p *Projection) Counts(ctx context.Context) (domain.DashboardCounts, error) {
	db, err := p.database(ctx)
	if err != nil {
		return domain.DashboardCounts{}, err
	}
	var row countsRow
	if err := db.NewRaw(countsQuery).Scan(ctx, &row); err != nil {
		return domain.DashboardCounts{}, err
	}
	return domain.DashboardCounts{Organizations: row.Organizations, Schools: row.Schools, Accounts: row.Accounts}, nil
}

// organizationSummariesQuery aggregates the child counts in single-pass CTEs
// and joins them onto the organisation rows, so each child table is scanned
// once regardless of the organisation count.
const organizationSummariesQuery = `
WITH school_agg AS (
	SELECT "s".organization_id,
		COUNT(*) AS school_count
	FROM platform.schools AS "s"
	WHERE "s".deleted_at IS NULL
	GROUP BY "s".organization_id
),
account_agg AS (
	SELECT "s".organization_id,
		COUNT(DISTINCT "at".account_id) AS account_count
	FROM auth.account_tenants AS "at"
	INNER JOIN platform.schools AS "s" ON "s".id = "at".tenant_id
	WHERE "s".deleted_at IS NULL
		AND "at".status = 'active'
	GROUP BY "s".organization_id
)
SELECT
	"o".id,
	"o".name,
	"o".slug,
	"o".active,
	"o".created_at,
	"o".updated_at,
	"o".deleted_at,
	COALESCE("o".settings, '{}') AS settings,
	COALESCE("sa".school_count, 0) AS school_count,
	COALESCE("aa".account_count, 0) AS account_count
FROM platform.organizations AS "o"
LEFT JOIN school_agg AS "sa" ON "sa".organization_id = "o".id
LEFT JOIN account_agg AS "aa" ON "aa".organization_id = "o".id
ORDER BY "o".name ASC
`

type organizationSummaryRow struct {
	ID           int64      `bun:"id"`
	Name         string     `bun:"name"`
	Slug         string     `bun:"slug"`
	Active       bool       `bun:"active"`
	CreatedAt    time.Time  `bun:"created_at"`
	UpdatedAt    time.Time  `bun:"updated_at"`
	DeletedAt    *time.Time `bun:"deleted_at"`
	Settings     string     `bun:"settings"`
	SchoolCount  int        `bun:"school_count"`
	AccountCount int        `bun:"account_count"`
}

// OrganizationSummaries returns every organisation, deleted ones included,
// with the counts of its non-deleted schools and their active accounts. An
// account active in several schools of one organisation counts once.
func (p *Projection) OrganizationSummaries(ctx context.Context) ([]domain.OrganizationSummary, error) {
	db, err := p.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []organizationSummaryRow
	if err := db.NewRaw(organizationSummariesQuery).Scan(ctx, &rows); err != nil {
		return nil, err
	}
	result := make([]domain.OrganizationSummary, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.OrganizationSummary(row))
	}
	return result, nil
}

// schoolSummariesQuery aggregates the account counts in a single-pass CTE
// for the platform-wide school list.
const schoolSummariesQuery = `
WITH account_agg AS (
	SELECT "at".tenant_id,
		COUNT(DISTINCT "at".account_id) AS account_count
	FROM auth.account_tenants AS "at"
	WHERE "at".status = 'active'
	GROUP BY "at".tenant_id
)
SELECT
	"s".id,
	"s".organization_id,
	"o".name AS organization_name,
	"s".name,
	"s".slug,
	"s".subdomain,
	"s".active,
	"s".hidden,
	"s".created_at,
	"s".updated_at,
	"s".deleted_at,
	COALESCE("s".address, '') AS address,
	COALESCE("s".city, '') AS city,
	COALESCE("s".zip, '') AS zip,
	COALESCE("s".phone, '') AS phone,
	COALESCE("s".email, '') AS email,
	COALESCE("s".settings, '{}') AS settings,
	COALESCE("aa".account_count, 0) AS account_count
FROM platform.schools AS "s"
INNER JOIN platform.organizations AS "o" ON "o".id = "s".organization_id
LEFT JOIN account_agg AS "aa" ON "aa".tenant_id = "s".id
ORDER BY "o".name ASC, "s".name ASC
`

// organizationSchoolSummariesQuery keeps correlated subqueries because one
// organisation has few schools: the planner does cheap indexed lookups and
// the organisation filter avoids scanning every mapping.
const organizationSchoolSummariesQuery = `
SELECT
	"s".id,
	"s".organization_id,
	"o".name AS organization_name,
	"s".name,
	"s".slug,
	"s".subdomain,
	"s".active,
	"s".hidden,
	"s".created_at,
	"s".updated_at,
	"s".deleted_at,
	COALESCE("s".address, '') AS address,
	COALESCE("s".city, '') AS city,
	COALESCE("s".zip, '') AS zip,
	COALESCE("s".phone, '') AS phone,
	COALESCE("s".email, '') AS email,
	COALESCE("s".settings, '{}') AS settings,
	COALESCE((
		SELECT COUNT(DISTINCT "at".account_id)
		FROM auth.account_tenants AS "at"
		WHERE "at".tenant_id = "s".id
			AND "at".status = 'active'
	), 0) AS account_count
FROM platform.schools AS "s"
INNER JOIN platform.organizations AS "o" ON "o".id = "s".organization_id
WHERE "s".organization_id = ?
ORDER BY "s".name ASC
`

type schoolSummaryRow struct {
	ID               int64      `bun:"id"`
	OrganizationID   int64      `bun:"organization_id"`
	OrganizationName string     `bun:"organization_name"`
	Name             string     `bun:"name"`
	Slug             string     `bun:"slug"`
	Subdomain        string     `bun:"subdomain"`
	Active           bool       `bun:"active"`
	Hidden           bool       `bun:"hidden"`
	CreatedAt        time.Time  `bun:"created_at"`
	UpdatedAt        time.Time  `bun:"updated_at"`
	DeletedAt        *time.Time `bun:"deleted_at"`
	Address          string     `bun:"address"`
	City             string     `bun:"city"`
	Zip              string     `bun:"zip"`
	Phone            string     `bun:"phone"`
	Email            string     `bun:"email"`
	Settings         string     `bun:"settings"`
	AccountCount     int        `bun:"account_count"`
}

// SchoolSummaries returns every school, or the schools of one organisation,
// deleted ones included, with the organisation's name and the count of
// active accounts.
func (p *Projection) SchoolSummaries(ctx context.Context, organizationID *int64) ([]domain.SchoolSummary, error) {
	db, err := p.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []schoolSummaryRow
	if organizationID == nil {
		err = db.NewRaw(schoolSummariesQuery).Scan(ctx, &rows)
	} else {
		err = db.NewRaw(organizationSchoolSummariesQuery, *organizationID).Scan(ctx, &rows)
	}
	if err != nil {
		return nil, err
	}
	result := make([]domain.SchoolSummary, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.SchoolSummary(row))
	}
	return result, nil
}

// The PWA usage queries bucket every active account-tenant mapping into the
// two report portals and join the standalone-usage rows inside the window.
// The guardian predicate is spelled exactly like the push-subscription
// audience filters (LOWER(role.name) = 'guardian' on auth.account_roles for
// that school), so numerator and denominator can never drift apart: a usage
// row only counts while its account still matches the same bucket. A
// dual-role account (staff and guardian at one school) appears in both
// buckets, mirroring push audience semantics. The two statements differ only
// in the school filter.
const pwaUsageQuery = `
WITH mappings AS (
	SELECT "at".tenant_id,
		"at".account_id,
		EXISTS (
			SELECT 1
			FROM auth.account_roles AS "gar"
			INNER JOIN auth.roles AS "gr" ON "gr".id = "gar".role_id
			WHERE "gar".account_id = "at".account_id
				AND "gar".tenant_id = "at".tenant_id
				AND LOWER("gr".name) = ?
		) AS is_guardian,
		EXISTS (
			SELECT 1
			FROM auth.account_roles AS "sar"
			INNER JOIN auth.roles AS "sr" ON "sr".id = "sar".role_id
			WHERE "sar".account_id = "at".account_id
				AND "sar".tenant_id = "at".tenant_id
				AND LOWER("sr".name) <> ?
		) AS is_staff
	FROM auth.account_tenants AS "at"
	INNER JOIN auth.accounts AS "a" ON "a".id = "at".account_id
	INNER JOIN platform.schools AS "s" ON "s".id = "at".tenant_id
	WHERE "at".status = 'active'
		AND "a".active = TRUE
		AND "s".deleted_at IS NULL
),
buckets AS (
	SELECT tenant_id, account_id, 'staff' AS portal FROM mappings WHERE is_staff
	UNION ALL
	SELECT tenant_id, account_id, 'parent' AS portal FROM mappings WHERE is_guardian
)
SELECT
	"b".tenant_id,
	"b".portal,
	COUNT(DISTINCT "b".account_id) AS eligible_users,
	COUNT(DISTINCT "b".account_id) FILTER (WHERE "u".account_id IS NOT NULL) AS standalone_users
FROM buckets AS "b"
LEFT JOIN iot.pwa_standalone_usage AS "u"
	ON "u".tenant_id = "b".tenant_id
	AND "u".account_id = "b".account_id
	AND "u".portal = "b".portal
	AND "u".last_seen_at >= ?
GROUP BY "b".tenant_id, "b".portal
`

const schoolPWAUsageQuery = `
WITH mappings AS (
	SELECT "at".tenant_id,
		"at".account_id,
		EXISTS (
			SELECT 1
			FROM auth.account_roles AS "gar"
			INNER JOIN auth.roles AS "gr" ON "gr".id = "gar".role_id
			WHERE "gar".account_id = "at".account_id
				AND "gar".tenant_id = "at".tenant_id
				AND LOWER("gr".name) = ?
		) AS is_guardian,
		EXISTS (
			SELECT 1
			FROM auth.account_roles AS "sar"
			INNER JOIN auth.roles AS "sr" ON "sr".id = "sar".role_id
			WHERE "sar".account_id = "at".account_id
				AND "sar".tenant_id = "at".tenant_id
				AND LOWER("sr".name) <> ?
		) AS is_staff
	FROM auth.account_tenants AS "at"
	INNER JOIN auth.accounts AS "a" ON "a".id = "at".account_id
	INNER JOIN platform.schools AS "s" ON "s".id = "at".tenant_id
	WHERE "at".status = 'active'
		AND "a".active = TRUE
		AND "s".deleted_at IS NULL
		AND "at".tenant_id = ?
),
buckets AS (
	SELECT tenant_id, account_id, 'staff' AS portal FROM mappings WHERE is_staff
	UNION ALL
	SELECT tenant_id, account_id, 'parent' AS portal FROM mappings WHERE is_guardian
)
SELECT
	"b".tenant_id,
	"b".portal,
	COUNT(DISTINCT "b".account_id) AS eligible_users,
	COUNT(DISTINCT "b".account_id) FILTER (WHERE "u".account_id IS NOT NULL) AS standalone_users
FROM buckets AS "b"
LEFT JOIN iot.pwa_standalone_usage AS "u"
	ON "u".tenant_id = "b".tenant_id
	AND "u".account_id = "b".account_id
	AND "u".portal = "b".portal
	AND "u".last_seen_at >= ?
GROUP BY "b".tenant_id, "b".portal
`

type pwaUsageRow struct {
	TenantID        int64  `bun:"tenant_id"`
	Portal          string `bun:"portal"`
	StandaloneUsers int    `bun:"standalone_users"`
	EligibleUsers   int    `bun:"eligible_users"`
}

// PWAUsage returns per-school, per-portal PWA standalone-usage counts within
// window. tenantID > 0 limits the result to one school; 0 returns every
// school.
func (p *Projection) PWAUsage(ctx context.Context, tenantID int64, window time.Duration) ([]domain.PWAUsageRow, error) {
	db, err := p.database(ctx)
	if err != nil {
		return nil, err
	}
	cutoff := time.Now().Add(-window)
	var rows []pwaUsageRow
	if tenantID > 0 {
		err = db.NewRaw(schoolPWAUsageQuery, guardianRoleName, guardianRoleName, tenantID, cutoff).Scan(ctx, &rows)
	} else {
		err = db.NewRaw(pwaUsageQuery, guardianRoleName, guardianRoleName, cutoff).Scan(ctx, &rows)
	}
	if err != nil {
		return nil, err
	}
	result := make([]domain.PWAUsageRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.PWAUsageRow(row))
	}
	return result, nil
}
