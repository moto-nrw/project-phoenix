package email

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// Email Struct Tests
// =============================================================================

func TestNewEmail(t *testing.T) {
	email := NewEmail("John Doe", "john@example.com")

	assert.Equal(t, "John Doe", email.Name)
	assert.Equal(t, "john@example.com", email.Address)
}

func TestNewEmail_EmptyValues(t *testing.T) {
	email := NewEmail("", "")

	assert.Empty(t, email.Name)
	assert.Empty(t, email.Address)
}

func TestNewEmail_SpecialCharacters(t *testing.T) {
	email := NewEmail("José O'Connor", "jose+tag@example.com")

	assert.Equal(t, "José O'Connor", email.Name)
	assert.Equal(t, "jose+tag@example.com", email.Address)
}

// =============================================================================
// Format Function Tests
// =============================================================================

func TestFormatAsDate(t *testing.T) {
	tests := []struct {
		name     string
		time     time.Time
		expected string
	}{
		{
			name:     "standard date",
			time:     time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC),
			expected: "15.3.2024",
		},
		{
			name:     "single digit day and month",
			time:     time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC),
			expected: "5.1.2024",
		},
		{
			name:     "end of year",
			time:     time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC),
			expected: "31.12.2024",
		},
		{
			name:     "beginning of year",
			time:     time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			expected: "1.1.2024",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatAsDate(tt.time)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatAsDuration(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		futureTime  time.Time
		shouldMatch func(string) bool
	}{
		{
			name:       "hours and minutes",
			futureTime: now.Add(2*time.Hour + 30*time.Minute),
			shouldMatch: func(s string) bool {
				return s == "2 hours and 150 minutes" || s == "2 hours and 149 minutes"
			},
		},
		{
			name:       "only minutes",
			futureTime: now.Add(45 * time.Minute),
			shouldMatch: func(s string) bool {
				return s == "45 minutes" || s == "44 minutes"
			},
		},
		{
			name:       "one hour",
			futureTime: now.Add(1*time.Hour + 10*time.Minute),
			shouldMatch: func(s string) bool {
				return s == "1 hours and 70 minutes" || s == "1 hours and 69 minutes"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatAsDuration(tt.futureTime)
			assert.True(t, tt.shouldMatch(result), "unexpected result: %s", result)
		})
	}
}

func TestFormatAsDuration_ZeroMinutes(t *testing.T) {
	now := time.Now()
	futureTime := now.Add(1 * time.Second)

	result := formatAsDuration(futureTime)
	assert.Contains(t, result, "minutes")
}

// =============================================================================
// MockMailer Tests
// =============================================================================

func TestMockMailer_NewMockMailer(t *testing.T) {
	mailer := NewMockMailer()

	assert.NotNil(t, mailer)
	assert.NotNil(t, mailer.SendFn)
	assert.False(t, mailer.SendInvoked)
}

func TestMockMailer_Send(t *testing.T) {
	mailer := NewMockMailer()

	msg := Message{
		From:    Email{Name: "Sender", Address: "sender@example.com"},
		To:      Email{Name: "Recipient", Address: "recipient@example.com"},
		Subject: "Test Subject",
	}

	err := mailer.Send(msg)

	assert.NoError(t, err)
	assert.True(t, mailer.SendInvoked)
}

func TestMockMailer_CustomSendFn(t *testing.T) {
	mailer := NewMockMailer()

	var capturedMessage Message
	mailer.SendFn = func(m Message) error {
		capturedMessage = m
		return nil
	}

	msg := Message{
		Subject: "Custom Test",
	}

	err := mailer.Send(msg)

	assert.NoError(t, err)
	assert.Equal(t, "Custom Test", capturedMessage.Subject)
}

func TestMockMailer_SendFnReturnsError(t *testing.T) {
	mailer := NewMockMailer()

	expectedErr := assert.AnError
	mailer.SendFn = func(_ Message) error {
		return expectedErr
	}

	err := mailer.Send(Message{})

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.True(t, mailer.SendInvoked)
}

// =============================================================================
// Message Struct Tests
// =============================================================================

func TestMessage_Fields(t *testing.T) {
	msg := Message{
		From:     Email{Name: "From Name", Address: "from@example.com"},
		To:       Email{Name: "To Name", Address: "to@example.com"},
		Subject:  "Test Subject",
		Template: "test_template.html",
		Content:  map[string]string{"key": "value"},
	}

	assert.Equal(t, "From Name", msg.From.Name)
	assert.Equal(t, "from@example.com", msg.From.Address)
	assert.Equal(t, "To Name", msg.To.Name)
	assert.Equal(t, "to@example.com", msg.To.Address)
	assert.Equal(t, "Test Subject", msg.Subject)
	assert.Equal(t, "test_template.html", msg.Template)
}

// =============================================================================
// FuncMap Tests
// =============================================================================

func TestFMap_Contains_FormatFunctions(t *testing.T) {
	assert.NotNil(t, fMap["formatAsDate"])
	assert.NotNil(t, fMap["formatAsDuration"])
}

// =============================================================================
// Mailer Interface Tests
// =============================================================================

func TestMailerInterface_MockMailerImplements(_ *testing.T) {
	var _ Mailer = &MockMailer{}
}

func TestMailerInterface_SMTPMailerImplements(_ *testing.T) {
	var _ Mailer = &SMTPMailer{}
}

// =============================================================================
// LogMessage Tests
// =============================================================================

func TestLogMessage_DoesNotPanic(t *testing.T) {
	// logMessage should not panic with various inputs
	assert.NotPanics(t, func() {
		logMessage(Message{
			To:       Email{Address: "test@example.com"},
			Subject:  "Test",
			Template: "test.html",
		})
	})
}

func TestLogMessage_EmptyFields(t *testing.T) {
	// Should not panic with empty fields
	assert.NotPanics(t, func() {
		logMessage(Message{})
	})
}
