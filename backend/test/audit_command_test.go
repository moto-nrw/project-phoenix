package test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type auditCommandEvent struct{ name string }

func TestNewAuditCommandAppendsOnlyItsEventType(t *testing.T) {
	t.Parallel()

	var created []*auditCommandEvent
	command := NewAuditCommand(func(_ context.Context, event *auditCommandEvent) error {
		created = append(created, event)
		return nil
	})

	event := &auditCommandEvent{name: "login"}
	require.NoError(t, command.Append(context.Background(), event))
	err := command.Append(context.Background(), auditCommandEvent{name: "by value"})
	require.ErrorContains(t, err, "unsupported event test.auditCommandEvent")
	assert.Equal(t, []*auditCommandEvent{event}, created)
}
