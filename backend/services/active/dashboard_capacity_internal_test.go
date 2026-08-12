package active

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetermineActivityStatusUnlimitedStaysActive(t *testing.T) {
	assert.Equal(t, "active", determineActivityStatus(1000, 0))
}
