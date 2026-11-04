package parentportal

import "github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/messaging"

type (
	MessageThreadView = messaging.MessageThreadView
	DeclarationProof  = messaging.DeclarationProof
	// LeadershipOption is the school's leadership option for parent
	// conversations (#5200).
	LeadershipOption = messaging.LeadershipOption
	// MessageTarget names the audience and group thread of a message.
	MessageTarget = messaging.MessageTarget
)

// Conversation audiences of a parent conversation (#5200).
const (
	MessageAudienceTeam       = messaging.AudienceTeam
	MessageAudienceLeadership = messaging.AudienceLeadership
)
