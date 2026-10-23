package ports

import (
	"context"
	"encoding/json"
	"time"
)

// ErasureStore commits each phase separately under the dedicated DB role.
type ErasureStore interface {
	Call(context.Context, time.Duration, time.Duration, string, ...any) (json.RawMessage, error)
	WithSchoolLock(context.Context, int64, func(context.Context) error) error
}

// ErasureLedger survives database restores. Check never creates a missing list.
type ErasureLedger interface {
	Check() error
	Record(json.RawMessage) error
}
