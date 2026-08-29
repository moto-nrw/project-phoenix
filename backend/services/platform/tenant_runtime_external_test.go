package platform_test

import (
	"testing"

	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func newTestOperatorProvisioningService(t testing.TB, cfg platformSvc.OperatorProvisioningServiceConfig) platformSvc.OperatorProvisioningService {
	t.Helper()
	service := platformSvc.NewOperatorProvisioningService(cfg)
	if cfg.DB != nil {
		testpkg.SetTenantRuntime(t, service, cfg.DB)
	}
	return service
}

func newTestOperatorAuthService(t testing.TB, cfg platformSvc.OperatorAuthServiceConfig) (platformSvc.OperatorAuthAndInvitationService, error) {
	t.Helper()
	service, err := platformSvc.NewOperatorAuthService(cfg)
	if err == nil && cfg.DB != nil {
		testpkg.SetTenantRuntime(t, service, cfg.DB)
	}
	return service, err
}
