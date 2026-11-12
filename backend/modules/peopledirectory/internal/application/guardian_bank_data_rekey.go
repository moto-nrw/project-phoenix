package application

import (
	"bytes"
	"context"
	"errors"

	peopleValues "github.com/moto-ogs/project-phoenix/modules/peopledirectory"
)

// GuardianBankRekeyStore locks one bounded batch of bank rows of a school
// and writes back what rewrite reports as changed: only the sealed values
// and the mask, clearing the plaintext columns.
type GuardianBankRekeyStore interface {
	RewriteGuardianBankData(ctx context.Context, schoolID, afterID int64, limit int,
		rewrite func(*peopleValues.GuardianPayment) (bool, error)) (peopleValues.BankDataRekeyResult, error)
}

// GuardianBankDataRekey is the maintenance backfill and rekey of the guardian
// bank values (#6332, ADR 0183): plaintext rows are sealed and masked,
// sealed rows authenticated and re-sealed on the active bank key.
type GuardianBankDataRekey struct {
	store  GuardianBankRekeyStore
	cipher peopleValues.BankDataRekeyCipher
}

func NewGuardianBankDataRekey(store GuardianBankRekeyStore, cipher peopleValues.BankDataRekeyCipher) *GuardianBankDataRekey {
	if store == nil || cipher == nil {
		panic("guardian bank data rekey requires store and cipher")
	}
	return &GuardianBankDataRekey{store: store, cipher: cipher}
}

var errBankRekeyArguments = errors.New("bank data rekey requires school, nonnegative cursor and batch size 1..1000")

func (r *GuardianBankDataRekey) Batch(ctx context.Context, schoolID, afterID int64, limit int) (peopleValues.BankDataRekeyResult, error) {
	if schoolID <= 0 || afterID < 0 || limit < 1 || limit > 1000 {
		return peopleValues.BankDataRekeyResult{}, errBankRekeyArguments
	}
	return r.store.RewriteGuardianBankData(ctx, schoolID, afterID, limit, func(row *peopleValues.GuardianPayment) (bool, error) {
		iban, ibanChanged, err := rekeyBankValue(r.cipher, guardianBankContext(guardianIBANField, schoolID, row.GuardianProfileID), row.IBANSealed, row.IBAN)
		if err != nil {
			return false, errors.New("bank data rekey: sealed IBAN does not open")
		}
		holder, holderChanged, err := rekeyBankValue(r.cipher, guardianBankContext(guardianHolderField, schoolID, row.GuardianProfileID), row.AccountHolderSealed, row.AccountHolder)
		if err != nil {
			return false, errors.New("bank data rekey: sealed account holder does not open")
		}
		if len(row.IBANSealed) == 0 && len(iban) > 0 {
			row.IBANMasked = maskTailPtr(row.IBAN, 4)
		}
		row.IBANSealed, row.AccountHolderSealed = iban, holder
		row.IBAN, row.AccountHolder = nil, nil
		return ibanChanged || holderChanged, nil
	})
}

// rekeyBankValue authenticates and re-seals a sealed value, or seals the
// plaintext of a row the backfill has not reached. A sealed value wins over
// a leftover plaintext, which is cleared. Current values keep their bytes.
func rekeyBankValue(cipher peopleValues.BankDataRekeyCipher, aad, sealed []byte, plain *string) ([]byte, bool, error) {
	if len(sealed) > 0 {
		next, err := cipher.Rewrap(sealed, aad)
		if err != nil {
			return nil, false, err
		}
		return next, plain != nil || !bytes.Equal(next, sealed), nil
	}
	if plain == nil {
		return nil, false, nil
	}
	if *plain == "" {
		return nil, true, nil
	}
	next, err := cipher.Seal([]byte(*plain), aad)
	if err != nil {
		return nil, false, err
	}
	return next, true, nil
}
