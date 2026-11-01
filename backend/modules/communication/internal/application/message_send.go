package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

var ErrMessageSendConflict = communication.ErrMessageSendConflict

func MessageSendID(ctx context.Context) string { return communication.MessageSendID(ctx) }
