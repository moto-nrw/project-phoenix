package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/delivery"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/adapters/webpush"
)

func NewWebPushSender(config delivery.WebPushConfig) delivery.PushSender {
	return webpush.New(config)
}
