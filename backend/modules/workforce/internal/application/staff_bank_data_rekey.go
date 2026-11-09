package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

// StaffBankRekeyStore locks one bounded batch of bank and tax rows of a
// school and writes back what rewrite reports as changed: only the sealed
// values and the mask, clearing the plaintext columns.
type StaffBankRekeyStore interface {
	RewriteStaffBankData(ctx context.Context, schoolID, afterID int64, limit int,
		rewrite func(*domain.StaffFinancialData) (bool, error)) (workforce.BankDataRekeyResult, error)
}

// StaffBankDataRekey is the maintenance backfill and rekey of the staff bank
// and tax values (#6332, ADR 0183): plaintext rows are sealed and masked,
// sealed rows authenticated and re-sealed on the active bank key.
type StaffBankDataRekey struct {
	store  StaffBankRekeyStore
	cipher workforce.BankDataRekeyCipher
}

func NewStaffBankDataRekey(store StaffBankRekeyStore, cipher workforce.BankDataRekeyCipher) *StaffBankDataRekey {
	if store == nil || cipher == nil {
		panic("staff bank data rekey requires store and cipher")
	}
	return &StaffBankDataRekey{store: store, cipher: cipher}
}

func (r *StaffBankDataRekey) Batch(ctx context.Context, schoolID, afterID int64, limit int) (workforce.BankDataRekeyResult, error) {
	if schoolID <= 0 || afterID < 0 || limit < 1 || limit > 1000 {
		return workforce.BankDataRekeyResult{}, errors.New("bank data rekey requires school, nonnegative cursor and batch size 1..1000")
	}
	return r.store.RewriteStaffBankData(ctx, schoolID, afterID, limit, func(row *domain.StaffFinancialData) (bool, error) {
		changed := false
		for _, field := range []struct {
			name   string
			sealed *[]byte
			plain  **string
		}{
			{staffIBANField, &row.IBANSealed, &row.IBAN},
			{staffTaxIDField, &row.TaxIDSealed, &row.TaxID},
			{staffSocialField, &row.SocialSecurityNumberSealed, &row.SocialSecurityNumber},
		} {
			next, fieldChanged, err := rekeyStaffValue(r.cipher, staffBankContext(field.name, schoolID, row.StaffID), *field.sealed, *field.plain)
			if err != nil {
				return false, errors.New("bank data rekey: a sealed staff value does not open")
			}
			if field.name == staffIBANField && len(*field.sealed) == 0 && len(next) > 0 {
				row.IBANMasked = domain.MaskIBAN(row.IBAN)
			}
			*field.sealed, *field.plain = next, nil
			changed = changed || fieldChanged
		}
		return changed, nil
	})
}
