package tenant

import (
	"errors"
	"fmt"
)

// ErrInvalidTenantID is returned when a raw tenant identifier is not positive.
var ErrInvalidTenantID = errors.New("tenant ID must be positive")

// TenantID is a validated school identifier. Construct it with NewTenantID;
// the zero value is invalid and represents a missing tenant.
type TenantID struct {
	value int64
}

// NewTenantID validates and constructs a TenantID.
func NewTenantID(value int64) (TenantID, error) {
	if value <= 0 {
		return TenantID{}, fmt.Errorf("%w: %d", ErrInvalidTenantID, value)
	}
	return TenantID{value: value}, nil
}

// IsZero reports whether the identifier is missing.
func (id TenantID) IsZero() bool { return id.value == 0 }

// Int64 returns the database and wire representation.
func (id TenantID) Int64() int64 { return id.value }
