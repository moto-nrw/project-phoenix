package owners

import (
	"context"
	"strings"
	"time"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"

	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
)

// ServiceConfig is the process configuration snapshot consumed by one service
// graph. Capturing it at the composition root keeps service construction pure
// and lets independent graphs coexist in the same process.
type ServiceConfig struct {
	EmailSMTPHost              string
	EmailSMTPPort              int
	EmailSMTPUser              string
	EmailSMTPPassword          string
	EmailFromName              string
	EmailFromAddress           string
	FrontendURL                string
	PublicAPIURL               string
	ParentsURL                 string
	SchoolURL                  string
	CatererURL                 string
	TraegerURL                 string
	AppEnv                     string
	InvitationTokenExpiryHours int
	PasswordResetExpiryMinutes int
	PostHogAPIKey              string
	PostHogHost                string
	RateLimitEnabled           bool
	JWTSecret                  string
	JWTExpiry                  time.Duration
	JWTRefreshExpiry           time.Duration
	TenantDomain               string
	OperatorHostname           string
	VAPIDPublicKey             string
	VAPIDPrivateKey            string
	VAPIDSubscriber            string
	StudentDailyCheckoutTime   string
}

type AuditAppendObserver func(eventType string, duration time.Duration, rows int, err error)

type DeliveryObserver func(transport, template, caller string, duration time.Duration, err error)

// DeviceFleetObserver records one Device Fleet operation. The composition
// root supplies it so this package keeps no metrics dependency.
type DeviceFleetObserver func(operation string, duration time.Duration, queries, rows int64, statementDuration time.Duration, code string, err error)

// IdentityAccessObserver records one Identity & Access operation. The
// composition root supplies it so this package keeps no metrics dependency.
type IdentityAccessObserver func(operation string, duration time.Duration, queries, rows int64, statementDuration time.Duration, code string, err error)

type DurableDeliveryObserver func(transport, template, operation string, duration time.Duration, count int, err error)

// ProductEventTracker captures the product analytics events of the services
// this graph composes. The composition root owns the tracker and its lifetime.
type ProductEventTracker interface {
	Capture(distinctID, event string, props map[string]any)
}

func enforceInstanceTimePolicy(appEnv string) bool {
	appEnv = strings.ToLower(strings.TrimSpace(appEnv))
	return appEnv == "production" || appEnv == "staging"
}

// staffAdministrationPermitted is the Security Runtime's staff:manage check
// the staff directory asks before a create adopts an existing staff record.
func staffAdministrationPermitted(actorPermissions []string) bool {
	return securityruntime.HasPermission(securityruntime.PermissionStaffManage, actorPermissions)
}

// MailOutbox is Delivery's mail outbox as the Identity & Access and
// Enrollment mails the root enqueues use it.
type MailOutbox interface {
	EnqueueOutbox(context.Context, deliveryCompose.MailEnqueueRequest) error
}

// MailResultOutbox also reports the stored intent, for a mail that a later
// mail must wait for.
type MailResultOutbox interface {
	MailOutbox
	EnqueueOutboxWithResult(context.Context, deliveryCompose.MailEnqueueRequest) (deliveryCompose.MailEnqueued, error)
}

// OutboxEnqueuer serves MailOutbox and MailResultOutbox over the Delivery
// enqueue facade.
type OutboxEnqueuer struct {
	Outbox  *deliveryCompose.EmailOutbox
	Runtime *deliveryCompose.Runtime
}

func (o OutboxEnqueuer) EnqueueOutbox(ctx context.Context, req deliveryCompose.MailEnqueueRequest) error {
	_, err := o.EnqueueOutboxWithResult(ctx, req)
	return err
}

func (o OutboxEnqueuer) EnqueueOutboxWithResult(ctx context.Context, req deliveryCompose.MailEnqueueRequest) (deliveryCompose.MailEnqueued, error) {
	stored, err := o.Outbox.Enqueue(ctx, req)
	if err != nil {
		return deliveryCompose.MailEnqueued{}, err
	}
	return deliveryCompose.MailEnqueued{ID: stored.ID}, nil
}

// CareRequestStaff resolves the staff member deciding a care request. found
// is false, without an error, for a caller who is no staff member.
type CareRequestStaff interface {
	CurrentStaffID(context.Context) (staffID int64, found bool, err error)
}
