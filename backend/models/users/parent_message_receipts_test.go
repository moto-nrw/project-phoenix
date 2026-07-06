package users

// Pure-logic tests for the read-receipt stampers (parent_message.go). They pin
// the composite (created_at, id) tie-break and the POSITIVE authorship gate that
// keeps a 'system' message from ever being marked staff- or guardian-read.

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func receiptMsg(id int64, at time.Time, kind string) *ParentMessage {
	m := &ParentMessage{SenderKind: kind, Body: "x"}
	m.ID = id
	m.CreatedAt = at
	return m
}

func TestStampStaffReadReceipts_NilCutoffStampsNothing(t *testing.T) {
	msgs := []*ParentMessage{receiptMsg(1, time.Now(), ParentMessageSenderGuardian)}
	StampStaffReadReceipts(msgs, nil)
	assert.False(t, msgs[0].ReadByStaff, "no staff read cursor → no receipt")
}

func TestStampStaffReadReceipts_OnlyGuardianMessagesAtOrBeforeCursor(t *testing.T) {
	base := time.Now().Truncate(time.Microsecond)
	cursor := &ReadCursor{LastReadAt: base, LastReadMessageID: 10}

	beforeByTime := receiptMsg(5, base.Add(-time.Minute), ParentMessageSenderGuardian)
	afterByTime := receiptMsg(6, base.Add(time.Minute), ParentMessageSenderGuardian)
	staffMsg := receiptMsg(4, base.Add(-time.Minute), ParentMessageSenderStaff)
	systemMsg := receiptMsg(3, base.Add(-time.Minute), ParentMessageSenderSystem)

	msgs := []*ParentMessage{beforeByTime, afterByTime, staffMsg, systemMsg}
	StampStaffReadReceipts(msgs, cursor)

	assert.True(t, beforeByTime.ReadByStaff, "guardian message before the cursor is read")
	assert.False(t, afterByTime.ReadByStaff, "guardian message after the cursor stays unread")
	assert.False(t, staffMsg.ReadByStaff, "a staff message is never a guardian receipt")
	assert.False(t, systemMsg.ReadByStaff, "a system message must never be marked staff-read (positive gate)")
}

func TestStampStaffReadReceipts_TieBreakOnMessageID(t *testing.T) {
	at := time.Now().Truncate(time.Microsecond)
	cursor := &ReadCursor{LastReadAt: at, LastReadMessageID: 20}

	// Same instant as the cursor: id <= 20 is read, id > 20 is not.
	atOrUnder := receiptMsg(20, at, ParentMessageSenderGuardian)
	over := receiptMsg(21, at, ParentMessageSenderGuardian)
	StampStaffReadReceipts([]*ParentMessage{atOrUnder, over}, cursor)

	assert.True(t, atOrUnder.ReadByStaff, "tied instant with id == cursor id is read")
	assert.False(t, over.ReadByStaff, "tied instant with a higher id stays unread until the cursor reaches it")
}

func TestStampGuardianReadReceipts_OnlyStaffMessages(t *testing.T) {
	base := time.Now().Truncate(time.Microsecond)
	cursor := &ReadCursor{LastReadAt: base, LastReadMessageID: 10}

	staffRead := receiptMsg(5, base.Add(-time.Minute), ParentMessageSenderStaff)
	guardianOwn := receiptMsg(4, base.Add(-time.Minute), ParentMessageSenderGuardian)
	systemMsg := receiptMsg(3, base.Add(-time.Minute), ParentMessageSenderSystem)

	msgs := []*ParentMessage{staffRead, guardianOwn, systemMsg}
	StampGuardianReadReceipts(msgs, cursor)

	assert.True(t, staffRead.ReadByGuardian, "staff message read by the guardian is stamped")
	assert.False(t, guardianOwn.ReadByGuardian, "a guardian's own message is never a guardian-read receipt")
	assert.False(t, systemMsg.ReadByGuardian, "a system message must never be marked guardian-read")
}

func TestStampGuardianReadReceipts_NilCutoffStampsNothing(t *testing.T) {
	msgs := []*ParentMessage{receiptMsg(1, time.Now(), ParentMessageSenderStaff)}
	StampGuardianReadReceipts(msgs, nil)
	assert.False(t, msgs[0].ReadByGuardian)
}

func TestIsCounterpartMessage(t *testing.T) {
	now := time.Now()
	guardianMsg := receiptMsg(1, now, ParentMessageSenderGuardian)
	staffMsg := receiptMsg(2, now, ParentMessageSenderStaff)
	staffEvent := receiptMsg(3, now, ParentMessageSenderSystem)
	staffEvent.EventActorKind = ParentMessageSenderStaff
	guardianEvent := receiptMsg(4, now, ParentMessageSenderSystem)
	guardianEvent.EventActorKind = ParentMessageSenderGuardian

	// Staff reader: the guardian side is the counterpart (system events attributed
	// via EventActorKind, never SenderKind='system').
	assert.True(t, IsCounterpartMessage(guardianMsg, true), "guardian message is the staff reader's counterpart")
	assert.False(t, IsCounterpartMessage(staffMsg, true), "a staff message is not the staff reader's counterpart")
	assert.True(t, IsCounterpartMessage(guardianEvent, true), "a guardian-triggered system event is the staff reader's counterpart")
	assert.False(t, IsCounterpartMessage(staffEvent, true), "a staff-triggered system event is not the staff reader's counterpart")

	// Guardian reader: the staff side is the counterpart — exact mirror.
	assert.True(t, IsCounterpartMessage(staffMsg, false), "staff message is the guardian reader's counterpart")
	assert.False(t, IsCounterpartMessage(guardianMsg, false), "a guardian message is not the guardian reader's counterpart")
	assert.True(t, IsCounterpartMessage(staffEvent, false), "a staff-triggered system event is the guardian reader's counterpart")
	assert.False(t, IsCounterpartMessage(guardianEvent, false), "a guardian-triggered system event is not the guardian reader's counterpart")

	// A request_created pill (guardian-triggered system event) is EXCLUDED for
	// both readers — in lock-step with counterpartUnread's
	// `event_type IS DISTINCT FROM 'request_created'`. Otherwise MarkReadToNewest
	// would advance the staff read cursor onto it and skip an earlier, still
	// committing guardian message the unread SQL never counted.
	requestCreated := receiptMsg(5, now, ParentMessageSenderSystem)
	requestCreated.EventActorKind = ParentMessageSenderGuardian
	requestCreated.EventType = ParentMessageEventRequestCreated
	assert.False(t, IsCounterpartMessage(requestCreated, true), "a request_created pill is never the staff reader's counterpart (queue badge, not unread chat)")
	assert.False(t, IsCounterpartMessage(requestCreated, false), "a request_created pill is not the guardian reader's counterpart either")

	// A request_status pill (the closing event) stays counted like any other
	// system event, attributed by EventActorKind.
	requestStatus := receiptMsg(6, now, ParentMessageSenderSystem)
	requestStatus.EventActorKind = ParentMessageSenderStaff
	requestStatus.EventType = ParentMessageEventRequestStatus
	assert.True(t, IsCounterpartMessage(requestStatus, false), "a staff-triggered request_status pill is the guardian reader's counterpart")
	assert.False(t, IsCounterpartMessage(requestStatus, true), "a staff-triggered request_status pill is not the staff reader's own counterpart")
}
