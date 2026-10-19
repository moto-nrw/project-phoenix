//go:build testhooks

package api

import (
	"context"
	"log/slog"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	eventContract "github.com/moto-nrw/project-phoenix/modules/communication/eventcontract"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/delivery/devices"
	settingsValues "github.com/moto-nrw/project-phoenix/modules/settings"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
	parentportal "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// NewParentEventEmitter builds Communication's real parent-event emitter over
// the given conversation stores, so the behavior tests of the request domains
// exercise the actual pill, wake, and conversation rules instead of
// hand-rolled fakes. runtime is the unit of work the detached pill and wake
// paths run under; tests supply the package test runtime for their database.
func NewParentEventEmitter(
	db transactionruntime.Database,
	runtime tenant.UnitOfWork,
	threadRepo communication.ParentThreadRecords,
	messageRepo communication.ParentMessageRecords,
	settings interface {
		ResolveBoolForTenant(ctx context.Context, tenantID int64, key string) (bool, error)
	},
	broadcaster realtime.Broadcaster,
	logger *slog.Logger,
) eventContract.Emitter {
	return communicationCompose.NewParentEventEmitter(communicationCompose.ParentEventEmitterConfig{
		Runtime: NewCommunicationEventRuntime(db.Pool() != nil, runtime), ThreadRepo: threadRepo, MessageRepo: messageRepo,
		Settings: settingsValues.NewParentEventValues(settings), Broadcaster: broadcaster, Logger: logger,
	})
}

// NewParentConversationCore binds Communication's shared conversation rules
// to the given stores for the parent-side service under test.
func NewParentConversationCore(
	threadRepo communication.ParentThreadRecords,
	messageRepo communication.ParentMessageRecords,
	readRepo communication.ParentReadRecords,
	broadcaster realtime.Broadcaster,
	logger *slog.Logger,
) parentportal.ConversationCore {
	return owners.BindParentConversationCore(communicationCompose.NewParentConversationCore(communicationCompose.ParentConversationConfig{
		ThreadRepo: threadRepo, MessageRepo: messageRepo, ReadRepo: readRepo, Broadcaster: broadcaster, Logger: logger,
	}))
}

// ParentConversationMessagesFixture reads the same message snapshot through
// the owner's native public facade, preserving nil/empty and the original error.
func ParentConversationMessagesFixture(ctx context.Context, source communication.PortalMessageReads, threadID int64, limit int) ([]*parentportal.ParentMessage, error) {
	rows, err := source.PortalListByThread(ctx, threadID, limit)
	return owners.ParentMessagingSlice(rows, func(row *communication.PortalParentMessage) *parentportal.ParentMessage {
		return owners.ParentMessagingPointer(row, owners.ParentMessagingFromOwnerParentMessage)
	}), err
}

// NewCommunicationTestNotifier constructs the original channel-free router
// through its owner composition, exposing only native public commands.
func NewCommunicationTestNotifier(settings interface {
	ResolveBoolForTenant(context.Context, int64, string) (bool, error)
	ResolveStringForTenant(context.Context, int64, string) (string, error)
}, logger *slog.Logger) devices.NotificationCommands {
	return deliveryCompose.NewNotifier(settings, logger, nil)
}

// NewCommunicationRequestDecisionNotifier binds the real producer to a native
// observation callback, preserving its consent and transaction policy.
func NewCommunicationRequestDecisionNotifier(send func(context.Context, communication.NotificationEvent) error, preferences interface {
	FilterOptedIn(context.Context, string, []int64) ([]int64, error)
}) interface {
	NotifyRequestDecision(context.Context, int64, int64, int64, string, string, int64, string, string) error
} {
	var notifier deliveryCompose.NotificationService
	if send != nil {
		notifier = communicationDecisionNotification(send)
	}
	return deliveryCompose.NewRequestDecisionNotifier(notifier, preferences)
}

type communicationDecisionNotification func(context.Context, communication.NotificationEvent) error

func (send communicationDecisionNotification) Notify(ctx context.Context, event deliveryCompose.NotificationEvent) error {
	return send(ctx, communication.NotificationEvent{
		Type: event.Type, IdempotencyKey: event.IdempotencyKey,
		RelatedType: event.RelatedType, RelatedID: event.RelatedID,
		Audience: communication.NotificationAudience{
			TenantID: event.Audience.TenantID, Scope: string(event.Audience.Scope),
			GuardianAccountID: event.Audience.GuardianAccountID, GuardianAccountIDs: event.Audience.GuardianAccountIDs,
			ActiveGroupID: event.Audience.ActiveGroupID, StaffAccountIDs: event.Audience.StaffAccountIDs, StudentIDs: event.Audience.StudentIDs,
		},
		Priority: event.Priority, Title: event.Title, Body: event.Body,
		DeepLink: event.DeepLink, SchoolDeepLink: event.SchoolDeepLink, Data: event.Data, Portal: event.Portal,
	})
}

// ParentStoreGuardianMessagesFixture reads the actual native store in the
// caller's tenant transaction. Assertions and transaction ownership stay with
// the calling suite.
func ParentStoreGuardianMessagesFixture(run func(func(context.Context) error) error, threads communication.ParentThreadRecords, messages communication.ParentMessageRecords, studentID, guardianAccountID int64, limit int) ([]*communication.PortalParentMessage, error) {
	var rows []*communication.PortalParentMessage
	err := run(func(ctx context.Context) error {
		thread, err := threads.ParentThreadFindByStudentGuardian(ctx, studentID, guardianAccountID)
		if err != nil || thread == nil {
			return err
		}
		rows, err = messages.ParentMessageListByThread(ctx, thread.ID, limit)
		return err
	})
	return rows, err
}

// ParentStoreLastGuardianEventFixture selects the same last event from the
// complete ordered native message snapshot.
func ParentStoreLastGuardianEventFixture(run func(func(context.Context) error) error, threads communication.ParentThreadRecords, messages communication.ParentMessageRecords, studentID, guardianAccountID int64, eventType string) (*communication.PortalParentMessage, error) {
	rows, err := ParentStoreGuardianMessagesFixture(run, threads, messages, studentID, guardianAccountID, 50)
	if err != nil {
		return nil, err
	}
	var pill *communication.PortalParentMessage
	for _, message := range rows {
		if message.EventType == eventType {
			pill = message
		}
	}
	return pill, nil
}
