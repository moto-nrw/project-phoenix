package compose

import (
	"context"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"

	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/adapters/webpush"
)

func NewWebPushSender(config deliveryContract.WebPushConfig, cleanup func(context.Context, deliveryContract.ClaimedIntent) error) deliveryContract.PushSender {
	return webpush.New(config, cleanup)
}
