package schedule

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstanceStaff_Validate(t *testing.T) {
	base := func() *InstanceStaff {
		return &InstanceStaff{InstanceID: 1, StaffID: 2}
	}

	t.Run("valid minimal", func(t *testing.T) {
		require.NoError(t, base().Validate())
	})

	t.Run("valid with room override", func(t *testing.T) {
		s := base()
		roomID := int64(17)
		s.RoomID = &roomID
		require.NoError(t, s.Validate())
	})

	t.Run("missing instance_id", func(t *testing.T) {
		s := base()
		s.InstanceID = 0
		err := s.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "instance_id is required")
	})

	t.Run("missing staff_id", func(t *testing.T) {
		s := base()
		s.StaffID = 0
		err := s.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "staff_id is required")
	})

	t.Run("negative room_id rejected", func(t *testing.T) {
		s := base()
		bad := int64(-1)
		s.RoomID = &bad
		err := s.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "room_id must be positive when set")
	})
}
