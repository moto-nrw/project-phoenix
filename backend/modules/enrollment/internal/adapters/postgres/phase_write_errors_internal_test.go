package postgres

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun/driver/pgdriver"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// markChildWriteError marks only the rollover source index violation; every
// other error passes through unmarked and unchanged.

func TestMarkChildWriteError_NilPassesThrough(t *testing.T) {
	t.Parallel()

	assert.NoError(t, markChildWriteError(nil))
}

func TestMarkChildWriteError_NonPGErrorNotMarked(t *testing.T) {
	t.Parallel()

	err := errors.New("synthetic")
	marked := markChildWriteError(err)
	assert.Same(t, err, marked)
	assert.NotErrorIs(t, marked, enrollment.ErrRolloverSourceChildTaken)
}

// pgdriver.Error has unexported fields and isn't constructable with
// arbitrary metadata; a zero value has no SQLSTATE and must not be marked.
func TestMarkChildWriteError_ZeroPGErrorNotMarked(t *testing.T) {
	t.Parallel()

	wrapped := errors.Join(errors.New("outer"), pgdriver.Error{})
	assert.NotErrorIs(t, markChildWriteError(wrapped), enrollment.ErrRolloverSourceChildTaken)
}
