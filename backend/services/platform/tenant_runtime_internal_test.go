package platform

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/uptrace/bun"
)

func newMockTenantRuntimePtr(t testing.TB, db *bun.DB) *tenant.Runtime {
	t.Helper()
	runtime := testpkg.TenantRuntime(t, db)
	return &runtime
}

func newMockOutboxWorker(t testing.TB, cfg OutboxWorkerConfig) *OutboxWorker {
	t.Helper()
	worker := NewOutboxWorker(cfg)
	if cfg.DB != nil {
		worker.SetTenantRuntime(*newMockTenantRuntimePtr(t, cfg.DB))
	}
	return worker
}
