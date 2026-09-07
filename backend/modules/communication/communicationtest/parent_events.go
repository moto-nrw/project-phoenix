// Package communicationtest composes real Communication components for the
// behavior tests of the request domains, so those tests exercise the actual
// pill, wake, and conversation rules instead of hand-rolled fakes. It is test
// support: production code never imports it.
package communicationtest

import (
	"log/slog"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	compose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// NewParentEventEmitter builds the real parent-event emitter over the given
// conversation stores. runtime is the unit of work the detached pill and wake
// paths run under; tests supply the package test runtime for their database.
func NewParentEventEmitter(
	db *bun.DB,
	runtime tenant.UnitOfWork,
	threadRepo usersModels.ParentMessageThreadRepository,
	messageRepo usersModels.ParentMessageRepository,
	settings parentmessaging.TenantSettingsResolver,
	broadcaster realtime.Broadcaster,
	logger *slog.Logger,
) *parentmessaging.Emitter {
	return compose.NewParentEventEmitter(compose.ParentEventEmitterConfig{
		DB: db, Runtime: runtime, ThreadRepo: threadRepo, MessageRepo: messageRepo,
		Settings: settings, Broadcaster: broadcaster, Logger: logger,
	})
}

// NewParentConversationCore binds Communication's shared conversation rules
// to the given stores for the parent-side service under test.
func NewParentConversationCore(
	threadRepo usersModels.ParentMessageThreadRepository,
	messageRepo usersModels.ParentMessageRepository,
	readRepo usersModels.ParentMessageReadRepository,
	broadcaster realtime.Broadcaster,
	logger *slog.Logger,
) compose.ParentConversationCore {
	return compose.NewParentConversationCore(compose.ParentConversationConfig{
		ThreadRepo: threadRepo, MessageRepo: messageRepo, ReadRepo: readRepo,
		Broadcaster: broadcaster, Logger: logger,
	})
}
