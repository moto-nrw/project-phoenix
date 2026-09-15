package api

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"testing"

	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// MailpitURL Tests
// =============================================================================

func TestMailpitURL_MissingEnv(t *testing.T) {
	t.Parallel()

	_, err := mailpitURLFrom(func(string) string { return "" })
	require.EqualError(t, err, "MAILPIT_URL is not set")
}

func TestMailpitURL_FromEnv(t *testing.T) {
	t.Parallel()

	url, err := mailpitURLFrom(func(string) string { return "http://localhost:8025" })
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:8025", url)
}

func TestMailpitURL_TrimsTrailingSlash(t *testing.T) {
	t.Parallel()

	url, err := mailpitURLFrom(func(string) string { return "http://mp.local:8025/" })
	require.NoError(t, err)
	assert.Equal(t, "http://mp.local:8025", url)
}

func TestMailpitURL_TrimsWhitespace(t *testing.T) {
	t.Parallel()

	url, err := mailpitURLFrom(func(string) string { return "   http://mp.local:8025  " })
	require.NoError(t, err)
	assert.Equal(t, "http://mp.local:8025", url)
}

// =============================================================================
// addressedTo Tests
// =============================================================================

func TestAddressedTo_Match(t *testing.T) {
	t.Parallel()

	addrs := []mailpitAddress{{Address: "op@test.de"}, {Address: "other@test.de"}}
	assert.True(t, addressedTo(addrs, "op@test.de"))
}

func TestAddressedTo_CaseInsensitive(t *testing.T) {
	t.Parallel()

	addrs := []mailpitAddress{{Address: "Op@Test.De"}}
	assert.True(t, addressedTo(addrs, "op@test.de"))
}

func TestAddressedTo_NoMatch(t *testing.T) {
	t.Parallel()

	addrs := []mailpitAddress{{Address: "other@test.de"}}
	assert.False(t, addressedTo(addrs, "op@test.de"))
}

func TestAddressedTo_Empty(t *testing.T) {
	t.Parallel()

	assert.False(t, addressedTo(nil, "op@test.de"))
}

// =============================================================================
// fetchCodeFromMessage Tests (helper that pulls the code out of one message)
// =============================================================================

func TestFetchCodeFromMessage_TextBody(t *testing.T) {
	t.Parallel()

	srv := newMailpitServer(t, map[string]mailpitMessageDetail{
		"abc": {ID: "abc", Text: "Ihr Code: 123456 — gültig 10 Minuten."},
	}, nil)
	defer srv.Close()

	code, err := fetchCodeFromMessage(context.Background(), &testpkg.HTTPClient{Timeout: 2 * time.Second}, srv.URL, "abc")
	require.NoError(t, err)
	assert.Equal(t, "123456", code)
}

func TestFetchCodeFromMessage_HTMLFallback(t *testing.T) {
	t.Parallel()

	srv := newMailpitServer(t, map[string]mailpitMessageDetail{
		"abc": {ID: "abc", HTML: `<p style="letter-spacing:0.4em;">654321</p>`},
	}, nil)
	defer srv.Close()

	code, err := fetchCodeFromMessage(context.Background(), &testpkg.HTTPClient{Timeout: 2 * time.Second}, srv.URL, "abc")
	require.NoError(t, err)
	assert.Equal(t, "654321", code)
}

func TestFetchCodeFromMessage_NoCode(t *testing.T) {
	t.Parallel()

	srv := newMailpitServer(t, map[string]mailpitMessageDetail{
		"abc": {ID: "abc", Text: "no digits here"},
	}, nil)
	defer srv.Close()

	code, err := fetchCodeFromMessage(context.Background(), &testpkg.HTTPClient{Timeout: 2 * time.Second}, srv.URL, "abc")
	require.NoError(t, err)
	assert.Empty(t, code)
}

func TestFetchCodeFromMessage_IgnoresShortRuns(t *testing.T) {
	t.Parallel()

	srv := newMailpitServer(t, map[string]mailpitMessageDetail{
		"abc": {ID: "abc", Text: "IP 192.168.1.1 — code 111222"},
	}, nil)
	defer srv.Close()

	code, err := fetchCodeFromMessage(context.Background(), &testpkg.HTTPClient{Timeout: 2 * time.Second}, srv.URL, "abc")
	require.NoError(t, err)
	// Only the standalone 6-digit run is picked up; the IP octets are too short.
	assert.Equal(t, "111222", code)
}

func TestFetchCodeFromMessage_NonOK(t *testing.T) {
	t.Parallel()

	srv := testpkg.NewHTTPTestServer(func(w testpkg.HTTPResponseWriter, _ *testpkg.HTTPRequest) {
		w.WriteHeader(testpkg.HTTPStatusInternalServerError)
	})
	defer srv.Close()

	_, err := fetchCodeFromMessage(context.Background(), &testpkg.HTTPClient{Timeout: 2 * time.Second}, srv.URL, "abc")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mailpit detail")
}

func TestFetchCodeFromMessage_InvalidJSON(t *testing.T) {
	t.Parallel()

	srv := testpkg.NewHTTPTestServer(func(w testpkg.HTTPResponseWriter, _ *testpkg.HTTPRequest) {
		w.WriteHeader(testpkg.HTTPStatusOK)
		_, _ = fmt.Fprint(w, `not json`)
	})
	defer srv.Close()

	_, err := fetchCodeFromMessage(context.Background(), &testpkg.HTTPClient{Timeout: 2 * time.Second}, srv.URL, "abc")
	require.Error(t, err)
}

// =============================================================================
// FetchLatestMFACode Tests (the public entry point)
// =============================================================================

func TestFetchLatestMFACode_HappyPath(t *testing.T) {
	t.Parallel()

	now := time.Now()
	srv := newMailpitServer(t,
		map[string]mailpitMessageDetail{
			"msg-1": {ID: "msg-1", Text: "Code: 424242"},
		},
		[]mailpitMessageSummary{
			{ID: "msg-1", To: []mailpitAddress{{Address: "op@test.de"}}, Created: now},
		},
	)
	defer srv.Close()

	code, err := fetchLatestMFACodeAt(context.Background(), srv.URL, "op@test.de", now.Add(-1*time.Second))
	require.NoError(t, err)
	assert.Equal(t, "424242", code)
}

func TestFetchLatestMFACode_FiltersOldMessages(t *testing.T) {
	t.Parallel()

	now := time.Now()
	stale := now.Add(-1 * time.Hour)
	fresh := now

	srv := newMailpitServer(t,
		map[string]mailpitMessageDetail{
			"old": {ID: "old", Text: "Code: 000000"},
			"new": {ID: "new", Text: "Code: 999999"},
		},
		[]mailpitMessageSummary{
			// Mailpit returns newest-first; both addressed to the same recipient.
			{ID: "new", To: []mailpitAddress{{Address: "op@test.de"}}, Created: fresh},
			{ID: "old", To: []mailpitAddress{{Address: "op@test.de"}}, Created: stale},
		},
	)
	defer srv.Close()

	// notBefore between stale and fresh → only the new message qualifies.
	code, err := fetchLatestMFACodeAt(context.Background(), srv.URL, "op@test.de", now.Add(-1*time.Minute))
	require.NoError(t, err)
	assert.Equal(t, "999999", code)
}

func TestFetchLatestMFACode_FiltersOtherRecipients(t *testing.T) {
	t.Parallel()

	now := time.Now()
	srv := newMailpitServer(t,
		map[string]mailpitMessageDetail{
			"other": {ID: "other", Text: "Code: 111111"},
			"ours":  {ID: "ours", Text: "Code: 222222"},
		},
		[]mailpitMessageSummary{
			{ID: "other", To: []mailpitAddress{{Address: "someone@else.de"}}, Created: now},
			{ID: "ours", To: []mailpitAddress{{Address: "op@test.de"}}, Created: now},
		},
	)
	defer srv.Close()

	code, err := fetchLatestMFACodeAt(context.Background(), srv.URL, "op@test.de", now.Add(-1*time.Second))
	require.NoError(t, err)
	assert.Equal(t, "222222", code)
}

func TestFetchLatestMFACode_ContextCanceled(t *testing.T) {
	t.Parallel()

	srv := newMailpitServer(t, nil, nil) // always empty
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Millisecond)
	defer cancel()
	_, err := fetchLatestMFACodeAt(ctx, srv.URL, "op@test.de", time.Now())
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestFetchLatestMFACode_EmptyRecipient(t *testing.T) {
	t.Parallel()

	_, err := fetchLatestMFACodeAt(context.Background(), "", "", time.Now())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "recipient is required")
}

func TestFetchLatestMFACode_ListServerError_RetriesUntilContext(t *testing.T) {
	t.Parallel()

	var hits int32
	srv := testpkg.NewHTTPTestServer(func(w testpkg.HTTPResponseWriter, _ *testpkg.HTTPRequest) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(testpkg.HTTPStatusInternalServerError)
	})
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
	defer cancel()
	_, err := fetchLatestMFACodeAt(ctx, srv.URL, "op@test.de", time.Now())
	require.Error(t, err)
	// The 500 errors are swallowed by the poll loop; the function keeps
	// retrying until ctx expires. At least two requests should land
	// (initial poll + at least one retry after the 500ms sleep).
	assert.GreaterOrEqual(t, atomic.LoadInt32(&hits), int32(2))
}

func TestFetchLatestMFACode_MessageAppearsOnSecondPoll(t *testing.T) {
	t.Parallel()

	now := time.Now()
	var pollCount int32

	srv := testpkg.NewHTTPTestServer(func(w testpkg.HTTPResponseWriter, r *testpkg.HTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/messages":
			n := atomic.AddInt32(&pollCount, 1)
			if n < 2 {
				// First poll: nothing yet.
				_ = json.NewEncoder(w).Encode(mailpitMessageList{})
				return
			}
			// Second poll onwards: message visible.
			_ = json.NewEncoder(w).Encode(mailpitMessageList{
				Total: 1,
				Messages: []mailpitMessageSummary{
					{ID: "late", To: []mailpitAddress{{Address: "op@test.de"}}, Created: now},
				},
			})
		case "/api/v1/message/late":
			_ = json.NewEncoder(w).Encode(mailpitMessageDetail{ID: "late", Text: "Code: 555555"})
		default:
			testpkg.HTTPNotFound(w, r)
		}
	})
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	code, err := fetchLatestMFACodeAt(ctx, srv.URL, "op@test.de", now.Add(-1*time.Second))
	require.NoError(t, err)
	assert.Equal(t, "555555", code)
	assert.GreaterOrEqual(t, atomic.LoadInt32(&pollCount), int32(2))
}

// =============================================================================
// Test helpers
// =============================================================================

// newMailpitServer returns an httptest.Server that mimics the subset of
// the mailpit REST API we depend on: GET /api/v1/messages (list) and
// GET /api/v1/message/{id} (detail). Pass nil for either argument to
// serve an empty list / unknown details.
func newMailpitServer(t *testing.T, details map[string]mailpitMessageDetail, messages []mailpitMessageSummary) *testpkg.HTTPServer {
	t.Helper()
	return testpkg.NewHTTPTestServer(func(w testpkg.HTTPResponseWriter, r *testpkg.HTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/messages" {
			_ = json.NewEncoder(w).Encode(mailpitMessageList{
				Total:    len(messages),
				Messages: messages,
			})
			return
		}
		const prefix = "/api/v1/message/"
		if len(r.URL.Path) > len(prefix) && r.URL.Path[:len(prefix)] == prefix {
			id := r.URL.Path[len(prefix):]
			d, ok := details[id]
			if !ok {
				testpkg.HTTPNotFound(w, r)
				return
			}
			_ = json.NewEncoder(w).Encode(d)
			return
		}
		testpkg.HTTPNotFound(w, r)
	})
}
