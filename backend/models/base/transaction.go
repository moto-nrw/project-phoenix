package base

import (
	"context"

	"github.com/uptrace/bun"
)

// txKey is the context key for storing a transaction
type txKey struct{}

// TransactionalService defines an interface for services that support transactions
type TransactionalService interface {
	// WithTx returns a new instance of the service that uses the provided transaction
	WithTx(tx bun.Tx) interface{}
}

// TransactionalRepository defines an interface for repositories that support transactions
type TransactionalRepository interface {
	// WithTx returns a new instance of the repository that uses the provided transaction
	WithTx(tx bun.Tx) interface{}
}

// ContextWithTx adds a transaction to a context
func ContextWithTx(ctx context.Context, tx *bun.Tx) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

// TxFromContext extracts a transaction from context if present
func TxFromContext(ctx context.Context) (*bun.Tx, bool) {
	tx, ok := ctx.Value(txKey{}).(bun.Tx)
	if !ok {
		return nil, false
	}
	return &tx, true
}

// TxHandler provides common transaction handling functionality for services
type TxHandler struct {
	DB *bun.DB
	Tx *bun.Tx
}

// NewTxHandler creates a new transaction handler
func NewTxHandler(db *bun.DB) *TxHandler {
	return &TxHandler{
		DB: db,
	}
}

// WithTx returns a new transaction handler with the specified transaction
func (h *TxHandler) WithTx(tx bun.Tx) *TxHandler {
	return &TxHandler{
		DB: h.DB,
		Tx: &tx,
	}
}

// GetTx returns the current transaction or creates a new one
// Returns the transaction, a boolean indicating if a new transaction was created, and any error
func (h *TxHandler) GetTx(ctx context.Context) (bun.Tx, bool, error) {
	// If we already have a transaction, use it
	if h.Tx != nil {
		return *h.Tx, false, nil
	}

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

	// If we created a new transaction, we need to handle commit/rollback
	if isNew {
		defer func() { _ = tx.Rollback() }()
	}

	// Add transaction to context
	txCtx := ContextWithTx(ctx, &tx)

	// Execute the function with transaction context
	if err := fn(txCtx, tx); err != nil {
		return err
	}

	// If we created a new transaction, commit it
	if isNew {
		return tx.Commit()
	}

	return nil
}
