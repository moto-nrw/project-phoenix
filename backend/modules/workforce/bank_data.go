package workforce

import "context"

// BankDataCipher seals and opens the bank and tax values of a staff member
// bound to their row (#6332). The root binds Security Runtime's independent
// bank ring (BANK_DATA_KEYS, ADR 0183); the marker keeps a health-only cipher
// from satisfying the port.
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

// BankDataRekeyResult contains counters only, never bank or tax data.
type BankDataRekeyResult struct {
	Scanned, Changed int
	LastID           int64
}

// StaffBankDataRekey seals remaining plaintext bank and tax values of staff
// members and re-seals sealed ones on the active bank key, one bounded batch
// of one school at a time. The CLI root opens the school transaction per
// batch; never bind it to HTTP. Resume by replaying from zero.
type StaffBankDataRekey interface {
	Batch(ctx context.Context, schoolID, afterID int64, limit int) (BankDataRekeyResult, error)
}
