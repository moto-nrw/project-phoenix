package staffmessaging

import (
	values "github.com/moto-nrw/project-phoenix/modules/communication/internal/application"
)

func NativeFacadeErrors() values.StaffMessageErrors {
	return values.StaffMessageErrors{ErrMessagingDisabled: ErrMessagingDisabled, ErrNotParticipant: ErrNotParticipant, ErrThreadNotFound: ErrThreadNotFound, ErrRecipientNotAvailable: ErrRecipientNotAvailable, ErrCounterpartUnavailable: ErrCounterpartUnavailable, ErrSelfConversation: ErrSelfConversation, ErrEmptyMessage: ErrEmptyMessage, ErrMessageTooLong: ErrMessageTooLong, ErrNoActor: ErrNoActor, ErrRetentionUnresolved: ErrRetentionUnresolved, ErrGroupCreateForbidden: ErrGroupCreateForbidden, ErrGroupManageForbidden: ErrGroupManageForbidden, ErrGroupNameInvalid: ErrGroupNameInvalid, GroupNameMaxCharacters: maxGroupNameLen, ErrGroupMembersRequired: ErrGroupMembersRequired, ErrGroupTooLarge: ErrGroupTooLarge, GroupMaxMembers: maxGroupMembers}
}
func NativeFacadeRows() values.StaffMessageRows[StaffInboxThread, ThreadDetail, StaffMessage, MessageableStaff, CleanupResult] {
	return values.StaffMessageRows[StaffInboxThread, ThreadDetail, StaffMessage, MessageableStaff, CleanupResult]{
		Inbox:     func(row *StaffInboxThread) *values.StaffInboxThread { return (*values.StaffInboxThread)(row) },
		Message:   func(row *StaffMessage) *values.StaffMessage { return (*values.StaffMessage)(row) },
		Colleague: func(row *MessageableStaff) *values.MessageableStaff { return (*values.MessageableStaff)(row) },
		Thread:    threadView,
	}
}

// threadView hands the chat window to the facade, which maps it onto the
// public contract.
func threadView(row *ThreadDetail) *values.StaffThreadView {
	return &values.StaffThreadView{
		Header: values.StaffInboxThread{
			ThreadID: row.ThreadID, CounterpartAccountID: row.CounterpartAccountID,
			CounterpartName: row.CounterpartName, CounterpartRoleKind: row.CounterpartRoleKind,
		},
		Kind: row.Kind, GroupName: row.GroupName, CanManage: row.CanManage,
		Members:  row.Members,
		Messages: projectMessages(row.Messages),
		Receipts: row.Receipts,
	}
}

func projectMessages(rows []*StaffMessage) []*values.StaffMessage {
	out := make([]*values.StaffMessage, 0, len(rows))
	for _, row := range rows {
		out = append(out, (*values.StaffMessage)(row))
	}
	return out
}
