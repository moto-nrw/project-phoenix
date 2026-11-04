package messaging

import (
	values "github.com/moto-nrw/project-phoenix/modules/communication/internal/application"
)

func NativeFacadeErrors() values.ParentMessageErrors {
	return values.ParentMessageErrors{ErrForbidden: ErrForbidden, ErrThreadNotFound: ErrThreadNotFound, ErrEmptyBody: ErrEmptyBody, ErrBodyTooLong: ErrBodyTooLong, ErrHandledBoundaryRequired: ErrHandledBoundaryRequired, ErrInvalidGuardian: ErrInvalidGuardian, ErrGuardianAccessRevoked: ErrGuardianAccessRevoked, ErrMessagingDisabled: ErrMessagingDisabled, ErrInvalidCountScope: ErrInvalidCountScope}
}
func NativeFacadeRows() values.ParentMessageRows[InboxThread, ThreadDetail, ParentMessage, MessageableGuardian] {
	return values.ParentMessageRows[InboxThread, ThreadDetail, ParentMessage, MessageableGuardian]{
		Inbox:    func(row *InboxThread) *values.InboxThread { return (*values.InboxThread)(row) },
		Message:  func(row *ParentMessage) *values.ParentMessage { return (*values.ParentMessage)(row) },
		Guardian: func(row *MessageableGuardian) *values.MessageableGuardian { return (*values.MessageableGuardian)(row) },
		Thread: func(row *ThreadDetail) (values.InboxThread, []*ParentMessage) {
			return values.InboxThread{
				ThreadID: row.ThreadID, StudentID: row.StudentID, StudentName: row.StudentName,
				GuardianAccountID: row.GuardianAccountID, GuardianName: row.GuardianName, RelationshipType: row.RelationshipType,
				Audience: row.Audience, LeadershipReaders: row.LeadershipReaders,
			}, row.Messages
		},
	}
}
