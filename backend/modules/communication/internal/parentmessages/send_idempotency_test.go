package messaging_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/communication"
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
