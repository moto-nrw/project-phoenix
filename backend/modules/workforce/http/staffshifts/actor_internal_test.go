package staffshifts

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewActorRequiresAStaffRecord(t *testing.T) {
	t.Parallel()
	staffID, accountID := int64(41), int64(73)

	actor, err := NewActor(staffID, &accountID)
	require.NoError(t, err)
	assert.Equal(t, Actor{StaffID: staffID, AccountID: &accountID}, actor)

	for _, missing := range []int64{0, -staffID} {
		_, err := NewActor(missing, &accountID)
		require.ErrorIs(t, err, ErrNoStaffActor)
		assert.EqualError(t, err, "staff record not found")
	}
}
