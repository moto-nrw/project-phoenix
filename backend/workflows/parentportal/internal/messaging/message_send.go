package messaging

import "context"

type messageSendIDKey struct{}

func WithMessageSendID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, messageSendIDKey{}, id)
}

func messageSendID(ctx context.Context) string {
	id, _ := ctx.Value(messageSendIDKey{}).(string)
	return id
}
