package staffmessaging

import (
	values "github.com/moto-nrw/project-phoenix/modules/communication/internal/application"
)

func NativeFacadeErrors() values.StaffMessageErrors {
	return values.StaffMessageErrors{ErrMessagingDisabled: ErrMessagingDisabled, ErrNotParticipant: ErrNotParticipant, ErrThreadNotFound: ErrThreadNotFound, ErrRecipientNotAvailable: ErrRecipientNotAvailable, ErrCounterpartUnavailable: ErrCounterpartUnavailable, ErrSelfConversation: ErrSelfConversation, ErrEmptyMessage: ErrEmptyMessage, ErrMessageTooLong: ErrMessageTooLong, ErrNoActor: ErrNoActor, ErrRetentionUnresolved: ErrRetentionUnresolved}
}
func NativeFacadeRows() values.StaffMessageRows[StaffInboxThread, ThreadDetail, StaffMessage, MessageableStaff, CleanupResult] {
	return values.StaffMessageRows[StaffInboxThread, ThreadDetail, StaffMessage, MessageableStaff, CleanupResult]{
		Inbox:     func(row *StaffInboxThread) *values.StaffInboxThread { return (*values.StaffInboxThread)(row) },
		Message:   func(row *StaffMessage) *values.StaffMessage { return (*values.StaffMessage)(row) },
		Colleague: func(row *MessageableStaff) *values.MessageableStaff { return (*values.MessageableStaff)(row) },
		Thread: func(row *ThreadDetail) (values.StaffInboxThread, []*StaffMessage) {
			return values.StaffInboxThread{ThreadID: row.ThreadID, CounterpartAccountID: row.CounterpartAccountID, CounterpartName: row.CounterpartName, CounterpartRoleKind: row.CounterpartRoleKind}, row.Messages
		},
	}
}
