package compose

import (
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	parentMessages "github.com/moto-nrw/project-phoenix/modules/communication/internal/parentmessages"
	staffMessages "github.com/moto-nrw/project-phoenix/modules/communication/internal/staffmessages"
	"github.com/stretchr/testify/assert"
)

func TestParentMessagingErrorsCrossThePublicBoundary(t *testing.T) {
	t.Parallel()
	tests := []struct {
		legacy error
		public error
	}{
		{parentMessages.ErrForbidden, communication.ErrParentMessagingForbidden},
		{parentMessages.ErrThreadNotFound, communication.ErrParentMessageThreadNotFound},
		{parentMessages.ErrEmptyBody, communication.ErrParentMessageEmptyBody},
		{parentMessages.ErrBodyTooLong, communication.ErrParentMessageBodyTooLong},
		{parentMessages.ErrHandledBoundaryRequired, communication.ErrParentMessageHandledBoundaryRequired},
		{parentMessages.ErrInvalidGuardian, communication.ErrParentMessageInvalidGuardian},
		{parentMessages.ErrGuardianAccessRevoked, communication.ErrParentMessageGuardianAccessRevoked},
		{parentMessages.ErrMessagingDisabled, communication.ErrParentMessagingDisabled},
	}
	for _, test := range tests {
		mapped := mapParentMessagingError(test.legacy)
		assert.ErrorIs(t, mapped, test.public)
		assert.ErrorIs(t, mapped, test.legacy)
	}
}

func TestStaffMessagingErrorsCrossThePublicBoundary(t *testing.T) {
	t.Parallel()
	tests := []struct {
		legacy error
		public error
	}{
		{staffMessages.ErrMessagingDisabled, communication.ErrStaffMessagingDisabled},
		{staffMessages.ErrNotParticipant, communication.ErrStaffMessagingNotParticipant},
		{staffMessages.ErrThreadNotFound, communication.ErrStaffMessageThreadNotFound},
		{staffMessages.ErrRecipientNotAvailable, communication.ErrStaffMessageRecipientNotAvailable},
		{staffMessages.ErrCounterpartUnavailable, communication.ErrStaffMessageCounterpartUnavailable},
		{staffMessages.ErrSelfConversation, communication.ErrStaffMessageSelfConversation},
		{staffMessages.ErrEmptyMessage, communication.ErrStaffMessageEmptyBody},
		{staffMessages.ErrMessageTooLong, communication.ErrStaffMessageBodyTooLong},
		{staffMessages.ErrNoActor, communication.ErrStaffMessagingNoActor},
		{staffMessages.ErrRetentionUnresolved, communication.ErrStaffMessageRetentionUnresolved},
	}
	for _, test := range tests {
		mapped := mapStaffMessagingError(test.legacy)
		assert.ErrorIs(t, mapped, test.public)
		assert.ErrorIs(t, mapped, test.legacy)
	}
}

func TestUnknownMessagingErrorsRemainStable(t *testing.T) {
	t.Parallel()
	databaseErr := errors.New("database unavailable")
	assert.Same(t, databaseErr, mapParentMessagingError(databaseErr))
	assert.Same(t, databaseErr, mapStaffMessagingError(databaseErr))
}
