package schoolstructure_test

import (
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/stretchr/testify/assert"
)

func TestErrorCodeIsStable(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "none", schoolstructure.ErrorCode(nil))
	assert.Equal(t, "not_found", schoolstructure.ErrorCode(schoolstructure.ErrGroupNotFound))
	assert.Equal(t, "invalid", schoolstructure.ErrorCode(schoolstructure.ErrInvalidGroup))
	assert.Equal(t, "internal_error", schoolstructure.ErrorCode(errors.New("boom")))
}
