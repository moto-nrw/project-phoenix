package messaging

import values "github.com/moto-nrw/project-phoenix/modules/communication/internal/application"

const (
	ParentMessageKindMessage            = values.ParentMessageKindMessage
	ParentMessageKindEvent              = values.ParentMessageKindEvent
	ParentMessageKindRequest            = values.ParentMessageKindRequest
	ParentMessageRequestStatusOpen      = values.ParentMessageRequestStatusOpen
	ParentMessageRequestStatusDone      = values.ParentMessageRequestStatusDone
	ParentMessageRequestStatusRejected  = values.ParentMessageRequestStatusRejected
	ParentMessageRequestStatusWithdrawn = values.ParentMessageRequestStatusWithdrawn
	ParentMessageRequestCareSchedule    = values.ParentMessageRequestCareSchedule
	ParentMessageRequestPickupChange    = values.ParentMessageRequestPickupChange
	ParentMessageRequestMasterData      = values.ParentMessageRequestMasterData
	ParentMessageRequestExcusedAbsence  = values.ParentMessageRequestExcusedAbsence
	ParentMessageRequestSickAbsence     = values.ParentMessageRequestSickAbsence
	ParentMessageEventRequestCreated    = values.ParentMessageEventRequestCreated
	ParentMessageEventRequestStatus     = values.ParentMessageEventRequestStatus
	ParentMessageSenderGuardian         = values.ParentMessageSenderGuardian
	ParentMessageSenderStaff            = values.ParentMessageSenderStaff
	ParentMessageSenderSystem           = values.ParentMessageSenderSystem
)

func IsCounterpartMessage(row *ParentMessage, staff bool) bool {
	return values.IsCounterpartMessage((*values.ParentMessage)(row), staff)
}
func IsReaderAuthored(row *ParentMessage, id int64) bool {
	return values.IsReaderAuthored((*values.ParentMessage)(row), id)
}
func StampStaffReadReceipts(rows []*ParentMessage, cursor *ReadCursor) {
	values.StampStaffReadReceipts(parentMessagesForPolicy(rows), (*values.ReadCursor)(cursor))
}
func StampGuardianReadReceipts(rows []*ParentMessage, cursor *ReadCursor) {
	values.StampGuardianReadReceipts(parentMessagesForPolicy(rows), (*values.ReadCursor)(cursor))
}
func parentMessagesForPolicy(rows []*ParentMessage) []*values.ParentMessage {
	if rows == nil {
		return nil
	}
	out := make([]*values.ParentMessage, len(rows))
	for i, row := range rows {
		out[i] = (*values.ParentMessage)(row)
	}
	return out
}
func parentMessagesFromApplication(rows []*values.ParentMessage) []*ParentMessage {
	if rows == nil {
		return nil
	}
	out := make([]*ParentMessage, len(rows))
	for i, row := range rows {
		out[i] = (*ParentMessage)(row)
	}
	return out
}
