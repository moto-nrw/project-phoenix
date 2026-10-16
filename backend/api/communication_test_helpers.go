package api

import (
	"context"
	"log/slog"

	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// NewParentEventEmitter builds Communication's real parent-event emitter over
// the given conversation stores, so the behavior tests of the request domains
// exercise the actual pill, wake, and conversation rules instead of
// hand-rolled fakes. runtime is the unit of work the detached pill and wake
// paths run under; tests supply the package test runtime for their database.
func NewParentEventEmitter(
	db transactionruntime.Database,
	runtime tenant.UnitOfWork,
	threadRepo userModels.ParentMessageThreadRepository,
	messageRepo userModels.ParentMessageRepository,
	settings interface {
		ResolveBoolForTenant(ctx context.Context, tenantID int64, key string) (bool, error)
	},
	broadcaster realtime.Broadcaster,
	logger *slog.Logger,
) *communicationCompose.ParentEventEmitter {
	return communicationCompose.NewParentEventEmitter(communicationCompose.ParentEventEmitterConfig{
		DB: db.Pool(), Runtime: runtime, ThreadRepo: threadRepo, MessageRepo: messageRepo,
		Settings: settings, Broadcaster: broadcaster, Logger: logger,
	})
}

// NewParentConversationCore binds Communication's shared conversation rules
// to the given stores for the parent-side service under test.
func NewParentConversationCore(
	threadRepo userModels.ParentMessageThreadRepository,
	messageRepo userModels.ParentMessageRepository,
	readRepo userModels.ParentMessageReadRepository,
	broadcaster realtime.Broadcaster,
	logger *slog.Logger,
) communicationCompose.ParentConversationCore {
	return communicationCompose.NewParentConversationCore(communicationCompose.ParentConversationConfig{
		ThreadRepo: threadRepo, MessageRepo: messageRepo, ReadRepo: readRepo,
		Broadcaster: broadcaster, Logger: logger,
	})
}
