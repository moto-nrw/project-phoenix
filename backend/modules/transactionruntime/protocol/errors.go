// Package protocol defines persistence-neutral transaction outcomes.
package protocol

// TransactionStartError marks a failure before the transaction starts.
type TransactionStartError struct{ Err error }

func (e *TransactionStartError) Error() string {
	return "unit of work: start transaction: " + e.Err.Error()
}
func (e *TransactionStartError) Unwrap() error          { return e.Err }
func (e *TransactionStartError) TransactionNotStarted() {}

// CommitOutcomeUnknownError marks a commit whose durable outcome is unknown.
type CommitOutcomeUnknownError struct{ Err error }

func (e *CommitOutcomeUnknownError) Error() string {
	return "unit of work: commit outcome unknown: " + e.Err.Error()
}
func (e *CommitOutcomeUnknownError) Unwrap() error         { return e.Err }
func (e *CommitOutcomeUnknownError) CommitOutcomeUnknown() {}

// LockNotAvailableError marks a lock wait that lock_timeout ended (55P03);
// its text is the database error's.
type LockNotAvailableError struct{ error }

// NewLockNotAvailableError marks err as a lock wait lock_timeout ended.
func NewLockNotAvailableError(err error) error { return LockNotAvailableError{err} }

func (e LockNotAvailableError) Unwrap() error   { return e.error }
func (LockNotAvailableError) LockWaitExceeded() {}
