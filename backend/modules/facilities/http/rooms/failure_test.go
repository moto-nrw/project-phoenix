package rooms

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/stretchr/testify/require"
)

func TestFailureClassifiesToiletRoomReleaseAsInvalid(t *testing.T) {
	t.Parallel()

	kind, code := classifyFailure(facilities.ErrToiletRoomNotReleasable)

	require.Equal(t, FailureInvalid, kind)
	require.Equal(t, "toilet_room_not_releasable", code)
}
