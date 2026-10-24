package timetable_test

import (
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/stretchr/testify/assert"
)

func TestSystemActivityNamesAreExact(t *testing.T) {
	t.Parallel()
	assert.True(t, timetable.IsSystemActivityName(timetable.SchulhofActivityName))
	assert.True(t, timetable.IsSystemActivityName(timetable.WCActivityName))
	assert.False(t, timetable.IsSystemActivityName("wc"))
}

func TestErrorCodeIsStable(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "none", timetable.ErrorCode(nil))
	assert.Equal(t, "not_found", timetable.ErrorCode(timetable.ErrCategoryNotFound))
	assert.Equal(t, "category_name_exists", timetable.ErrorCode(timetable.ErrCategoryNameExists))
	assert.Equal(t, "internal_error", timetable.ErrorCode(errors.New("boom")))
}

func ptr[T any](value T) *T { return &value }
