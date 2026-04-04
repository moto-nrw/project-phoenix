package platform

import (
	"errors"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun/driver/pgdriver"
)

// isUniqueViolation returns true when err (or a wrapped DatabaseError) carries
// PostgreSQL error code 23505 (unique_violation).
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var dbErr *modelBase.DatabaseError
	if errors.As(err, &dbErr) {
		err = dbErr.Err
	}
	var pgErr pgdriver.Error
	if errors.As(err, &pgErr) {
		return pgErr.IntegrityViolation() && pgErr.Field('C') == "23505"
	}
	return false
}
