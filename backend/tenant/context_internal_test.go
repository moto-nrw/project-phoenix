package tenant

import (
	"context"
	"testing"
)

func TestWithAdminTxFlag_KeepsTenantID(t *testing.T) {
	ctx := withAdminTxFlag(WithTenantID(context.Background(), 1))
	if !IsAdminTx(ctx) {
		t.Fatal("expected admin tx flag")
	}
	if got := FromContext(ctx); got != 1 {
		t.Fatalf("FromContext() = %d, want 1", got)
	}
}
