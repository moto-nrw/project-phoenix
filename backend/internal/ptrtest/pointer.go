// Package ptrtest provides dependency-free value helpers for tests without
// adding helpers to production APIs.
package ptrtest

import (
	"sync/atomic"
	"time"
)

// Ptr returns a pointer to value.
func Ptr[T any](value T) *T { return &value }

var tenantIDs atomic.Int64

func init() {
	tenantIDs.Store(time.Now().UnixMilli())
}

// NewTenantID returns a process-local tenant identity.
func NewTenantID() int64 {
	return tenantIDs.Add(1)
}
