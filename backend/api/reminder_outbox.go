package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/delivery"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	reminderCompose "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/compose"
	reminderPorts "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/ports"
)

const reminderPushKind = "parent_appointment_reminder"

type reminderPushOutbox struct {
	module     *delivery.Module
	dispatcher *delivery.PushDispatcher
}

func (o reminderPushOutbox) Enqueue(ctx context.Context, key string, snapshot reminderPorts.ReminderPushSnapshot) (int64, error) {
	related := delivery.RelatedEntity{Type: "appointment", ID: snapshot.Appointment.ID}
	// A manual retry reuses the committed request, including its original scan
	// window and terminal result. Generic outbox conflict detection stays strict.
	statuses, err := o.module.Statuses(ctx, snapshot.Appointment.TenantID, delivery.TransportPush, related)
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
	queued, err := o.module.EnqueueDeferredPush(ctx, delivery.DeferredPushIntent{
		TenantID: snapshot.Appointment.TenantID, Template: reminderPushKind, IdempotencyKey: key,
		Related: related, Payload: payload,
	})
	return queued.ID, err
}

func (o reminderPushOutbox) Deliver(ctx context.Context, tenantID, id int64) error {
	err := o.dispatcher.Deliver(ctx, tenantID, id)
	if errors.Is(err, delivery.ErrCancelled) {
		return nil
	}
	return err
}

type reminderPushProvider struct {
	deliver func(context.Context, []byte) error
}

func (p reminderPushProvider) SendEmail(context.Context, delivery.ClaimedIntent) (delivery.ProviderResult, error) {
	return delivery.ProviderResult{}, errors.New("reminder dispatcher only sends push")
}

func (p reminderPushProvider) SendPush(ctx context.Context, intent delivery.ClaimedIntent) (delivery.ProviderResult, error) {
	if intent.Template != reminderPushKind {
		return delivery.ProviderResult{}, errors.New("unsupported deferred push kind")
	}
	var snapshot reminderPorts.ReminderPushSnapshot
	if err := json.Unmarshal(intent.DeferredPayload, &snapshot); err != nil {
		return delivery.ProviderResult{}, err
	}
	if snapshot.Appointment.TenantID != intent.TenantID || intent.RelatedEntityID == nil || snapshot.Appointment.ID != *intent.RelatedEntityID {
		return delivery.ProviderResult{}, fmt.Errorf("%w: reminder scope mismatch", delivery.ErrCancelled)
	}
	err := p.deliver(ctx, intent.DeferredPayload)
	if errors.Is(err, reminderPorts.ErrReminderAudienceUnavailable) {
		err = fmt.Errorf("%w: %w", delivery.ErrDeferred, err)
	}
	if errors.Is(err, reminderPorts.ErrReminderSkipped) {
		err = fmt.Errorf("%w: %w", delivery.ErrCancelled, err)
	}
	return delivery.ProviderResult{StatusCode: delivery.ProviderAcceptedStatusCode}, err
}

func reminderDeliveryDependencies(db transactionruntime.Database, runtime *deliveryCompose.Runtime, source CalendarReminderSource) reminderPorts.DeliveryDependencies {
	deps := CalendarReminderDelivery(source)
	provider := reminderPushProvider{deliver: reminderCompose.NewPushDelivery(db.Pool(), deps)}
	deps.Outbox = reminderPushOutbox{module: runtime.Module, dispatcher: deliveryCompose.NewPushDispatcher(runtime, provider)}
	return deps
}

func reminderRecovery(db transactionruntime.Database, source CalendarReminderSource) func(context.Context, delivery.ClaimedIntent) error {
	if source == nil {
		return nil
	}
	provider := reminderPushProvider{deliver: reminderCompose.NewPushDelivery(db.Pool(), CalendarReminderDelivery(source))}
	return func(ctx context.Context, intent delivery.ClaimedIntent) error {
		_, err := provider.SendPush(ctx, intent)
		return err
	}
}
