package exporttransfer

import "context"

// JournalRetention deletes the school's journal entries of transfer attempts
// after the fixed window of the retention list (#4044): an attempt stays
// until the end of the year after it, like the access log.
type JournalRetention interface {
	// DeleteExpiredTransfers reports the entries it deleted and the age of
	// the cutoff in days.
	DeleteExpiredTransfers(ctx context.Context) (deleted, retentionDays int, err error)
}
