package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
	"github.com/uptrace/bun/dialect/pgdialect"
)

// BillingStore keeps the billing key day and the captured counts of the
// operator billing report (#2791) in platform.billing_settings and
// platform.billing_key_date_counts. Every statement is a compile-time
// constant so the architecture evaluator can read which tables it touches.
type BillingStore struct{ database Database }

// NewBillingStore builds the store over the ambient transaction runtime.
func NewBillingStore(database Database) *BillingStore {
	if database == nil {
		panic("organization tenancy postgres: database runtime is required")
	}
	return &BillingStore{database: database}
}

const selectBillingSettingsSQL = `SELECT key_day, updated_at, updated_by_operator_id
FROM platform.billing_settings WHERE id = 1`

const updateBillingKeyDaySQL = `UPDATE platform.billing_settings
SET key_day = ?, updated_at = CURRENT_TIMESTAMP, updated_by_operator_id = ?
WHERE id = 1
RETURNING key_day, updated_at, updated_by_operator_id`

// selectSchoolsMissingBillingPeriodSQL takes the key date twice: once to
// skip schools created after it, once for its month. A school created on the
// key date itself counts; creation is compared on the Berlin calendar.
const selectSchoolsMissingBillingPeriodSQL = `SELECT school.id, school.name, organization.name AS organization_name
FROM platform.schools AS school
JOIN platform.organizations AS organization ON organization.id = school.organization_id
WHERE school.deleted_at IS NULL
  AND (school.created_at AT TIME ZONE 'Europe/Berlin')::date <= ?::date
  AND NOT EXISTS (
    SELECT 1 FROM platform.billing_key_date_counts AS captured
    WHERE captured.school_id = school.id
      AND captured.period = date_trunc('month', ?::date)::date
  )
ORDER BY school.id`

// insertBillingKeyDateCountsSQL writes every school of one capture in one
// statement: one array per column, zipped by unnest.
const insertBillingKeyDateCountsSQL = `INSERT INTO platform.billing_key_date_counts
  (school_id, period, key_date, school_name, organization_name, active_students, active_terminals)
SELECT captured.school_id, captured.period::date, captured.key_date::date,
  captured.school_name, captured.organization_name, captured.active_students, captured.active_terminals
FROM unnest(?::bigint[], ?::text[], ?::text[], ?::text[], ?::text[], ?::integer[], ?::integer[])
  AS captured(school_id, period, key_date, school_name, organization_name, active_students, active_terminals)
ON CONFLICT (school_id, period) DO NOTHING`

const selectBillingKeyDateCountsSQL = `SELECT school_id, school_name, organization_name,
  period::text AS period, key_date::text AS key_date,
  active_students, active_terminals, recorded_at
FROM platform.billing_key_date_counts
ORDER BY period DESC, organization_name, school_name, school_id`

type billingSettingsRow struct {
	KeyDay              int       `bun:"key_day"`
	UpdatedAt           time.Time `bun:"updated_at"`
	UpdatedByOperatorID *int64    `bun:"updated_by_operator_id"`
}

func (r billingSettingsRow) toDomain() domain.BillingSettings {
	return domain.BillingSettings{KeyDay: r.KeyDay, UpdatedAt: r.UpdatedAt, UpdatedByOperatorID: r.UpdatedByOperatorID}
}

// BillingSettings reads the key day. The migration seeds its one row.
func (s *BillingStore) BillingSettings(ctx context.Context) (domain.BillingSettings, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.BillingSettings{}, err
	}
	var row billingSettingsRow
	if err := db.NewRaw(selectBillingSettingsSQL).Scan(ctx, &row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.BillingSettings{}, errors.New("organization tenancy postgres: billing settings row is missing")
		}
		return domain.BillingSettings{}, fmt.Errorf("organization tenancy postgres: read billing settings: %w", err)
	}
	return row.toDomain(), nil
}

// UpdateBillingKeyDay changes the key day and records who changed it.
func (s *BillingStore) UpdateBillingKeyDay(ctx context.Context, keyDay int, operatorID int64) (domain.BillingSettings, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.BillingSettings{}, err
	}
	var row billingSettingsRow
	if err := db.NewRaw(updateBillingKeyDaySQL, keyDay, operatorID).Scan(ctx, &row); err != nil {
		return domain.BillingSettings{}, fmt.Errorf("organization tenancy postgres: update billing key day: %w", err)
	}
	return row.toDomain(), nil
}

// SchoolsMissingBillingPeriod lists the schools still without a row for the
// month of keyDate.
func (s *BillingStore) SchoolsMissingBillingPeriod(ctx context.Context, keyDate string) ([]domain.BillingSchool, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		ID               int64  `bun:"id"`
		Name             string `bun:"name"`
		OrganizationName string `bun:"organization_name"`
	}
	if err := db.NewRaw(selectSchoolsMissingBillingPeriodSQL, keyDate, keyDate).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("organization tenancy postgres: list schools missing billing period: %w", err)
	}
	schools := make([]domain.BillingSchool, 0, len(rows))
	for _, row := range rows {
		schools = append(schools, domain.BillingSchool{ID: row.ID, Name: row.Name, OrganizationName: row.OrganizationName})
	}
	return schools, nil
}

// InsertBillingKeyDateCounts writes one row per school and month. A row that
// exists already stays as it is: captured figures are never overwritten.
func (s *BillingStore) InsertBillingKeyDateCounts(ctx context.Context, counts []domain.BillingKeyDateCount) (int, error) {
	if len(counts) == 0 {
		return 0, nil
	}
	db, err := s.database(ctx)
	if err != nil {
		return 0, err
	}
	schoolIDs := make([]int64, 0, len(counts))
	periods := make([]string, 0, len(counts))
	keyDates := make([]string, 0, len(counts))
	schoolNames := make([]string, 0, len(counts))
	organizationNames := make([]string, 0, len(counts))
	students := make([]int, 0, len(counts))
	terminals := make([]int, 0, len(counts))
	for _, count := range counts {
		schoolIDs = append(schoolIDs, count.SchoolID)
		periods = append(periods, count.Period)
		keyDates = append(keyDates, count.KeyDate)
		schoolNames = append(schoolNames, count.SchoolName)
		organizationNames = append(organizationNames, count.OrganizationName)
		students = append(students, count.ActiveStudents)
		terminals = append(terminals, count.ActiveTerminals)
	}
	result, err := db.NewRaw(insertBillingKeyDateCountsSQL,
		pgdialect.Array(schoolIDs), pgdialect.Array(periods), pgdialect.Array(keyDates),
		pgdialect.Array(schoolNames), pgdialect.Array(organizationNames),
		pgdialect.Array(students), pgdialect.Array(terminals),
	).Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("organization tenancy postgres: insert billing key-date counts: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("organization tenancy postgres: insert billing key-date counts: %w", err)
	}
	return int(affected), nil
}

// ListBillingKeyDateCounts reads every captured row, newest month first.
func (s *BillingStore) ListBillingKeyDateCounts(ctx context.Context) ([]domain.BillingKeyDateCount, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		SchoolID         int64     `bun:"school_id"`
		SchoolName       string    `bun:"school_name"`
		OrganizationName string    `bun:"organization_name"`
		Period           string    `bun:"period"`
		KeyDate          string    `bun:"key_date"`
		ActiveStudents   int       `bun:"active_students"`
		ActiveTerminals  int       `bun:"active_terminals"`
		RecordedAt       time.Time `bun:"recorded_at"`
	}
	if err := db.NewRaw(selectBillingKeyDateCountsSQL).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("organization tenancy postgres: list billing key-date counts: %w", err)
	}
	counts := make([]domain.BillingKeyDateCount, 0, len(rows))
	for _, row := range rows {
		counts = append(counts, domain.BillingKeyDateCount{
			SchoolID: row.SchoolID, SchoolName: row.SchoolName, OrganizationName: row.OrganizationName,
			Period: row.Period, KeyDate: row.KeyDate,
			ActiveStudents: row.ActiveStudents, ActiveTerminals: row.ActiveTerminals, RecordedAt: row.RecordedAt,
		})
	}
	return counts, nil
}
