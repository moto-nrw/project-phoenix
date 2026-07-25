package platform

import (
	"errors"
	"strings"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// EmailDispatchAttempt preserves the correlation identifiers minted for one
// transport submission. Outbox summary columns point at the current attempt;
// this history keeps delayed provider callbacks resolvable without allowing
// them to overwrite a later attempt's delivery state.
type EmailDispatchAttempt struct {
	base.Model `bun:"schema:platform,table:email_dispatch_attempts"`
	base.TenantModel
	OutboxID          int64   `bun:"outbox_id,notnull" json:"-"`
	AttemptNumber     int     `bun:"attempt_number,notnull" json:"-"`
	CorrelationToken  string  `bun:"correlation_token,notnull" json:"-"`
	MessageID         string  `bun:"message_id,notnull" json:"-"`
	ProviderMessageID *string `bun:"provider_message_id" json:"-"`
}

func (a *EmailDispatchAttempt) Validate() error {
	if a.OutboxID <= 0 {
		return errors.New("email dispatch attempt outbox_id is required")
	}
	if a.AttemptNumber <= 0 {
		return errors.New("email dispatch attempt number is required")
	}
	a.CorrelationToken = strings.TrimSpace(a.CorrelationToken)
	if a.CorrelationToken == "" {
		return errors.New("email dispatch attempt correlation token is required")
	}
	a.MessageID = strings.Trim(strings.TrimSpace(a.MessageID), "<>")
	if a.MessageID == "" {
		return errors.New("email dispatch attempt message_id is required")
	}
	return nil
}
