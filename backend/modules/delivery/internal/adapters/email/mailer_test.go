package email

import (
	"testing"

	mailcontract "github.com/moto-nrw/project-phoenix/modules/delivery/mail"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// Email Struct Tests
// =============================================================================

func TestNewEmail(t *testing.T) {
	t.Parallel()

	email := mailcontract.NewEmail("John Doe", "john@example.com")

	assert.Equal(t, "John Doe", email.Name)
	assert.Equal(t, "john@example.com", email.Address)
}

func TestNewEmail_EmptyValues(t *testing.T) {
	t.Parallel()

	email := mailcontract.NewEmail("", "")

	assert.Empty(t, email.Name)
	assert.Empty(t, email.Address)
}

func TestNewEmail_SpecialCharacters(t *testing.T) {
	t.Parallel()

	email := mailcontract.NewEmail("José O'Connor", "jose+tag@example.com")

	assert.Equal(t, "José O'Connor", email.Name)
	assert.Equal(t, "jose+tag@example.com", email.Address)
}

// =============================================================================
// MockMailer Tests
// =============================================================================

func TestMockMailer_NewMockMailer(t *testing.T) {
	t.Parallel()

	mailer := NewMockMailer()

	assert.NotNil(t, mailer)
	assert.NotNil(t, mailer.SendFn)
	assert.False(t, mailer.SendInvoked.Load())
}

func TestMockMailer_Send(t *testing.T) {
	t.Parallel()

	mailer := NewMockMailer()

	msg := mailcontract.Message{
		From:    mailcontract.Email{Name: "Sender", Address: "sender@example.com"},
		To:      mailcontract.Email{Name: "Recipient", Address: "recipient@example.com"},
		Subject: "Test Subject",
	}

	err := mailer.Send(msg)

	assert.ErrorIs(t, err, mailcontract.ErrDeliveryUnavailable)
	assert.True(t, mailer.SendInvoked.Load())
}

func TestMockMailer_CustomSendFn(t *testing.T) {
	t.Parallel()

	mailer := NewMockMailer()

	var capturedMessage mailcontract.Message
	mailer.SendFn = func(m mailcontract.Message) error {
		capturedMessage = m
		return nil
	}

	msg := mailcontract.Message{
		Subject: "Custom Test",
	}

	err := mailer.Send(msg)

	assert.NoError(t, err)
	assert.Equal(t, "Custom Test", capturedMessage.Subject)
}

func TestMockMailer_SendFnReturnsError(t *testing.T) {
	t.Parallel()

	mailer := NewMockMailer()

	expectedErr := assert.AnError
	mailer.SendFn = func(_ mailcontract.Message) error {
		return expectedErr
	}

	err := mailer.Send(mailcontract.Message{})

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.True(t, mailer.SendInvoked.Load())
}

// =============================================================================
// LogMessage Tests
// =============================================================================

func TestLogMessage_DoesNotPanic(t *testing.T) {
	t.Parallel()

	// logMessage should not panic with various inputs
	assert.NotPanics(t, func() {
		logMessage(mailcontract.Message{
			To:       mailcontract.Email{Address: "test@example.com"},
			Subject:  "Test",
			Template: "test.html",
		})
	})
}

func TestLogMessage_EmptyFields(t *testing.T) {
	t.Parallel()

	// Should not panic with empty fields
	assert.NotPanics(t, func() {
		logMessage(mailcontract.Message{})
	})
}
