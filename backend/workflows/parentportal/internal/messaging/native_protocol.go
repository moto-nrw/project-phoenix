package messaging

// Stored protocol values are named by their workflow consumer.
const (
	DeclarationActionAgreed                = "agreed"
	DeclarationActionDeclined              = "declined"
	DeclarationActionRevoked               = "revoked"
	DeclarationKindConsent                 = "consent"
	DeclarationMethodSimpleElectronic      = "simple_electronic"
	ParentAnnouncementDeliveryDeclaration  = "declaration"
	ParentAnnouncementResponseMultiChoice  = "multi_choice"
	ParentAnnouncementResponseSingleChoice = "single_choice"
	ParentMessageKindMessage               = "message"
	ParentMessageSenderGuardian            = "guardian"
	ParentMessageSenderStaff               = "staff"
	ParentRequestEventShared               = "shared"
)
