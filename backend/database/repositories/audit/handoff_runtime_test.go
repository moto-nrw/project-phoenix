package audit

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// handoffTenant is the tenant the handoffs below report; no row is read.
const handoffTenant int64 = 4013

// newFakeRoot is a fake root database: the runtime only hands it back and no
// query runs against it.
func newFakeRoot() *bun.DB { return &bun.DB{} }

func fixedHandoff(transaction any, ok bool) Handoff {
	return func(context.Context) (any, bool, int64) { return transaction, ok, handoffTenant }
}

func TestHandoffRuntimeResolvesTheCallersTransaction(t *testing.T) {
	t.Parallel()

	fallback := newFakeRoot()
	t.Run("transaction value", func(t *testing.T) {
		t.Parallel()
		db, tenantID := NewHandoffRuntime(fixedHandoff(bun.Tx{}, true), fallback)(context.Background())
		require.Equal(t, bun.Tx{}, db)
		require.Equal(t, handoffTenant, tenantID)
	})
	t.Run("transaction pointer", func(t *testing.T) {
		t.Parallel()
		db, tenantID := NewHandoffRuntime(fixedHandoff(&bun.Tx{}, true), fallback)(context.Background())
		require.Equal(t, &bun.Tx{}, db)
		require.Equal(t, handoffTenant, tenantID)
	})
}

func TestHandoffRuntimeOutsideATransaction(t *testing.T) {
	t.Parallel()

	t.Run("falls back to the root database", func(t *testing.T) {
		t.Parallel()
		fallback := newFakeRoot()
		db, tenantID := NewHandoffRuntime(fixedHandoff(nil, false), fallback)(context.Background())
		require.Same(t, fallback, db)
		require.Equal(t, handoffTenant, tenantID)
	})
	t.Run("without a fallback resolves no database", func(t *testing.T) {
		t.Parallel()
		db, tenantID := NewHandoffRuntime(fixedHandoff(nil, false), nil)(context.Background())
		require.Nil(t, db)
		require.Equal(t, handoffTenant, tenantID)
	})
}

func TestHandoffRuntimeRejectsAnUnknownTransaction(t *testing.T) {
	t.Parallel()

	for name, transaction := range map[string]any{"nil pointer": (*bun.Tx)(nil), "foreign type": "tx"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			runtime := NewHandoffRuntime(fixedHandoff(transaction, true), newFakeRoot())
			require.Panics(t, func() { runtime(context.Background()) })
		})
	}
}
