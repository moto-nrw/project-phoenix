package legacy

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

type ScheduleDate = timezone.Date
type ScheduleQueryOptions = modelBase.QueryOptions

func ScheduleID(raw any) (int64, error) {
	switch value := raw.(type) {
	case int64:
		return value, nil
	case int:
		return int64(value), nil
	case int32:
		return int64(value), nil
	default:
		return 0, fmt.Errorf("unsupported id type %T", raw)
	}
}

func ScheduleError(op string, err error) error {
	if errors.Is(err, careplan.ErrStudentScheduleNotFound) {
		return &modelBase.DatabaseError{Op: op, Err: errors.Join(modelBase.ErrNotFound, sql.ErrNoRows)}
	}
	if err != nil {
		return &modelBase.DatabaseError{Op: op, Err: err}
	}
	return nil
}

func TodayScheduleDate() careplan.Date { return careplan.Date(timezone.TodayDate().String()) }
func PublicScheduleDate(value ScheduleDate) careplan.Date {
	return careplan.Date(value.String())
}
