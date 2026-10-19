package messaging_test

import (
	api "github.com/moto-nrw/project-phoenix/api"
	eventContract "github.com/moto-nrw/project-phoenix/modules/communication/eventcontract"
	settingsValues "github.com/moto-nrw/project-phoenix/modules/settings"

	"log/slog"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/communication/internal/application/parentmessaging"
	messaging "github.com/moto-nrw/project-phoenix/modules/communication/internal/parentmessages"
	"github.com/moto-nrw/project-phoenix/realtime"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/uptrace/bun"
)

func newMockEmitter(
	t testing.TB,
	db *bun.DB,
	threadRepo messaging.ParentMessageThreadRepository,
	messageRepo messaging.ParentMessageRepository,
	settings eventContract.TenantSettingsResolver,
	broadcaster realtime.Broadcaster,
	logger *slog.Logger,
	decisions ...eventContract.DecisionNotifier,
) eventContract.Emitter {
	t.Helper()
	var notifier eventContract.DecisionNotifier
	if len(decisions) > 0 {
		notifier = decisions[0]
	}
	return parentmessaging.NewEmitterWithDecisions(messaging.NewEventEmitter(api.NewCommunicationEventRuntime(db != nil, testpkg.TenantRuntime(t, db)), threadRepo, messageRepo, settingsValues.NewParentEventValues(settings), broadcaster, logger), notifier)
}
