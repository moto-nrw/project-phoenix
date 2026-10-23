package identityaccess

import (
	"context"
	"net"
)

type operatorAuditClientIPKey struct{}

// WithOperatorAuditClientIP binds the parsed source address to this request's
// operator audit evidence. It changes neither session claims nor authorization.
func WithOperatorAuditClientIP(ctx context.Context, clientIP net.IP) context.Context {
	if clientIP == nil {
		return ctx
	}
	return context.WithValue(ctx, operatorAuditClientIPKey{}, clientIP.String())
}

// OperatorAuditClientAddress returns the source address, or no address for a
// non-HTTP operation. Transaction contexts retain this request-local evidence.
func OperatorAuditClientAddress(ctx context.Context) string {
	address, _ := ctx.Value(operatorAuditClientIPKey{}).(string)
	return address
}
