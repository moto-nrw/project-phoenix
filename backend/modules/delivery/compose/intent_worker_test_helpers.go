//go:build testhooks

package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/application"
)

func DrainEmailForTest(ctx context.Context, runtime *Runtime, provider Provider, tenantID, id int64) error {
	return application.DrainEmailForTest(ctx, runtime.store, providerAdapter{provider: provider}, runtime.observe, tenantID, id)
}
func DeleteSystemEmailForTest(ctx context.Context, runtime *Runtime, id int64) error {
	return runtime.store.WorkerRecords.(interface {
		DeleteSystemEmailForTest(context.Context, int64) error
	}).DeleteSystemEmailForTest(ctx, id)
}
