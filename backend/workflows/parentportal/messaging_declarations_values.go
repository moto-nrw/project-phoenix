package parentportal

import "github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/messaging"

// Messaging values the portal hands across its boundary. They name the
// workflow's own values, so a field cannot be lost between two copies (#4604).
type (
	AnnouncementDeclarationSettings  = messaging.AnnouncementDeclarationSettings
	AnnouncementFeedDeclaration      = messaging.AnnouncementFeedDeclaration
	AnnouncementFeedDeclarationChild = messaging.AnnouncementFeedDeclarationChild
	DeclarationAttachmentDigest      = messaging.DeclarationAttachmentDigest
	DeclarationChild                 = messaging.DeclarationChild
	DeclarationSigner                = messaging.DeclarationSigner
	DeclarationSignerContext         = messaging.DeclarationSignerContext
	DeclarationSignerState           = messaging.DeclarationSignerState
	DeclarationSubmission            = messaging.DeclarationSubmission
	DeclarationSignature             = messaging.DeclarationSignature
	DeclarationSignatureInput        = messaging.DeclarationSignatureInput
	DeclarationSignatureEntry        = messaging.DeclarationSignatureEntry
	SignatureLedger                  = messaging.SignatureLedger
	DeclarationVersion               = messaging.DeclarationVersion
)
