// Package parentportal is the public surface of the guardian portal workflow
// (ADR 0022, #3420). The workflow owns no data: care and messaging resolve the
// guardian's child, check the relationship's parent_portal.* permission and the
// school's settings, open one tenant unit of work and call the owners'
// commands inside it. compose binds the owners to the flows' ports.
package parentportal

import (
	"github.com/moto-nrw/project-phoenix/workflows/parentportal/care"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal/messaging"
)

// Portal is the composed guardian portal: the child flows of care and the
// conversation, announcement and request-sharing flows of messaging. Both
// method sets are promoted, so one value serves the HTTP composition.
type Portal struct {
	*ChildFlows
	*MessagingFlows
}

// ChildFlows and MessagingFlows name the two flow services inside Portal.
type (
	ChildFlows     = care.Service
	MessagingFlows = messaging.Service
)
