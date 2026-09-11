package postgres

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPersonRowBirthdayUsesCalendarDate(t *testing.T) {
	t.Parallel()

	var row personRow
	assert.Equal(t, "*timezone.Date", reflect.TypeOf(row.Birthday).String())
}

func TestStudentRowCareWindowUsesCalendarDates(t *testing.T) {
	t.Parallel()

	var row studentRow
	assert.Equal(t, "*timezone.Date", reflect.TypeOf(row.EnrolledFrom).String())
	assert.Equal(t, "*timezone.Date", reflect.TypeOf(row.EnrolledUntil).String())
}
