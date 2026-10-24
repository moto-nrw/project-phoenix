package compose

import (
	deliveryRepo "github.com/moto-nrw/project-phoenix/modules/delivery/internal/adapters/deliverypostgres"
	deliveryPorts "github.com/moto-nrw/project-phoenix/modules/delivery/internal/ports"
	"github.com/uptrace/bun"
)

// NewPushSubscriptionRepository exposes Delivery's subscription persistence
// to the legacy root composition while ownership is cut over package by
// package.
func NewPushSubscriptionRepository(db *bun.DB) deliveryPorts.PushSubscriptionRepository {
	return &deliveryRepo.PushSubscriptionRepository{DB: db}
}
