package legacy

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// The helpers below decode the generic repository's untyped arguments and
// restore its error shapes for the group substitution adapter
// (substitution_repository.go).

// legacyID narrows the untyped ID of the generic repository contract.
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

// readError restores the legacy repository error shape: a missing
// row is reported as the persistence-neutral not-found sentinel wrapped in a
// DatabaseError, every other failure keeps its cause behind the operation.
func readError(op string, err error, notFound error) error {
	if notFound != nil && errors.Is(err, notFound) {
		return &modelBase.DatabaseError{Op: op, Err: errors.Join(modelBase.ErrNotFound, sql.ErrNoRows)}
	}
	return &modelBase.DatabaseError{Op: op, Err: err}
}

// writeError keeps the legacy write error shape. Validation
// failures surface bare, as the model's own Validate() did, so callers that
// compare the reason text keep seeing it; everything else is a DatabaseError
// whose chain still reaches the driver error of a rejected duplicate.
func writeError(op string, err error) error {
	if errors.Is(err, workforce.ErrInvalidGroupSubstitution) {
		return errors.New(err.Error())
	}
	return &modelBase.DatabaseError{Op: op, Err: err}
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

func conditionDate(value any) (string, bool) {
	switch typed := value.(type) {
	case timezone.Date:
		return typed.String(), true
	case *timezone.Date:
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
