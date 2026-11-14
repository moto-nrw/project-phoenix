package application

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMapParentMessagePreservesSystemEventActor(t *testing.T) {
	t.Parallel()

	for _, actor := range []string{"guardian", "staff"} {
		mapped, err := mapParentMessage(&ParentMessage{
			SenderKind: "system", EventType: "request_status", EventActorKind: actor,
		})
		require.NoError(t, err)
		require.Equal(t, actor, mapped.EventActorKind)
	}
}
