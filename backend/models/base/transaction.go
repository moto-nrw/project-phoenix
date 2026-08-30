package base

import (
	"context"
	"errors"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

// txKey is the context key for storing a transaction
type txKey struct{}

// ContextWithTx adds a transaction to a context
func ContextWithTx(ctx context.Context, tx *bun.Tx) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

// ContextWithoutTx masks any transaction stored on ctx while preserving
// cancellation, deadlines, and other context values. Use this when a nested
// operation must commit independently from an ambient request transaction.
func ContextWithoutTx(ctx context.Context) context.Context {
	return context.WithValue(ctx, txKey{}, (*bun.Tx)(nil))
}

// TxFromContext extracts a transaction from context if present.
// ContextWithTx stores *bun.Tx, so the type assertion must match.
func TxFromContext(ctx context.Context) (*bun.Tx, bool) {
	tx, ok := ctx.Value(txKey{}).(*bun.Tx)
	if !ok || tx == nil {
		return nil, false
	}
	return tx, true
}

// TxHandler provides common transaction handling functionality for services
type TxHandler struct {
	DB *bun.DB
}

// NewTxHandler creates a new transaction handler
func NewTxHandler(db *bun.DB) *TxHandler {
	return &TxHandler{
		DB: db,
	}
}

// GetTx returns the current transaction or creates a new one
// Returns the transaction, a boolean indicating if a new transaction was created, and any error
func (h *TxHandler) GetTx(ctx context.Context) (bun.Tx, bool, error) {
	// If there's a transaction in the context, use it
	if tx, ok := TxFromContext(ctx); ok {
		return *tx, false, nil
	}

	// Start a new transaction
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return tx, false, err
	}

	return tx, true, nil
}

// RunInTx executes the provided function within a transaction
// If the transaction handler already has a transaction, it uses that one
// Otherwise, it creates a new transaction and handles commit/rollback
func (h *TxHandler) RunInTx(ctx context.Context, fn func(ctx context.Context, tx bun.Tx) error) error {
	tx, isNew, err := h.GetTx(ctx)
	if err != nil {
		return err
	}

	// Add transaction to context
	txCtx := ContextWithTx(ctx, &tx)

	// Execute the function with transaction context
	if err := fn(txCtx, tx); err != nil {
		if isNew {
			_ = tx.Rollback()
		}
		return err
	}

	// If we created a new transaction, commit it
	if isNew {
		return tx.Commit()
	}

	return nil
}

const defaultTxRetries = 3

// IsRetryableTxError reports whether err is a transient PostgreSQL transaction
// failure that is safe to retry by re-running the whole transaction:
// deadlock_detected (40P01) or serialization_failure (40001). These are not
// data errors — Postgres aborts one victim transaction and expects the caller
// to retry. errors.As unwraps service/repo error wrappers down to the driver
// error.
func IsRetryableTxError(err error) bool {
	var pgErr pgdriver.Error
	if !errors.As(err, &pgErr) {
		return false
	}
	switch pgErr.Field('C') {
	case "40P01", "40001": // deadlock_detected, serialization_failure
		return true
	default:
		return false
	}
}

// RunInTxWithRetry runs a standalone transaction and retries transient
// deadlock and serialization failures. An ambient transaction is never
// replayed because Postgres has already aborted it after such a failure.
func (h *TxHandler) RunInTxWithRetry(ctx context.Context, fn func(context.Context, bun.Tx) error) error {
	if _, ok := TxFromContext(ctx); ok {
		return h.RunInTx(ctx, fn)
	}
	var err error
	for attempt := 0; attempt <= defaultTxRetries; attempt++ {
		err = h.RunInTx(ctx, fn)
		if err == nil || !IsRetryableTxError(err) {
			return err
		}
	}
	return err
}
