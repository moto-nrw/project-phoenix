package compose

import (
	"database/sql"
	"errors"
	"fmt"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The helpers below decode the generic repository's untyped arguments and
// restore its error shapes for the work-session and staff absence row
// adapters (worksession_rows.go, absence_rows.go).

// rowsAffectedError is the legacy repository result for an update that
// matched no row: the same DatabaseError the generic repository produced.
func rowsAffectedError(op string) error {
	return &modelBase.DatabaseError{Op: op, Err: fmt.Errorf("expected %d rows affected, got %d", 1, 0)}
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

func conditionStringList(value any) ([]string, bool) {
	switch typed := value.(type) {
	case []string:
		return typed, true
	case []any:
		values := make([]string, 0, len(typed))
		for _, entry := range typed {
			text, ok := entry.(string)
			if !ok {
				return nil, false
			}
			values = append(values, text)
		}
		return values, true
	default:
		return nil, false
	}
}

func conditionDate(value any) (string, bool) {
	switch typed := value.(type) {
	case calendar.Date:
		return typed.String(), true
	case *calendar.Date:
		if typed == nil {
			return "", false
		}
		return typed.String(), true
	case string:
		return typed, true
	default:
		return "", false
	}
}

func conditionInt64(value any) (int64, bool) {
	switch typed := value.(type) {
	case int64:
		return typed, true
	case int:
		return int64(typed), true
	case *int64:
		if typed == nil {
			return 0, false
		}
		return *typed, true
	default:
		return 0, false
	}
}

func conditionInt64List(value any) ([]int64, bool) {
	switch typed := value.(type) {
	case []int64:
		return typed, true
	case []any:
		ids := make([]int64, 0, len(typed))
		for _, entry := range typed {
			id, ok := conditionInt64(entry)
			if !ok {
				return nil, false
			}
			ids = append(ids, id)
		}
		return ids, true
	default:
		return nil, false
	}
}

// rowReadError restores the legacy read error shape: a missing row is the
// persistence-neutral not-found sentinel wrapped in a DatabaseError, every
// other failure keeps its cause behind the operation.
func rowReadError(op string, err error, notFound error) error {
	if notFound != nil && errors.Is(err, notFound) {
		return &modelBase.DatabaseError{Op: op, Err: errors.Join(modelBase.ErrNotFound, sql.ErrNoRows)}
	}
	return &modelBase.DatabaseError{Op: op, Err: err}
}

// ErrRowNotFound is the retained repositories' not-found sentinel the row
// adapters wrap (models/base.ErrNotFound).
var ErrRowNotFound = modelBase.ErrNotFound
