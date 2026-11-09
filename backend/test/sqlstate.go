package test

import (
	"errors"

	"github.com/uptrace/bun/driver/pgdriver"
)

// PostgresSQLState returns the structured code of a PostgreSQL error, including
// wrapped errors. Non-PostgreSQL errors and nil have no SQLSTATE.
func PostgresSQLState(err error) string {
	var postgresError pgdriver.Error
	if errors.As(err, &postgresError) {
		return postgresError.Field('C')
	}
	return ""
}
