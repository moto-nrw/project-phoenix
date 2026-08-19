package platform

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/uptrace/bun"
)

// OperatorSummariesRepository implements platform.OperatorSummariesRepository
// for read-only operator dashboard aggregates. All queries are CTE-based and
// run against the active context transaction when one exists, so callers can
// compose them inside TxHandler.RunInTx without leaking tenant scope.
type OperatorSummariesRepository struct {
	db *bun.DB
}

// NewOperatorSummariesRepository creates a new operator summaries repository.
func NewOperatorSummariesRepository(db *bun.DB) platform.OperatorSummariesRepository {
	return &OperatorSummariesRepository{db: db}
}

// konten_count semantics: an account that is active in N schools counts ONCE
// at the platform level (DISTINCT account_id). The same account may, however,
// contribute one row to each of those N schools' own konten_count, so the
// platform total will not equal the sum of school totals. Org-scoped counts
// follow the same DISTINCT-account rule within their org boundary.
const provisioningStatsQuery = `
SELECT
	(SELECT COUNT(*) FROM platform.organizations WHERE deleted_at IS NULL) AS traeger_count,
	(SELECT COUNT(*) FROM platform.schools WHERE deleted_at IS NULL) AS schulen_count,
	(
		SELECT COUNT(DISTINCT "at".account_id)
		FROM auth.account_tenants AS "at"
		INNER JOIN platform.schools AS "s" ON "s".id = "at".tenant_id
		WHERE "s".deleted_at IS NULL
			AND "at".status = 'active'
	) AS konten_count,
	(
		SELECT COUNT(*)
		FROM iot.devices AS "d"
		INNER JOIN platform.schools AS "s" ON "s".id = "d".tenant_id
		WHERE "s".deleted_at IS NULL
			AND "d".archived_at IS NULL
	) AS geraete_count
`

// Stats returns platform-wide counts.
func (r *OperatorSummariesRepository) Stats(ctx context.Context) (*platform.ProvisioningStats, error) {
	var result platform.ProvisioningStats
	if err := base.GetDB(ctx, r.db).NewRaw(provisioningStatsQuery).Scan(ctx, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// organizationSummariesQuery aggregates child counts in single-pass CTEs and
// LEFT JOINs them onto the organization rows. Each child table is scanned once
// regardless of organization count, replacing the previous per-row correlated
// subqueries (which were O(orgs * 4) executions).
const organizationSummariesQuery = `
WITH school_agg AS (
	SELECT "s".organization_id,
		COUNT(*) AS schulen_count
	FROM platform.schools AS "s"
	WHERE "s".deleted_at IS NULL
	GROUP BY "s".organization_id
),
account_agg AS (
	SELECT "s".organization_id,
		COUNT(DISTINCT "at".account_id) AS konten_count
	FROM auth.account_tenants AS "at"
	INNER JOIN platform.schools AS "s" ON "s".id = "at".tenant_id
	WHERE "s".deleted_at IS NULL
		AND "at".status = 'active'
	GROUP BY "s".organization_id
),
device_agg AS (
	SELECT "s".organization_id,
		COUNT(*) AS geraete_count
	FROM iot.devices AS "d"
	INNER JOIN platform.schools AS "s" ON "s".id = "d".tenant_id
	WHERE "s".deleted_at IS NULL
		AND "d".archived_at IS NULL
	GROUP BY "s".organization_id
),
person_agg AS (
	SELECT "s".organization_id,
		COUNT(*) AS personen_count
	FROM users.persons AS "p"
	INNER JOIN platform.schools AS "s" ON "s".id = "p".tenant_id
	WHERE "s".deleted_at IS NULL
		AND "p".deleted_at IS NULL
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
	COALESCE("sa".schulen_count, 0) AS schulen_count,
	COALESCE("aa".konten_count, 0) AS konten_count,
	COALESCE("da".geraete_count, 0) AS geraete_count,
	COALESCE("pa".personen_count, 0) AS personen_count
FROM platform.organizations AS "o"
LEFT JOIN school_agg AS "sa" ON "sa".organization_id = "o".id
LEFT JOIN account_agg AS "aa" ON "aa".organization_id = "o".id
LEFT JOIN device_agg AS "da" ON "da".organization_id = "o".id
LEFT JOIN person_agg AS "pa" ON "pa".organization_id = "o".id
ORDER BY "o".name ASC
`

// OrganizationSummaries returns all organizations (including soft-deleted)
// with per-row aggregate counts. Counts only include non-deleted child entities.
func (r *OperatorSummariesRepository) OrganizationSummaries(ctx context.Context) ([]*platform.OrganizationSummary, error) {
	var result []*platform.OrganizationSummary
	if err := base.GetDB(ctx, r.db).NewRaw(organizationSummariesQuery).Scan(ctx, &result); err != nil {
		return nil, err
	}
	if result == nil {
		result = []*platform.OrganizationSummary{}
	}
	return result, nil
}

// schoolSummariesQueryGlobal aggregates child counts in single-pass CTEs for
// the platform-wide Schulen list. Cheaper than per-row correlated subqueries
// once the school count grows.
const schoolSummariesQueryGlobal = `
WITH account_agg AS (
	SELECT "at".tenant_id,
		COUNT(DISTINCT "at".account_id) AS konten_count
	FROM auth.account_tenants AS "at"
	WHERE "at".status = 'active'
	GROUP BY "at".tenant_id
),
device_agg AS (
	SELECT "d".tenant_id,
		COUNT(*) AS geraete_count
	FROM iot.devices AS "d"
	WHERE "d".archived_at IS NULL
	GROUP BY "d".tenant_id
),
person_agg AS (
	SELECT "p".tenant_id,
		COUNT(*) AS personen_count
	FROM users.persons AS "p"
	WHERE "p".deleted_at IS NULL
	GROUP BY "p".tenant_id
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
	COALESCE("aa".konten_count, 0) AS konten_count,
	COALESCE("da".geraete_count, 0) AS geraete_count,
	COALESCE("pa".personen_count, 0) AS personen_count
FROM platform.schools AS "s"
INNER JOIN platform.organizations AS "o" ON "o".id = "s".organization_id
LEFT JOIN account_agg AS "aa" ON "aa".tenant_id = "s".id
LEFT JOIN device_agg AS "da" ON "da".tenant_id = "s".id
LEFT JOIN person_agg AS "pa" ON "pa".tenant_id = "s".id
ORDER BY "o".name ASC, "s".name ASC
`

// SchoolSummaries returns every school globally with per-row counts.
func (r *OperatorSummariesRepository) SchoolSummaries(ctx context.Context) ([]*platform.SchoolSummary, error) {
	var result []*platform.SchoolSummary
	if err := base.GetDB(ctx, r.db).NewRaw(schoolSummariesQueryGlobal).Scan(ctx, &result); err != nil {
		return nil, err
	}
	if result == nil {
		result = []*platform.SchoolSummary{}
	}
	return result, nil
}

// schoolSummariesQueryByOrg keeps correlated subqueries because per-org school
// counts are small (typically <20 schools) — the planner does cheap indexed
// lookups, and the org filter avoids scanning every child table.
const schoolSummariesQueryByOrg = `
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
	), 0) AS konten_count,
	COALESCE((
		SELECT COUNT(*)
		FROM iot.devices AS "d"
		WHERE "d".tenant_id = "s".id
			AND "d".archived_at IS NULL
	), 0) AS geraete_count,
	COALESCE((
		SELECT COUNT(*)
		FROM users.persons AS "p"
		WHERE "p".tenant_id = "s".id
			AND "p".deleted_at IS NULL
	), 0) AS personen_count
FROM platform.schools AS "s"
INNER JOIN platform.organizations AS "o" ON "o".id = "s".organization_id
WHERE "s".organization_id = ?
ORDER BY "s".name ASC
`

// SchoolSummariesByOrganization returns schools for a single organization.
func (r *OperatorSummariesRepository) SchoolSummariesByOrganization(ctx context.Context, organizationID int64) ([]*platform.SchoolSummary, error) {
	var result []*platform.SchoolSummary
	if err := base.GetDB(ctx, r.db).NewRaw(schoolSummariesQueryByOrg, organizationID).Scan(ctx, &result); err != nil {
		return nil, err
	}
	if result == nil {
		result = []*platform.SchoolSummary{}
	}
	return result, nil
}

// operatorPersonBaseQuery is the shared SELECT/JOIN body for listing persons
// with school/org context. Callers append a tenant filter and ORDER BY clause.
const operatorPersonBaseQuery = `
SELECT
	"p".id,
	"p".first_name,
	"p".last_name,
	("p".account_id IS NOT NULL) AS has_account,
	"a".email AS account_email,
	(EXISTS (SELECT 1 FROM users.staff WHERE person_id = "p".id AND deleted_at IS NULL)) AS is_staff,
	(EXISTS (SELECT 1 FROM users.students WHERE person_id = "p".id)) AS is_student,
	("p".tag_id IS NOT NULL) AS has_rfid_card,
	"s".id AS school_id,
	"s".name AS school_name,
	"o".id AS organization_id,
	"o".name AS organization_name,
	"p".created_at
FROM users.persons AS "p"
INNER JOIN platform.schools AS "s" ON "s".id = "p".tenant_id
INNER JOIN platform.organizations AS "o" ON "o".id = "s".organization_id
LEFT JOIN auth.accounts AS "a" ON "a".id = "p".account_id
WHERE "p".deleted_at IS NULL
`

// PersonsBySchool returns non-deleted persons in a single, non-deleted school.
// Mirrors PersonsByOrganization in excluding soft-deleted schools so a Papierkorb
// drill-in does not surface a populated Personen tab.
func (r *OperatorSummariesRepository) PersonsBySchool(ctx context.Context, schoolID int64) ([]platform.OperatorPersonInfo, error) {
	var result []platform.OperatorPersonInfo
	q := operatorPersonBaseQuery + ` AND "p".tenant_id = ? AND "s".deleted_at IS NULL ORDER BY "p".last_name, "p".first_name`
	if err := base.GetDB(ctx, r.db).NewRaw(q, schoolID).Scan(ctx, &result); err != nil {
		return nil, err
	}
	if result == nil {
		result = []platform.OperatorPersonInfo{}
	}
	return result, nil
}

// PersonsByOrganization returns non-deleted persons across every (non-deleted)
// school under the given organization.
func (r *OperatorSummariesRepository) PersonsByOrganization(ctx context.Context, organizationID int64) ([]platform.OperatorPersonInfo, error) {
	var result []platform.OperatorPersonInfo
	q := operatorPersonBaseQuery + ` AND "s".organization_id = ? AND "s".deleted_at IS NULL ORDER BY "p".last_name, "p".first_name`
	if err := base.GetDB(ctx, r.db).NewRaw(q, organizationID).Scan(ctx, &result); err != nil {
		return nil, err
	}
	if result == nil {
		result = []platform.OperatorPersonInfo{}
	}
	return result, nil
}

// pwaUsageQuery buckets every active account-tenant mapping into the two
// report portals and LEFT JOINs the standalone-usage rows inside the window.
// The guardian predicate is spelled exactly like the push-subscription
// audience filters (LOWER(role.name) = 'guardian' on auth.account_roles for
// that tenant) so numerator and denominator can never drift apart: a usage
// row only counts while its account still matches the same bucket. A
// dual-role account (staff AND guardian at one school) appears in both
// buckets, mirroring push audience semantics.
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
		%s
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

// PWAUsage returns per-school, per-portal PWA standalone-usage counts within
// the given window. tenantID > 0 limits the result to one school; 0 returns
// every school (metrics export). Cross-tenant read for the operator
// dashboard — callers must run inside an admin transaction.
func (r *OperatorSummariesRepository) PWAUsage(ctx context.Context, tenantID int64, window time.Duration) ([]platform.SchoolPWAUsageRow, error) {
	cutoff := time.Now().Add(-window)
	tenantFilter := ""
	args := []any{authModels.BaseRoleGuardian, authModels.BaseRoleGuardian}
	if tenantID > 0 {
		tenantFilter = `AND "at".tenant_id = ?`
		args = append(args, tenantID)
	}
	args = append(args, cutoff)

	var result []platform.SchoolPWAUsageRow
	q := fmt.Sprintf(pwaUsageQuery, tenantFilter)
	if err := base.GetDB(ctx, r.db).NewRaw(q, args...).Scan(ctx, &result); err != nil {
		return nil, err
	}
	if result == nil {
		result = []platform.SchoolPWAUsageRow{}
	}
	return result, nil
}

const operatorDeviceQuery = `
SELECT
	"d".id,
	"d".device_id,
	"d".device_type,
	"d".name,
	"d".status,
	"d".api_key,
	"d".last_seen,
	"d".created_at,
	"d".updated_at,
	"s".id AS school_id,
	"s".name AS school_name,
	"o".id AS organization_id,
	"o".name AS organization_name
FROM iot.devices AS "d"
INNER JOIN platform.schools AS "s" ON "s".id = "d".tenant_id
INNER JOIN platform.organizations AS "o" ON "o".id = "s".organization_id
`

// ListDeviceRows runs the shared operator device listing with an optional
// filter (by device row, school, or organization). Custom raw-SQL method
// (backend-conventions Rule 2): cross-schema join for the operator
// dashboard, deliberately cross-tenant inside the caller's admin
// transaction.
//
// Devices belonging to a soft-deleted school or organization are filtered
// out unconditionally so global listings (operator dashboard) never surface
// entries for tenants that are in the Papierkorb. Detail endpoints
// (ListSchoolDevices, ListOrganizationDevices) still reject deleted targets
// via their explicit IsDeleted pre-check before reaching this query; the
// SQL filter is the safety net for the global ListAllDevices path.
func (r *OperatorSummariesRepository) ListDeviceRows(ctx context.Context, filter platform.OperatorDeviceFilter) ([]platform.OperatorDeviceRow, error) {
	q := operatorDeviceQuery + ` WHERE "s".deleted_at IS NULL AND "o".deleted_at IS NULL AND "d".archived_at IS NULL`
	var args []interface{}
	switch {
	case filter.DeviceRowID != nil:
		q += ` AND "d".id = ?`
		args = append(args, *filter.DeviceRowID)
	case filter.SchoolID != nil:
		q += ` AND "d".tenant_id = ?`
		args = append(args, *filter.SchoolID)
	case filter.OrganizationID != nil:
		q += ` AND "o".id = ?`
		args = append(args, *filter.OrganizationID)
	}
	q += ` ORDER BY "o".name, "s".name, "d".device_id`

	var result []platform.OperatorDeviceRow
	if err := base.GetDB(ctx, r.db).NewRaw(q, args...).Scan(ctx, &result); err != nil {
		return nil, err
	}
	if result == nil {
		result = []platform.OperatorDeviceRow{}
	}
	return result, nil
}
