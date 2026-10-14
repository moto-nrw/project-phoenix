package services

import (
	"context"
	"log/slog"
	"strings"
	"time"

	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	deliveryModule "github.com/moto-nrw/project-phoenix/modules/delivery"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/emailoutbox"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/delivery/compose/email"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// NewNotificationConsentTestStore builds Communication's consent capability
// for tests that need the real rows. It lives at this composition seam because
// a consumer's test package may not reach into another owner's composition.
func NewNotificationConsentTestStore(db *bun.DB) notifications.ConsentStore {
	consent, err := communicationCompose.NewNotificationConsent(communicationCompose.NotificationConsentConfig{DB: db})
	if err != nil {
		panic(err)
	}
	return consent
}

// NewEnrollmentAdminSubscribersForTests binds the intake's opt-in admin mail
// (#3780) to Communication's real consent rows and Identity & Access, and
// returns the consent store the suite records decisions with.
func NewEnrollmentAdminSubscribersForTests(db *bun.DB) (enrollmentCompose.AdminMailSubscribers, notifications.ConsentStore, error) {
	consent, err := communicationCompose.NewNotificationConsent(communicationCompose.NotificationConsentConfig{DB: db})
	if err != nil {
		return nil, nil, err
	}
	identity, err := NewIdentityAccessForTests(db)
	if err != nil {
		return nil, nil, err
	}
	return NewEnrollmentAdminSubscribers(consent, identity), consent, nil
}

type DeliveryTestModule struct {
	Delivery                *deliveryModule.Module
	EmailOutbox             *emailoutbox.Service
	Notifications           notifications.Notifier
	PushSubscriptions       notifications.PushSubscriptionService
	NotificationPreferences notifications.PreferenceService
}

func NewPushSubscriptionTestService(db *bun.DB, unit tenant.UnitOfWork, vapid notifications.VAPIDConfig) (notifications.PushSubscriptionService, error) {
	identity, err := NewIdentityAccessForTests(db)
	if err != nil {
		return nil, err
	}
	service := notifications.NewPushSubscriptionService(db, deliveryCompose.NewPushSubscriptionRepository(db), identity, vapid, slog.Default())
	service.(interface{ SetTenantRuntime(tenant.UnitOfWork) }).SetTenantRuntime(unit)
	return service, nil
}

func NewDeliveryTestModule(db *bun.DB, unit tenant.UnitOfWork) (DeliveryTestModule, error) {
	settings, err := NewSettingsTestModule(db, unit)
	if err != nil {
		return DeliveryTestModule{}, err
	}
	members, err := NewIdentityAccessForTests(db)
	if err != nil {
		return DeliveryTestModule{}, err
	}
	organizations, err := NewOrganizationTenancy(db)
	if err != nil {
		return DeliveryTestModule{}, err
	}
	logger := slog.Default()
	cfg := CurrentFactoryConfig()
	vapid := notifications.VAPIDConfig{PublicKey: strings.TrimSpace(cfg.VAPIDPublicKey), PrivateKey: strings.TrimSpace(cfg.VAPIDPrivateKey), Subscriber: strings.TrimSpace(cfg.VAPIDSubscriber)}
	if err := vapid.Validate(); err != nil {
		return DeliveryTestModule{}, err
	}
	pushRepo := deliveryCompose.NewPushSubscriptionRepository(db)
	identity := emailoutbox.NewTenantMailIdentity(SchoolContactDirectory{Schools: organizations},
		func(ctx context.Context, tenantID int64) (string, error) {
			return settings.Settings.ResolveStringForTenant(ctx, tenantID, configModels.KeyEmailReplyToAddress)
		}, logger)
	guardians := peopleCompose.NewGuardianService(peopleCompose.GuardianServiceDependencies{GuardianProfileRepo: NewGuardianProfileTestRepository(db), DB: db})
	delivery, err := deliveryCompose.New(deliveryCompose.Dependencies{
		DB: db, People: GuardianDisplayResolver{Query: guardians}, Observe: func(deliveryModule.Observation) {},
		Provider: &DeliveryProvider{Registry: emailoutbox.NewTemplateRegistry(nil), Mailer: email.NewMockMailer(), MailIdentity: identity,
			Push:   deliveryCompose.NewWebPushSender(deliveryModule.WebPushConfig{Subscriber: vapid.Subscriber, PublicKey: vapid.PublicKey, PrivateKey: vapid.PrivateKey}, NewExpiredPushSubscriptionCleaner(deliveryCompose.NewTenantTransactions(db), pushRepo)),
			Logger: logger, Tenants: deliveryCompose.NewTenantTransactions(db), PushAuthorized: NewPushAuthorizationChecker(deliveryCompose.NewTenantTransactions(db), pushRepo)},
	})
	if err != nil {
		return DeliveryTestModule{}, err
	}
	parents, err := NewParentRouteTestRepositories(db, MustNewMembershipTestRepositories(db))
	if err != nil {
		return DeliveryTestModule{}, err
	}
	service := notifications.NewServiceWithDeliveryObserver(settings.Settings, logger, func(string, string, string, time.Duration, error) {},
		notifications.NewSSEChannel(deliveryCompose.NewRealtimeHub(logger), notifications.WithGuardianChildAccess(db, parents.StudentGuardian, logger)),
		notifications.NewDurableWebPushChannel(db, pushRepo, vapid, deliveryCompose.NewDurablePushOutbox(delivery.Module), logger))
	service.(interface{ SetTenantRuntime(tenant.UnitOfWork) }).SetTenantRuntime(unit)
	push := notifications.NewPushSubscriptionService(db, pushRepo, members, vapid, logger)
	push.(interface{ SetTenantRuntime(tenant.UnitOfWork) }).SetTenantRuntime(unit)
	preferences := notifications.NewPreferenceService(NewNotificationConsentTestStore(db), settings.Settings, db, members)
	preferences.(interface{ SetTenantRuntime(tenant.UnitOfWork) }).SetTenantRuntime(unit)
	return DeliveryTestModule{Delivery: delivery.Module, EmailOutbox: emailoutbox.NewService(DurableEmailAdapter{Module: delivery.Module}),
		Notifications: service, PushSubscriptions: push, NotificationPreferences: preferences}, nil
}
