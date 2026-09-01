package tenant_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/tenant"
)

func TestTenantID_RejectsNonPositiveValues(t *testing.T) {
	t.Parallel()

	for _, raw := range []int64{-1, 0} {
		if _, err := tenant.NewTenantID(raw); err == nil {
			t.Errorf("NewTenantID(%d) should fail", raw)
		}
	}
}

func TestTenantID_RoundTrip(t *testing.T) {
	t.Parallel()

	id, err := tenant.NewTenantID(42)
	if err != nil {
		t.Fatalf("NewTenantID(42) error = %v", err)
	}
	if got := id.Int64(); got != 42 {
		t.Fatalf("Int64() = %d, want 42", got)
	}
}
