package messaging

// Unit tests for the shared parent-OGS conversation core. These exercise the
// load-bearing rules — append-then-touch, mark-to-newest (never NOW(), never
// the reader's own row), read-receipt stamping, and the fire-and-forget SSE
// fan-out guards — with narrow fakes, no DB. The rules live here precisely so
// a regression on either side (staff or parent) is caught once.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/realtime"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// --- fakes --------------------------------------------------------------

type fakeMsgRepo struct {
	ParentMessageRepository
	createErr    error
	nextID       int64
	stampAt      time.Time
	created      []*ParentMessage
	beforeCreate func()
}

func (f *fakeMsgRepo) Create(_ context.Context, m *ParentMessage) error {
	if f.beforeCreate != nil {
		f.beforeCreate()
	}
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
	ParentMessageThreadRepository
	touchErr error
	touched  *touchCall
	locked   bool
	lockErr  error
}

func (f *fakeThreadRepo) LockForMessageAppend(context.Context, int64) error {
	f.locked = true
	return f.lockErr
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
	ParentMessageReadRepository
	markAdvanced      bool
	markErr           error
	marks             []markCall
	staffCursor       *ReadCursor
	staffCursorErr    error
	guardianCursor    *ReadCursor
	guardianCursorErr error
	handled           []markCall
	handledErr        error
}

func (f *fakeReadRepo) MarkReadUpTo(_ context.Context, tenantID, threadID, accountID int64, readAt time.Time, messageID int64) (bool, error) {
	f.marks = append(f.marks, markCall{tenantID, threadID, accountID, messageID, readAt})
	return f.markAdvanced, f.markErr
}

func (f *fakeReadRepo) MarkStaffHandledUpTo(_ context.Context, tenantID, threadID int64, handledAt time.Time, messageID int64) error {
	f.handled = append(f.handled, markCall{tenantID: tenantID, threadID: threadID, messageID: messageID, readAt: handledAt})
	return f.handledErr
}

func (f *fakeReadRepo) LatestReadCursorByOther(context.Context, int64, int64) (*ReadCursor, error) {
	return f.staffCursor, f.staffCursorErr
}

func (f *fakeReadRepo) GuardianReadCursor(context.Context, int64) (*ReadCursor, error) {
	return f.guardianCursor, f.guardianCursorErr
}

func msg(id, sender int64, kind string) *ParentMessage {
	m := &ParentMessage{SenderAccountID: sender, SenderKind: kind, Body: "x"}
	m.ID = id
	m.CreatedAt = time.Now()
	return m
}

// --- AppendMessage ------------------------------------------------------

func TestAppendMessage_PersistsThenTouchesOffDBStampedRow(t *testing.T) {
	t.Parallel()

	stamp := time.Now().Add(-time.Minute).Truncate(time.Microsecond)
	tr := &fakeThreadRepo{}
	mr := &fakeMsgRepo{stampAt: stamp, beforeCreate: func() {
		assert.True(t, tr.locked, "the thread must be locked before the message insert")
	}}
	m := &ParentMessage{ThreadID: 42, SenderKind: ParentMessageSenderStaff, Body: "Hallo"}

	require.NoError(t, AppendMessage(context.Background(), mr, tr, m))
	require.Len(t, mr.created, 1)
	require.NotNil(t, tr.touched)
	// The thread touch must be driven off the row's DB-stamped created_at + id,
	// NOT a fresh app clock — otherwise the monotonic preview guard desyncs.
	assert.Equal(t, stamp, tr.touched.at)
	assert.Equal(t, m.ID, tr.touched.messageID)
	assert.Equal(t, ParentMessageSenderStaff, tr.touched.kind)
	assert.Equal(t, "Hallo", tr.touched.body)
}

func TestAppendMessage_LockErrorSkipsInsert(t *testing.T) {
	t.Parallel()

	mr := &fakeMsgRepo{}
	tr := &fakeThreadRepo{lockErr: errors.New("lock failed")}
	err := AppendMessage(context.Background(), mr, tr, &ParentMessage{ThreadID: 42})
	require.Error(t, err)
	assert.Empty(t, mr.created)
}

func TestAppendMessage_CreateErrorSkipsTouch(t *testing.T) {
	t.Parallel()

	mr := &fakeMsgRepo{createErr: errors.New("insert failed")}
	tr := &fakeThreadRepo{}
	err := AppendMessage(context.Background(), mr, tr, &ParentMessage{})
	require.Error(t, err)
	assert.Nil(t, tr.touched, "a failed insert must not touch the thread preview")
}

// TestConversationCore_BindsTheSharedRules pins that the parent-side port
// runs the very same functions: a bound append locks, inserts, and touches
// exactly like the package-level rule.
func TestConversationCore_BindsTheSharedRules(t *testing.T) {
	t.Parallel()

	tr := &fakeThreadRepo{}
	mr := &fakeMsgRepo{}
	rr := &fakeReadRepo{markAdvanced: true}
	core := NewConversationCore(tr, mr, rr, nil, nil)

	m := &ParentMessage{ThreadID: 42, SenderKind: ParentMessageSenderGuardian, Body: "Hallo"}
	require.NoError(t, core.AppendMessage(context.Background(), m))
	assert.True(t, tr.locked)
	require.NotNil(t, tr.touched)

	advanced, err := core.MarkReadToNewest(context.Background(), 1, 42, 100, false, []*ParentMessage{msg(1, 300, ParentMessageSenderStaff)})
	require.NoError(t, err)
	assert.True(t, advanced)
	require.Len(t, rr.marks, 1)

	// A nil broadcaster is a no-op, never a panic.
	core.Broadcast(1, 200, 42, 7, communication.ParentMessageAudienceTeam)
	core.BroadcastRead(1, 200, 42, 7, communication.ParentMessageAudienceTeam)
}

// --- MarkReadToNewest ---------------------------------------------------

func TestMarkReadToNewest_AdvancesToNewestCounterpartMessage(t *testing.T) {
	t.Parallel()

	reader := int64(100)
	// Oldest-first snapshot: two counterpart (guardian) messages then the reader's
	// own just-sent staff message. The cursor must land on the newest COUNTERPART
	// (id 2), never the reader's own id-3 row.
	oldestGuardian := msg(1, 200, ParentMessageSenderGuardian)
	newestGuardian := msg(2, 200, ParentMessageSenderGuardian)
	readerOwn := msg(3, reader, ParentMessageSenderStaff)
	messages := []*ParentMessage{oldestGuardian, newestGuardian, readerOwn}

	rr := &fakeReadRepo{markAdvanced: true}
	advanced, err := MarkReadToNewest(context.Background(), rr, 1, 42, reader, true, messages)
	require.NoError(t, err)
	assert.True(t, advanced)
	require.Len(t, rr.marks, 1)
	assert.Equal(t, newestGuardian.ID, rr.marks[0].messageID, "cursor must bound to the newest counterpart message the reader did not author, never the reader's own")
}

func TestMarkReadToNewest_DoesNotAdvancePastCounterpartViaOtherSideRow(t *testing.T) {
	t.Parallel()

	reader := int64(100)
	// A staff reader's snapshot: a guardian message, then a LATER staff row from a
	// DIFFERENT staff account and a LATER system event the staff side triggered.
	// Neither later row is a counterpart (the unread query only counts guardian-side
	// rows), so the cursor must bound to the guardian message (id 1) — not the later
	// staff/system rows. Binding past them would skip a guardian message still
	// committing with an earlier created_at in a concurrent send/read race.
	guardian := msg(1, 200, ParentMessageSenderGuardian)
	otherStaff := msg(2, 300, ParentMessageSenderStaff)
	staffEvent := msg(3, 300, ParentMessageSenderSystem)
	staffEvent.EventActorKind = ParentMessageSenderStaff
	messages := []*ParentMessage{guardian, otherStaff, staffEvent}

	rr := &fakeReadRepo{markAdvanced: true}
	advanced, err := MarkReadToNewest(context.Background(), rr, 1, 42, reader, true, messages)
	require.NoError(t, err)
	assert.True(t, advanced)
	require.Len(t, rr.marks, 1)
	assert.Equal(t, guardian.ID, rr.marks[0].messageID,
		"cursor must bound to the newest counterpart row, never a later staff/system row")
}

func TestMarkReadToNewest_GuardianReaderBoundsToStaffSide(t *testing.T) {
	t.Parallel()

	reader := int64(100)
	// A guardian reader: a staff message and a staff-triggered system event are the
	// counterpart; the reader's own guardian message is not. The cursor must land on
	// the newest staff-side row (the system event, id 3).
	staffMsg := msg(1, 300, ParentMessageSenderStaff)
	readerOwn := msg(2, reader, ParentMessageSenderGuardian)
	staffEvent := msg(3, 300, ParentMessageSenderSystem)
	staffEvent.EventActorKind = ParentMessageSenderStaff
	messages := []*ParentMessage{staffMsg, readerOwn, staffEvent}

	rr := &fakeReadRepo{markAdvanced: true}
	advanced, err := MarkReadToNewest(context.Background(), rr, 1, 42, reader, false, messages)
	require.NoError(t, err)
	assert.True(t, advanced)
	require.Len(t, rr.marks, 1)
	assert.Equal(t, staffEvent.ID, rr.marks[0].messageID,
		"guardian reader's cursor bounds to the newest staff-side row")
}

func TestMarkReadToNewest_DualRoleOwnSystemEventStillAdvances(t *testing.T) {
	t.Parallel()

	reader := int64(100)
	// Dual-role staff+guardian account reading the GUARDIAN portal. It triggered a
	// confirm/reject as STAFF, so the system event carries its OWN account in
	// SenderAccountID but is attributed to the staff side via EventActorKind. The
	// unread SQL counts it (notReaderAuthored exempts system rows), so the cursor
	// MUST advance onto it — a bare SenderAccountID == reader skip stranded it as
	// permanently unread once viewed.
	guardianOwn := msg(1, reader, ParentMessageSenderGuardian)
	ownStaffEvent := msg(2, reader, ParentMessageSenderSystem)
	ownStaffEvent.EventActorKind = ParentMessageSenderStaff
	messages := []*ParentMessage{guardianOwn, ownStaffEvent}

	rr := &fakeReadRepo{markAdvanced: true}
	advanced, err := MarkReadToNewest(context.Background(), rr, 1, 42, reader, false, messages)
	require.NoError(t, err)
	assert.True(t, advanced)
	require.Len(t, rr.marks, 1)
	assert.Equal(t, ownStaffEvent.ID, rr.marks[0].messageID,
		"a dual-role account's own staff-triggered system event must advance the guardian-side cursor, not stay stuck unread")
}

func TestMarkReadToNewest_DoesNotAdvanceOntoRequestCreatedPill(t *testing.T) {
	t.Parallel()

	reader := int64(100)
	// A staff reader's snapshot: an unread guardian message, then a LATER
	// request_created pill (guardian-triggered system event). The unread SQL
	// excludes request_created (it is surfaced on the queue badge, not the chat),
	// so the cursor must bound to the guardian message (id 1) — NOT the later pill.
	// Advancing onto the pill would skip the guardian message when it commits with
	// an earlier created_at in a concurrent send/read race, and it would never
	// resurface as unread.
	guardian := msg(1, 200, ParentMessageSenderGuardian)
	requestCreated := msg(2, 200, ParentMessageSenderSystem)
	requestCreated.EventActorKind = ParentMessageSenderGuardian
	requestCreated.EventType = ParentMessageEventRequestCreated
	messages := []*ParentMessage{guardian, requestCreated}

	rr := &fakeReadRepo{markAdvanced: true}
	advanced, err := MarkReadToNewest(context.Background(), rr, 1, 42, reader, true, messages)
	require.NoError(t, err)
	assert.True(t, advanced)
	require.Len(t, rr.marks, 1)
	assert.Equal(t, guardian.ID, rr.marks[0].messageID,
		"cursor must skip a request_created pill in lock-step with the unread SQL, bounding to the newest counted counterpart")
}

func TestMarkReadToNewest_RequestCreatedOnlyDoesNotMark(t *testing.T) {
	t.Parallel()

	reader := int64(100)
	// The newest (and only counterpart-side) row is a request_created pill. Since
	// it is excluded from the unread set, there is nothing to mark seen and the
	// cursor stays untouched — matching a snapshot with no counted counterpart.
	requestCreated := msg(1, 200, ParentMessageSenderSystem)
	requestCreated.EventActorKind = ParentMessageSenderGuardian
	requestCreated.EventType = ParentMessageEventRequestCreated

	rr := &fakeReadRepo{markAdvanced: true}
	advanced, err := MarkReadToNewest(context.Background(), rr, 1, 42, reader, true, []*ParentMessage{requestCreated})
	require.NoError(t, err)
	assert.False(t, advanced)
	assert.Empty(t, rr.marks, "a lone request_created pill is not counted, so no cursor write")
}

func TestMarkReadToNewest_OwnOnlyOrEmptyDoesNotMark(t *testing.T) {
	t.Parallel()

	reader := int64(100)
	rr := &fakeReadRepo{}
	// Empty snapshot.
	advanced, err := MarkReadToNewest(context.Background(), rr, 1, 42, reader, true, nil)
	require.NoError(t, err)
	assert.False(t, advanced)
	// Only the reader's own messages → nothing to mark seen.
	own := []*ParentMessage{msg(1, reader, ParentMessageSenderStaff)}
	advanced, err = MarkReadToNewest(context.Background(), rr, 1, 42, reader, true, own)
	require.NoError(t, err)
	assert.False(t, advanced)
	assert.Empty(t, rr.marks, "no counterpart message means no cursor write")
}

func TestMarkReadToNewest_PropagatesRepoError(t *testing.T) {
	t.Parallel()

	rr := &fakeReadRepo{markErr: errors.New("upsert failed")}
	messages := []*ParentMessage{msg(1, 200, ParentMessageSenderGuardian)}
	_, err := MarkReadToNewest(context.Background(), rr, 1, 42, 100, true, messages)
	require.Error(t, err)
}

func TestMarkStaffHandledToVisible_BoundsToVisibleGuardianActivity(t *testing.T) {
	t.Parallel()

	guardian := msg(1, 200, ParentMessageSenderGuardian)
	staffReply := msg(2, 100, ParentMessageSenderStaff)
	concurrentGuardian := msg(3, 200, ParentMessageSenderGuardian)
	requestCreated := msg(4, 200, ParentMessageSenderSystem)
	requestCreated.EventActorKind = ParentMessageSenderGuardian
	requestCreated.EventType = ParentMessageEventRequestCreated

	rr := &fakeReadRepo{}
	err := MarkStaffHandledToVisible(context.Background(), rr, 1, 42, staffReply.ID, []*ParentMessage{guardian, staffReply, concurrentGuardian, requestCreated})
	require.NoError(t, err)
	require.Len(t, rr.handled, 1)
	assert.Equal(t, guardian.ID, rr.handled[0].messageID, "the team boundary must stop at the client snapshot and leave concurrent guardian activity open")
}

func TestMarkStaffHandledToVisible_PropagatesRepoError(t *testing.T) {
	t.Parallel()

	rr := &fakeReadRepo{handledErr: errors.New("update failed")}
	err := MarkStaffHandledToVisible(context.Background(), rr, 1, 42, 1, []*ParentMessage{
		msg(1, 200, ParentMessageSenderGuardian),
	})
	require.Error(t, err)
}

// --- Decorate*ReadReceipts ----------------------------------------------

func TestDecorateReadReceipts_StampsGuardianMessagesUpToStaffCursor(t *testing.T) {
	t.Parallel()

	guardianMsg := msg(1, 200, ParentMessageSenderGuardian)
	guardianMsg.CreatedAt = time.Now().Add(-time.Hour)
	messages := []*ParentMessage{guardianMsg}
	// Staff has read up to a cursor at/after the message → ReadByStaff stamped.
	rr := &fakeReadRepo{staffCursor: &ReadCursor{LastReadAt: time.Now(), LastReadMessageID: 1}}
	DecorateReadReceipts(context.Background(), rr, nil, 42, 200, messages)
	assert.True(t, messages[0].ReadByStaff, "guardian message read by staff gets the 'OGS hat gelesen' receipt")
}

func TestDecorateReadReceipts_LookupErrorLeavesUnstamped(t *testing.T) {
	t.Parallel()

	messages := []*ParentMessage{msg(1, 200, ParentMessageSenderGuardian)}
	rr := &fakeReadRepo{staffCursorErr: errors.New("cursor lookup failed")}
	DecorateReadReceipts(context.Background(), rr, nil, 42, 200, messages)
	assert.False(t, messages[0].ReadByStaff, "a lookup error must hide the receipt, not blank the chat")
}

func TestDecorateGuardianReadReceipts_StampsStaffMessagesUpToGuardianCursor(t *testing.T) {
	t.Parallel()

	staffMsg := msg(1, 300, ParentMessageSenderStaff)
	staffMsg.CreatedAt = time.Now().Add(-time.Hour)
	messages := []*ParentMessage{staffMsg}
	rr := &fakeReadRepo{guardianCursor: &ReadCursor{LastReadAt: time.Now(), LastReadMessageID: 1}}
	DecorateGuardianReadReceipts(context.Background(), rr, nil, 42, messages)
	assert.True(t, messages[0].ReadByGuardian, "staff message read by the guardian gets the 'von den Eltern gelesen' receipt")
}

func TestDecorateGuardianReadReceipts_LookupErrorLeavesUnstamped(t *testing.T) {
	t.Parallel()

	messages := []*ParentMessage{msg(1, 300, ParentMessageSenderStaff)}
	rr := &fakeReadRepo{guardianCursorErr: errors.New("cursor lookup failed")}
	DecorateGuardianReadReceipts(context.Background(), rr, nil, 42, messages)
	assert.False(t, messages[0].ReadByGuardian)
}

// --- Broadcast / BroadcastRead ------------------------------------------

func TestBroadcast_FiresParentMessageEvent(t *testing.T) {
	t.Parallel()

	bc := testpkg.NewRecordingBroadcaster()
	Broadcast(bc, nil, 1, 200, 42, 7, communication.ParentMessageAudienceTeam)
	require.Len(t, bc.Events(), 1)
	assert.Equal(t, realtime.EventParentMessage, bc.Events()[0].Type)
}

// TestBroadcast_LeadershipConversationWakesOnlyLeadership pins the SSE
// routing of a leadership conversation (#5200): it never goes through the
// tenant-wide staff fan-out.
func TestBroadcast_LeadershipConversationWakesOnlyLeadership(t *testing.T) {
	t.Parallel()

	bc := testpkg.NewRecordingBroadcaster()
	Broadcast(bc, nil, 1, 200, 42, 7, communication.ParentMessageAudienceLeadership)
	BroadcastRead(bc, nil, 1, 200, 42, 7, communication.ParentMessageAudienceLeadership)
	assert.Empty(t, bc.CallsByMethod("parent"), "a leadership conversation must not reach every staff tab")
	leadership := bc.CallsByMethod("leadership")
	require.Len(t, leadership, 2)
	assert.Equal(t, int64(200), leadership[0].GuardianID, "the guardian's own tabs still wake")
	assert.Equal(t, realtime.EventParentMessage, leadership[0].Event.Type)
	assert.Equal(t, realtime.EventParentMessageRead, leadership[1].Event.Type)
}

func TestBroadcastRead_FiresReadEvent(t *testing.T) {
	t.Parallel()

	bc := testpkg.NewRecordingBroadcaster()
	BroadcastRead(bc, nil, 1, 200, 42, 7, communication.ParentMessageAudienceTeam)
	require.Len(t, bc.Events(), 1)
	assert.Equal(t, realtime.EventParentMessageRead, bc.Events()[0].Type)
}

func TestBroadcast_NilBroadcasterAndBadTenantAreNoOps(t *testing.T) {
	t.Parallel()

	// nil broadcaster: nothing to call, no panic.
	Broadcast(nil, nil, 1, 200, 42, 7, communication.ParentMessageAudienceTeam)
	BroadcastRead(nil, nil, 1, 200, 42, 7, communication.ParentMessageAudienceTeam)
	// Non-positive tenant: a no-op even with a live broadcaster (never fan out an
	// un-scoped event across the whole platform).
	bc := testpkg.NewRecordingBroadcaster()
	Broadcast(bc, nil, 0, 200, 42, 7, communication.ParentMessageAudienceTeam)
	BroadcastRead(bc, nil, -1, 200, 42, 7, communication.ParentMessageAudienceTeam)
	assert.Empty(t, bc.Events(), "a non-positive tenant must not broadcast")
}

func TestBroadcast_DeliveryErrorIsSwallowed(t *testing.T) {
	t.Parallel()

	// A delivery error is logged, never returned (the message already committed).
	bc := testpkg.NewRecordingBroadcaster()
	bc.Err = errors.New("client buffer full")
	Broadcast(bc, nil, 1, 200, 42, 7, communication.ParentMessageAudienceTeam)
	BroadcastRead(bc, nil, 1, 200, 42, 7, communication.ParentMessageAudienceTeam)
	assert.Len(t, bc.Events(), 2, "the fan-out is attempted; the error does not propagate")
}
