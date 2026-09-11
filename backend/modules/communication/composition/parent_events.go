package compose

import (
	"context"
	"log/slog"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	parentMessages "github.com/moto-nrw/project-phoenix/modules/communication/internal/parentmessages"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// ParentEventEmitterConfig supplies the retained conversation stores the
// parent-event emitter writes through, the unit of work its detached pill
// and wake paths open their tenant transactions through, plus the settings
// gate and realtime fan-out. The stores stay injected until the conversation
// repositories migrate with #2727.
type ParentEventEmitterConfig struct {
	DB          *bun.DB
	Runtime     tenant.UnitOfWork
	ThreadRepo  usersModels.ParentMessageThreadRepository
	MessageRepo usersModels.ParentMessageRepository
	Settings    parentmessaging.TenantSettingsResolver
	Broadcaster realtime.Broadcaster
	Logger      *slog.Logger
}

// NewParentEventEmitter composes the parent-event emitter the request domains
// hold: Communication's thread and pill writes behind the consumer-facing
// shell.
func NewParentEventEmitter(cfg ParentEventEmitterConfig) *parentmessaging.Emitter {
	return parentmessaging.NewEmitter(parentMessages.NewEventEmitter(
		cfg.DB, cfg.Runtime, cfg.ThreadRepo, cfg.MessageRepo, cfg.Settings, cfg.Broadcaster, cfg.Logger,
	))
}

// ParentConversationCore is Communication's shared conversation rule set bound
// to one set of stores, in the shape the parent-side service's port expects.
type ParentConversationCore interface {
	AppendMessage(ctx context.Context, msg *usersModels.ParentMessage) error
	MarkReadToNewest(ctx context.Context, tenantID, threadID, accountID int64, staffReader bool, messages []*usersModels.ParentMessage) (bool, error)
	DecorateReadReceipts(ctx context.Context, threadID, otherAccountID int64, messages []*usersModels.ParentMessage)
	Broadcast(tenantID, guardianAccountID, threadID, studentID int64)
	BroadcastRead(tenantID, guardianAccountID, threadID, studentID int64)
}

// ParentConversationConfig supplies the stores and broadcaster the shared
// conversation rules act on.
type ParentConversationConfig struct {
	ThreadRepo  usersModels.ParentMessageThreadRepository
	MessageRepo usersModels.ParentMessageRepository
	ReadRepo    usersModels.ParentMessageReadRepository
	Broadcaster realtime.Broadcaster
	Logger      *slog.Logger
}

// NewParentConversationCore binds the shared conversation rules for the parent
// side, so both portals mark reads, stamp receipts, and fan out identically.
func NewParentConversationCore(cfg ParentConversationConfig) ParentConversationCore {
	return parentMessages.NewConversationCore(cfg.ThreadRepo, cfg.MessageRepo, cfg.ReadRepo, cfg.Broadcaster, cfg.Logger)
}
