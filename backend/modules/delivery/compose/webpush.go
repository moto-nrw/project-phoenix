package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/delivery"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/adapters/webpush"
)

func NewWebPushSender(config delivery.WebPushConfig, cleanup func(context.Context, delivery.ClaimedIntent) error) delivery.PushSender {
	return webpush.New(config, cleanup)
}
