package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

// MailpitURL returns the configured base URL used to reach mailpit's REST API.
func MailpitURL() (string, error) {
	return mailpitURLFrom(os.Getenv)
}

func mailpitURLFrom(getenv func(string) string) (string, error) {
	if v := strings.TrimSpace(getenv("MAILPIT_URL")); v != "" {
		return strings.TrimSuffix(v, "/"), nil
	}
	return "", fmt.Errorf("MAILPIT_URL is not set")
}

type mailpitAddress struct {
	Name    string `json:"Name"`
	Address string `json:"Address"`
}

type mailpitMessageSummary struct {
	ID      string           `json:"ID"`
	To      []mailpitAddress `json:"To"`
	Subject string           `json:"Subject"`
	Created time.Time        `json:"Created"`
}

type mailpitMessageList struct {
	Total    int                     `json:"total"`
	Messages []mailpitMessageSummary `json:"messages"`
}

type mailpitMessageDetail struct {
	ID   string `json:"ID"`
	HTML string `json:"HTML"`
	Text string `json:"Text"`
}

var sixDigitCode = regexp.MustCompile(`\b(\d{6})\b`)

// FetchLatestMFACode polls mailpit until a message addressed to recipient
// (created at or after notBefore) arrives whose body contains a 6-digit
// numeric code, then returns the code. Times out after 30s. Dev-only —
// the seeder uses this to drive the operator MFA enrollment introduced
// by branch feat/1308-2fa-email.
func FetchLatestMFACode(ctx context.Context, recipient string, notBefore time.Time) (string, error) {
	baseURL, err := MailpitURL()
	if err != nil {
		return "", err
	}
	return fetchLatestMFACodeAt(ctx, baseURL, recipient, notBefore)
}

func fetchLatestMFACodeAt(ctx context.Context, baseURL, recipient string, notBefore time.Time) (string, error) {
	recipient = strings.ToLower(strings.TrimSpace(recipient))
	if recipient == "" {
		return "", fmt.Errorf("recipient is required")
	}
	deadline := time.Now().Add(30 * time.Second)
	client := &http.Client{Timeout: 5 * time.Second}
	for {
		if time.Now().After(deadline) {
			return "", fmt.Errorf("timed out waiting for MFA email to %s at %s", recipient, baseURL)
		}
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		code, err := tryFetchCode(ctx, client, baseURL, recipient, notBefore)
		if err == nil && code != "" {
			return code, nil
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func tryFetchCode(ctx context.Context, client *http.Client, baseURL, recipient string, notBefore time.Time) (string, error) {
	listURL := baseURL + "/api/v1/messages?limit=20"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, listURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("mailpit list: status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var list mailpitMessageList
	if err := json.Unmarshal(body, &list); err != nil {
		return "", fmt.Errorf("decode mailpit list: %w", err)
	}
	// Mailpit returns messages newest-first.
	for _, m := range list.Messages {
		if m.Created.Before(notBefore.Add(-2 * time.Second)) {
			continue
		}
		if !addressedTo(m.To, recipient) {
			continue
		}
		code, err := fetchCodeFromMessage(ctx, client, baseURL, m.ID)
		if err != nil {
			continue
		}
		if code != "" {
			return code, nil
		}
	}
	return "", nil
}

func addressedTo(addrs []mailpitAddress, recipient string) bool {
	for _, a := range addrs {
		if strings.EqualFold(a.Address, recipient) {
			return true
		}
	}
	return false
}

func fetchCodeFromMessage(ctx context.Context, client *http.Client, baseURL, id string) (string, error) {
	detailURL := baseURL + "/api/v1/message/" + id
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, detailURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("mailpit detail: status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var detail mailpitMessageDetail
	if err := json.Unmarshal(body, &detail); err != nil {
		return "", err
	}
	if m := sixDigitCode.FindStringSubmatch(detail.Text); len(m) == 2 {
		return m[1], nil
	}
	if m := sixDigitCode.FindStringSubmatch(detail.HTML); len(m) == 2 {
		return m[1], nil
	}
	return "", nil
}
