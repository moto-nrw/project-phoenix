package active

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ============================================================================
// GroupMapping Model Tests
// ============================================================================

func TestGroupMapping_GetID(t *testing.T) {
	t.Parallel()

	gm := &GroupMapping{}
	gm.ID = 123
	assert.Equal(t, int64(123), gm.GetID())
}

func TestGroupMapping_Validate(t *testing.T) {
	t.Parallel()

	t.Run("valid mapping", func(t *testing.T) {
		gm := &GroupMapping{
			ActiveCombinedGroupID: 1,
			ActiveGroupID:         2,
		}
		err := gm.Validate()
		assert.NoError(t, err)
	})

	t.Run("missing combined group ID", func(t *testing.T) {
		gm := &GroupMapping{
			ActiveGroupID: 2,
		}
		err := gm.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "active combined group ID")
	})

	t.Run("missing active group ID", func(t *testing.T) {
		gm := &GroupMapping{
			ActiveCombinedGroupID: 1,
		}
		err := gm.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "active group ID")
	})
}

// ============================================================================
// GroupSupervisor Model Tests
// ============================================================================

func TestGroupSupervisor_GetID(t *testing.T) {
	t.Parallel()

	gs := &GroupSupervisor{}
	gs.ID = 456
	assert.Equal(t, int64(456), gs.GetID())
}

// ============================================================================
// Visit Model Tests
// ============================================================================

func TestVisit_GetID(t *testing.T) {
	t.Parallel()

	v := &Visit{}
	v.ID = 789
	assert.Equal(t, int64(789), v.GetID())
}
