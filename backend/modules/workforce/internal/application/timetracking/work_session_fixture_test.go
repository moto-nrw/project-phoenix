package timetracking_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/application/timetracking"
	"github.com/stretchr/testify/require"
)

func createSessionFixture(t *testing.T, records timetracking.WorkSessionRecords, ctx context.Context, session *timetracking.WorkSession) {
	t.Helper()
	created, err := records.CreateWorkSession(ctx, timetracking.WorkSessionToCapability(session))
	require.NoError(t, err)
	session.ID, session.TenantID = created.ID, created.TenantID
	session.CreatedAt, session.UpdatedAt = created.CreatedAt, created.UpdatedAt
}

func createBreakFixture(t *testing.T, records timetracking.WorkSessionBreakRecords, ctx context.Context, entry *timetracking.WorkSessionBreak) {
	t.Helper()
	created, err := records.CreateWorkSessionBreak(ctx, timetracking.WorkSessionBreakToCapability(entry))
	require.NoError(t, err)
	entry.ID, entry.TenantID = created.ID, created.TenantID
	entry.CreatedAt, entry.UpdatedAt = created.CreatedAt, created.UpdatedAt
}
