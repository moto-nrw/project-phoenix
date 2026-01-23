package base

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

// txKey is the context key for storing a transaction
type txKey struct{}

// ServiceTransactor defines an interface for services that support transactions
// Named following Go single-method interface conventions (method name + er suffix)
type ServiceTransactor interface {
	// WithTx returns a new instance of the service that uses the provided transaction
	WithTx(tx bun.Tx) interface{}
}

// RepoTransactor defines an interface for repositories that support transactions
// Named following Go single-method interface conventions (method name + er suffix)
type RepoTransactor interface {
	// WithTx returns a new instance of the repository that uses the provided transaction
	WithTx(tx bun.Tx) interface{}
}

// Aliases for backward compatibility (deprecated - use ServiceTransactor and RepoTransactor)
type TransactionalService = ServiceTransactor
type TransactionalRepository = RepoTransactor

// ContextWithTx adds a transaction to a context
func ContextWithTx(ctx context.Context, tx *bun.Tx) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

// TxFromContext extracts a transaction from context if present
func TxFromContext(ctx context.Context) (*bun.Tx, bool) {
	tx, ok := ctx.Value(txKey{}).(*bun.Tx)
	if !ok {
		return nil, false
	}
	return tx, true
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
// This function automatically sets app.ogs_id if a tenant context is present.
func (h *TxHandler) RunInTx(ctx context.Context, fn func(ctx context.Context, tx bun.Tx) error) error {
	tx, isNew, err := h.GetTx(ctx)
	if err != nil {
		return err
	}

	// If we created a new transaction, we need to handle commit/rollback
	if isNew {
		defer func() { _ = tx.Rollback() }()

		// Set app.ogs_id for multitenancy RLS
		// Extract OGS ID from context - check tenant context first, then direct value
		ogsID := extractOGSID(ctx)
		if ogsID != "" {
			query := fmt.Sprintf("SET LOCAL app.ogs_id = '%s'", ogsID)
			if _, err := tx.ExecContext(ctx, query); err != nil {
				return fmt.Errorf("failed to set tenant context: %w", err)
			}
		}
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

// ogsIDKey is the context key for storing OGS ID
type ogsIDKey struct{}

// ContextWithOGSID adds an OGS ID to a context
func ContextWithOGSID(ctx context.Context, ogsID string) context.Context {
	return context.WithValue(ctx, ogsIDKey{}, ogsID)
}

// extractOGSID attempts to extract the OGS ID from context
func extractOGSID(ctx context.Context) string {
	if id, ok := ctx.Value(ogsIDKey{}).(string); ok {
		return id
	}
	return ""
}
