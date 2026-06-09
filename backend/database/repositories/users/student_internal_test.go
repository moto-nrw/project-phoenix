package users

import (
	"errors"
	"reflect"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun/driver/pgdriver"
)

func TestIsUndefinedColumnError(t *testing.T) {
	assert.True(t, isUndefinedColumnError(testPgError("42703")))
	assert.False(t, isUndefinedColumnError(testPgError("23514")))
	assert.False(t, isUndefinedColumnError(errors.New("check_students_bus_days mentions bus_days")))
}

func testPgError(code string) error {
	pgErr := pgdriver.Error{}
	v := reflect.ValueOf(&pgErr).Elem()
	mField := v.FieldByName("m")
	ptr := unsafe.Pointer(mField.UnsafeAddr()) //nolint:gosec
	*(*map[byte]string)(ptr) = map[byte]string{'C': code}
	return pgErr
}
