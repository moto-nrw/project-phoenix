package parentportal

import "github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/messaging"

// Messaging values the portal hands across its boundary. They name the
// workflow's own values, so a field cannot be lost between two copies (#4604).
type (
	FamilyProtectionEvent   = messaging.FamilyProtectionEvent
	ParentRequestShareEvent = messaging.ParentRequestShareEvent
)
