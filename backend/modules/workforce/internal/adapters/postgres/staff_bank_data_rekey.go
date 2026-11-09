package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// StaffBankRekeyStore joins the CLI maintenance school transaction, never
// HTTP (#6332). It registers no query logger: ciphertext, plaintext and
// errors are no diagnostics.
type StaffBankRekeyStore struct{}

func NewStaffBankRekeyStore() *StaffBankRekeyStore { return &StaffBankRekeyStore{} }

type staffBankRekeyRow struct {
	ID                         int64   `bun:"id"`
	StaffID                    int64   `bun:"staff_id"`
	IBAN                       *string `bun:"iban"`
	TaxID                      *string `bun:"tax_id"`
	SocialSecurityNumber       *string `bun:"social_security_number"`
	IBANSealed                 []byte  `bun:"iban_sealed"`
	IBANMasked                 *string `bun:"iban_masked"`
	TaxIDSealed                []byte  `bun:"tax_id_sealed"`
	SocialSecurityNumberSealed []byte  `bun:"social_security_number_sealed"`
}

// RewriteStaffBankData locks one bounded batch of one school, ordered by ID
// and without skipped locks, so the cursor never passes a concurrent writer.
// It changes only the sealed values, the mask and the plaintext columns;
// every business field and timestamp stays put.
func (s *StaffBankRekeyStore) RewriteStaffBankData(ctx context.Context, schoolID, afterID int64, limit int,
	rewrite func(*domain.StaffFinancialData) (bool, error),
) (workforce.BankDataRekeyResult, error) {
	tx, err := maintenanceSchoolTransaction(ctx, schoolID)
	if err != nil {
		return workforce.BankDataRekeyResult{}, errors.New("bank data rekey requires a privileged maintenance school transaction")
	}
	result, err := rewriteStaffBankRows(ctx, tx, schoolID, afterID, limit, rewrite)
	if err != nil {
		tenant.MarkRollback(ctx)
		return workforce.BankDataRekeyResult{}, errors.New("bank data rekey batch failed; transaction must roll back")
	}
	return result, nil
}

func rewriteStaffBankRows(ctx context.Context, tx bun.IDB, schoolID, afterID int64, limit int,
	rewrite func(*domain.StaffFinancialData) (bool, error),
) (workforce.BankDataRekeyResult, error) {
	var rows []staffBankRekeyRow
	// The school filter stays explicit: the maintenance login bypasses RLS.
	if err := tx.NewRaw(`SELECT id, staff_id, iban, tax_id, social_security_number,
       iban_sealed, iban_masked, tax_id_sealed, social_security_number_sealed
   FROM users.staff_financial_data WHERE tenant_id = ? AND id > ? ORDER BY id LIMIT ? FOR UPDATE`,
		schoolID, afterID, limit).Scan(ctx, &rows); err != nil {
		return workforce.BankDataRekeyResult{}, err
	}
	result := workforce.BankDataRekeyResult{}
	for _, row := range rows {
		value := &domain.StaffFinancialData{ID: row.ID, TenantID: schoolID, StaffID: row.StaffID,
			IBAN: row.IBAN, TaxID: row.TaxID, SocialSecurityNumber: row.SocialSecurityNumber,
			IBANSealed: row.IBANSealed, IBANMasked: row.IBANMasked, TaxIDSealed: row.TaxIDSealed,
			SocialSecurityNumberSealed: row.SocialSecurityNumberSealed}
		changed, err := rewrite(value)
		if err != nil {
			return result, err
		}
		result.Scanned++
		result.LastID = row.ID
		if !changed {
			continue
		}
		if _, err := tx.NewRaw(`UPDATE users.staff_financial_data
   SET iban_sealed = ?, iban_masked = ?, tax_id_sealed = ?, social_security_number_sealed = ?,
       iban = NULL, tax_id = NULL, social_security_number = NULL
   WHERE tenant_id = ? AND id = ?`, value.IBANSealed, value.IBANMasked, value.TaxIDSealed, value.SocialSecurityNumberSealed,
			schoolID, row.ID).Exec(ctx); err != nil {
			return result, err
		}
		result.Changed++
	}
	return result, nil
}

// maintenanceSchoolTransaction requires the school transaction of exactly
// this school and drops its request role back to the maintenance login. Only
// a superuser login proceeds: an HTTP login never gains the bypass. Role and
// lock timeout die with the transaction.
func maintenanceSchoolTransaction(ctx context.Context, schoolID int64) (bun.IDB, error) {
	if schoolID <= 0 || tenant.FromContext(ctx) != schoolID {
		return nil, errors.New("workforce postgres: the school transaction of this school is required")
	}
	transaction, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return nil, errors.New("workforce postgres: transaction is required")
	}
	tx, ok := transaction.(bun.Tx)
	if !ok {
		return nil, fmt.Errorf("workforce postgres: unsupported transaction %T", transaction)
	}
	if _, err := tx.ExecContext(ctx, "RESET ROLE; SET LOCAL lock_timeout = '5s'"); err != nil {
		tenant.MarkRollback(ctx)
		return nil, err
	}
	var superuser string
	if err := tx.NewRaw("SELECT current_setting('is_superuser')").Scan(ctx, &superuser); err != nil || superuser != "on" {
		tenant.MarkRollback(ctx)
		return nil, errors.New("workforce postgres: maintenance login required")
	}
	return tx, nil
}
