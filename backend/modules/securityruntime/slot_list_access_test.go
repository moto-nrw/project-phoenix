package securityruntime

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSlotListStudentAccess(t *testing.T) {
	t.Parallel()
	asked := 0
	staffRecord := func(found bool, err error) func() (bool, error) {
		return func() (bool, error) { asked++; return found, err }
	}

	allowed, err := SlotListStudentAccess([]string{"admin:*"}, staffRecord(false, nil))
	require.NoError(t, err)
	assert.True(t, allowed)
	assert.Zero(t, asked, "an administrator is not asked for a staff record")

	allowed, err = SlotListStudentAccess([]string{"groups:read"}, staffRecord(true, nil))
	require.NoError(t, err)
	assert.True(t, allowed)

	allowed, err = SlotListStudentAccess(nil, staffRecord(false, nil))
	require.NoError(t, err)
	assert.False(t, allowed)

	failure := errors.New("staff lookup failed")
	_, err = SlotListStudentAccess(nil, staffRecord(true, failure))
	assert.ErrorIs(t, err, failure)
}
