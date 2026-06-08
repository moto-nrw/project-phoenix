package users

import (
	"errors"
	"reflect"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun/driver/pgdriver"
)

func TestIsUndefinedBusDaysColumn(t *testing.T) {
	assert.True(t, isUndefinedBusDaysColumn(testPgError("42703")))
	assert.False(t, isUndefinedBusDaysColumn(testPgError("23514")))
	assert.False(t, isUndefinedBusDaysColumn(errors.New("check_students_bus_days mentions bus_days")))
}

func testPgError(code string) error {
	pgErr := pgdriver.Error{}
	v := reflect.ValueOf(&pgErr).Elem()
	mField := v.FieldByName("m")
	ptr := unsafe.Pointer(mField.UnsafeAddr()) //nolint:gosec
	*(*map[byte]string)(ptr) = map[byte]string{'C': code}
	return pgErr
}
