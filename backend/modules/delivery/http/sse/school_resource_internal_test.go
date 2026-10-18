package sse

import (
	"context"
	"log/slog"
	"testing"

	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubSubscriptions struct{}

func (stubSubscriptions) ResolveSSETopics(context.Context) (int64, []string, []string, []string, error) {
	return 0, nil, nil, nil, nil
}

func TestNewResourceWithSchoolAccessRequiresItsDependencies(t *testing.T) {
	t.Parallel()
	hub, runtime, access := realtime.NewHub(slog.Default()), testSSETransportRuntime(), &stubSchoolAccess{allowed: true}

	_, err := NewResourceWithSchoolAccess[realtime.Event, *realtime.Client](nil, stubSubscriptions{}, runtime, access, nil)
	require.Error(t, err, "hub")
	_, err = NewResourceWithSchoolAccess[realtime.Event](hub, nil, runtime, access, nil)
	require.Error(t, err, "subscription resolver")
	_, err = NewResourceWithSchoolAccess[realtime.Event](hub, stubSubscriptions{}, runtime, nil, nil)
	require.Error(t, err, "school access")

	resource, err := NewResourceWithSchoolAccess[realtime.Event](hub, stubSubscriptions{}, runtime, access, slog.Default())
	require.NoError(t, err)
	assert.Equal(t, hub, resource.hub)
	assert.Same(t, access, resource.schoolAccess)
}
