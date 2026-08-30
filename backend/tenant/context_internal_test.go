package tenant

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/tenanttest"
)

func TestWithAdminTxFlag_KeepsTenantID(t *testing.T) {
	t.Parallel()

	tenantID := tenanttest.NewTenantID()
	ctx := withAdminTxFlag(WithTenantID(context.Background(), tenantID))
	if !IsAdminTx(ctx) {
		t.Fatal("expected admin tx flag")
	}
	if got := FromContext(ctx); got != tenantID {
		t.Fatalf("FromContext() = %d, want %d", got, tenantID)
	}
}
