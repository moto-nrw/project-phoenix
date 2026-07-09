package parentmessaging

// Unit tests for the shared parent-OGS messaging core. These exercise the
// load-bearing rules — fail-OPEN feature gate, append-then-touch, mark-to-newest
// (never NOW(), never the reader's own row), read-receipt stamping, and the
// fire-and-forget SSE fan-out guards — with narrow fakes, no DB. The rules live
// here precisely so a regression on either side (staff or parent) is caught once.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// --- fakes --------------------------------------------------------------

type fakeSettings struct {
	val bool
	err error
}

func (f fakeSettings) ResolveBool(context.Context, string) (bool, error) { return f.val, f.err }

type fakeTenantSettings struct {
	val bool
	err error
}

func (f fakeTenantSettings) ResolveBoolForTenant(context.Context, int64, string) (bool, error) {
	return f.val, f.err
}

type fakeMsgRepo struct {
	usersModels.ParentMessageRepository
	createErr error
	nextID    int64
	stampAt   time.Time
	created   []*usersModels.ParentMessage
}

func (f *fakeMsgRepo) Create(_ context.Context, m *usersModels.ParentMessage) error {
	if f.createErr != nil {
		return f.createErr
	}
	// Simulate the DB stamping id + created_at so AppendMessage's touch reads them.
	f.nextID++
	m.ID = f.nextID
	if !f.stampAt.IsZero() {
		m.CreatedAt = f.stampAt
	}
	f.created = append(f.created, m)
	return nil
}

type touchCall struct {
	threadID, messageID int64
	at                  time.Time
	kind, body          string
}

type fakeThreadRepo struct {
	usersModels.ParentMessageThreadRepository
	touchErr     error
	touched      *touchCall
	guardians    []*usersModels.MessageableGuardian
	guardiansErr error
}

func (f *fakeThreadRepo) ListGuardiansForStudent(context.Context, int64) ([]*usersModels.MessageableGuardian, error) {
	return f.guardians, f.guardiansErr
}

func (f *fakeThreadRepo) TouchLastMessage(_ context.Context, threadID int64, at time.Time, messageID int64, kind, body string) error {
	if f.touchErr != nil {
		return f.touchErr
	}
	f.touched = &touchCall{threadID, messageID, at, kind, body}
	return nil
}

type markCall struct {
	tenantID, threadID, accountID, messageID int64
	readAt                                   time.Time
}

type fakeReadRepo struct {
	usersModels.ParentMessageReadRepository
	markAdvanced      bool
	markErr           error
	marks             []markCall
	staffCursor       *usersModels.ReadCursor
	staffCursorErr    error
	guardianCursor    *usersModels.ReadCursor
	guardianCursorErr error
}

func (f *fakeReadRepo) MarkReadUpTo(_ context.Context, tenantID, threadID, accountID int64, readAt time.Time, messageID int64) (bool, error) {
	f.marks = append(f.marks, markCall{tenantID, threadID, accountID, messageID, readAt})
	return f.markAdvanced, f.markErr
}

func (f *fakeReadRepo) LatestReadCursorByOther(context.Context, int64, int64) (*usersModels.ReadCursor, error) {
	return f.staffCursor, f.staffCursorErr
}

func (f *fakeReadRepo) GuardianReadCursor(context.Context, int64) (*usersModels.ReadCursor, error) {
	return f.guardianCursor, f.guardianCursorErr
}

func msg(id, sender int64, kind string) *usersModels.ParentMessage {
	m := &usersModels.ParentMessage{SenderAccountID: sender, SenderKind: kind, Body: "x"}
	m.ID = id
	m.CreatedAt = time.Now()
	return m
}

// --- MessagingEnabled ---------------------------------------------------

func TestMessagingEnabled(t *testing.T) {
	ctx := context.Background()
	assert.True(t, MessagingEnabled(ctx, fakeSettings{val: true}, nil))
	assert.False(t, MessagingEnabled(ctx, fakeSettings{val: false}, nil))
	// Fail OPEN: a transient resolve error counts as enabled so the read and write
	// paths can't disagree during a config-DB blip.
	assert.True(t, MessagingEnabled(ctx, fakeSettings{err: errors.New("settings down")}, nil),
		"a resolve error must fail open (enabled)")
}

func TestMessagingEnabledForTenant(t *testing.T) {
	ctx := context.Background()
	assert.True(t, MessagingEnabledForTenant(ctx, fakeTenantSettings{val: true}, 5, nil))
	assert.False(t, MessagingEnabledForTenant(ctx, fakeTenantSettings{val: false}, 5, nil))
	assert.True(t, MessagingEnabledForTenant(ctx, fakeTenantSettings{err: errors.New("down")}, 5, nil),
		"a per-tenant resolve error must fail open")
}

// --- GuardianHasChildAccess ---------------------------------------------

// TestGuardianHasChildAccess pins the linked-guardian / parent_portal.access
// gate shared by the emitter's staff-decision-pill suppression and the
// care-schedule request approve gate: an account still present in the
// messageable-guardian list has access; an unlinked/revoked account (absent
// from the list) does not; a repo error propagates; and an unwired emitter
// returns a configuration error rather than silently granting access.
func TestGuardianHasChildAccess(t *testing.T) {
	ctx := context.Background()

	linked := &Emitter{threadRepo: &fakeThreadRepo{
		guardians: []*usersModels.MessageableGuardian{{AccountID: 7}, {AccountID: 9}},
	}}
	ok, err := linked.GuardianHasChildAccess(ctx, 1, 7)
	require.NoError(t, err)
	assert.True(t, ok, "an account in the messageable-guardian list has access")

	ok, err = linked.GuardianHasChildAccess(ctx, 1, 42)
	require.NoError(t, err)
	assert.False(t, ok, "a revoked/unlinked account is absent from the list → no access")

	failing := &Emitter{threadRepo: &fakeThreadRepo{guardiansErr: errors.New("db down")}}
	_, err = failing.GuardianHasChildAccess(ctx, 1, 7)
	require.Error(t, err, "a repo error must propagate, not be read as revoked")

	var unwired *Emitter
	_, err = unwired.GuardianHasChildAccess(ctx, 1, 7)
	require.Error(t, err, "an unwired emitter must error, never silently grant access")
}

// --- AppendMessage ------------------------------------------------------

func TestAppendMessage_PersistsThenTouchesOffDBStampedRow(t *testing.T) {
	stamp := time.Now().Add(-time.Minute).Truncate(time.Microsecond)
	mr := &fakeMsgRepo{stampAt: stamp}
	tr := &fakeThreadRepo{}
	m := &usersModels.ParentMessage{ThreadID: 42, SenderKind: usersModels.ParentMessageSenderStaff, Body: "Hallo"}

	require.NoError(t, AppendMessage(context.Background(), mr, tr, m))
	require.Len(t, mr.created, 1)
	require.NotNil(t, tr.touched)
	// The thread touch must be driven off the row's DB-stamped created_at + id,
	// NOT a fresh app clock — otherwise the monotonic preview guard desyncs.
	assert.Equal(t, stamp, tr.touched.at)
	assert.Equal(t, m.ID, tr.touched.messageID)
	assert.Equal(t, usersModels.ParentMessageSenderStaff, tr.touched.kind)
	assert.Equal(t, "Hallo", tr.touched.body)
}

func TestAppendMessage_CreateErrorSkipsTouch(t *testing.T) {
	mr := &fakeMsgRepo{createErr: errors.New("insert failed")}
	tr := &fakeThreadRepo{}
	err := AppendMessage(context.Background(), mr, tr, &usersModels.ParentMessage{})
	require.Error(t, err)
	assert.Nil(t, tr.touched, "a failed insert must not touch the thread preview")
}

// --- isTerminalRequestEvent ---------------------------------------------

// TestIsTerminalRequestEvent pins the gate that lets a request-closing pill
// through EmitChildEvent while messaging is disabled: ONLY a request_status
// event that references the request row (ref_table + ref_id). Everything else —
// the request_created pill, self-service mirrors, and a status event with no ref
// — stays dropped, so a disabled school never accumulates threads or dangling
// pills.
func TestIsTerminalRequestEvent(t *testing.T) {
	refID := int64(7)
	terminal := ChildEvent{
		EventType: usersModels.ParentMessageEventRequestStatus,
		RefTable:  "schedule.care_schedule_change_requests",
		RefID:     &refID,
	}
	assert.True(t, isTerminalRequestEvent(terminal), "a request_status event with a ref closes a request and must reconcile")

	assert.False(t, isTerminalRequestEvent(ChildEvent{
		EventType: usersModels.ParentMessageEventRequestCreated, RefTable: "schedule.care_schedule_change_requests", RefID: &refID,
	}), "a request_created pill carries no reconcile obligation")
	assert.False(t, isTerminalRequestEvent(ChildEvent{
		EventType: "sick_note", RefTable: "x", RefID: &refID,
	}), "a self-service mirror is not a terminal request event")
	assert.False(t, isTerminalRequestEvent(ChildEvent{
		EventType: usersModels.ParentMessageEventRequestStatus, RefTable: "schedule.care_schedule_change_requests",
	}), "a status event without a ref_id cannot locate a created pill to close")
	assert.False(t, isTerminalRequestEvent(ChildEvent{
		EventType: usersModels.ParentMessageEventRequestStatus, RefID: &refID,
	}), "a status event without a ref_table cannot locate a created pill to close")
}

// --- MarkReadToNewest ---------------------------------------------------

func TestMarkReadToNewest_AdvancesToNewestCounterpartMessage(t *testing.T) {
	reader := int64(100)
	// Oldest-first snapshot: two counterpart (guardian) messages then the reader's
	// own just-sent staff message. The cursor must land on the newest COUNTERPART
	// (id 2), never the reader's own id-3 row.
	oldestGuardian := msg(1, 200, usersModels.ParentMessageSenderGuardian)
	newestGuardian := msg(2, 200, usersModels.ParentMessageSenderGuardian)
	readerOwn := msg(3, reader, usersModels.ParentMessageSenderStaff)
	messages := []*usersModels.ParentMessage{oldestGuardian, newestGuardian, readerOwn}

	rr := &fakeReadRepo{markAdvanced: true}
	advanced, err := MarkReadToNewest(context.Background(), rr, 1, 42, reader, true, messages)
	require.NoError(t, err)
	assert.True(t, advanced)
	require.Len(t, rr.marks, 1)
	assert.Equal(t, newestGuardian.ID, rr.marks[0].messageID, "cursor must bound to the newest counterpart message the reader did not author, never the reader's own")
}

func TestMarkReadToNewest_DoesNotAdvancePastCounterpartViaOtherSideRow(t *testing.T) {
	reader := int64(100)
	// A staff reader's snapshot: a guardian message, then a LATER staff row from a
	// DIFFERENT staff account and a LATER system event the staff side triggered.
	// Neither later row is a counterpart (the unread query only counts guardian-side
	// rows), so the cursor must bound to the guardian message (id 1) — not the later
	// staff/system rows. Binding past them would skip a guardian message still
	// committing with an earlier created_at in a concurrent send/read race.
	guardian := msg(1, 200, usersModels.ParentMessageSenderGuardian)
	otherStaff := msg(2, 300, usersModels.ParentMessageSenderStaff)
	staffEvent := msg(3, 300, usersModels.ParentMessageSenderSystem)
	staffEvent.EventActorKind = usersModels.ParentMessageSenderStaff
	messages := []*usersModels.ParentMessage{guardian, otherStaff, staffEvent}

	rr := &fakeReadRepo{markAdvanced: true}
	advanced, err := MarkReadToNewest(context.Background(), rr, 1, 42, reader, true, messages)
	require.NoError(t, err)
	assert.True(t, advanced)
	require.Len(t, rr.marks, 1)
	assert.Equal(t, guardian.ID, rr.marks[0].messageID,
		"cursor must bound to the newest counterpart row, never a later staff/system row")
}

func TestMarkReadToNewest_GuardianReaderBoundsToStaffSide(t *testing.T) {
	reader := int64(100)
	// A guardian reader: a staff message and a staff-triggered system event are the
	// counterpart; the reader's own guardian message is not. The cursor must land on
	// the newest staff-side row (the system event, id 3).
	staffMsg := msg(1, 300, usersModels.ParentMessageSenderStaff)
	readerOwn := msg(2, reader, usersModels.ParentMessageSenderGuardian)
	staffEvent := msg(3, 300, usersModels.ParentMessageSenderSystem)
	staffEvent.EventActorKind = usersModels.ParentMessageSenderStaff
	messages := []*usersModels.ParentMessage{staffMsg, readerOwn, staffEvent}

	rr := &fakeReadRepo{markAdvanced: true}
	advanced, err := MarkReadToNewest(context.Background(), rr, 1, 42, reader, false, messages)
	require.NoError(t, err)
	assert.True(t, advanced)
	require.Len(t, rr.marks, 1)
	assert.Equal(t, staffEvent.ID, rr.marks[0].messageID,
		"guardian reader's cursor bounds to the newest staff-side row")
}

func TestMarkReadToNewest_DualRoleOwnSystemEventStillAdvances(t *testing.T) {
	reader := int64(100)
	// Dual-role staff+guardian account reading the GUARDIAN portal. It triggered a
	// confirm/reject as STAFF, so the system event carries its OWN account in
	// SenderAccountID but is attributed to the staff side via EventActorKind. The
	// unread SQL counts it (notReaderAuthored exempts system rows), so the cursor
	// MUST advance onto it — a bare SenderAccountID == reader skip stranded it as
	// permanently unread once viewed.
	guardianOwn := msg(1, reader, usersModels.ParentMessageSenderGuardian)
	ownStaffEvent := msg(2, reader, usersModels.ParentMessageSenderSystem)
	ownStaffEvent.EventActorKind = usersModels.ParentMessageSenderStaff
	messages := []*usersModels.ParentMessage{guardianOwn, ownStaffEvent}

	rr := &fakeReadRepo{markAdvanced: true}
	advanced, err := MarkReadToNewest(context.Background(), rr, 1, 42, reader, false, messages)
	require.NoError(t, err)
	assert.True(t, advanced)
	require.Len(t, rr.marks, 1)
	assert.Equal(t, ownStaffEvent.ID, rr.marks[0].messageID,
		"a dual-role account's own staff-triggered system event must advance the guardian-side cursor, not stay stuck unread")
}

func TestMarkReadToNewest_DoesNotAdvanceOntoRequestCreatedPill(t *testing.T) {
	reader := int64(100)
	// A staff reader's snapshot: an unread guardian message, then a LATER
	// request_created pill (guardian-triggered system event). The unread SQL
	// excludes request_created (it is surfaced on the queue badge, not the chat),
	// so the cursor must bound to the guardian message (id 1) — NOT the later pill.
	// Advancing onto the pill would skip the guardian message when it commits with
	// an earlier created_at in a concurrent send/read race, and it would never
	// resurface as unread.
	guardian := msg(1, 200, usersModels.ParentMessageSenderGuardian)
	requestCreated := msg(2, 200, usersModels.ParentMessageSenderSystem)
	requestCreated.EventActorKind = usersModels.ParentMessageSenderGuardian
	requestCreated.EventType = usersModels.ParentMessageEventRequestCreated
	messages := []*usersModels.ParentMessage{guardian, requestCreated}

	rr := &fakeReadRepo{markAdvanced: true}
	advanced, err := MarkReadToNewest(context.Background(), rr, 1, 42, reader, true, messages)
	require.NoError(t, err)
	assert.True(t, advanced)
	require.Len(t, rr.marks, 1)
	assert.Equal(t, guardian.ID, rr.marks[0].messageID,
		"cursor must skip a request_created pill in lock-step with the unread SQL, bounding to the newest counted counterpart")
}

func TestMarkReadToNewest_RequestCreatedOnlyDoesNotMark(t *testing.T) {
	reader := int64(100)
	// The newest (and only counterpart-side) row is a request_created pill. Since
	// it is excluded from the unread set, there is nothing to mark seen and the
	// cursor stays untouched — matching a snapshot with no counted counterpart.
	requestCreated := msg(1, 200, usersModels.ParentMessageSenderSystem)
	requestCreated.EventActorKind = usersModels.ParentMessageSenderGuardian
	requestCreated.EventType = usersModels.ParentMessageEventRequestCreated

	rr := &fakeReadRepo{markAdvanced: true}
	advanced, err := MarkReadToNewest(context.Background(), rr, 1, 42, reader, true, []*usersModels.ParentMessage{requestCreated})
	require.NoError(t, err)
	assert.False(t, advanced)
	assert.Empty(t, rr.marks, "a lone request_created pill is not counted, so no cursor write")
}

func TestMarkReadToNewest_OwnOnlyOrEmptyDoesNotMark(t *testing.T) {
	reader := int64(100)
	rr := &fakeReadRepo{}
	// Empty snapshot.
	advanced, err := MarkReadToNewest(context.Background(), rr, 1, 42, reader, true, nil)
	require.NoError(t, err)
	assert.False(t, advanced)
	// Only the reader's own messages → nothing to mark seen.
	own := []*usersModels.ParentMessage{msg(1, reader, usersModels.ParentMessageSenderStaff)}
	advanced, err = MarkReadToNewest(context.Background(), rr, 1, 42, reader, true, own)
	require.NoError(t, err)
	assert.False(t, advanced)
	assert.Empty(t, rr.marks, "no counterpart message means no cursor write")
}

func TestMarkReadToNewest_PropagatesRepoError(t *testing.T) {
	rr := &fakeReadRepo{markErr: errors.New("upsert failed")}
	messages := []*usersModels.ParentMessage{msg(1, 200, usersModels.ParentMessageSenderGuardian)}
	_, err := MarkReadToNewest(context.Background(), rr, 1, 42, 100, true, messages)
	require.Error(t, err)
}

// --- Decorate*ReadReceipts ----------------------------------------------

func TestDecorateReadReceipts_StampsGuardianMessagesUpToStaffCursor(t *testing.T) {
	guardianMsg := msg(1, 200, usersModels.ParentMessageSenderGuardian)
	guardianMsg.CreatedAt = time.Now().Add(-time.Hour)
	messages := []*usersModels.ParentMessage{guardianMsg}
	// Staff has read up to a cursor at/after the message → ReadByStaff stamped.
	rr := &fakeReadRepo{staffCursor: &usersModels.ReadCursor{LastReadAt: time.Now(), LastReadMessageID: 1}}
	DecorateReadReceipts(context.Background(), rr, nil, 42, 200, messages)
	assert.True(t, messages[0].ReadByStaff, "guardian message read by staff gets the 'OGS hat gelesen' receipt")
}

func TestDecorateReadReceipts_LookupErrorLeavesUnstamped(t *testing.T) {
	messages := []*usersModels.ParentMessage{msg(1, 200, usersModels.ParentMessageSenderGuardian)}
	rr := &fakeReadRepo{staffCursorErr: errors.New("cursor lookup failed")}
	DecorateReadReceipts(context.Background(), rr, nil, 42, 200, messages)
	assert.False(t, messages[0].ReadByStaff, "a lookup error must hide the receipt, not blank the chat")
}

func TestDecorateGuardianReadReceipts_StampsStaffMessagesUpToGuardianCursor(t *testing.T) {
	staffMsg := msg(1, 300, usersModels.ParentMessageSenderStaff)
	staffMsg.CreatedAt = time.Now().Add(-time.Hour)
	messages := []*usersModels.ParentMessage{staffMsg}
	rr := &fakeReadRepo{guardianCursor: &usersModels.ReadCursor{LastReadAt: time.Now(), LastReadMessageID: 1}}
	DecorateGuardianReadReceipts(context.Background(), rr, nil, 42, messages)
	assert.True(t, messages[0].ReadByGuardian, "staff message read by the guardian gets the 'von den Eltern gelesen' receipt")
}

func TestDecorateGuardianReadReceipts_LookupErrorLeavesUnstamped(t *testing.T) {
	messages := []*usersModels.ParentMessage{msg(1, 300, usersModels.ParentMessageSenderStaff)}
	rr := &fakeReadRepo{guardianCursorErr: errors.New("cursor lookup failed")}
	DecorateGuardianReadReceipts(context.Background(), rr, nil, 42, messages)
	assert.False(t, messages[0].ReadByGuardian)
}

// --- Broadcast / BroadcastRead ------------------------------------------

func TestBroadcast_FiresParentMessageEvent(t *testing.T) {
	bc := testpkg.NewRecordingBroadcaster()
	Broadcast(bc, nil, 1, 200, 42, 7)
	require.Len(t, bc.Events(), 1)
	assert.Equal(t, realtime.EventParentMessage, bc.Events()[0].Type)
}

func TestBroadcastRead_FiresReadEvent(t *testing.T) {
	bc := testpkg.NewRecordingBroadcaster()
	BroadcastRead(bc, nil, 1, 200, 42, 7)
	require.Len(t, bc.Events(), 1)
	assert.Equal(t, realtime.EventParentMessageRead, bc.Events()[0].Type)
}

func TestBroadcast_NilBroadcasterAndBadTenantAreNoOps(t *testing.T) {
	// nil broadcaster: nothing to call, no panic.
	Broadcast(nil, nil, 1, 200, 42, 7)
	BroadcastRead(nil, nil, 1, 200, 42, 7)
	// Non-positive tenant: a no-op even with a live broadcaster (never fan out an
	// un-scoped event across the whole platform).
	bc := testpkg.NewRecordingBroadcaster()
	Broadcast(bc, nil, 0, 200, 42, 7)
	BroadcastRead(bc, nil, -1, 200, 42, 7)
	assert.Empty(t, bc.Events(), "a non-positive tenant must not broadcast")
}

func TestBroadcast_DeliveryErrorIsSwallowed(t *testing.T) {
	// A delivery error is logged, never returned (the message already committed).
	bc := testpkg.NewRecordingBroadcaster()
	bc.Err = errors.New("client buffer full")
	Broadcast(bc, nil, 1, 200, 42, 7)
	BroadcastRead(bc, nil, 1, 200, 42, 7)
	assert.Len(t, bc.Events(), 2, "the fan-out is attempted; the error does not propagate")
}
