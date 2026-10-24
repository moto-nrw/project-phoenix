// Package parentportal is the public surface of the guardian portal workflow
// (ADR 0022, #3420). The workflow owns no data: care and messaging resolve the
// guardian's child, check the relationship's parent_portal.* permission and the
// school's settings, open one tenant unit of work and call the owners'
// commands inside it. compose binds the owners to the flows' ports.
package parentportal

import (
	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/care"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/messaging"
)

// Portal is the composed guardian portal: the child flows of care and the
// conversation, announcement and request-sharing flows of messaging. Both
// method sets are promoted, so one value serves the HTTP composition.
type Portal struct {
	*ChildFlows
	*MessagingFlows
}

// The flow services, their configurations and shared collaborator contracts
// form the public construction surface of the workflow.
type (
	ChildFlows             = care.Service
	MessagingFlows         = messaging.Service
	ChildConfig            = care.Config
	MessagingConfig        = messaging.Config
	ChildResolver          = messaging.ChildResolver
	RequestShareVisibility = care.RequestShareVisibility
	// AnnouncementAttachments is the guardian's attachment entry; the root
	// binds it to File Storage's parent download routes (#4825).
	AttachmentAudience = messaging.AttachmentAudience
)

// AnnouncementAttachments is the guardian's attachment entry; the root binds
// it to File Storage's parent download routes (#4825). A and C are File
// Storage's attachment and opened content, handed out unchanged.
type AnnouncementAttachments[A, C any] = messaging.AnnouncementAttachments[A, C]

// AttachmentFiles is File Storage's tenant-scoped attachment reads.
type AttachmentFiles[A, C any] = messaging.AttachmentFiles[A, C]

// ErrAttachmentNotFound is every refusal of the guardian attachment entry.
var ErrAttachmentNotFound = messaging.ErrAttachmentNotFound

// NewChildFlows builds the child flows from the workflow's read and command
// ports, retaining the required request-sharing collaborator.
func NewChildFlows(cfg ChildConfig) *ChildFlows {
	return care.New(cfg)
}

// NewAnnouncementAttachments binds the portal's attachment audience gate to
// File Storage's tenant-scoped attachment reads.
func NewAnnouncementAttachments[A, C any](audience AttachmentAudience, files AttachmentFiles[A, C]) (*AnnouncementAttachments[A, C], error) {
	return messaging.NewAnnouncementAttachments(audience, files)
}

// NewMessagingFlows builds the announcement, conversation and request-sharing
// flows behind the public native response facade. The composition binds
// their child resolver to the child flows.
func NewMessagingFlows(cfg MessagingConfig) *Portal {
	return &Portal{MessagingFlows: messaging.New(cfg)}
}

const (
	DayStateNoCare                     = care.DayStateNoCare
	KeyEnrollmentBookingsAuthoritative = care.KeyEnrollmentBookingsAuthoritative
	KeyParentNewsEnabled               = care.KeyParentNewsEnabled
)
