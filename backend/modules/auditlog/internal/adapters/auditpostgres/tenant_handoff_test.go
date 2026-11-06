package audit

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

type handoffKey struct{ name string }

// The append handoff joins the Träger transaction without a school (#4692);
// the read handoff and a unit of work without school, admin or Träger hand
// no transaction.
func TestAppendHandoffJoinsTheTraegerTransaction(t *testing.T) {
	t.Parallel()
	school := func(ctx context.Context) int64 { v, _ := ctx.Value(handoffKey{"school"}).(int64); return v }
	admin := func(ctx context.Context) bool { v, _ := ctx.Value(handoffKey{"admin"}).(bool); return v }
	traeger := func(ctx context.Context) int64 { v, _ := ctx.Value(handoffKey{"traeger"}).(int64); return v }
	tx := func(ctx context.Context) (any, bool) { v := ctx.Value(handoffKey{"tx"}); return v, v != nil }

	inTraeger := context.WithValue(context.WithValue(t.Context(), handoffKey{"traeger"}, int64(3)), handoffKey{"tx"}, "träger-tx")
	transaction, ok, schoolID := NewAppendHandoff(school, admin, traeger, tx)(inTraeger)
	assert.True(t, ok)
	assert.Equal(t, "träger-tx", transaction)
	assert.Zero(t, schoolID, "the Träger transaction has no school")

	_, ok, _ = NewTenantHandoff(school, admin, tx)(inTraeger)
	assert.False(t, ok, "the read handoff stays with school and admin units")

	outside := context.WithValue(t.Context(), handoffKey{"tx"}, "any-tx")
	_, ok, _ = NewAppendHandoff(school, admin, traeger, tx)(outside)
	assert.False(t, ok, "without school, admin or Träger nothing is handed over")
}
