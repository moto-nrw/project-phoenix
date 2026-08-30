package auth

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/uptrace/bun"
)

func newMockTenantRuntime(t *testing.T, db *bun.DB) *tenant.UnitOfWork {
	t.Helper()
	runtime := testpkg.TenantRuntime(t, db)
	return &runtime
}

func newTestInvitationService(t *testing.T, config InvitationServiceConfig) InvitationService {
	t.Helper()
	if config.DB != nil {
		service := NewInvitationService(config)
		service.(interface{ SetTenantRuntime(tenant.UnitOfWork) }).SetTenantRuntime(*newMockTenantRuntime(t, config.DB))
		return service
	}
	return NewInvitationService(config)
}
