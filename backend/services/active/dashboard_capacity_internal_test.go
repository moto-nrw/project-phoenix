package active

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetermineActivityStatusUnlimitedStaysActive(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "active", determineActivityStatus(1000, 0))
}
