// Package operatordashboard implements the tenant-safe read projection behind
// the operator dashboard (#3253): organisation and school rows with their
// account counts, and the per-school PWA standalone-usage buckets.
//
// It is a read-only projection because every one of those answers joins
// Organisation & Tenancy's platform rows with Identity & Access account
// mappings and roles and with the Observability usage rows. The account
// counts use bounded owner aggregates; the PWA buckets still
// read the mappings and roles directly. It never writes.
// Every statement is a compile-time constant so the architecture evaluator
// can read which tables it touches. It answers in its own row types; the
// owner's composition maps them onto its values (Map).
package operatordashboard

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/operatordashboard/contract"

	"github.com/uptrace/bun"
)

// Database resolves the caller's ambient administrative transaction.
type Database func(context.Context) (bun.IDB, error)

// ActiveAccountCounts counts distinct accounts with active mappings within each supplied
// school group. The caller defines groups and school visibility; Identity owns
// membership status and account deduplication.
type ActiveAccountCounts func(context.Context, map[int64][]int64) (map[int64]int, error)

var errActiveAccountCountsRequired = errors.New("organization tenancy operator dashboard: active account counts are not bound")

// guardianRoleName mirrors the Identity & Access base role that marks a
// parent account. It is a literal here because a projection reads another
// owner's rows without importing that owner's package.
const guardianRoleName = "guardian"

// Projection answers the operator dashboard reads.
type Projection struct {
	database      Database
	accountCounts ActiveAccountCounts
}

// New builds the projection over the ambient transaction runtime. Without
// accountCounts the account-counting reads fail closed.
func New(database Database, accountCounts ActiveAccountCounts) *Projection {
	if database == nil {
		panic("organization tenancy operator dashboard: database runtime is required")
	}
	return &Projection{database: database, accountCounts: accountCounts}
}

// accountReads fails closed when account facts are not composed.
func (p *Projection) accountReads(ctx context.Context) (bun.IDB, error) {
	if p.accountCounts == nil {
		return nil, errActiveAccountCountsRequired
	}
	return p.database(ctx)
}

// schoolGroups resolves the non-deleted schools of each organization. Account
// aggregation remains inside Identity, so multi-school accounts are not counted
// twice within an organization or the platform total.
func schoolGroups(ctx context.Context, db bun.IDB) (map[int64][]int64, error) {
	var rows []struct{ ID, OrganizationID int64 }
	if err := db.NewRaw("SELECT id, organization_id FROM platform.schools WHERE deleted_at IS NULL").Scan(ctx, &rows); err != nil {
		return nil, err
	}
	groups := make(map[int64][]int64)
	for _, row := range rows {
		groups[row.OrganizationID] = append(groups[row.OrganizationID], row.ID)
	}
	return groups, nil
}

const countsQuery = `
SELECT
	(SELECT COUNT(*) FROM platform.organizations WHERE deleted_at IS NULL) AS organizations,
	(SELECT COUNT(*) FROM platform.schools WHERE deleted_at IS NULL) AS schools
`

// Counts are the platform-wide organisation, school and account counts.

// Counts returns the platform-wide organisation, school and account counts.
func (p *Projection) Counts(ctx context.Context) (contract.Counts, error) {
	db, err := p.accountReads(ctx)
	if err != nil {
		return contract.Counts{}, err
	}
	var row contract.Counts
	if err := db.NewRaw(countsQuery).Scan(ctx, &row); err != nil {
		return contract.Counts{}, err
	}
	groups, err := schoolGroups(ctx, db)
	if err != nil {
		return contract.Counts{}, err
	}
	var schools []int64
	for _, ids := range groups {
		schools = append(schools, ids...)
	}
	counts, err := p.accountCounts(ctx, map[int64][]int64{0: schools})
	if err != nil {
		return contract.Counts{}, err
	}
	row.Accounts = counts[0]
	return row, nil
}

// organizationSummariesQuery attaches non-deleted school counts to organizations.
// Identity supplies the corresponding distinct-account counts separately.
const organizationSummariesQuery = `
WITH school_agg AS (
	SELECT "s".organization_id,
		COUNT(*) AS school_count
	FROM platform.schools AS "s"
	WHERE "s".deleted_at IS NULL
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
	COALESCE("sa".school_count, 0) AS school_count
FROM platform.organizations AS "o"
LEFT JOIN school_agg AS "sa" ON "sa".organization_id = "o".id
ORDER BY "o".name ASC
`

// contract.OrganizationSummary is an organisation with its school and account counts.

// OrganizationSummaries returns every organisation, deleted ones included,
// with the counts of its non-deleted schools and their active accounts. An
// account active in several schools of one organisation counts once.
func (p *Projection) OrganizationSummaries(ctx context.Context) ([]contract.OrganizationSummary, error) {
	db, err := p.accountReads(ctx)
	if err != nil {
		return nil, err
	}
	var rows []contract.OrganizationSummary
	if err := db.NewRaw(organizationSummariesQuery).Scan(ctx, &rows); err != nil {
		return nil, err
	}
	groups, err := schoolGroups(ctx, db)
	if err != nil {
		return nil, err
	}
	counts, err := p.accountCounts(ctx, groups)
	if err != nil {
		return nil, err
	}
	result := make([]contract.OrganizationSummary, 0, len(rows))
	for _, row := range rows {
		row.AccountCount = counts[row.ID]
		result = append(result, row)
	}
	return result, nil
}

// schoolSummariesQuery reads the platform-wide school list. Account counts are
// supplied by one bounded Identity aggregate for all returned schools.
const schoolSummariesQuery = `
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
	"s".child_quota_bundles,
	"s".child_quota_bundle_size
FROM platform.schools AS "s"
INNER JOIN platform.organizations AS "o" ON "o".id = "s".organization_id
ORDER BY "o".name ASC, "s".name ASC
`

// organizationSchoolSummariesQuery limits school rows to one organization.
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
	"s".child_quota_bundles,
	"s".child_quota_bundle_size
FROM platform.schools AS "s"
INNER JOIN platform.organizations AS "o" ON "o".id = "s".organization_id
WHERE "s".organization_id = ?
ORDER BY "s".name ASC
`

// contract.SchoolSummary is a school with its organisation's name and account count.

// SchoolSummaries returns every school, or the schools of one organisation,
// deleted ones included, with the organisation's name and the count of
// active accounts.
func (p *Projection) SchoolSummaries(ctx context.Context, organizationID *int64) ([]contract.SchoolSummary, error) {
	db, err := p.accountReads(ctx)
	if err != nil {
		return nil, err
	}
	var rows []contract.SchoolSummary
	if organizationID == nil {
		err = db.NewRaw(schoolSummariesQuery).Scan(ctx, &rows)
	} else {
		err = db.NewRaw(organizationSchoolSummariesQuery, *organizationID).Scan(ctx, &rows)
	}
	if err != nil {
		return nil, err
	}
	groups := make(map[int64][]int64, len(rows))
	for _, row := range rows {
		groups[row.ID] = []int64{row.ID}
	}
	counts, err := p.accountCounts(ctx, groups)
	if err != nil {
		return nil, err
	}
	result := make([]contract.SchoolSummary, 0, len(rows))
	for _, row := range rows {
		row.AccountCount = counts[row.ID]
		result = append(result, row)
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

// contract.PWAUsageRow is one school's PWA standalone usage in one portal.

// PWAUsage reports one explicitly selected school.
func (p *Projection) PWAUsage(ctx context.Context, tenantID int64, window time.Duration) ([]contract.PWAUsageRow, error) {
	if tenantID <= 0 {
		return nil, fmt.Errorf("operator dashboard: school ID is required")
	}
	db, err := p.database(ctx)
	if err != nil {
		return nil, err
	}
	return scanPWAUsage(ctx, db.NewRaw(schoolPWAUsageQuery, guardianRoleName, guardianRoleName, tenantID, time.Now().Add(-window)))
}

// AllSchoolsPWAUsage is the operator's cross-school aggregate.
func (p *Projection) AllSchoolsPWAUsage(ctx context.Context, window time.Duration) ([]contract.PWAUsageRow, error) {
	db, err := p.database(ctx)
	if err != nil {
		return nil, err
	}
	return scanPWAUsage(ctx, db.NewRaw(pwaUsageQuery, guardianRoleName, guardianRoleName, time.Now().Add(-window)))
}

func scanPWAUsage(ctx context.Context, query *bun.RawQuery) ([]contract.PWAUsageRow, error) {
	var rows []contract.PWAUsageRow
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, err
	}
	return append(make([]contract.PWAUsageRow, 0, len(rows)), rows...), nil
}
