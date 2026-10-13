package compose

import "github.com/moto-nrw/project-phoenix/services/parentmessaging"

// The parent-event vocabulary the legacy composition binds request domains
// to (#2747): the emitter NewParentEventEmitter builds, the events it fans
// out, a decision's guardian audience and the share-visibility resolver.
type (
	ParentEventEmitter            = parentmessaging.Emitter
	ParentChildEvent              = parentmessaging.ChildEvent
	ParentDecisionAudience        = parentmessaging.DecisionAudience
	ParentShareVisibilityResolver = parentmessaging.ShareVisibilityResolver
)
