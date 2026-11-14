package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-ogs/project-phoenix/modules/workforce/internal/domain"
	"github.com/moto-ogs/project-phoenix/tenant"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

// The school's and the moto team's side of the Personalstamm (#4705).

// schoolStaffDB returns the tenant transaction of one school.
func schoolStaffDB(ctx context.Context) (bun.IDB, int64, error) {
	tx, schoolID, err := tenant.CurrentTenantTransaction[bun.Tx](ctx)
	if err != nil || schoolID <= 0 || tenant.TraegerTransactionFromContext(ctx) != 0 {
		return nil, 0, errors.New("workforce postgres: the tenant transaction of one school is required")
	}
	return tx, schoolID, nil
}

// adminStaffDB returns the administrative transaction of the moto team.
func adminStaffDB(ctx context.Context) (bun.IDB, error) {
	if !tenant.IsAdminTx(ctx) {
		return nil, errors.New("workforce postgres: the administrative transaction is required")
	}
	return tenant.CurrentTransaction[bun.Tx](ctx)
}

// KeptStaffIDs returns which of the school's employments a record of its
// Träger holds. RLS shows a school only its own links.
func (s *TraegerStaffStore) KeptStaffIDs(ctx context.Context, staffIDs []int64) ([]int64, error) {
	db, schoolID, err := schoolStaffDB(ctx)
	if err != nil {
		return nil, err
	}
	var kept []int64
	if err := db.NewRaw(`SELECT membership_id FROM workforce.staff_record_links WHERE school_id = ? AND membership_id = ANY(?)`,
		schoolID, pgdialect.Array(staffIDs)).Scan(ctx, &kept); err != nil {
		return nil, fmt.Errorf("workforce postgres: read kept staff: %w", err)
	}
	return kept, nil
}

// SchoolFinancialRows reads the bank and tax rows of the school's
// employments, sealed, in one statement.
func (s *TraegerStaffStore) SchoolFinancialRows(ctx context.Context, staffIDs []int64) ([]domain.StaffFinancialData, error) {
	db, schoolID, err := schoolStaffDB(ctx)
	if err != nil {
		return nil, err
	}
	var rows []staffFinancialDataRow
	if err := db.NewSelect().Model(&rows).ModelTableExpr(tableStaffFinancialData+` AS "staff_financial_data"`).
		Where(`"staff_financial_data".tenant_id = ? AND "staff_financial_data".staff_id = ANY(?)`, schoolID, pgdialect.Array(staffIDs)).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("workforce postgres: read school bank rows: %w", err)
	}
	result := make([]domain.StaffFinancialData, 0, len(rows))
	for _, row := range rows {
		result = append(result, staffFinancialDataToDomain(row))
	}
	return result, nil
}

// DeleteSchoolFinancialRow removes the school's bank and tax row of one
// employment once its Träger keeps the values (decision E1 of 13.11.2026).
func (s *TraegerStaffStore) DeleteSchoolFinancialRow(ctx context.Context, staffID int64) error {
	db, schoolID, err := schoolStaffDB(ctx)
	if err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM users.staff_financial_data WHERE tenant_id = ? AND staff_id = ?`, schoolID, staffID); err != nil {
		return fmt.Errorf("workforce postgres: delete school bank row: %w", err)
	}
	return nil
}

// InsertSchoolFinancialRow gives the school its bank and tax row back when
// its Träger dissolves a record. A row the school has meanwhile kept stays.
func (s *TraegerStaffStore) InsertSchoolFinancialRow(ctx context.Context, data domain.StaffFinancialData) error {
	db, schoolID, err := schoolStaffDB(ctx)
	if err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO users.staff_financial_data (tenant_id, staff_id, iban_sealed, iban_masked, tax_id_sealed, social_security_number_sealed)
		VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT (staff_id) DO NOTHING`,
		schoolID, data.StaffID, data.IBANSealed, data.IBANMasked, data.TaxIDSealed, data.SocialSecurityNumberSealed); err != nil {
		return fmt.Errorf("workforce postgres: restore school bank row: %w", err)
	}
	return nil
}

// ReleaseOfTraeger returns the release of a Träger, nil without one, in the
// administrative transaction.
func (s *TraegerStaffStore) ReleaseOfTraeger(ctx context.Context, organizationID int64) (*domain.TraegerStaffRelease, error) {
	db, err := adminStaffDB(ctx)
	if err != nil {
		return nil, err
	}
	return findStaffRelease(ctx, db, organizationID)
}

// CountStaffRecords counts the records a Träger keeps, in the
// administrative transaction.
func (s *TraegerStaffStore) CountStaffRecords(ctx context.Context, organizationID int64) (int, error) {
	db, err := adminStaffDB(ctx)
	if err != nil {
		return 0, err
	}
	var records int
	if err := db.NewRaw(`SELECT COUNT(*) FROM workforce.staff_records WHERE organization_id = ?`, organizationID).Scan(ctx, &records); err != nil {
		return 0, fmt.Errorf("workforce postgres: count staff records: %w", err)
	}
	return records, nil
}

// InsertRelease records the release of a Träger in the administrative
// transaction; a second release fails with domain.ErrTraegerStaffReleaseTaken.
func (s *TraegerStaffStore) InsertRelease(ctx context.Context, release domain.TraegerStaffRelease) (domain.TraegerStaffRelease, error) {
	db, err := adminStaffDB(ctx)
	if err != nil {
		return domain.TraegerStaffRelease{}, err
	}
	var rows []staffReleaseRow
	if err := db.NewRaw(`INSERT INTO workforce.staff_releases (organization_id, representation, released_by_operator_id)
		VALUES (?, ?, ?) ON CONFLICT (organization_id) DO NOTHING
		RETURNING organization_id, representation, released_by_operator_id, released_at`,
		release.OrganizationID, release.Representation, release.ReleasedByOperatorID).Scan(ctx, &rows); err != nil {
		return domain.TraegerStaffRelease{}, fmt.Errorf("workforce postgres: insert staff release: %w", err)
	}
	if len(rows) == 0 {
		return domain.TraegerStaffRelease{}, domain.ErrTraegerStaffReleaseTaken
	}
	return rows[0].toDomain(), nil
}

// DeleteRelease withdraws the release in the administrative transaction.
// The records' foreign key refuses it while the Träger keeps records
// (domain.ErrTraegerStaffReleaseInUse); false means there was none.
func (s *TraegerStaffStore) DeleteRelease(ctx context.Context, organizationID int64) (bool, error) {
	db, err := adminStaffDB(ctx)
	if err != nil {
		return false, err
	}
	result, err := db.ExecContext(ctx, `DELETE FROM workforce.staff_releases WHERE organization_id = ?`, organizationID)
	if pgErr, ok := errors.AsType[pgdriver.Error](err); ok && pgErr.Field('C') == "23503" {
		return false, domain.ErrTraegerStaffReleaseInUse
	}
	if err != nil {
		return false, fmt.Errorf("workforce postgres: delete staff release: %w", err)
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}
