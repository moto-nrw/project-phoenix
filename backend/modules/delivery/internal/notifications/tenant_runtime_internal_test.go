package notifications

import (
	"context"
	"log/slog"
	"testing"

	deliveryPorts "github.com/moto-nrw/project-phoenix/modules/delivery/internal/ports"

	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/uptrace/bun"
)

func newMockTenantRuntime(t testing.TB, db *bun.DB) tenant.UnitOfWork {
	if db != nil {
		return testpkg.TenantRuntime(t, db)
	}
	runtime, err := tenant.NewUnitOfWork(
		func(ctx context.Context, _ int64, fn func(context.Context, any) error) error {
			return fn(ctx, bun.Tx{})
		},
		func(ctx context.Context, fn func(context.Context, any) error) error {
			return fn(ctx, bun.Tx{})
		},
		func(context.Context, tenant.SavepointAction) error { return nil },
		func(error) bool { return false },
	)
	if err != nil {
		panic(err)
	}
	return runtime
}

func newMockPushSubscriptionService(
	t testing.TB,
	db *bun.DB,
	repo deliveryPorts.PushSubscriptionRepository,
	accountTenants GuardianSchools,
	vapid VAPIDConfig,
	logger *slog.Logger,
) PushSubscriptionService {
	service := NewPushSubscriptionService(db, repo, accountTenants, vapid, logger)
	service.(*pushSubscriptionService).tenantRuntime = new(newMockTenantRuntime(t, db))
	return service
}

func newMockSSEChannel(t testing.TB, broadcaster realtime.Broadcaster, opts ...SSEChannelOption) Channel {
	channel := NewSSEChannel(broadcaster, opts...)
	sse := channel.(*sseChannel)
	sse.tenantRuntime = new(newMockTenantRuntime(t, sse.db))
	return channel
}
