package parentportal_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/moto-nrw/project-phoenix/realtime"

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

type sendNoticeRecorder struct{ calls atomic.Int64 }

func (n *sendNoticeRecorder) NotifyStaffParentMessage(context.Context, parentportal.StaffParentMessageReport) error {
	n.calls.Add(1)
	return nil
}

func TestGuardianConcurrentSendRetriesNotifyStaffOnlyOnce(t *testing.T) {
	t.Parallel()
	notice := &sendNoticeRecorder{}
	svc, bc, db, _ := buildMessagingWriteService(t, true, true, notice)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := parentportal.WithMessageSendID(testpkg.WithPackageTenantRuntime(context.Background()), "guardian-concurrent-retry")
	type result struct {
		id    int64
		count int
		err   error
	}
	results := make(chan result, 4)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			<-start
			view, err := svc.PostChildReply(ctx, chain.AccountID, chain.StudentID, "Bitte anrufen", 0)
			if err != nil {
				results <- result{err: err}
				return
			}
			results <- result{id: view.Messages[0].ID, count: len(view.Messages)}
		})
	}
	close(start)
	wg.Wait()
	close(results)
	var id int64
	for r := range results {
		require.NoError(t, r.err)
		require.Equal(t, 1, r.count)
		if id == 0 {
			id = r.id
		}
		require.Equal(t, id, r.id)
	}
	require.Positive(t, id)
	require.EqualValues(t, 1, notice.calls.Load())
	events := 0
	for _, call := range bc.CallsByMethod("parent") {
		if call.Event.Type == realtime.EventParentMessage {
			events++
		}
	}
	require.Equal(t, 1, events)
}
