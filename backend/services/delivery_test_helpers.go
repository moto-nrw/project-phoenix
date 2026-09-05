package services

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/email"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	deliveryModule "github.com/moto-nrw/project-phoenix/modules/delivery"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/services/platform"
	"github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type DeliveryTestModule struct {
	Delivery                *deliveryModule.Module
	EmailOutbox             *platform.OutboxService
	Notifications           notifications.Notifier
	PushSubscriptions       notifications.PushSubscriptionService
	NotificationPreferences notifications.PreferenceService
}

func NewDeliveryTestModule(db *bun.DB, unit tenant.UnitOfWork) (DeliveryTestModule, error) {
	settings, err := NewSettingsTestModule(db, unit)
	if err != nil {
		return DeliveryTestModule{}, err
	}
	members, err := repositories.NewMembershipTestRepositories(db)
	if err != nil {
		return DeliveryTestModule{}, err
	}
	organizations, err := repositories.NewOrganizationTenancy(db)
	if err != nil {
		return DeliveryTestModule{}, err
	}
	logger := slog.Default()
	cfg := currentFactoryConfig()
	vapid := notifications.VAPIDConfig{PublicKey: strings.TrimSpace(cfg.VAPIDPublicKey), PrivateKey: strings.TrimSpace(cfg.VAPIDPrivateKey), Subscriber: strings.TrimSpace(cfg.VAPIDSubscriber)}
	if err := vapid.Validate(); err != nil {
		return DeliveryTestModule{}, err
	}
	pushRepo := deliveryCompose.NewPushSubscriptionRepository(db)
	identity := platform.NewTenantMailIdentityService(repositories.NewSchoolCapabilityAdapter(organizations, members.AccountTenant),
		func(ctx context.Context, tenantID int64) (string, error) {
			return settings.Settings.ResolveStringForTenant(ctx, tenantID, configModels.KeyEmailReplyToAddress)
		}, logger)
	guardians := users.NewGuardianService(users.GuardianServiceDependencies{GuardianProfileRepo: repositories.NewGuardianProfileTestRepository(db), DB: db})
	delivery, err := deliveryCompose.New(deliveryCompose.Dependencies{
		DB: db, People: guardianDisplayResolver{query: guardians}, Observe: func(deliveryModule.Observation) {},
		Provider: &deliveryProvider{registry: platform.NewTemplateRegistry(), mailer: email.NewMockMailer(), mailIdentity: identity,
			push:   deliveryCompose.NewWebPushSender(deliveryModule.WebPushConfig{Subscriber: vapid.Subscriber, PublicKey: vapid.PublicKey, PrivateKey: vapid.PrivateKey}, newExpiredPushSubscriptionCleaner(db, pushRepo)),
			logger: logger, db: db, pushAuthorized: newPushAuthorizationChecker(db, pushRepo)},
	})
	if err != nil {
		return DeliveryTestModule{}, err
	}
	parents, err := repositories.NewParentRouteTestRepositories(db)
	if err != nil {
		return DeliveryTestModule{}, err
	}
	service := notifications.NewServiceWithDeliveryObserver(settings.Settings, logger, func(string, string, string, time.Duration, error) {},
		notifications.NewSSEChannel(deliveryCompose.NewRealtimeHub(logger), notifications.WithGuardianChildAccess(db, parents.StudentGuardian, logger)),
		notifications.NewDurableWebPushChannel(db, pushRepo, vapid, durablePushAdapter{module: delivery.Module}, logger))
	service.(tenantRuntimeSetter).SetTenantRuntime(unit)
	push := notifications.NewPushSubscriptionService(db, pushRepo, members.AccountTenant, vapid, logger)
	push.(tenantRuntimeSetter).SetTenantRuntime(unit)
	preferences := notifications.NewPreferenceService(repositories.NewNotificationPreferenceTestRepository(db), settings.Settings, db, members.AccountTenant)
	preferences.(tenantRuntimeSetter).SetTenantRuntime(unit)
	return DeliveryTestModule{Delivery: delivery.Module, EmailOutbox: platform.NewOutboxService(durableEmailAdapter{module: delivery.Module}),
		Notifications: service, PushSubscriptions: push, NotificationPreferences: preferences}, nil
}
