package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
)

const (
	apiBaseURL      = "https://api.lettermint.co/v1"
	apiTimeout      = 10 * time.Second
	apiMaxBody      = 1 << 20
	apiPageSize     = "100"
	apiMaxPages     = 10
	apiReasonBounce = "hard_bounce"
	apiReasonSpam   = "spam_complaint"
	apiTypeEmail    = "email"
)

// lettermintClientConfig configures the Team API client. BaseURL is empty in production;
// tests point it at their own server.
type lettermintClientConfig struct {
	BaseURL   string
	TeamToken string
	WebhookID string
	HTTP      *http.Client
}

// lettermintClient calls Lettermint's Team API.
type lettermintClient struct {
	baseURL   string
	token     string
	webhookID string
	http      *http.Client
}

// newLettermintClient builds the client with a bounded HTTP timeout.
func newLettermintClient(config lettermintClientConfig) *lettermintClient {
	client := &lettermintClient{baseURL: config.BaseURL, token: config.TeamToken, webhookID: config.WebhookID, http: config.HTTP}
	if client.baseURL == "" {
		client.baseURL = apiBaseURL
	}
	if client.http == nil {
		client.http = &http.Client{Timeout: apiTimeout}
	}
	return client
}

// get decodes one answer into target. The token never enters an error.
func (c *lettermintClient) get(ctx context.Context, path string, query url.Values, target any) error {
	if query != nil {
		path += "?" + query.Encode()
	}
	return c.call(ctx, http.MethodGet, path, target)
}

func (c *lettermintClient) call(ctx context.Context, method, path string, target any) error {
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("lettermint api: build request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+c.token)
	request.Header.Set("Accept", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return fmt.Errorf("lettermint api: %s failed: %w", method, lettermintRedactURL(err))
	}
	defer func() { _ = response.Body.Close() }()
	switch {
	case response.StatusCode == http.StatusNotFound:
		return domain.ErrProviderNotFound
	case response.StatusCode == http.StatusTooManyRequests:
		return domain.ErrProviderRateLimited
	case response.StatusCode < 200 || response.StatusCode >= 300:
		return fmt.Errorf("lettermint api: %s answered %d", method, response.StatusCode)
	}
	if target == nil {
		return nil
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, apiMaxBody)).Decode(target); err != nil {
		return fmt.Errorf("lettermint api: decode answer: %w", err)
	}
	return nil
}

// lettermintRedactURL drops the request URL of a transport error: it can hold a
// recipient's address in its query.
func lettermintRedactURL(err error) error {
	if urlErr, ok := err.(*url.Error); ok {
		return urlErr.Err
	}
	return err
}

// MessageEvents reads what Lettermint knows about one mail, within its 28 days
// of retention. These events have no ID and no server answer: the ID is derived
// so the same event read twice is stored once, and a blocklist bounce cannot be
// recognized here. Only the webhook can tell it, so a bounce read here shows its
// status and leaves the school's list alone.
func (c *lettermintClient) MessageEvents(ctx context.Context, providerMessageID string) ([]domain.EmailDeliveryEvent, error) {
	path := "/messages/" + url.PathEscape(providerMessageID) + "/events"
	query := url.Values{"page[size]": {apiPageSize}, "sort": {"timestamp"}}
	events := []domain.EmailDeliveryEvent{}
	for range apiMaxPages {
		var page struct {
			Data []struct {
				Event     string `json:"event"`
				Timestamp string `json:"timestamp"`
			} `json:"data"`
			NextCursor *string `json:"next_cursor"`
		}
		if err := c.get(ctx, path, query, &page); err != nil {
			return nil, err
		}
		for _, item := range page.Data {
			if event, ok := lettermintAPIEvent(item.Event, item.Timestamp, providerMessageID); ok {
				events = append(events, event)
			}
		}
		if page.NextCursor == nil || *page.NextCursor == "" {
			return events, nil
		}
		query.Set("page[cursor]", *page.NextCursor)
	}
	return events, nil
}

func lettermintAPIEvent(name, timestamp, providerMessageID string) (domain.EmailDeliveryEvent, bool) {
	outcome, known := lettermintOutcomes[strings.TrimPrefix(name, "message.")]
	occurredAt, err := time.Parse(time.RFC3339, timestamp)
	if !known || err != nil {
		return domain.EmailDeliveryEvent{}, false
	}
	event := outcome.event("")
	event.KeepsAddress = event.KeepsAddress || event.State == domain.EmailDeliveryFailed
	event.ProviderEventID = "api:" + name + ":" + timestamp
	event.ProviderMessageID = providerMessageID
	event.OccurredAt = occurredAt
	return event, true
}

// WebhookEnabled reports whether Lettermint still calls moto's webhook. It
// disables a webhook after 72 hours of failures. An answer without the field is
// an error: it must not read as either state.
func (c *lettermintClient) WebhookEnabled(ctx context.Context) (bool, error) {
	var answer struct {
		Enabled *bool `json:"enabled"`
		Data    struct {
			Enabled *bool `json:"enabled"`
		} `json:"data"`
	}
	if err := c.get(ctx, "/webhooks/"+url.PathEscape(c.webhookID), nil, &answer); err != nil {
		return false, err
	}
	switch {
	case answer.Enabled != nil:
		return *answer.Enabled, nil
	case answer.Data.Enabled != nil:
		return *answer.Data.Enabled, nil
	}
	return false, fmt.Errorf("lettermint api: webhook answer has no enabled state")
}

// LiftSuppression removes the hard bounce block of one exact address, which
// applies team-wide and never expires. Nothing is removed if any other block of
// the address exists, since it stays blocked either way: a complaint's block
// needs a review by the provider's support, and one set by hand or through an
// unsubscribe is someone's decision.
func (c *lettermintClient) LiftSuppression(ctx context.Context, address string) error {
	bounces, err := c.suppressionBounces(ctx, address)
	if err != nil {
		return err
	}
	for _, id := range bounces {
		err := c.call(ctx, http.MethodDelete, "/suppressions/"+url.PathEscape(id), nil)
		if err != nil && !errors.Is(err, domain.ErrProviderNotFound) {
			return fmt.Errorf("lettermint api: remove suppression: %w", err)
		}
	}
	return nil
}

// SuppressionActive reads the block of one exact address. The list stays with the
// caller's question: the filter matches parts of a value, and other addresses on a
// page are neither kept nor reported.
func (c *lettermintClient) SuppressionActive(ctx context.Context, address string) (bool, error) {
	query := url.Values{"filter[value]": {address}, "page[size]": {apiPageSize}}
	for range apiMaxPages {
		var page lettermintSuppressionPage
		if err := c.get(ctx, "/suppressions", query, &page); err != nil {
			return false, err
		}
		for _, entry := range page.Data {
			if entry.Type == apiTypeEmail && strings.EqualFold(entry.Value, address) {
				return true, nil
			}
		}
		if page.NextCursor == nil || *page.NextCursor == "" {
			return false, nil
		}
		query.Set("page[cursor]", *page.NextCursor)
	}
	return false, fmt.Errorf("lettermint api: suppression pagination limit reached")
}

// lettermintSuppressionPage is one page of the provider's list.
type lettermintSuppressionPage struct {
	Data []struct {
		ID     string `json:"id"`
		Type   string `json:"type"`
		Value  string `json:"value"`
		Reason string `json:"reason"`
	} `json:"data"`
	NextCursor *string `json:"next_cursor"`
}

// Read and validate the complete list before making any destructive request.
// A failed page or an excessive list leaves both provider and local blocks in place.
func (c *lettermintClient) suppressionBounces(ctx context.Context, address string) ([]string, error) {
	query := url.Values{"filter[value]": {address}, "page[size]": {apiPageSize}}
	bounces := []string{}
	var remains error
	for range apiMaxPages {
		var page lettermintSuppressionPage
		if err := c.get(ctx, "/suppressions", query, &page); err != nil {
			return nil, err
		}
		found, pageRemains := page.exactBlocks(address)
		bounces = append(bounces, found...)
		// A complaint outranks a block that merely stays, on whichever page.
		if pageRemains != nil && !errors.Is(remains, domain.ErrSuppressionNeedsReview) {
			remains = pageRemains
		}
		if page.NextCursor == nil || *page.NextCursor == "" {
			if remains != nil {
				return nil, remains
			}
			return bounces, nil
		}
		query.Set("page[cursor]", *page.NextCursor)
	}
	return nil, fmt.Errorf("lettermint api: suppression pagination limit reached")
}

// exactBlocks returns the page's hard bounces of the address and why a block on
// this page must remain, if one does. The filter matches parts of a value, so it
// can name other addresses.
func (p lettermintSuppressionPage) exactBlocks(address string) ([]string, error) {
	bounces := []string{}
	var remains error
	for _, entry := range p.Data {
		if entry.Type != apiTypeEmail || !strings.EqualFold(entry.Value, address) {
			continue
		}
		switch entry.Reason {
		case apiReasonSpam:
			return nil, domain.ErrSuppressionNeedsReview
		case apiReasonBounce:
			bounces = append(bounces, entry.ID)
		default:
			remains = domain.ErrSuppressionStaysAtProvider
		}
	}
	return bounces, remains
}
