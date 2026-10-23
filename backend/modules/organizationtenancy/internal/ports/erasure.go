package ports

import (
	"context"
	"encoding/json"
	"time"
)

// ErasureStore commits each phase separately under the dedicated DB role.
type ErasureStore interface {
	Call(context.Context, time.Duration, time.Duration, string, ...any) (json.RawMessage, error)
	WithSchoolLock(context.Context, int64, func() error) error
}
