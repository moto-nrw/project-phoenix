package ports

import (
	"context"
	"encoding/json"
	"time"
)

// ErasureStore commits each phase separately under the dedicated DB role.
type ErasureStore interface {
	CheckSchoolLock(context.Context) error
	Call(context.Context, time.Duration, time.Duration, string, ...any) (json.RawMessage, error)
	WithSchoolLock(context.Context, int64, func(context.Context) error) error
}
