package messaging_test

import (
	"context"
	"sync"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	messaging "github.com/moto-nrw/project-phoenix/modules/communication/internal/parentmessages"
	"github.com/moto-nrw/project-phoenix/realtime"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestParentSendRetryReturnsStoredMessageWithoutAnotherNotice(t *testing.T) {
	t.Parallel()
	f := newFixture(t, true)
	ctx := communication.WithMessageSendID(adminCtx(t, f.staffAccount), "parent-send-retry")
	first, err := testpkg.TenantQuery4(t, ctx, f.db, f.svc.StartThread, f.chain.StudentID, f.chain.AccountID, "Bitte zurückrufen", "")
	require.NoError(t, err)
	retry, err := testpkg.TenantQuery4(t, ctx, f.db, f.svc.StartThread, f.chain.StudentID, f.chain.AccountID, "Bitte zurückrufen", "")
	require.NoError(t, err)
	require.Len(t, retry.Messages, 1)
	require.Equal(t, first.Messages[0].ID, retry.Messages[0].ID)
	require.Equal(t, "parent-send-retry", retry.Messages[0].ClientMessageID)
	require.Equal(t, 1, parentEventCount(f.bc, realtime.EventParentMessage))
	_, err = testpkg.TenantQuery4(t, ctx, f.db, f.svc.StartThread, f.chain.StudentID, f.chain.AccountID, "Anderer Text", "")
	require.ErrorIs(t, err, communication.ErrMessageSendConflict)
	// Another reply has a new key; its own retry must also return just one row.
	replyCtx := communication.WithMessageSendID(adminCtx(t, f.staffAccount), "parent-reply-retry")
	sent, err := testpkg.TenantQuery3(t, replyCtx, f.db, f.svc.PostMessage, first.ThreadID, "Danke", first.Messages[0].ID)
	require.NoError(t, err)
	again, err := testpkg.TenantQuery3(t, replyCtx, f.db, f.svc.PostMessage, first.ThreadID, "Danke", first.Messages[0].ID)
	require.NoError(t, err)
	require.Len(t, again, 2)
	require.Equal(t, sent[1].ID, again[1].ID)
	require.Equal(t, 2, parentEventCount(f.bc, realtime.EventParentMessage))
}

func TestParentReplyRetryAfterNewGuardianActivity(t *testing.T) {
	t.Parallel()
	f := newFixture(t, true)
	staffCtx := adminCtx(t, f.staffAccount)
	first, err := testpkg.TenantQuery4(t, staffCtx, f.db, f.svc.StartThread, f.chain.StudentID, f.chain.AccountID, "Hallo", "")
	require.NoError(t, err)
	ctx := communication.WithMessageSendID(staffCtx, "reply-before-guardian")
	sent, err := testpkg.TenantQuery3(t, ctx, f.db, f.svc.PostMessage, first.ThreadID, "Noch eine Information", 0)
	require.NoError(t, err)
	createGuardianMessage(t, f.db, f.chain, first.ThreadID, "Neue Frage")

	retry, err := testpkg.TenantQuery3(t, ctx, f.db, f.svc.PostMessage, first.ThreadID, "Noch eine Information", 0)
	require.NoError(t, err)
	require.Len(t, retry, 3)
	require.Equal(t, sent[1].ID, retry[1].ID)
	require.Equal(t, sent[1].CreatedAt, retry[1].CreatedAt)
	require.Equal(t, 2, parentEventCount(f.bc, realtime.EventParentMessage))

	_, err = testpkg.TenantQuery3(t, ctx, f.db, f.svc.PostMessage, first.ThreadID, "Anderer Inhalt", 0)
	require.ErrorIs(t, err, communication.ErrMessageSendConflict)
	newCtx := communication.WithMessageSendID(staffCtx, "new-reply-after-guardian")
	_, err = testpkg.TenantQuery3(t, newCtx, f.db, f.svc.PostMessage, first.ThreadID, "Neue Antwort", 0)
	require.ErrorIs(t, err, messaging.ErrHandledBoundaryRequired)
}

func TestGroupSendRetriesKeepOneThreadAndOneAudience(t *testing.T) {
	t.Parallel()
	f, run := newGroupFixture(t)
	input := communication.ParentGroupThreadInput{Target: classTarget("1a"), Title: "Ausflug", Body: "Bitte an Wasser denken"}
	ctx := communication.WithMessageSendID(adminCtx(t, f.staff), "group-start-retry")
	var first, retried *communication.ParentGroupThreadDetail
	require.NoError(t, run(ctx, func(tx context.Context) error {
		var err error
		first, err = f.svc.CreateParentGroupThread(tx, input)
		return err
	}))
	require.NoError(t, run(ctx, func(tx context.Context) error {
		var err error
		retried, err = f.svc.CreateParentGroupThread(tx, input)
		return err
	}))
	require.Equal(t, first.Thread.ID, retried.Thread.ID)
	require.Len(t, retried.Messages, 1)
	require.Equal(t, first.Messages[0].ID, retried.Messages[0].ID)
	replyCtx := communication.WithMessageSendID(adminCtx(t, f.staff), "group-reply-retry")
	var sent *communication.ParentGroupThreadDetail
	require.NoError(t, run(replyCtx, func(tx context.Context) error {
		var err error
		sent, err = f.svc.PostParentGroupMessage(tx, first.Thread.ID, "Treffpunkt 8 Uhr")
		return err
	}))
	require.NoError(t, run(adminCtx(t, f.staff), func(tx context.Context) error {
		_, err := f.svc.CloseParentGroupThread(tx, first.Thread.ID)
		return err
	}))
	require.NoError(t, run(replyCtx, func(tx context.Context) error {
		var err error
		retried, err = f.svc.PostParentGroupMessage(tx, first.Thread.ID, "Treffpunkt 8 Uhr")
		return err
	}))
	require.Len(t, retried.Messages, 2)
	require.Equal(t, sent.Messages[1].ID, retried.Messages[1].ID)
}

func assertConcurrentSendIdentity(t *testing.T, send func() (int64, int, error)) {
	t.Helper()
	type result struct {
		id    int64
		count int
		err   error
	}
	results := make(chan result, 4)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() { <-start; id, count, err := send(); results <- result{id, count, err} })
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
}

func TestParentConcurrentSendRetries(t *testing.T) {
	t.Parallel()
	f := newFixture(t, true)
	ctx := communication.WithMessageSendID(adminCtx(t, f.staffAccount), "parallel-parent-send")
	assertConcurrentSendIdentity(t, func() (int64, int, error) {
		view, err := testpkg.TenantQuery4(t, ctx, f.db, f.svc.StartThread, f.chain.StudentID, f.chain.AccountID, "Hallo", "")
		if err != nil {
			return 0, 0, err
		}
		return view.Messages[0].ID, len(view.Messages), nil
	})
	require.Equal(t, 1, parentEventCount(f.bc, realtime.EventParentMessage))
}

func TestGroupConcurrentSendRetries(t *testing.T) {
	t.Parallel()
	f, run := newGroupFixture(t)
	ctx := communication.WithMessageSendID(adminCtx(t, f.staff), "parallel-group-send")
	assertConcurrentSendIdentity(t, func() (int64, int, error) {
		var view *communication.ParentGroupThreadDetail
		err := run(ctx, func(tx context.Context) error {
			var err error
			view, err = f.svc.CreateParentGroupThread(tx, communication.ParentGroupThreadInput{Target: classTarget("1a"), Title: "Ausflug", Body: "Bitte Wasser mitbringen"})
			return err
		})
		if err != nil {
			return 0, 0, err
		}
		return view.Messages[0].ID, len(view.Messages), nil
	})
	require.NoError(t, run(adminCtx(t, f.staff), func(tx context.Context) error {
		threads, err := f.svc.ListParentGroupThreads(tx)
		require.NoError(t, err)
		require.Len(t, threads, 1)
		return nil
	}))
}
