package notifications

import (
	"errors"
	"fmt"
	"log/slog"

	deliveryPorts "github.com/moto-nrw/project-phoenix/modules/delivery/internal/ports"

	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// The constructors below take the root's tenant unit of work and every
// collaborator at construction (ADR 0059). A missing one is a construction
// error. Loggers stay optional.

func requireTenantRuntime(service string, runtime tenant.UnitOfWork, dependenciesPresent bool) error {
	if runtime.IsZero() {
		return fmt.Errorf("%s: %w", service, tenant.ErrRuntimeRequired)
	}
	if !dependenciesPresent {
		return errors.New(service + ": all dependencies are required")
	}
	return nil
}

// NewTenantSSEChannel is NewSSEChannel with WithGuardianChildAccess over the
// tenant unit of work.
func NewTenantSSEChannel(runtime tenant.UnitOfWork, broadcaster realtime.Broadcaster, db *bun.DB, guardians GuardianChildAccessRepository, logger *slog.Logger) (Channel, error) {
	if err := requireTenantRuntime("sse channel", runtime, broadcaster != nil && db != nil && guardians != nil); err != nil {
		return nil, err
	}
	channel := NewSSEChannel(broadcaster, WithGuardianChildAccess(db, guardians, logger)).(*sseChannel)
	channel.tenantRuntime = &runtime
	return channel, nil
}

// NewTenantDurableWebPushChannel is NewDurableWebPushChannel over the tenant
// unit of work. An unconfigured VAPID key pair keeps the channel inert, as
// there.
func NewTenantDurableWebPushChannel(runtime tenant.UnitOfWork, db *bun.DB, repo deliveryPorts.PushSubscriptionRepository, vapid VAPIDConfig, outbox PushOutbox, logger *slog.Logger) (Channel, error) {
	if err := requireTenantRuntime("web push channel", runtime, db != nil && repo != nil && outbox != nil); err != nil {
		return nil, err
	}
	channel := NewDurableWebPushChannel(db, repo, vapid, outbox, logger).(*webPushChannel)
	channel.tenantRuntime = &runtime
	return channel, nil
}

// NewTenantAbsenceNotifier is NewAbsenceNotifier over the tenant unit of
// work.
func NewTenantAbsenceNotifier(runtime tenant.UnitOfWork, notifier Service, recipients StaffRecipientResolver, db *bun.DB, logger *slog.Logger, opts ...AbsenceNotifierOption) (AbsenceNotifier, error) {
	if err := requireTenantRuntime("absence notifier", runtime, notifier != nil && recipients != nil && db != nil); err != nil {
		return nil, err
	}
	service := NewAbsenceNotifier(notifier, recipients, db, logger, opts...).(*absenceNotifier)
	service.tenantRuntime = &runtime
	return service, nil
}

// NewTenantPreferenceService is NewPreferenceService over the tenant unit of
// work, with the guardian-portal and e-mail collaborators required.
func NewTenantPreferenceService(runtime tenant.UnitOfWork, consent ConsentStore, settings PreferenceSettings, db *bun.DB, accountTenants GuardianSchools, permissions PermissionMatch) (PreferenceService, error) {
	if err := requireTenantRuntime("notification preferences", runtime, consent != nil && settings != nil && db != nil && accountTenants != nil && permissions != nil); err != nil {
		return nil, err
	}
	service := NewPreferenceService(consent, settings, db, accountTenants, permissions).(*preferenceService)
	service.tenantRuntime = &runtime
	return service, nil
}

// NewTenantPushSubscriptionService is NewPushSubscriptionService over the
// tenant unit of work.
func NewTenantPushSubscriptionService(runtime tenant.UnitOfWork, db *bun.DB, repo deliveryPorts.PushSubscriptionRepository, accountTenants GuardianSchools, vapid VAPIDConfig, logger *slog.Logger) (PushSubscriptionService, error) {
	if err := requireTenantRuntime("push subscriptions", runtime, db != nil && repo != nil && accountTenants != nil); err != nil {
		return nil, err
	}
	service := NewPushSubscriptionService(db, repo, accountTenants, vapid, logger).(*pushSubscriptionService)
	service.tenantRuntime = &runtime
	return service, nil
}
