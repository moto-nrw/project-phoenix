package repositories

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

type recordingAuditCommand struct{ event any }

func (command *recordingAuditCommand) Append(_ context.Context, event any) error {
	command.event = event
	return nil
}

func requireCreateRoutesThroughCommand(t *testing.T, repository any, command *recordingAuditCommand) {
	t.Helper()

	create := reflect.ValueOf(repository).MethodByName("Create")
	require.True(t, create.IsValid())
	event := reflect.New(create.Type().In(1).Elem())
	results := create.Call([]reflect.Value{reflect.ValueOf(context.Background()), event})
	require.Len(t, results, 1)
	require.True(t, results[0].IsNil())
	require.Same(t, event.Interface(), command.event)
}

func TestCleanupCommandRootsRouteAuditWritesThroughCommand(t *testing.T) {
	t.Parallel()

	t.Run("auth event", func(t *testing.T) {
		t.Parallel()

		command := &recordingAuditCommand{}
		repos := NewAuthCleanupRepositories(nil, command)
		requireCreateRoutesThroughCommand(t, repos.AuthEvent, command)
	})

	t.Run("data deletion", func(t *testing.T) {
		t.Parallel()

		command := &recordingAuditCommand{}
		repos := NewRetentionCleanupRepositories(nil, command)
		requireCreateRoutesThroughCommand(t, repos.Deletion, command)
	})
}
