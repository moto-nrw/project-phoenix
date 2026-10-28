package sse

import "context"

// Identity contains only the caller facts the stream uses. Root resolves them
// through the session runtime; Delivery imports no Identity & Access package.
type Identity struct {
	AccountID int64
	TenantID  int64
	IsAdmin   bool
	// ReadsLeadershipMessages: the account holds parent_messages:leadership
	// (or admin:*), so it hears about leadership conversations (#5200).
	ReadsLeadershipMessages bool
}

// Runtime binds authentication facts, the accepted token's lifetime and the
// tenant unit of work. No credential, claim implementation or database leaks
// into the Delivery HTTP adapter. E and C are the native types selected by
// Root, so the adapter retains typed channels without importing the hub.
type Runtime[E Event, C Client[E]] struct {
	Identity      func(context.Context) Identity
	TokenDeadline func(context.Context) (context.Context, context.CancelFunc, bool)
	WithinTenant  func(context.Context, int64, func(context.Context) error) error
	NewClient     func(chan E, int64, int64, int64, bool, map[string]bool) C
}
