package parentmessaging_test

import (
	"log/slog"
	"testing"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
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
	broadcaster parentmessaging.Broadcaster,
	logger *slog.Logger,
) *parentmessaging.Emitter {
	t.Helper()
	emitter := parentmessaging.NewEmitter(db, threadRepo, messageRepo, settings, broadcaster, logger)
	if db != nil {
		testpkg.SetTenantRuntime(t, emitter, db)
	}
	return emitter
}
