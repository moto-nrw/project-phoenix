package compose

import (
	"log/slog"

	eventContract "github.com/moto-nrw/project-phoenix/modules/communication/eventcontract"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	deliveryPorts "github.com/moto-nrw/project-phoenix/modules/communication/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/application/parentmessaging"
	parentMessages "github.com/moto-nrw/project-phoenix/modules/communication/internal/parentmessages"
	"github.com/moto-nrw/project-phoenix/realtime"
)

// ParentEventEmitterConfig supplies the retained conversation stores the
// parent-event emitter writes through, the unit of work its detached pill
// and wake paths open their tenant transactions through, plus the settings
// gate and realtime fan-out. The stores stay injected until the conversation
// repositories migrate with #2727.
type ParentEventEmitterConfig struct {
	Runtime          parentMessages.EventRuntime
	ThreadRepo       deliveryPorts.ParentMessageThreadRepositorySource
	MessageRepo      deliveryPorts.ParentMessageRepositorySource
	Settings         parentMessages.EventSettings
	Broadcaster      realtime.Broadcaster
	Logger           *slog.Logger
	DecisionNotifier eventContract.DecisionNotifier // decided requests to guardian devices; nil: none
}

// NewParentEventEmitter composes the parent-event emitter the request domains
// hold: Communication's thread and pill writes behind the consumer-facing
// shell.
func NewParentEventEmitter(cfg ParentEventEmitterConfig) eventContract.Emitter {
	return NewDecisionEmitter(parentMessages.NewEventEmitter(
		cfg.Runtime, parentMessages.NewThreadRepository(cfg.ThreadRepo), parentMessages.NewMessageRepository(cfg.MessageRepo), cfg.Settings, cfg.Broadcaster, cfg.Logger,
	), cfg.DecisionNotifier)
}

// ParentConversationCore is Communication's shared conversation rule set bound
// to one set of stores, in the shape the parent-side service's port expects.
type ParentConversationCore interface {
	communication.PortalConversationCore
}

// ParentConversationConfig supplies the stores and broadcaster the shared
// conversation rules act on.
type ParentConversationConfig struct {
	ThreadRepo  deliveryPorts.ParentMessageThreadRepositorySource
	MessageRepo deliveryPorts.ParentMessageRepositorySource
	ReadRepo    deliveryPorts.ParentMessageReadRepositorySource
	Broadcaster realtime.Broadcaster
	Logger      *slog.Logger
}

// NewParentConversationCore binds the shared conversation rules for the parent
// side, so both portals mark reads, stamp receipts, and fan out identically.
func NewParentConversationCore(cfg ParentConversationConfig) ParentConversationCore {
	return parentMessages.BindPortalConversationCore(parentMessages.NewConversationCore(parentMessages.NewThreadRepository(cfg.ThreadRepo), parentMessages.NewMessageRepository(cfg.MessageRepo), parentMessages.NewReadRepository(cfg.ReadRepo), cfg.Broadcaster, cfg.Logger))
}

// NewDecisionEmitter builds the native event capability over constructor ports.
func NewDecisionEmitter(events eventContract.Events, notifier eventContract.DecisionNotifier) eventContract.Emitter {
	return parentmessaging.NewEmitterWithDecisions(events, notifier)
}
