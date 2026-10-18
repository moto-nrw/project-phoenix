package messaging_test

import (
	"github.com/moto-nrw/project-phoenix/api"
	settingsValues "github.com/moto-nrw/project-phoenix/modules/settings"

	"log/slog"
	"testing"

	messaging "github.com/moto-nrw/project-phoenix/modules/communication/internal/parentmessages"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/uptrace/bun"
)

func newMockEmitter(
	t testing.TB,
	db *bun.DB,
	threadRepo messaging.ParentMessageThreadRepository,
	messageRepo messaging.ParentMessageRepository,
	settings parentmessaging.TenantSettingsResolver,
	broadcaster realtime.Broadcaster,
	logger *slog.Logger,
	decisions ...parentmessaging.DecisionNotifier,
) *parentmessaging.Emitter {
	t.Helper()
	var notifier parentmessaging.DecisionNotifier
	if len(decisions) > 0 {
		notifier = decisions[0]
	}
	return parentmessaging.NewEmitterWithDecisions(messaging.NewEventEmitter(api.NewCommunicationEventRuntime(db != nil, testpkg.TenantRuntime(t, db)), threadRepo, messageRepo, settingsValues.NewParentEventValues(settings), broadcaster, logger), notifier)
}
