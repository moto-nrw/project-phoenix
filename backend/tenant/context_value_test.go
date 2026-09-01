package tenant_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/tenant"
)

func TestTenantContext_RoundTripsValidatedTenantID(t *testing.T) {
	t.Parallel()

	id, err := tenant.NewTenantID(42)
	if err != nil {
		t.Fatalf("NewTenantID(42) error = %v", err)
	}
	ctx := tenant.WithTenant(context.Background(), id)

	got, err := tenant.TenantFromContext(ctx)
	if err != nil {
		t.Fatalf("TenantFromContext() error = %v", err)
	}
	if got != id {
		t.Fatalf("TenantFromContext() = %v, want %v", got, id)
	}
}

func TestTenantContext_RejectsMissingTenantID(t *testing.T) {
	t.Parallel()

	_, err := tenant.TenantFromContext(context.Background())
	if !errors.Is(err, tenant.ErrTenantRequired) {
		t.Fatalf("TenantFromContext() error = %v, want ErrTenantRequired", err)
	}
}

func TestWithTenantIDRejectsNegativeLegacyValue(t *testing.T) {
	t.Parallel()

	defer func() {
		if recover() == nil {
			t.Fatal("WithTenantID(-1) did not panic")
		}
	}()
	tenant.WithTenantID(context.Background(), -1)
}
