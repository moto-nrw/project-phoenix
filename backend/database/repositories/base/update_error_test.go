package base

import (
	"errors"
	"testing"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/stretchr/testify/assert"
)

func TestUpdateOperationErrorPreservesRowsAffectedFailure(t *testing.T) {
	t.Parallel()

	rowsErr := errors.New("rows affected failed")
	err := &modelBase.DatabaseError{
		Op:  "update columns",
		Err: &rowsAffectedError{err: rowsErr},
	}

	mapped := UpdateOperationError(err, "update setting value")
	assert.EqualError(t, mapped, "update setting value: rows affected: rows affected failed")
	assert.ErrorIs(t, mapped, rowsErr)

	var databaseErr *modelBase.DatabaseError
	assert.NotErrorAs(t, mapped, &databaseErr)
}
