package compose

import (
	deliveryRepo "github.com/moto-nrw/project-phoenix/database/repositories/delivery"
	deliveryModels "github.com/moto-nrw/project-phoenix/models/delivery"
	"github.com/uptrace/bun"
)

// NewPushSubscriptionRepository exposes Delivery's subscription persistence
// to the legacy root composition while ownership is cut over package by
// package.
func NewPushSubscriptionRepository(db *bun.DB) deliveryModels.PushSubscriptionRepository {
	return deliveryRepo.NewPushSubscriptionRepository(db)
}
