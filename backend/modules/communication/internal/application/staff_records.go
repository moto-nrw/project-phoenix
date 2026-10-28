package application

import "github.com/moto-nrw/project-phoenix/modules/communication"

// Native workflow values have their own identity, without aliases or ORM data.
type StaffMessage communication.StaffMessageRecord
type StaffMessageThread communication.StaffThreadRecord
type StaffInboxThread communication.StaffInboxRecord
type MessageableStaff communication.AddressableStaffRecord

// These are the exact owned public facets supplied by the module root.
type StaffThreadSource interface {
	communication.StaffThreadRecords
}

// StaffGroupSource is the group-chat facet of the thread store (#4776).
type StaffGroupSource interface {
	communication.StaffGroupRecords
}

// StaffConversationSource is the thread store with both facets.
type StaffConversationSource interface {
	StaffThreadSource
	StaffGroupSource
}

// StaffParticipantState is one member with the start of their visible
// history and their read cursor.
type StaffParticipantState communication.StaffParticipantRecord

func StaffParticipantStatesFromRecords(values []*communication.StaffParticipantRecord) []*StaffParticipantState {
	out := make([]*StaffParticipantState, 0, len(values))
	for _, value := range values {
		out = append(out, (*StaffParticipantState)(value))
	}
	return out
}

// StaffThreadView is the chat window as the workflow builds it, before the
// facade maps it onto the public contract: the header, the members of a
// group and each group message's read receipt.
type StaffThreadView struct {
	Header    StaffInboxThread
	Kind      string
	GroupName string
	CanManage bool
	Members   []MessageableStaff
	Messages  []*StaffMessage
	Receipts  map[int64]StaffReadReceipt
}

// StaffReadReceipt is who of the members read one group message, and how
// many could have.
type StaffReadReceipt struct {
	ReadBy                   []int64
	EligibleReaderAccountIDs []int64
	ReaderCount              int
}
type StaffMessageSource interface {
	communication.StaffMessageRecords
}
type StaffReadSource interface{ communication.StaffReadRecords }
type StaffSenderNames interface{ communication.StaffSenderNames }

func StaffMessageFromRecord(value *communication.StaffMessageRecord) *StaffMessage {
	return (*StaffMessage)(value)
}
func StaffMessageToRecord(value *StaffMessage) *communication.StaffMessageRecord {
	return (*communication.StaffMessageRecord)(value)
}
func StaffThreadFromRecord(value *communication.StaffThreadRecord) *StaffMessageThread {
	return (*StaffMessageThread)(value)
}
func StaffInboxFromRecord(value *communication.StaffInboxRecord) *StaffInboxThread {
	return (*StaffInboxThread)(value)
}
func MessageableStaffFromRecord(value *communication.AddressableStaffRecord) *MessageableStaff {
	return (*MessageableStaff)(value)
}
