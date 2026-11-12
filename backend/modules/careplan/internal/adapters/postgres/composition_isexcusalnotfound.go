package postgres

import (
	"database/sql"
	"errors"

	"github.com/moto-ogs/project-phoenix/internal/careplanning"
)

func IsExcusalNotFound(err error) bool {
	return errors.Is(err, careplanning.ErrStudentNotFound) || errors.Is(err, sql.ErrNoRows)
}
