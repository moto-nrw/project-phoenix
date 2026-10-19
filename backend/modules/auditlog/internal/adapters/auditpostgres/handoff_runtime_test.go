package audit

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// handoffTenant is the tenant the handoffs below report; no row is read.
const handoffTenant int64 = 4013

func fixedHandoff(transaction any, ok bool) Handoff {
	return func(context.Context) (any, bool, int64) { return transaction, ok, handoffTenant }
}

func TestHandoffRuntimeResolvesTheCallersTransaction(t *testing.T) {
	t.Parallel()

	t.Run("transaction value", func(t *testing.T) {
		t.Parallel()
		db, tenantID := NewHandoffRuntime(fixedHandoff(bun.Tx{}, true))(context.Background())
		require.Equal(t, bun.Tx{}, db)
		require.Equal(t, handoffTenant, tenantID)
	})
	t.Run("transaction pointer", func(t *testing.T) {
		t.Parallel()
		db, tenantID := NewHandoffRuntime(fixedHandoff(&bun.Tx{}, true))(context.Background())
		require.Equal(t, &bun.Tx{}, db)
		require.Equal(t, handoffTenant, tenantID)
	})
}

func TestHandoffRuntimeOutsideATransaction(t *testing.T) {
	t.Parallel()

	t.Run("resolves no database", func(t *testing.T) {
		t.Parallel()
		db, tenantID := NewHandoffRuntime(fixedHandoff(nil, false))(context.Background())
		require.Nil(t, db)
		require.Equal(t, handoffTenant, tenantID)
	})
	t.Run("without a fallback resolves no database", func(t *testing.T) {
		t.Parallel()
		db, tenantID := NewHandoffRuntime(fixedHandoff(nil, false))(context.Background())
		require.Nil(t, db)
		require.Equal(t, handoffTenant, tenantID)
	})
}

func TestHandoffRuntimeRejectsAnUnknownTransaction(t *testing.T) {
	t.Parallel()

	for name, transaction := range map[string]any{"nil pointer": (*bun.Tx)(nil), "foreign type": "tx"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			runtime := NewHandoffRuntime(fixedHandoff(transaction, true))
			db, _ := runtime(context.Background())
			require.Nil(t, db)
		})
	}
}
