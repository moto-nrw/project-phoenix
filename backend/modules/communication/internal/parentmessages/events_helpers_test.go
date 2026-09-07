package messaging_test

import (
	"log/slog"
	"testing"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	messaging "github.com/moto-nrw/project-phoenix/modules/communication/internal/parentmessages"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/uptrace/bun"
)

func newMockEmitter(
	t testing.TB,
	db *bun.DB,
	threadRepo usersModels.ParentMessageThreadRepository,
	messageRepo usersModels.ParentMessageRepository,
	settings parentmessaging.TenantSettingsResolver,
	broadcaster realtime.Broadcaster,
	logger *slog.Logger,
) *parentmessaging.Emitter {
	t.Helper()
	emitter := parentmessaging.NewEmitter(messaging.NewEventEmitter(db, testpkg.TenantRuntime(t, db), threadRepo, messageRepo, settings, broadcaster, logger))
	return emitter
}
