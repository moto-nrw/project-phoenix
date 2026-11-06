package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/ports"
)

const (
	// eventProviderMoto names events moto itself observed while sending.
	eventProviderMoto = "moto"
	maxDiagnosticLen  = 1000
)

// errAddressSuppressed is the reason of a mail held back by the suppression
// list. It carries no address, because it ends up in last_error, and it is
// German, because the letter status list shows that text to the school.
//
//nolint:staticcheck // user-visible sentence, not a wrapped message
var errAddressSuppressed = errors.New("Diese Adresse ist gesperrt, weil eine frühere Mail nicht zugestellt werden konnte.")

var eventStates = []domain.EmailDeliveryState{
	domain.EmailDeliveryDelayed, domain.EmailDeliveryDelivered, domain.EmailDeliverySuppressed,
	domain.EmailDeliveryFailed, domain.EmailDeliveryComplained,
}

// RecordEmailDeliveryEvent validates one reported event and stores it. An
// event may arrive twice, late or for a mail moto never sent; none of these is
// an error. The school of the mail comes from the matched row, never from the
// event.
func RecordEmailDeliveryEvent(ctx context.Context, store ports.EmailDeliveryEventStore, event domain.EmailDeliveryEvent) (domain.EmailDeliveryEventResult, error) {
	event.Provider = strings.TrimSpace(event.Provider)
	event.ProviderEventID = strings.TrimSpace(event.ProviderEventID)
	if event.Provider == "" || event.ProviderEventID == "" {
		return domain.EmailDeliveryEventResult{}, errors.New("delivery: event provider and ID are required")
	}
	if !slices.Contains(eventStates, event.State) {
		return domain.EmailDeliveryEventResult{}, fmt.Errorf("delivery: unknown email event state %q", event.State)
	}
	if event.OccurredAt.IsZero() {
		return domain.EmailDeliveryEventResult{}, errors.New("delivery: event time is required")
	}
	event.MessageID = strings.Trim(strings.TrimSpace(event.MessageID), "<>")
	if local, _, ok := strings.Cut(event.MessageID, "@"); ok && event.Token == "" {
		// A mail's Message-ID is stored only after acceptance, and the
		// event may be faster. Its local part is the token.
		event.Token = local
	}
	event.Diagnostic = truncateDiagnostic(domain.RedactDiagnostic(event.Diagnostic))
	return store.RecordEmailDeliveryEvent(ctx, event)
}

func truncateDiagnostic(text string) string {
	if len(text) <= maxDiagnosticLen {
		return text
	}
	text = text[:maxDiagnosticLen]
	for !utf8.ValidString(text) {
		text = text[:len(text)-1]
	}
	return text
}

// addressSuppressed reports whether the suppression list holds this mail back.
// It applies to mail a school sends to parents only: staff mail goes out until
// the list has proven itself in production (#5935), security mail always goes
// out, and system and Träger mail has no school list.
func (w *Worker) addressSuppressed(ctx context.Context, intent domain.Intent) (bool, error) {
	if intent.Transport != domain.TransportEmail || intent.Security || intent.TenantID <= 0 ||
		!deliveryContract.IsParentMail(intent.Template) {
		return false, nil
	}
	store, ok := w.store.(ports.EmailDeliveryEventStore)
	if !ok {
		return false, nil
	}
	var recipient deliveryContract.EmailRecipient
	if err := json.Unmarshal(intent.Recipient, &recipient); err != nil {
		return false, fmt.Errorf("delivery: decode email recipient: %w", err)
	}
	suppressed, err := store.EmailAddressSuppressed(ctx, intent.TenantID, recipient.Address)
	if err != nil {
		return false, fmt.Errorf("delivery: read suppression list: %w", err)
	}
	return suppressed, nil
}

// recordSuppressedSend marks the mail the list held back, so the school sees
// why it did not go out. The mail is already final: a failure here is only
// observed.
func (w *Worker) recordSuppressedSend(ctx context.Context, intent domain.Intent) {
	w.recordOwnEvent(ctx, intent, domain.EmailDeliverySuppressed, "suppressed", errAddressSuppressed.Error())
}

// recordRecipientRejection puts an address the mail server refused for good on
// the school's list, as a bounce would.
func (w *Worker) recordRecipientRejection(ctx context.Context, intent domain.Intent, sendErr error) {
	if errors.Is(sendErr, domain.ErrRecipientRejected) {
		w.recordOwnEvent(ctx, intent, domain.EmailDeliveryFailed, "recipient-rejected", sendErr.Error())
	}
}

func (w *Worker) recordOwnEvent(ctx context.Context, intent domain.Intent, state domain.EmailDeliveryState, eventID, diagnostic string) {
	store, ok := w.store.(ports.EmailDeliveryEventStore)
	if !ok || intent.Transport != domain.TransportEmail || intent.DeliveryToken == "" {
		return
	}
	recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), providerTimeout)
	defer cancel()
	_, err := RecordEmailDeliveryEvent(recordCtx, store, domain.EmailDeliveryEvent{
		Token: intent.DeliveryToken, Provider: eventProviderMoto, ProviderEventID: eventID,
		State: state, OccurredAt: time.Now(), Diagnostic: diagnostic,
	})
	w.observe(domain.Observation{Operation: "email_delivery_event", Transport: string(intent.Transport), Template: intent.Template, Count: boolCount(err == nil), Err: err})
}
