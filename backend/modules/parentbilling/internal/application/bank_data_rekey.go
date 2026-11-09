package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

type BankDataRekeyStore interface {
	RewriteBankData(context.Context, int64, string, int64, int, func(string, []byte, []byte) ([]byte, []byte, error)) (parentbilling.BankDataRekeyResult, error)
}

type BankDataRekey struct {
	store  BankDataRekeyStore
	cipher parentbilling.BankDataRewrapper
}

func NewBankDataRekey(store BankDataRekeyStore, cipher parentbilling.BankDataRewrapper) *BankDataRekey {
	if store == nil || cipher == nil {
		panic("bank data rekey requires store and cipher")
	}
	return &BankDataRekey{store: store, cipher: cipher}
}

func (r *BankDataRekey) Batch(ctx context.Context, organizationID int64, source string, afterID int64, limit int) (parentbilling.BankDataRekeyResult, error) {
	if (source != "mandates" && source != "debits" && source != "consents" && source != "debit_files") || organizationID <= 0 || afterID < 0 || limit < 1 || limit > 1000 {
		return parentbilling.BankDataRekeyResult{}, errors.New("bank data rekey requires organization, nonnegative cursor and batch size 1..1000")
	}
	if source == "debit_files" {
		// One sealed value per row: the stored PAIN.008 file (ADR 0187).
		return r.store.RewriteBankData(ctx, organizationID, source, afterID, limit, func(messageID string, file, _ []byte) ([]byte, []byte, error) {
			rewrapped, err := r.cipher.Rewrap(file, debitFileContext(organizationID, messageID))
			if err != nil {
				return nil, nil, errors.New("bank data rekey: sealed SEPA debit file does not open")
			}
			return rewrapped, nil, nil
		})
	}
	return r.store.RewriteBankData(ctx, organizationID, source, afterID, limit, func(reference string, iban, holder []byte) ([]byte, []byte, error) {
		if source == "consents" {
			text, err := r.cipher.Rewrap(iban, consentTextContext(organizationID, reference))
			if err != nil {
				return nil, nil, errors.New("bank data rekey: sealed consent text does not open")
			}
			return text, nil, nil
		}
		newIBAN, err := r.cipher.Rewrap(iban, ibanContext(organizationID, reference))
		if err != nil {
			return nil, nil, errors.New("bank data rekey: sealed IBAN does not open")
		}
		newHolder, err := r.cipher.Rewrap(holder, holderContext(organizationID, reference))
		if err != nil {
			return nil, nil, errors.New("bank data rekey: sealed account holder does not open")
		}
		return newIBAN, newHolder, nil
	})
}
