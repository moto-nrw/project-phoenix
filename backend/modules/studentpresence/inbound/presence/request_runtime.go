package presence

import "context"

// RequestRuntime binds request identity and rollback handling to the HTTP runtime.
type RequestRuntime struct {
	WithStaff    func(context.Context, int64, int64) context.Context
	TenantID     func(context.Context) int64
	MarkRollback func(context.Context)
}
