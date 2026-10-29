package owners

import (
	"context"
	"log/slog"
	"strings"
	"time"

	email "github.com/moto-nrw/project-phoenix/modules/delivery/mail"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
)

// PasswordResetWiring is the configuration the reset flows are composed
// with. dispatcher may be nil: the link is then not mailed, as before.
type PasswordResetWiring struct {
	Dispatcher       email.Dispatcher
	Durable          DurableMailOutbox
	DefaultFrom      email.Email
	StaffURL         string
	ParentsURL       string
	SchoolURL        string
	CatererURL       string
	TraegerURL       string
	Expiry           time.Duration
	RateLimitEnabled bool
	// Backoff spaces the send retries; nil uses the production spacing.
	Backoff []time.Duration
}

func PasswordResetDependencies(wiring *PasswordResetWiring, logger *slog.Logger) *identityaccessCompose.PasswordResetDependencies {
	if wiring == nil {
		return nil
	}
	if logger == nil {
		logger = slog.Default()
	}
	backoff := wiring.Backoff
	hosts := map[identityaccess.PasswordResetScope]string{
		identityaccess.PasswordResetScopeStaff:   wiring.StaffURL,
		identityaccess.PasswordResetScopeParent:  wiring.ParentsURL,
		identityaccess.PasswordResetScopeSchool:  wiring.SchoolURL,
		identityaccess.PasswordResetScopeCaterer: wiring.CatererURL,
		identityaccess.PasswordResetScopeTraeger: wiring.TraegerURL,
	}
	return &identityaccessCompose.PasswordResetDependencies{
		DeliveryFor: func(records identityaccess.PasswordResetDeliveryRecorder) identityPorts.PasswordResetDelivery {
			return passwordResetDelivery{
				dispatcher: wiring.Dispatcher, durable: wiring.Durable, from: wiring.DefaultFrom, records: records, logger: logger,
				backoff: backoff,
				host:    func(scope identityaccess.PasswordResetScope) string { return hosts[scope] },
			}
		},
		Passwords:        passwordPolicy{},
		Expiry:           wiring.Expiry,
		RateLimitEnabled: wiring.RateLimitEnabled,
		Logger:           logger,
	}
}

// passwordResetRecords records a reset mail's outcome in Identity & Access.
type passwordResetRecords interface {
	RecordPasswordResetDelivery(ctx context.Context, id int64, delivery identityaccess.TokenDelivery) error
}

// passwordResetDelivery mails a reset link and records the outcome through
// the module once the send settles.
type passwordResetDelivery struct {
	dispatcher email.Dispatcher
	durable    DurableMailOutbox
	from       email.Email
	// host is the portal origin of a reset scope.
	host    func(identityaccess.PasswordResetScope) string
	backoff []time.Duration
	records passwordResetRecords
	logger  *slog.Logger
}

func (d passwordResetDelivery) DispatchPasswordReset(ctx context.Context, link identityaccess.PasswordResetLink, recipient string, scope identityaccess.PasswordResetScope, expiry time.Duration) error {
	if d.dispatcher == nil {
		return nil
	}
	message := *identityMessage(d.from, email.NewEmail("", recipient), identityaccess.PasswordResetMail(link, d.host(scope), expiry))
	return enqueueDurableMail(ctx, d.durable, DurableMail{
		Message: message, Type: "password_reset", ReferenceID: link.ID,
		Backoff: d.backoff, BaseRetry: link.Delivery.RetryCount, ExpiresAt: link.Expiry,
	})
}

func (d passwordResetDelivery) recordDelivery(ctx context.Context, meta email.DeliveryMetadata, baseRetry int, result email.DeliveryResult) {
	delivery := identityaccess.TokenDelivery{RetryCount: baseRetry + result.Attempt}
	if result.Status == email.DeliveryStatusSent {
		sentAt := result.SentAt
		delivery.SentAt = &sentAt
	} else if result.Err != nil {
		message := strings.TrimSpace(result.Err.Error())
		delivery.Error = &message
	}
	if err := d.records.RecordPasswordResetDelivery(ctx, meta.ReferenceID, delivery); err != nil {
		d.logger.Error("failed to update password reset delivery status",
			slog.Int64("token_id", meta.ReferenceID),
			slog.Any("error", err),
		)
		return
	}
	if result.Final && result.Status == email.DeliveryStatusFailed {
		d.logger.Error("password reset email permanently failed",
			slog.Int64("token_id", meta.ReferenceID),
			slog.String("recipient", identityaccessCompose.MaskEmail(meta.Recipient)),
			slog.Any("error", result.Err),
		)
	}
}
