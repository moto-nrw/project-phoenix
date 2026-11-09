package parentbilling

import "context"

// BankDataRewrapper authenticates and re-seals a value on the active bank key.
type BankDataRewrapper interface {
	Rewrap(sealed, aad []byte) ([]byte, error)
}

// BankDataRekeyResult contains counters only, never bank data or references.
type BankDataRekeyResult struct {
	Scanned, Changed int
	LastID           int64
}

// BankDataRekey is a privileged, bounded maintenance operation. The CLI root
// opens and commits a Träger transaction per batch, including revoked mandates.
// Resume by replaying from zero.
type BankDataRekey interface {
	Batch(ctx context.Context, organizationID int64, source string, afterID int64, limit int) (BankDataRekeyResult, error)
}
