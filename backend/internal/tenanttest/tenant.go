// Package tenanttest generates identities for pure unit tests that cannot
// import the database-backed test package without creating an import cycle.
package tenanttest

import (
	"sync/atomic"
	"time"
)

var tenantIDs atomic.Int64

func init() {
	tenantIDs.Store(time.Now().UnixMilli())
}

// NewTenantID returns a process-local tenant identity.
func NewTenantID() int64 {
	return tenantIDs.Add(1)
}
