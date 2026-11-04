package parentportal

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/messaging"
)

var ErrMessageSendConflict = errors.New("message send key already belongs to another message")

// WithMessageSendID correlates one guardian send without granting access.
func WithMessageSendID(ctx context.Context, id string) context.Context {
	return messaging.WithMessageSendID(ctx, id)
}
