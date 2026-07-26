package users_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/users"
)

// msg is a tiny constructor for the read-receipt stamping tests: only the fields
// the Stamp*ReadReceipts helpers read (id, sender_kind, created_at) matter here.
func msg(id int64, kind string, createdAt time.Time) *users.ParentMessage {
	m := &users.ParentMessage{
		ThreadID:   1,
		SenderKind: kind,
	}
	m.ID = id
	m.CreatedAt = createdAt
	return m
}

func TestStampGuardianReadReceipts(t *testing.T) {
	base := time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC)
	staff1 := msg(10, users.ParentMessageSenderStaff, base)
	guardian := msg(11, users.ParentMessageSenderGuardian, base.Add(time.Minute))
	staff2 := msg(12, users.ParentMessageSenderStaff, base.Add(2*time.Minute))
	system := msg(13, users.ParentMessageSenderSystem, base)
	messages := []*users.ParentMessage{staff1, guardian, staff2, system}

	t.Run("nil cursor stamps nothing", func(t *testing.T) {
		for _, m := range messages {
			m.ReadByGuardian = false
		}
		users.StampGuardianReadReceipts(messages, nil)
		for _, m := range messages {
			if m.ReadByGuardian {
				t.Fatalf("message %d stamped read with a nil cursor", m.ID)
			}
		}
	})

	t.Run("stamps only staff messages at or before the cursor", func(t *testing.T) {
		for _, m := range messages {
			m.ReadByGuardian = false
		}
		// Guardian has read up to staff1 (its id/timestamp).
		cursor := &users.ReadCursor{LastReadAt: staff1.CreatedAt, LastReadMessageID: staff1.ID}
		users.StampGuardianReadReceipts(messages, cursor)

		if !staff1.ReadByGuardian {
			t.Error("staff1 at the cursor must be stamped read")
		}
		if staff2.ReadByGuardian {
			t.Error("staff2 after the cursor must NOT be stamped read")
		}
		// The guardian's own message is never a guardian-read receipt target.
		if guardian.ReadByGuardian {
			t.Error("guardian-authored message must never be stamped ReadByGuardian")
		}
		// A system message must never be stamped (positive staff-authorship gate).
		if system.ReadByGuardian {
			t.Error("system message must never be stamped ReadByGuardian")
		}
	})

	t.Run("composite tie-break: equal timestamp, higher id stays unread", func(t *testing.T) {
		tied := base
		low := msg(20, users.ParentMessageSenderStaff, tied)
		high := msg(21, users.ParentMessageSenderStaff, tied)
		set := []*users.ParentMessage{low, high}
		// Cursor sits exactly on the lower id at the shared timestamp.
		cursor := &users.ReadCursor{LastReadAt: tied, LastReadMessageID: low.ID}
		users.StampGuardianReadReceipts(set, cursor)

		if !low.ReadByGuardian {
			t.Error("message at the cursor (id == cursor id) must be stamped read")
		}
		if high.ReadByGuardian {
			t.Error("tied-timestamp message with a higher id than the cursor must stay unread")
		}
	})
}

// TestStampStaffReadReceipts pins the mirror direction so the two receipts cannot
// drift: guardian messages are stamped ReadByStaff, staff/system messages are not.
func TestStampStaffReadReceipts(t *testing.T) {
	base := time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC)
	guardian1 := msg(30, users.ParentMessageSenderGuardian, base)
	staff := msg(31, users.ParentMessageSenderStaff, base.Add(time.Minute))
	guardian2 := msg(32, users.ParentMessageSenderGuardian, base.Add(2*time.Minute))
	system := msg(33, users.ParentMessageSenderSystem, base)
	messages := []*users.ParentMessage{guardian1, staff, guardian2, system}

	cursor := &users.ReadCursor{LastReadAt: guardian1.CreatedAt, LastReadMessageID: guardian1.ID}
	users.StampStaffReadReceipts(messages, cursor)

	if !guardian1.ReadByStaff {
		t.Error("guardian1 at the cursor must be stamped ReadByStaff")
	}
	if guardian2.ReadByStaff {
		t.Error("guardian2 after the cursor must NOT be stamped ReadByStaff")
	}
	if staff.ReadByStaff {
		t.Error("staff-authored message must never be stamped ReadByStaff")
	}
	if system.ReadByStaff {
		t.Error("system message must never be stamped ReadByStaff")
	}
}
