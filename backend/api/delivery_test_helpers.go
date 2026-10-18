//go:build testhooks

package api

import (
	"context"
	"log/slog"
	"strings"
	"time"

	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	deliveryModule "github.com/moto-nrw/project-phoenix/modules/delivery"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/delivery/compose/email"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// NewNotificationConsentTestStore builds Communication's consent capability
// for tests that need the real rows. It lives at this composition seam because
// a consumer's test package may not reach into another owner's composition.
func NewNotificationConsentTestStore(db transactionruntime.Database) deliveryCompose.NotificationConsentStore {
	consent, err := communicationCompose.NewNotificationConsent(communicationCompose.NotificationConsentConfig{DB: db.Pool()})
	if err != nil {
		panic(err)
	}
	return consent
}

// NewEnrollmentAdminSubscribersForTests binds the intake's opt-in admin mail
// (#3780) to Communication's real consent rows and Identity & Access, and
// returns the consent store the suite records decisions with.
func NewEnrollmentAdminSubscribersForTests(db transactionruntime.Database) (enrollmentCompose.AdminMailSubscribers, deliveryCompose.NotificationConsentStore, error) {
	consent, err := communicationCompose.NewNotificationConsent(communicationCompose.NotificationConsentConfig{DB: db.Pool()})
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
	EmailOutbox             *deliveryCompose.EmailOutbox
	Notifications           deliveryCompose.Notifier
	PushSubscriptions       deliveryCompose.PushSubscriptionService
	NotificationPreferences deliveryCompose.NotificationPreferenceService
}

func NewPushSubscriptionTestService(db transactionruntime.Database, unit tenant.UnitOfWork, vapid deliveryCompose.VAPIDConfig) (deliveryCompose.PushSubscriptionService, error) {
	identity, err := NewIdentityAccessForTests(db)
	if err != nil {
		return nil, err
	}
	return deliveryCompose.NewTenantPushSubscriptionService(unit, db.Pool(), deliveryCompose.NewPushSubscriptionRepository(db.Pool()), identity, vapid, slog.Default())
}

func NewDeliveryTestModule(db transactionruntime.Database, unit tenant.UnitOfWork) (DeliveryTestModule, error) {
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
	cfg := CurrentServiceConfig()
	vapid := deliveryCompose.VAPIDConfig{PublicKey: strings.TrimSpace(cfg.VAPIDPublicKey), PrivateKey: strings.TrimSpace(cfg.VAPIDPrivateKey), Subscriber: strings.TrimSpace(cfg.VAPIDSubscriber)}
	if err := vapid.Validate(); err != nil {
		return DeliveryTestModule{}, err
	}
	pushRepo := deliveryCompose.NewPushSubscriptionRepository(db.Pool())
	identity := deliveryCompose.NewTenantMailIdentity(SchoolContactDirectory{Schools: organizations},
		func(ctx context.Context, tenantID int64) (string, error) {
			return settings.Settings.ResolveStringForTenant(ctx, tenantID, configModels.KeyEmailReplyToAddress)
		}, logger)
	guardians := peopleCompose.NewGuardianService(peoplePorts.GuardianServiceDependencies{GuardianProfileRepo: NewGuardianProfileTestRepository(db)}, db.Pool())
	delivery, err := deliveryCompose.NewModule(deliveryCompose.ModuleDependencies{
		DB: db.Pool(), People: GuardianDisplayResolver{Query: guardians}, Observe: func(deliveryModule.Observation) {},
	})
	if err != nil {
		return DeliveryTestModule{}, err
	}
	_, err = deliveryCompose.NewWorker(delivery, &deliveryCompose.MailPushProvider{Registry: deliveryCompose.NewEmailTemplateRegistry(nil), Mailer: email.NewMockMailer(), MailIdentity: identity,
		Push:   deliveryCompose.NewWebPushSender(deliveryModule.WebPushConfig{Subscriber: vapid.Subscriber, PublicKey: vapid.PublicKey, PrivateKey: vapid.PrivateKey}, deliveryCompose.NewExpiredPushSubscriptionCleaner(deliveryCompose.NewTenantTransactions(db.Pool()), pushRepo)),
		Logger: logger, Tenants: deliveryCompose.NewTenantTransactions(db.Pool()), PushAuthorized: deliveryCompose.NewPushAuthorizationChecker(deliveryCompose.NewTenantTransactions(db.Pool()), pushRepo)})
	if err != nil {
		return DeliveryTestModule{}, err
	}
	parents, err := NewParentRouteTestRepositories(db, MustNewMembershipTestRepositories(db))
	if err != nil {
		return DeliveryTestModule{}, err
	}
	sse, err := deliveryCompose.NewTenantSSEChannel(unit, deliveryCompose.NewRealtimeHub(logger), db.Pool(), parents.StudentGuardian, logger)
	if err != nil {
		return DeliveryTestModule{}, err
	}
	webPush, err := deliveryCompose.NewTenantDurableWebPushChannel(unit, db.Pool(), pushRepo, vapid, deliveryCompose.NewDurablePushOutbox(delivery.Module), logger)
	if err != nil {
		return DeliveryTestModule{}, err
	}
	service := deliveryCompose.NewNotifier(settings.Settings, logger, func(string, string, string, time.Duration, error) {}, sse, webPush)
	push, err := deliveryCompose.NewTenantPushSubscriptionService(unit, db.Pool(), pushRepo, members, vapid, logger)
	if err != nil {
		return DeliveryTestModule{}, err
	}
	preferences, err := deliveryCompose.NewTenantPreferenceService(unit, NewNotificationConsentTestStore(db), settings.Settings, db.Pool(), members, securityruntime.HasPermission)
	if err != nil {
		return DeliveryTestModule{}, err
	}
	return DeliveryTestModule{Delivery: delivery.Module, EmailOutbox: deliveryCompose.NewEmailOutbox(deliveryCompose.NewDurableEmails(delivery.Module)),
		Notifications: service, PushSubscriptions: push, NotificationPreferences: preferences}, nil
}
