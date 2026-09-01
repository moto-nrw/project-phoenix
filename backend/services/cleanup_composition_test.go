package services

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
)

type cleanupAuditTestEvent struct{ tenantID int64 }

func (event *cleanupAuditTestEvent) GetTenantID() int64   { return event.tenantID }
func (event *cleanupAuditTestEvent) SetTenantID(id int64) { event.tenantID = id }

func TestCleanupAuditCommandRequiresProducerTransaction(t *testing.T) {
	t.Parallel()

	command, err := NewCleanupAuditCommand(slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)

	event := &cleanupAuditTestEvent{tenantID: 1}
	err = command.Append(context.Background(), event)

	require.ErrorContains(t, err, "transaction is required")
}
