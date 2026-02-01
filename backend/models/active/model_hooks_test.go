package active

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"
)

// ============================================================================
// GroupMapping Model Tests
// ============================================================================

func TestGroupMapping_BeforeAppendModel(t *testing.T) {
	gm := &GroupMapping{}

	t.Run("handles SelectQuery", func(t *testing.T) {
		// The BeforeAppendModel uses type assertion, so we test with actual bun types
		// but the actual query modification happens through the bun internals
		err := gm.BeforeAppendModel(&bun.SelectQuery{})
		assert.NoError(t, err)
	})

	t.Run("handles InsertQuery", func(t *testing.T) {
		err := gm.BeforeAppendModel(&bun.InsertQuery{})
		assert.NoError(t, err)
	})

	t.Run("handles UpdateQuery", func(t *testing.T) {
		err := gm.BeforeAppendModel(&bun.UpdateQuery{})
		assert.NoError(t, err)
	})

	t.Run("handles DeleteQuery", func(t *testing.T) {
		err := gm.BeforeAppendModel(&bun.DeleteQuery{})
		assert.NoError(t, err)
	})

	t.Run("handles unknown query type", func(t *testing.T) {
		err := gm.BeforeAppendModel("unknown")
		assert.NoError(t, err)
	})
}

func TestGroupMapping_TableName(t *testing.T) {
	gm := &GroupMapping{}
	assert.Equal(t, "active.group_mappings", gm.TableName())
}

func TestGroupMapping_GetID(t *testing.T) {
	gm := &GroupMapping{}
	gm.ID = 123
	assert.Equal(t, int64(123), gm.GetID())
}

func TestGroupMapping_Validate(t *testing.T) {
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

// TestGroupSupervisor_BeforeAppendModel is skipped because the hook is now commented out
// (the repository controls the table expression directly, following the pattern used by active.Group)

func TestGroupSupervisor_TableName(t *testing.T) {
	gs := &GroupSupervisor{}
	assert.Equal(t, "active.group_supervisors", gs.TableName())
}

func TestGroupSupervisor_GetID(t *testing.T) {
	gs := &GroupSupervisor{}
	gs.ID = 456
	assert.Equal(t, int64(456), gs.GetID())
}

// ============================================================================
// Visit Model Tests
// ============================================================================

func TestVisit_BeforeAppendModel(t *testing.T) {
	v := &Visit{}

	t.Run("handles SelectQuery", func(t *testing.T) {
		err := v.BeforeAppendModel(&bun.SelectQuery{})
		assert.NoError(t, err)
	})

	t.Run("handles InsertQuery", func(t *testing.T) {
		err := v.BeforeAppendModel(&bun.InsertQuery{})
		assert.NoError(t, err)
	})

	t.Run("handles UpdateQuery", func(t *testing.T) {
		err := v.BeforeAppendModel(&bun.UpdateQuery{})
		assert.NoError(t, err)
	})

	t.Run("handles DeleteQuery", func(t *testing.T) {
		err := v.BeforeAppendModel(&bun.DeleteQuery{})
		assert.NoError(t, err)
	})

	t.Run("handles unknown query type", func(t *testing.T) {
		err := v.BeforeAppendModel("unknown")
		assert.NoError(t, err)
	})
}

func TestVisit_TableName(t *testing.T) {
	v := &Visit{}
	assert.Equal(t, "active.visits", v.TableName())
}

func TestVisit_GetID(t *testing.T) {
	v := &Visit{}
	v.ID = 789
	assert.Equal(t, int64(789), v.GetID())
}
