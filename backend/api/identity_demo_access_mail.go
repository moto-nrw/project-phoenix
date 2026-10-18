package api

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/delivery/compose/email"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
)

// The two mails of the public demo (#3465), bound to the Delivery dispatcher:
// the link back into the demo for the prospect and the lead mail for the team.

// demoContact receives the lead mail and the prospects' answers.
var demoContact = email.NewEmail(identityaccess.DemoContactName, identityaccess.DemoContactAddress)

// DemoAccessWiring configures the demo access; nil composes the module
// without it, as every environment but APP_ENV=demo does.
type DemoAccessWiring struct {
	Dispatcher  *email.Dispatcher
	DefaultFrom email.Email
	FrontendURL string
	Logger      *slog.Logger
	// Backoff spaces the send retries; nil uses the production spacing.
	Backoff []time.Duration
	// MaxActiveSchools caps the demo schools that may exist at once (#3466).
	MaxActiveSchools int
	// OperatorWithoutSecondFactor is set for APP_ENV=demo only (#3460).
	OperatorWithoutSecondFactor bool
	// NewToken mints the opaque capability token of a demo access link and
	// fingerprint fingerprints a presented one.
	NewToken    func() (raw, fingerprint string, err error)
	Fingerprint func(raw string) string
}

// IsDemoEnvironment reports whether appEnv names the public demo, the only
// environment that mounts the demo routes (#2736).
func IsDemoEnvironment(appEnv string) bool {
	return email.IsDemoEnvironment(appEnv)
}

// demoAccessWiringFor composes the demo access for APP_ENV=demo only.
func demoAccessWiringFor(appEnv string, dispatcher *email.Dispatcher, defaultFrom email.Email, frontendURL string, maxActiveSchools int, tokens TokenAdapter, logger *slog.Logger) *DemoAccessWiring {
	if !email.IsDemoEnvironment(appEnv) {
		return nil
	}
	return &DemoAccessWiring{
		Dispatcher: dispatcher, DefaultFrom: defaultFrom, FrontendURL: frontendURL, MaxActiveSchools: maxActiveSchools, Logger: logger,
		OperatorWithoutSecondFactor: identityaccess.DemoOperatorWithoutSecondFactor,
		NewToken:                    tokens.NewCapabilityToken, Fingerprint: tokens.CapabilityFingerprint,
	}
}

type demoAccessMail struct {
	dispatcher *email.Dispatcher
	from       email.Email
	origin     string
	backoff    []time.Duration
	logger     *slog.Logger
}

func newDemoAccessMail(wiring *DemoAccessWiring) demoAccessMail {
	logger := wiring.Logger
	if logger == nil {
		logger = slog.Default()
	}
	backoff := wiring.Backoff
	return demoAccessMail{
		dispatcher: wiring.Dispatcher, from: wiring.DefaultFrom, backoff: backoff, logger: logger,
		origin: wiring.FrontendURL,
	}
}

func (m demoAccessMail) SendDemoAccessLink(ctx context.Context, access identityaccess.DemoAccessMessage, entryURL string) {
	message := *identityMessage(m.from, email.NewEmail("", access.Email), identityaccess.DemoAccessLinkMail(entryURL, m.origin))
	message.ReplyTo = demoContact
	m.dispatch(ctx, "demo_access", access.AccessID, message)
}

func (m demoAccessMail) SendDemoLead(ctx context.Context, access identityaccess.DemoAccessMessage) {
	message := *identityMessage(m.from, demoContact, identityaccess.DemoLeadMail(access, m.origin))
	message.ReplyTo = email.NewEmail(access.PersonName(), access.Email)
	m.dispatch(ctx, "demo_lead", access.AccessID, message)
}

func (m demoAccessMail) dispatch(ctx context.Context, kind string, accessID int64, message email.Message) {
	if m.dispatcher == nil {
		m.logger.Warn("email dispatcher unavailable, skipping demo mail",
			slog.String("type", kind),
			slog.Int64("demo_access_id", accessID))
		return
	}
	m.dispatcher.Dispatch(identityaccessCompose.DetachedMailContext(ctx), email.WithInteractiveRetry(email.DeliveryRequest{
		Message:  message,
		Metadata: email.DeliveryMetadata{Type: kind, ReferenceID: accessID, Recipient: message.To.Address},
		Callback: func(_ context.Context, result email.DeliveryResult) {
			if result.Final && result.Status == email.DeliveryStatusFailed {
				m.logger.Error("demo mail permanently failed",
					slog.String("type", kind),
					slog.Int64("demo_access_id", accessID),
					slog.Any("error", result.Err))
			}
		},
	}, m.backoff))
}
