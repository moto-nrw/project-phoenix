package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

const mfaEmailSubject = "Ihr moto-Anmeldecode"

// mailLink matches a link in a mail body, up to the quote or tag around it.
var mailLink = regexp.MustCompile(`https?://[^\s"'<>]+`)

// mailExtractor finds what a poll waits for in one message body; "" means
// the body does not hold it.
type mailExtractor func(body string) string

// mailMatcher narrows a poll to the kind of message that carries its value.
// A recipient can receive invitation and MFA messages in either delivery
// order, so a six-digit number alone does not identify an MFA challenge.
type mailMatcher func(mailpitMessageSummary) bool

func isMFAMail(message mailpitMessageSummary) bool {
	return strings.EqualFold(strings.TrimSpace(message.Subject), mfaEmailSubject)
}

func anyMail(mailpitMessageSummary) bool { return true }

func extractMFACode(body string) string {
	if m := sixDigitCode.FindStringSubmatch(body); len(m) == 2 {
		return m[1]
	}
	return ""
}

func linkExtractor(marker string) mailExtractor {
	return func(body string) string {
		for _, link := range mailLink.FindAllString(body, -1) {
			if strings.Contains(link, marker) {
				return link
			}
		}
		return ""
	}
}

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

// FetchLatestMailLink polls mailpit like FetchLatestMFACode for a message
// whose body links to a URL containing marker, and returns that URL.
// Dev-only: the seeder accepts the caterer's mailed invitation with it, as
// the caterer would (ADR 0077).
func FetchLatestMailLink(ctx context.Context, recipient, marker string, notBefore time.Time) (string, error) {
	baseURL, err := MailpitURL()
	if err != nil {
		return "", err
	}
	return pollMailpit(ctx, baseURL, recipient, notBefore, "mail with a link", linkExtractor(marker))
}

func fetchLatestMFACodeAt(ctx context.Context, baseURL, recipient string, notBefore time.Time) (string, error) {
	return pollMailpitAt(ctx, baseURL, "/api/v1/messages?limit=20", recipient, notBefore, 30*time.Second, "MFA email", isMFAMail, extractMFACode)
}

// pollMailpit searches the recipient's messages for up to a minute: a seed
// queues hundreds of mails, so the one asked for need not be among the
// newest of the inbox, and the outbox sends it in its turn.
func pollMailpit(ctx context.Context, baseURL, recipient string, notBefore time.Time, what string, extract mailExtractor) (string, error) {
	search := "/api/v1/search?limit=20&query=" + url.QueryEscape(`to:"`+strings.ToLower(strings.TrimSpace(recipient))+`"`)
	return pollMailpitAt(ctx, baseURL, search, recipient, notBefore, time.Minute, what, anyMail, extract)
}

func pollMailpitAt(ctx context.Context, baseURL, listPath, recipient string, notBefore time.Time, timeout time.Duration, what string, match mailMatcher, extract mailExtractor) (string, error) {
	recipient = strings.ToLower(strings.TrimSpace(recipient))
	if recipient == "" {
		return "", fmt.Errorf("recipient is required")
	}
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 5 * time.Second}
	for {
		if time.Now().After(deadline) {
			return "", fmt.Errorf("timed out waiting for %s to %s at %s", what, recipient, baseURL)
		}
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		found, err := tryFetchMail(ctx, client, baseURL, listPath, recipient, notBefore, match, extract)
		if err == nil && found != "" {
			return found, nil
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func tryFetchMail(ctx context.Context, client *http.Client, baseURL, listPath, recipient string, notBefore time.Time, match mailMatcher, extract mailExtractor) (string, error) {
	listURL := baseURL + listPath
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
		if !matchesMail(m, recipient, notBefore, match) {
			continue
		}
		found, err := fetchFromMessage(ctx, client, baseURL, m.ID, extract)
		if err != nil {
			continue
		}
		if found != "" {
			return found, nil
		}
	}
	return "", nil
}

func matchesMail(message mailpitMessageSummary, recipient string, notBefore time.Time, match mailMatcher) bool {
	return !message.Created.Before(notBefore.Add(-2*time.Second)) &&
		addressedTo(message.To, recipient) &&
		match(message)
}

func addressedTo(addrs []mailpitAddress, recipient string) bool {
	for _, a := range addrs {
		if strings.EqualFold(a.Address, recipient) {
			return true
		}
	}
	return false
}

// fetchFromMessage extracts from the text body of one message, else from
// its HTML body.
func fetchFromMessage(ctx context.Context, client *http.Client, baseURL, id string, extract mailExtractor) (string, error) {
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
	if found := extract(detail.Text); found != "" {
		return found, nil
	}
	return extract(detail.HTML), nil
}
