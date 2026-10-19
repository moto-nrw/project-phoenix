package postgres

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

func IsPickupScheduleNotFound(err error, isNotFound func(error) bool) bool {
	return errors.Is(err, careplan.ErrStudentScheduleNotFound) || isNotFound(err)
}
