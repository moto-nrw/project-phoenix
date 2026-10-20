package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/notifications"
	delivery "github.com/moto-nrw/project-phoenix/modules/delivery/runtime"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// NewDurablePushOutbox binds the push outbox to module.
func NewDurablePushOutbox(module delivery.Capability) DurablePushOutbox {
	return DurablePushOutbox{DurablePushOutbox: notifications.DurablePushOutbox{Module: module}}
}

type DurablePushOutbox struct {
	notifications.DurablePushOutbox
}

// NewTenantTransactions binds provider steps to the intent's explicit scope.
func NewTenantTransactions(db *bun.DB) delivery.IntentTransactions {
	return delivery.NewIntentTransactions(func(ctx context.Context, id int64, fn func(context.Context) error) error {
		return tenant.WithTenantTx(ctx, db, id, func(txCtx context.Context, _ bun.Tx) error { return fn(txCtx) })
	}, tenant.WithinAdmin)
}
