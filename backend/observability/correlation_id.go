package observability

import (
	"context"

	"github.com/gofrs/uuid"
)

type correlationIDKey struct{}

// CorrelationID identifies one request or job across logs, metrics, and traces.
// Its zero value represents a missing identifier.
type CorrelationID struct {
	value string
}

// NewCorrelationID returns a cryptographically random correlation ID.
func NewCorrelationID() (CorrelationID, error) {
	id, err := uuid.NewV4()
	if err != nil {
		return CorrelationID{}, err
	}
	return CorrelationID{value: id.String()}, nil
}

// CorrelationIDFromRequest preserves a caller-supplied UUID. Invalid or
// missing values are replaced so untrusted header text never reaches logs.
func CorrelationIDFromRequest(requestValue string) (CorrelationID, error) {
	if requestValue != "" {
		if id, err := uuid.FromString(requestValue); err == nil {
			return CorrelationID{value: id.String()}, nil
		}
	}
	return NewCorrelationID()
}

// WithCorrelationID binds the request or job correlation ID to ctx.
func WithCorrelationID(ctx context.Context, id CorrelationID) context.Context {
	return context.WithValue(ctx, correlationIDKey{}, id)
}

// CorrelationIDFromContext returns the correlation ID bound at the Serve or
// Worker root.
func CorrelationIDFromContext(ctx context.Context) (CorrelationID, bool) {
	id, ok := ctx.Value(correlationIDKey{}).(CorrelationID)
	return id, ok && id.String() != ""
}

// String returns the value used by logging and propagation adapters.
func (id CorrelationID) String() string { return id.value }
