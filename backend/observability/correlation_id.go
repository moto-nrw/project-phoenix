package observability

import "github.com/gofrs/uuid"

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

// CorrelationIDFromRequest preserves a caller-supplied request value and
// generates an identifier only when that value is empty.
func CorrelationIDFromRequest(requestValue string) (CorrelationID, error) {
	if requestValue == "" {
		return NewCorrelationID()
	}
	return CorrelationID{value: requestValue}, nil
}

// String returns the value used by logging and propagation adapters.
func (id CorrelationID) String() string { return id.value }
