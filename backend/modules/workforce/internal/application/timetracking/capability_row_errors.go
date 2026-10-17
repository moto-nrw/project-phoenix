package timetracking

import (
	"errors"
	"fmt"
)

// RowErrorShapes restores the error shapes of the retained generic
// repositories for the capability row adapters: Database wraps a failed
// operation, Missing is the cause of a missing row. The composition binds
// them, so the callers that still classify on those shapes keep seeing them.
type RowErrorShapes struct {
	Database func(op string, err error) error
	Missing  error
}

// rowsAffected is the legacy repository result for an update that matched no
// row: the same error the generic repository produced.
func (s RowErrorShapes) rowsAffected(op string) error {
	return s.Database(op, fmt.Errorf("expected %d rows affected, got %d", 1, 0))
}

// legacyID accepts the integer shapes the generic repository's untyped id
// parameter carried.
func legacyID(id any) (int64, error) {
	switch value := id.(type) {
	case int64:
		return value, nil
	case int:
		return int64(value), nil
	case int32:
		return int64(value), nil
	case uint:
		return int64(value), nil
	case uint32:
		return int64(value), nil
	case uint64:
		return int64(value), nil
	default:
		return 0, fmt.Errorf("unsupported id type %T", id)
	}
}

// read restores the legacy read error shape: a missing row is the
// persistence-neutral not-found cause wrapped like a failed operation, every
// other failure keeps its cause behind the operation.
func (s RowErrorShapes) read(op string, err error, notFound error) error {
	if notFound != nil && errors.Is(err, notFound) {
		return s.Database(op, s.Missing)
	}
	return s.Database(op, err)
}
