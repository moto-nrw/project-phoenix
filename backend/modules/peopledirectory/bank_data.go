package peopledirectory

import (
	"context"
	"encoding/json"
)

// BankDataCipher seals and opens the bank values of a guardian bound to their
// row (#6332). The root binds Security Runtime's independent bank ring
// (BANK_DATA_KEYS, ADR 0183); the marker keeps a health-only cipher from
// satisfying the port.
type BankDataCipher interface {
	// BankDataRing prevents health-only ciphers from satisfying this port.
	BankDataRing()
	Seal(plaintext, context []byte) ([]byte, error)
	Open(sealed, context []byte) ([]byte, error)
}

// BankDataRekeyCipher additionally re-seals a value on the active bank key;
// only the maintenance command holds it.
type BankDataRekeyCipher interface {
	BankDataCipher
	Rewrap(sealed, context []byte) ([]byte, error)
}

// BankDataRekeyResult contains counters only, never bank data.
type BankDataRekeyResult struct {
	Scanned, Changed int
	LastID           int64
}

// GuardianBankDataRekey seals remaining plaintext bank values of guardians
// and re-seals sealed ones on the active bank key, one bounded batch of one
// school at a time. The CLI root opens the school transaction per batch;
// never bind it to HTTP. Resume by replaying from zero.
type GuardianBankDataRekey interface {
	Batch(ctx context.Context, schoolID, afterID int64, limit int) (BankDataRekeyResult, error)
}

// GuardianBankExportFields opens the sealed IBAN and account holder of an
// exported users.guardian_financial_data row for the return export of a
// school (#6412). Only the administrative export CLI binds it, never HTTP.
type GuardianBankExportFields interface {
	OpenExportField(table, column string, row map[string]json.RawMessage, sealed []byte) (string, error)
}
