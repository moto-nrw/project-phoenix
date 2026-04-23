package platform

import (
	"context"
	"time"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// ProvisioningStats holds platform-wide aggregate counts for the operator
// overview KPI strip. All counts exclude soft-deleted rows.
type ProvisioningStats struct {
	TraegerCount int `bun:"traeger_count" json:"traeger_count"`
	SchulenCount int `bun:"schulen_count" json:"schulen_count"`
	KontenCount  int `bun:"konten_count" json:"konten_count"`
	GeraeteCount int `bun:"geraete_count" json:"geraete_count"`
}

// OrganizationSummary is an organization row enriched with aggregate counts
// scoped to that organization. Counts reflect non-deleted child entities only.
type OrganizationSummary struct {
	ID            int64      `bun:"id" json:"id"`
	Name          string     `bun:"name" json:"name"`
	Slug          string     `bun:"slug" json:"slug"`
	Active        bool       `bun:"active" json:"active"`
	CreatedAt     time.Time  `bun:"created_at" json:"created_at"`
	UpdatedAt     time.Time  `bun:"updated_at" json:"updated_at"`
	DeletedAt     *time.Time `bun:"deleted_at" json:"deleted_at,omitempty"`
	Settings      string     `bun:"settings" json:"settings,omitempty"`
	SchulenCount  int        `bun:"schulen_count" json:"schulen_count"`
	KontenCount   int        `bun:"konten_count" json:"konten_count"`
	GeraeteCount  int        `bun:"geraete_count" json:"geraete_count"`
	PersonenCount int        `bun:"personen_count" json:"personen_count"`
}

// SchoolSummary is a school row enriched with aggregate counts scoped to that
// school. Counts reflect non-deleted child entities only. OrganizationName is
// denormalized from the parent for display in the global Schulen list.
type SchoolSummary struct {
	ID               int64      `bun:"id" json:"id"`
	OrganizationID   int64      `bun:"organization_id" json:"organization_id"`
	OrganizationName string     `bun:"organization_name" json:"organization_name"`
	Name             string     `bun:"name" json:"name"`
	Slug             string     `bun:"slug" json:"slug"`
	Subdomain        string     `bun:"subdomain" json:"subdomain"`
	Active           bool       `bun:"active" json:"active"`
	Hidden           bool       `bun:"hidden" json:"hidden"`
	CreatedAt        time.Time  `bun:"created_at" json:"created_at"`
	UpdatedAt        time.Time  `bun:"updated_at" json:"updated_at"`
	DeletedAt        *time.Time `bun:"deleted_at" json:"deleted_at,omitempty"`
	Address          string     `bun:"address" json:"address,omitempty"`
	City             string     `bun:"city" json:"city,omitempty"`
	Zip              string     `bun:"zip" json:"zip,omitempty"`
	Phone            string     `bun:"phone" json:"phone,omitempty"`
	Email            string     `bun:"email" json:"email,omitempty"`
	Settings         string     `bun:"settings" json:"settings,omitempty"`
	KontenCount      int        `bun:"konten_count" json:"konten_count"`
	GeraeteCount     int        `bun:"geraete_count" json:"geraete_count"`
	PersonenCount    int        `bun:"personen_count" json:"personen_count"`
}

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
	) AS geraete_count
`

// GetProvisioningStats returns platform-wide counts for the operator overview.
func (s *operatorProvisioningService) GetProvisioningStats(ctx context.Context) (*ProvisioningStats, error) {
	var result ProvisioningStats
	err := s.withAdminTx(ctx, func(adminCtx context.Context) error {
		db := s.pickDB(adminCtx)
		return db.NewRaw(provisioningStatsQuery).Scan(adminCtx, &result)
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

const organizationSummariesQuery = `
SELECT
	"o".id,
	"o".name,
	"o".slug,
	"o".active,
	"o".created_at,
	"o".updated_at,
	"o".deleted_at,
	COALESCE("o".settings, '{}') AS settings,
	COALESCE((
		SELECT COUNT(*)
		FROM platform.schools AS "s"
		WHERE "s".organization_id = "o".id
			AND "s".deleted_at IS NULL
	), 0) AS schulen_count,
	COALESCE((
		SELECT COUNT(DISTINCT "at".account_id)
		FROM auth.account_tenants AS "at"
		INNER JOIN platform.schools AS "s" ON "s".id = "at".tenant_id
		WHERE "s".organization_id = "o".id
			AND "s".deleted_at IS NULL
			AND "at".status = 'active'
	), 0) AS konten_count,
	COALESCE((
		SELECT COUNT(*)
		FROM iot.devices AS "d"
		INNER JOIN platform.schools AS "s" ON "s".id = "d".tenant_id
		WHERE "s".organization_id = "o".id
			AND "s".deleted_at IS NULL
	), 0) AS geraete_count,
	COALESCE((
		SELECT COUNT(*)
		FROM users.persons AS "p"
		INNER JOIN platform.schools AS "s" ON "s".id = "p".tenant_id
		WHERE "s".organization_id = "o".id
			AND "s".deleted_at IS NULL
			AND "p".deleted_at IS NULL
	), 0) AS personen_count
FROM platform.organizations AS "o"
ORDER BY "o".name ASC
`

// ListOrganizationSummaries returns all organizations (including soft-deleted)
// with per-row aggregate counts. Counts only include non-deleted child entities.
func (s *operatorProvisioningService) ListOrganizationSummaries(ctx context.Context) ([]*OrganizationSummary, error) {
	var result []*OrganizationSummary
	err := s.withAdminTx(ctx, func(adminCtx context.Context) error {
		db := s.pickDB(adminCtx)
		return db.NewRaw(organizationSummariesQuery).Scan(adminCtx, &result)
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		result = []*OrganizationSummary{}
	}
	return result, nil
}

const schoolSummariesQueryBase = `
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
	), 0) AS geraete_count,
	COALESCE((
		SELECT COUNT(*)
		FROM users.persons AS "p"
		WHERE "p".tenant_id = "s".id
			AND "p".deleted_at IS NULL
	), 0) AS personen_count
FROM platform.schools AS "s"
INNER JOIN platform.organizations AS "o" ON "o".id = "s".organization_id
`

// ListSchoolSummaries returns all schools (global scope) with per-row counts.
func (s *operatorProvisioningService) ListSchoolSummaries(ctx context.Context) ([]*SchoolSummary, error) {
	var result []*SchoolSummary
	err := s.withAdminTx(ctx, func(adminCtx context.Context) error {
		db := s.pickDB(adminCtx)
		q := schoolSummariesQueryBase + ` ORDER BY "o".name ASC, "s".name ASC`
		return db.NewRaw(q).Scan(adminCtx, &result)
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		result = []*SchoolSummary{}
	}
	return result, nil
}

// ListOrganizationSchoolSummaries returns schools under a specific organization
// with per-row counts. Returns OrganizationNotFoundError if the org does not exist.
func (s *operatorProvisioningService) ListOrganizationSchoolSummaries(ctx context.Context, organizationID int64) ([]*SchoolSummary, error) {
	var result []*SchoolSummary
	err := s.withAdminTx(ctx, func(adminCtx context.Context) error {
		org, findErr := s.organizationRepo.FindByID(adminCtx, organizationID)
		if findErr != nil {
			return findErr
		}
		if org == nil {
			return &OrganizationNotFoundError{OrganizationID: organizationID}
		}
		db := s.pickDB(adminCtx)
		q := schoolSummariesQueryBase + ` WHERE "s".organization_id = ? AND "s".deleted_at IS NULL ORDER BY "s".name ASC`
		return db.NewRaw(q, organizationID).Scan(adminCtx, &result)
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		result = []*SchoolSummary{}
	}
	return result, nil
}

// ListOrganizationPersons returns all persons across every school belonging to
// the given organization, with school/org context on each row. Returns
// OrganizationNotFoundError if the org does not exist. Replaces the former
// client-side fan-out that issued one request per school.
func (s *operatorProvisioningService) ListOrganizationPersons(ctx context.Context, organizationID int64) ([]OperatorPersonInfo, error) {
	var result []OperatorPersonInfo
	err := s.withAdminTx(ctx, func(adminCtx context.Context) error {
		org, findErr := s.organizationRepo.FindByID(adminCtx, organizationID)
		if findErr != nil {
			return findErr
		}
		if org == nil {
			return &OrganizationNotFoundError{OrganizationID: organizationID}
		}
		db := s.pickDB(adminCtx)
		q := operatorPersonQuery + ` AND "s".organization_id = ? AND "s".deleted_at IS NULL ORDER BY "p".last_name, "p".first_name`
		if scanErr := db.NewRaw(q, organizationID).Scan(adminCtx, &result); scanErr != nil {
			return scanErr
		}
		if result == nil {
			result = []OperatorPersonInfo{}
		}
		return nil
	})
	return result, err
}

// pickDB returns the active transaction from context if present, otherwise the
// service's default DB handle. Mirrors the pattern in queryDevices.
func (s *operatorProvisioningService) pickDB(ctx context.Context) bun.IDB {
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		return tx
	}
	return s.txHandler.DB
}
