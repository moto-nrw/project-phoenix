package application

const (
	ParentMessageKindMessage            = "message"
	ParentMessageKindEvent              = "event"
	ParentMessageKindRequest            = "request"
	ParentMessageRequestStatusOpen      = "offen"
	ParentMessageRequestStatusDone      = "erledigt"
	ParentMessageRequestStatusRejected  = "abgelehnt"
	ParentMessageRequestStatusWithdrawn = "zurueckgezogen"
	ParentMessageRequestCareSchedule    = "care_schedule"
	ParentMessageRequestPickupChange    = "pickup_change"
	ParentMessageRequestMasterData      = "master_data"
	ParentMessageRequestExcusedAbsence  = "excused_absence"
	ParentMessageRequestSickAbsence     = "sick_absence"
	ParentMessageEventRequestCreated    = "request_created"
	ParentMessageEventRequestStatus     = "request_status"
	ParentMessageSenderGuardian         = "guardian"
	ParentMessageSenderStaff            = "staff"
	ParentMessageSenderSystem           = "system"
)

// atOrBeforeCursor reports whether the message sits at or before the read cursor,
// comparing on the SAME composite (created_at, id) the unread predicates use, not
// the timestamp alone: two messages can share a created_at, and a timestamp-only
// "not after cutoff" test would mark a tied message with a higher id as read while
// the unread badge still counts it. Shared by both receipt directions so they
// cannot diverge from the unread maths.
func atOrBeforeCursor(msg *ParentMessage, cutoff *ReadCursor) bool {
	return msg.CreatedAt.Before(cutoff.LastReadAt) ||
		(msg.CreatedAt.Equal(cutoff.LastReadAt) && msg.ID <= cutoff.LastReadMessageID)
}

// StampStaffReadReceipts marks every guardian-authored message at or before the
// staff read cursor with ReadByStaff=true (the "OGS hat gelesen" indicator).
// cutoff is the furthest staff read cursor in the thread (nil = staff has not
// read anything yet, so nothing is stamped). Pure presentation logic shared by
// the parent side (workflows/parentportal/messaging) and the staff-side Communication adapter so
// the receipt rule cannot drift between the two views of the same thread.
func StampStaffReadReceipts(messages []*ParentMessage, cutoff *ReadCursor) {
	if cutoff == nil {
		return
	}
	for _, msg := range messages {
		// Gate POSITIVELY on guardian authorship. A "!= staff" test would also stamp
		// a system message (sender_kind 'system', introduced by the #1672 requests
		// follow-up) as staff-read — a wrong receipt on a row no staff member wrote.
		if msg.SenderKind != ParentMessageSenderGuardian {
			continue
		}
		if atOrBeforeCursor(msg, cutoff) {
			msg.ReadByStaff = true
		}
	}
}

// StampGuardianReadReceipts is the mirror of StampStaffReadReceipts for the staff
// view: it marks every STAFF-authored message at or before the guardian's read
// cursor with ReadByGuardian=true (the staff-facing "von den Eltern gelesen"
// indicator). cutoff is the thread guardian's read cursor (nil = the guardian has
// not read anything yet, so nothing is stamped). A thread has exactly one guardian
// account, so the cursor is that single account's position — no aggregation.
func StampGuardianReadReceipts(messages []*ParentMessage, cutoff *ReadCursor) {
	if cutoff == nil {
		return
	}
	for _, msg := range messages {
		// Gate POSITIVELY on staff authorship so a 'system' message is never stamped
		// as guardian-read (mirrors the guardian-authorship gate above).
		if msg.SenderKind != ParentMessageSenderStaff {
			continue
		}
		if atOrBeforeCursor(msg, cutoff) {
			msg.ReadByGuardian = true
		}
	}
}

// IsCounterpartMessage reports whether msg is "from the OTHER party" relative to
// a reader on the given side — the SAME attribution the unread SQL uses
// (database/repositories/users.counterpartUnread). A staff reader's counterpart
// is the guardian side; a guardian reader's is the staff side. A system event
// (request decision / withdrawal) carries sender_kind='system' and records the
// triggering side in EventActorKind, so for those rows the side comes from
// EventActorKind, not SenderKind. Keeping this in lock-step with counterpartUnread
// is what lets MarkReadToNewest bound the read cursor to exactly the set of
// messages the unread count considers — advancing past a later staff/system row
// (not the reader's, but not a counterpart either) would skip an earlier
// counterpart message still committing in a concurrent send/read race.
//
// request_created pills are excluded here in lock-step with counterpartUnread's
// `event_type IS DISTINCT FROM 'request_created'`: a submitted request is
// surfaced on the Änderungsanfragen queue badge, never as an unread chat message,
// so the unread SQL ignores it. Without the SAME exclusion here, MarkReadToNewest
// would advance the read cursor onto such a pill when it is the newest
// guardian-side row — leaping past an earlier, still-committing guardian message
// (lower created_at, later commit) that the unread query would then never surface.
// For a guardian reader it is a no-op (a guardian-side pill is not their
// counterpart anyway), so this only affects the staff side, exactly like the SQL.
func IsCounterpartMessage(msg *ParentMessage, staffReader bool) bool {
	if msg.EventType == ParentMessageEventRequestCreated {
		return false
	}
	side := ParentMessageSenderStaff
	if staffReader {
		side = ParentMessageSenderGuardian
	}
	if msg.SenderKind == ParentMessageSenderSystem {
		return msg.EventActorKind == side
	}
	return msg.SenderKind == side
}

// IsReaderAuthored reports whether msg is one the reader sent THEMSELVES and must
// therefore be excluded from their own unread set — the model mirror of the unread
// SQL's notReaderAuthored predicate
// (database/repositories/users.notReaderAuthored = sender_kind='system' OR
// sender_account_id <> ?). System events are DELIBERATELY never "reader authored":
// they are attributed by triggering SIDE via EventActorKind, not by account, so a
// dual-role (staff+guardian) account's own confirm/reject/withdrawal must still
// count — and advance the cursor — on the opposite portal. Applying a bare
// SenderAccountID == reader check to those rows would strand them as permanently
// unread once viewed. Keeping this in lock-step with notReaderAuthored is what lets
// MarkReadToNewest bound the read cursor to exactly the unread set.
func IsReaderAuthored(msg *ParentMessage, accountID int64) bool {
	return msg.SenderKind != ParentMessageSenderSystem && msg.SenderAccountID == accountID
}
