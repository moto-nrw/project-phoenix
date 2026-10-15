package domain

import "time"

// The fixed retention windows of #4046.
const (
	// StudentDocumentCleanupRetentionWindow: a finished file cleanup of a
	// child document keeps only the stored file name and goes 30 days later.
	StudentDocumentCleanupRetentionWindow = 30 * 24 * time.Hour
	// WithdrawalCompletionRetentionWindow: a resolved or obsolete
	// care-withdrawal completion goes 365 days after it was finished.
	WithdrawalCompletionRetentionWindow = 365 * 24 * time.Hour
)
