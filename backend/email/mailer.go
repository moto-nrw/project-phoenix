// Package email provides backwards-compatibility re-exports for the email functionality.
// DEPRECATED: Import github.com/moto-nrw/project-phoenix/internal/adapter/mailer instead.
// This package will be removed in a future version.
package email

import (
	"github.com/moto-nrw/project-phoenix/internal/adapter/mailer"
	"github.com/moto-nrw/project-phoenix/internal/core/port"
)

// Type aliases for backwards compatibility
// These allow existing code to continue using email.X types

// Mailer interface - DEPRECATED: Use port.EmailSender instead.
type Mailer = port.EmailSender

// Message type - DEPRECATED: Use port.EmailMessage instead.
type Message = port.EmailMessage

// Email type - DEPRECATED: Use port.EmailAddress instead.
type Email = port.EmailAddress

// Dispatcher is a type alias - DEPRECATED: Use mailer.Dispatcher instead.
type Dispatcher = mailer.Dispatcher

// MockMailer is a type alias - DEPRECATED: Use mailer.MockMailer instead.
type MockMailer = mailer.MockMailer

// SMTPMailer is a type alias - DEPRECATED: Use mailer.SMTPMailer instead.
type SMTPMailer = mailer.SMTPMailer

// Delivery types - re-exported for backwards compatibility
type (
	DeliveryStatus   = mailer.DeliveryStatus
	DeliveryMetadata = mailer.DeliveryMetadata
	DeliveryResult   = mailer.DeliveryResult
	DeliveryCallback = mailer.DeliveryCallback
	DeliveryRequest  = mailer.DeliveryRequest
)

// Re-export constants
const (
	DeliveryStatusPending = mailer.DeliveryStatusPending
	DeliveryStatusSent    = mailer.DeliveryStatusSent
	DeliveryStatusFailed  = mailer.DeliveryStatusFailed
)

// NewMailer returns a configured SMTP Mailer.
// DEPRECATED: Use mailer.NewSMTPMailer() instead.
func NewMailer() (Mailer, error) {
	return mailer.NewSMTPMailer()
}

// NewMockMailer creates a MockMailer that logs emails instead of sending them.
// DEPRECATED: Use mailer.NewMockMailer() instead.
func NewMockMailer() *MockMailer {
	return mailer.NewMockMailer()
}

// NewDispatcher constructs a Dispatcher with sensible defaults.
// DEPRECATED: Use mailer.NewDispatcher() instead.
func NewDispatcher(m Mailer) *Dispatcher {
	return mailer.NewDispatcher(m)
}
