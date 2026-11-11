package staffmessaging

import (
	"context"

	deliveryPorts "github.com/moto-nrw/project-phoenix/modules/communication/internal/application"
)

// Photos and files in the team chat (#6685). Membership stays the only
// authority: a file is read by the participants of its conversation, and a
// group member reads the files written since they joined.

// PrepareMessageAttachment stores the row of one upload for the caller.
func (s *Service) PrepareMessageAttachment(ctx context.Context, upload deliveryPorts.MessageAttachmentUpload) (*deliveryPorts.PreparedMessageAttachment, error) {
	accountID, err := s.attachmentActor(ctx)
	if err == nil {
		err = s.requireActiveMember(ctx, accountID)
	}
	if err != nil {
		return nil, deliveryPorts.MapStaffMessagingError(err, NativeFacadeErrors())
	}
	return deliveryPorts.PrepareMessageAttachment(ctx, s.Attachments, deliveryPorts.MessageAttachmentScopeStaff, accountID, upload)
}

// DiscardMessageAttachment removes an upload the caller did not send.
func (s *Service) DiscardMessageAttachment(ctx context.Context, attachmentID int64) error {
	accountID, err := s.actor(ctx)
	if err != nil {
		return deliveryPorts.MapStaffMessagingError(err, NativeFacadeErrors())
	}
	return deliveryPorts.DiscardMessageAttachment(ctx, s.Attachments, accountID, attachmentID)
}

// OpenMessageAttachment locates a file for a participant of its
// conversation. Every refusal of the conversation answers as a missing file.
func (s *Service) OpenMessageAttachment(ctx context.Context, attachmentID int64) (*deliveryPorts.MessageAttachmentFile, error) {
	accountID, err := s.attachmentActor(ctx)
	if err != nil {
		return nil, deliveryPorts.MapStaffMessagingError(err, NativeFacadeErrors())
	}
	record, done, err := deliveryPorts.FindMessageAttachment(ctx, s.Attachments, deliveryPorts.MessageAttachmentScopeStaff, accountID, attachmentID)
	if err != nil {
		return nil, err
	}
	if !done {
		if visible, err := s.readsAttachment(ctx, accountID, record); err != nil || !visible {
			return nil, deliveryPorts.ErrMessageAttachmentNotFound
		}
	}
	return deliveryPorts.MessageAttachmentFileOf(record), nil
}

// attachmentActor is the caller of an attachment route while the school has
// the team chat switched on.
func (s *Service) attachmentActor(ctx context.Context) (int64, error) {
	if err := s.requireEnabled(ctx); err != nil {
		return 0, err
	}
	return s.actor(ctx)
}

// readsAttachment reports whether the account takes part in the conversation
// of the file and, in a group, had joined when its message was written: the
// chat window shows a member the messages since they joined, and the files
// follow the same line.
func (s *Service) readsAttachment(ctx context.Context, accountID int64, record *deliveryPorts.MessageAttachmentRecord) (bool, error) {
	thread, err := s.authorizeThread(ctx, record.ThreadID, accountID)
	if err != nil {
		return false, err
	}
	if thread.Kind != groupKind {
		return true, nil
	}
	records, err := s.Groups.StaffParticipantStates(ctx, thread.ID)
	if err != nil {
		return false, err
	}
	for _, state := range deliveryPorts.StaffParticipantStatesFromRecords(records) {
		if state.AccountID == accountID {
			return state.HistoryFrom == nil || !record.MessageCreatedAt.Before(*state.HistoryFrom), nil
		}
	}
	return false, nil
}
