package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// The two mails of the public demo (#3465), bound to the Delivery dispatcher:
// the link back into the demo for the prospect and the lead mail for the team.

// demoContact receives the lead mail and the prospects' answers.
var demoContact = email.NewEmail("moto", "kontakt@moto.nrw")

// demoAccessWiring configures the demo access; nil composes the module
// without it, as every environment but APP_ENV=demo does.
type demoAccessWiring struct {
	dispatcher  *email.Dispatcher
	defaultFrom email.Email
	frontendURL string
	logger      *slog.Logger
	// backoff spaces the send retries; nil uses the production spacing.
	backoff []time.Duration
	// maxActiveSchools caps the demo schools that may exist at once (#3466).
	maxActiveSchools int
	// operatorWithoutSecondFactor is set for APP_ENV=demo only (#3460).
	operatorWithoutSecondFactor bool
}

// IsDemoEnvironment reports whether appEnv names the public demo, the only
// environment that mounts the demo routes (#2736).
func IsDemoEnvironment(appEnv string) bool {
	return email.IsDemoEnvironment(appEnv)
}

// demoAccessWiringFor composes the demo access for APP_ENV=demo only.
func demoAccessWiringFor(appEnv string, dispatcher *email.Dispatcher, defaultFrom email.Email, frontendURL string, maxActiveSchools int, logger *slog.Logger) *demoAccessWiring {
	if !email.IsDemoEnvironment(appEnv) {
		return nil
	}
	return &demoAccessWiring{
		dispatcher: dispatcher, defaultFrom: defaultFrom, frontendURL: frontendURL, maxActiveSchools: maxActiveSchools, logger: logger,
		operatorWithoutSecondFactor: true,
	}
}

type demoAccessMail struct {
	dispatcher *email.Dispatcher
	from       email.Email
	logoURL    string
	backoff    []time.Duration
	logger     *slog.Logger
}

func newDemoAccessMail(wiring *demoAccessWiring) demoAccessMail {
	logger := wiring.logger
	if logger == nil {
		logger = slog.Default()
	}
	backoff := wiring.backoff
	if backoff == nil {
		backoff = passwordResetEmailBackoff
	}
	return demoAccessMail{
		dispatcher: wiring.dispatcher, from: wiring.defaultFrom, backoff: backoff, logger: logger,
		logoURL: motoLogoURL(wiring.frontendURL),
	}
}

func (m demoAccessMail) SendDemoAccessLink(ctx context.Context, access identityaccess.DemoAccessMessage, entryURL string) {
	m.dispatch(ctx, "demo_access", access.AccessID, email.Message{
		From: m.from,
		// Anyone can type a foreign address into the public form, so nothing
		// the form carried reaches this mail: no name, no school.
		To:       email.NewEmail("", access.Email),
		ReplyTo:  demoContact,
		Subject:  "Ihr Link zur moto-Demo",
		Template: email.TemplateDemoAccess,
		Content: map[string]any{
			"EntryURL": entryURL,
			"LogoURL":  m.logoURL,
		},
	})
}

func (m demoAccessMail) SendDemoLead(ctx context.Context, access identityaccess.DemoAccessMessage) {
	m.dispatch(ctx, "demo_lead", access.AccessID, email.Message{
		From:     m.from,
		To:       demoContact,
		ReplyTo:  email.NewEmail(access.PersonName, access.Email),
		Subject:  "Neuer Demo-Zugang: " + access.SchoolName,
		Template: email.TemplateDemoLead,
		Content: map[string]any{
			"PersonName":   access.PersonName,
			"OGSName":      access.SchoolName,
			"Email":        access.Email,
			"Source":       access.Source,
			"ContactOptIn": access.ContactOptIn,
			"AccessID":     access.AccessID,
			"LogoURL":      m.logoURL,
		},
	})
}

func (m demoAccessMail) dispatch(ctx context.Context, kind string, accessID int64, message email.Message) {
	if m.dispatcher == nil {
		m.logger.Warn("email dispatcher unavailable, skipping demo mail",
			slog.String("type", kind),
			slog.Int64("demo_access_id", accessID))
		return
	}
	m.dispatcher.Dispatch(detachedContext(ctx), email.DeliveryRequest{
		Message:       message,
		Metadata:      email.DeliveryMetadata{Type: kind, ReferenceID: accessID, Recipient: message.To.Address},
		BackoffPolicy: m.backoff,
		MaxAttempts:   3,
		Callback: func(_ context.Context, result email.DeliveryResult) {
			if result.Final && result.Status == email.DeliveryStatusFailed {
				m.logger.Error("demo mail permanently failed",
					slog.String("type", kind),
					slog.Int64("demo_access_id", accessID),
					slog.Any("error", result.Err))
			}
		},
	})
}
