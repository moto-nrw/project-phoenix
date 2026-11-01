package parentportal_test

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	parentportal "github.com/moto-nrw/project-phoenix/workflows/parentportal"
	"github.com/stretchr/testify/require"
)

func TestGuardianSendRetryReturnsTheStoredMessage(t *testing.T) {
	t.Parallel()
	svc, _, db, _ := buildMessagingWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := parentportal.WithMessageSendID(testpkg.WithPackageTenantRuntime(context.Background()), "guardian-send-retry")
	first, err := svc.PostChildReply(ctx, chain.AccountID, chain.StudentID, "Bitte anrufen", 0)
	require.NoError(t, err)
	retry, err := svc.PostChildReply(ctx, chain.AccountID, chain.StudentID, "Bitte anrufen", 0)
	require.NoError(t, err)
	require.Len(t, retry.Messages, 1)
	require.Equal(t, first.Messages[0].ID, retry.Messages[0].ID)
	require.Equal(t, "guardian-send-retry", retry.Messages[0].ClientMessageID)
	_, err = svc.PostChildReply(ctx, chain.AccountID, chain.StudentID, "Anderer Inhalt", 0)
	require.ErrorIs(t, err, parentportal.ErrMessageSendConflict)
}
