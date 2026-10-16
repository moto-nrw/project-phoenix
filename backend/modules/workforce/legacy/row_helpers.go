package legacy

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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
		return &databaseError{op: op, err: errors.Join(errRecordNotFound, sql.ErrNoRows)}
	}
	return &databaseError{op: op, err: err}
}

// writeError keeps the legacy write error shape. Validation
// failures surface bare, as the model's own Validate() did, so callers that
// compare the reason text keep seeing it; everything else is a DatabaseError
// whose chain still reaches the driver error of a rejected duplicate.
func writeError(op string, err error) error {
	if errors.Is(err, workforce.ErrInvalidGroupSubstitution) {
		return errors.New(err.Error())
	}
	return &databaseError{op: op, err: err}
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
