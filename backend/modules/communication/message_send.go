package communication

import (
	"context"
	"errors"
)

type messageSendIDKey struct{}

var ErrMessageSendConflict = errors.New("message send key already belongs to another message")

// WithMessageSendID carries a client-generated send attempt into the command.
// It is not authority: the command still authorizes the school, actor and target.
func WithMessageSendID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, messageSendIDKey{}, id)
}

func MessageSendID(ctx context.Context) string {
	id, _ := ctx.Value(messageSendIDKey{}).(string)
	return id
}
