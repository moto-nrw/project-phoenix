package owners

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"

	"github.com/moto-nrw/project-phoenix/tenant"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	delivery "github.com/moto-nrw/project-phoenix/modules/delivery/runtime"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	reminderCompose "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/compose"
	reminderPorts "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/ports"
)

const reminderPushKind = "parent_appointment_reminder"

type reminderPushOutbox struct {
	module     delivery.Capability
	dispatcher *delivery.PushDispatcher
}

func (o reminderPushOutbox) Enqueue(ctx context.Context, key string, snapshot reminderPorts.ReminderPushSnapshot) (int64, error) {
	related := deliveryContract.RelatedEntity{Type: "appointment", ID: snapshot.Appointment.ID}
	// A manual retry reuses the committed request, including its original scan
	// window and terminal result. Generic outbox conflict detection stays strict.
	statuses, err := o.module.Statuses(ctx, snapshot.Appointment.TenantID, deliveryContract.TransportPush, related)
	if err != nil {
		return 0, err
	}
	for _, status := range statuses {
		if status.IdempotencyKey != nil && *status.IdempotencyKey == key {
			return status.ID, nil
		}
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return 0, err
	}
	queued, err := o.module.EnqueueDeferredPush(ctx, deliveryContract.DeferredPushIntent{
		TenantID: snapshot.Appointment.TenantID, Template: reminderPushKind, IdempotencyKey: key,
		Related: related, Payload: payload,
	})
	return queued.ID, err
}

func (o reminderPushOutbox) Deliver(ctx context.Context, tenantID, id int64) error {
	err := o.dispatcher.Deliver(ctx, tenantID, id)
	if errors.Is(err, deliveryContract.ErrCancelled) {
		return nil
	}
	return err
}

type ReminderPushProvider struct {
	Deliver func(context.Context, []byte) error
}

func (p ReminderPushProvider) SendEmail(context.Context, deliveryContract.ClaimedIntent) (deliveryContract.ProviderResult, error) {
	return deliveryContract.ProviderResult{}, errors.New("reminder dispatcher only sends push")
}

func (p ReminderPushProvider) SendPush(ctx context.Context, intent deliveryContract.ClaimedIntent) (deliveryContract.ProviderResult, error) {
	if intent.Template != reminderPushKind {
		return deliveryContract.ProviderResult{}, errors.New("unsupported deferred push kind")
	}
	var snapshot reminderPorts.ReminderPushSnapshot
	if err := json.Unmarshal(intent.DeferredPayload, &snapshot); err != nil {
		return deliveryContract.ProviderResult{}, err
	}
	if snapshot.Appointment.TenantID != intent.TenantID || intent.RelatedEntityID == nil || snapshot.Appointment.ID != *intent.RelatedEntityID {
		return deliveryContract.ProviderResult{}, fmt.Errorf("%w: reminder scope mismatch", deliveryContract.ErrCancelled)
	}
	err := p.Deliver(ctx, intent.DeferredPayload)
	if errors.Is(err, reminderPorts.ErrReminderAudienceUnavailable) {
		err = fmt.Errorf("%w: %w", deliveryContract.ErrDeferred, err)
	}
	if errors.Is(err, reminderPorts.ErrReminderSkipped) {
		err = fmt.Errorf("%w: %w", deliveryContract.ErrCancelled, err)
	}
	return deliveryContract.ProviderResult{StatusCode: deliveryContract.ProviderAcceptedStatusCode}, err
}

func ReminderDeliveryDependencies(db transactionruntime.Database, runtime *deliveryCompose.Runtime, source CalendarReminderSource) reminderPorts.DeliveryDependencies {
	deps := CalendarReminderDelivery(source)
	provider := ReminderPushProvider{Deliver: reminderCompose.NewPushDelivery(ReminderCommandRuntime(db), deps)}
	deps.Outbox = reminderPushOutbox{module: runtime.Module, dispatcher: deliveryCompose.NewPushDispatcher(runtime, provider)}
	return deps
}

func reminderRecovery(db transactionruntime.Database, source CalendarReminderSource) func(context.Context, deliveryContract.ClaimedIntent) error {
	if source == nil {
		return nil
	}
	provider := ReminderPushProvider{Deliver: reminderCompose.NewPushDelivery(ReminderCommandRuntime(db), CalendarReminderDelivery(source))}
	return func(ctx context.Context, intent deliveryContract.ClaimedIntent) error {
		_, err := provider.SendPush(ctx, intent)
		return err
	}
}

func ReminderCommandRuntime(db transactionruntime.Database) reminderPorts.CommandRuntime {
	transactions := deliveryCompose.NewTenantTransactions(db.Pool())
	return reminderPorts.CommandRuntime{
		TenantID: tenant.FromContext,
		Detached: func(ctx context.Context) context.Context {
			return tenant.ContextWithoutAfterCommitHooks(tenant.ContextWithoutTransaction(ctx))
		},
		WithinTenant: transactions.WithinTenant,
	}
}
