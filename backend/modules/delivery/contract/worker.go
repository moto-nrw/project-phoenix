// Package contract publishes Delivery's native intent and provider values.
package contract

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

type WebPushConfig struct {
	Subscriber string
	PublicKey  string
	PrivateKey string
}

func (c WebPushConfig) Configured() bool {
	return c.Subscriber != "" && c.PublicKey != "" && c.PrivateKey != ""
}

type PushSender interface {
	SendPush(context.Context, ClaimedIntent) (ProviderResult, error)
}

type ProviderResult struct {
	MessageID  string          `json:"message_id,omitempty"`
	StatusCode int             `json:"status_code,omitempty"`
	Details    json.RawMessage `json:"details,omitempty"`
}

const ProviderAcceptedStatusCode = 202

type ClaimedIntent struct {
	ID       int64
	TenantID int64
	// OrganizationID is the Träger of an intent without a school (ADR 0110,
	// ADR 0126); zero for a school or system intent.
	OrganizationID    int64
	Transport         Transport
	Template          string
	RelatedEntityType *string
	RelatedEntityID   *int64
	EmailRecipient    EmailRecipient
	PushRecipient     PushRecipient
	EmailPayload      json.RawMessage
	PushPayload       PushPayload
	DeferredPayload   json.RawMessage
	Attempts          int
	LeaseToken        string
	LeaseExpiresAt    time.Time
}

type WorkerStats struct {
	Claimed      int
	Sent         int
	Cancelled    int
	Retried      int
	DeadLettered int
	LeaseLost    int
	// DeadLetters says why the deliveries counted in DeadLettered failed for
	// good: transport, template, delivery id and last error, or the expired
	// dispatches of a transport. ErrDeadLettered carries them to Sentry.
	DeadLetters []string
}

// ErrDeadLettered fails a run in which deliveries failed for good after all
// their attempts. That is the end of a delivery's retries, so the worker's
// caller reports it like any failed run (#3640).
var ErrDeadLettered = errors.New("delivery worker: deliveries failed for good after all attempts")
