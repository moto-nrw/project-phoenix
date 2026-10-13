package messaging

import (
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
)

// The conversation, announcement and declaration results the guardian portal
// HTTP composition renders, named by the flow that returns them. The
// composition matches them through the workflow port instead of importing
// the owner models (#3421).
type (
	AnnouncementFeedDeclaration      = usersModels.AnnouncementFeedDeclaration
	AnnouncementFeedDeclarationChild = usersModels.AnnouncementFeedDeclarationChild
	AnnouncementFeedItem             = usersModels.AnnouncementFeedItem
	DeclarationAttachmentDigest      = usersModels.DeclarationAttachmentDigest
	DeclarationSignerState           = usersModels.DeclarationSignerState
	DeclarationSubmission            = usersModels.DeclarationSubmission
	DeclarationVersion               = usersModels.DeclarationVersion
	InboxThread                      = usersModels.InboxThread
	ParentMessage                    = usersModels.ParentMessage
)

const (
	DeclarationActionAgreed     = usersModels.DeclarationActionAgreed
	DeclarationActionDeclined   = usersModels.DeclarationActionDeclined
	DeclarationActionRevoked    = usersModels.DeclarationActionRevoked
	DeclarationKindConsent      = usersModels.DeclarationKindConsent
	ParentMessageSenderGuardian = usersModels.ParentMessageSenderGuardian
	ParentMessageSenderStaff    = usersModels.ParentMessageSenderStaff
)
