// Package email names the Delivery mail transport for the legacy composition (#2747): the messages, addresses, dispatcher and delivery results it binds, and the logging mock mailer it detects when SMTP is unset.
package email

import upstream "github.com/moto-nrw/project-phoenix/email"

type (
	DeliveryMetadata = upstream.DeliveryMetadata
	DeliveryObserver = upstream.DeliveryObserver
	DeliveryRequest  = upstream.DeliveryRequest
	DeliveryResult   = upstream.DeliveryResult
	Dispatcher       = upstream.Dispatcher
	Email            = upstream.Email
	Mailer           = upstream.Mailer
	MailerConfig     = upstream.MailerConfig
	Message          = upstream.Message
	MockMailer       = upstream.MockMailer
	ReplyToResolver  = upstream.ReplyToResolver
)

const (
	DeliveryStatusFailed = upstream.DeliveryStatusFailed
	DeliveryStatusSent   = upstream.DeliveryStatusSent
	TemplateDemoAccess   = upstream.TemplateDemoAccess
	TemplateDemoLead     = upstream.TemplateDemoLead
)

var (
	ErrDeliveryUnavailable = upstream.ErrDeliveryUnavailable
	WithInteractiveRetry   = upstream.WithInteractiveRetry
	IsDemoEnvironment      = upstream.IsDemoEnvironment
	NewDispatcher          = upstream.NewDispatcher
	NewEmail               = upstream.NewEmail
	NewMailer              = upstream.NewMailer
	NewMockMailer          = upstream.NewMockMailer
	ResolveReplyToIdentity = upstream.ResolveReplyToIdentity
)
