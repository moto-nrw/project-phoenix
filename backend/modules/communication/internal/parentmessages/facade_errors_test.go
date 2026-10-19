package messaging_test

import (
	"errors"
	"testing"

	composition "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	values "github.com/moto-nrw/project-phoenix/modules/communication/internal/application"

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
		mapped := values.MapParentMessagingError(test.legacy, parentMessages.NativeFacadeErrors())
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
		mapped := values.MapStaffMessagingError(test.legacy, staffMessages.NativeFacadeErrors())
		assert.ErrorIs(t, mapped, test.public)
		assert.ErrorIs(t, mapped, test.legacy)
	}
}

func TestUnknownMessagingErrorsRemainStable(t *testing.T) {
	t.Parallel()
	databaseErr := errors.New("database unavailable")
	assert.Same(t, databaseErr, values.MapParentMessagingError(databaseErr, parentMessages.NativeFacadeErrors()))
	assert.Same(t, databaseErr, values.MapStaffMessagingError(databaseErr, staffMessages.NativeFacadeErrors()))
}

func TestMessagingRequiresTheRequestSession(t *testing.T) {
	t.Parallel()
	assert.PanicsWithValue(t, "parent messaging: the request session is required", func() {
		composition.NewParentMessaging(composition.ParentMessagingConfig{}, nil, nil)
	})
	assert.PanicsWithValue(t, "staff messaging: the request session is required", func() {
		composition.NewStaffMessaging(composition.StaffMessagingConfig{}, nil, nil)
	})
}
