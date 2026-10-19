package owners

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStaffMessageCleanupSkipsMissingRuntime(t *testing.T) {
	t.Parallel()

	require.Nil(t, staffMessageCleanup(nil))
}
