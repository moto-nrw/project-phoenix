package ports

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ============================================================================
// GroupSupervisor Model Tests
// ============================================================================

func TestGroupSupervisor_GetID(t *testing.T) {
	t.Parallel()

	gs := &GroupSupervisor{}
	gs.ID = 456
	assert.Equal(t, int64(456), gs.GetID())
}
