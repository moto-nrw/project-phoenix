package application

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const lettermintTestToken = "lm_team_test" //nolint:gosec // fake value for the httptest server

func lettermintAPIClient(t *testing.T, handler http.HandlerFunc) *lettermintClient {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer "+lettermintTestToken, r.Header.Get("Authorization"))
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	return newLettermintClient(lettermintClientConfig{BaseURL: server.URL + "/v1", TeamToken: lettermintTestToken, WebhookID: "hook-1"})
}

func TestLettermintMessageEventsReadsAllPagesAndMapsTheEvents(t *testing.T) {
	t.Parallel()
	client := lettermintAPIClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/messages/m-1/events", r.URL.Path)
		assert.Equal(t, "timestamp", r.URL.Query().Get("sort"))
		if r.URL.Query().Get("page[cursor]") == "" {
			_, _ = w.Write([]byte(`{"data":[{"message_id":"m-1","event":"queued","timestamp":"2026-11-06T12:00:00Z"},
				{"message_id":"m-1","event":"delivered","timestamp":"2026-11-06T12:00:30Z"}],"next_cursor":"c2"}`))
			return
		}
		assert.Equal(t, "c2", r.URL.Query().Get("page[cursor]"))
		_, _ = w.Write([]byte(`{"data":[{"message_id":"m-1","event":"spam_complaint","timestamp":"2026-11-06T13:00:00Z"}],"next_cursor":null}`))
	})

	events, err := client.MessageEvents(context.Background(), "m-1")

	require.NoError(t, err)
	require.Len(t, events, 2, "queued has no state")
	assert.Equal(t, domain.EmailDeliveryDelivered, events[0].State)
	assert.Equal(t, domain.EmailDeliveryComplained, events[1].State)
	assert.Equal(t, "m-1", events[0].ProviderMessageID)
	assert.Equal(t, "lettermint", events[0].Provider)
	assert.Equal(t, "api:delivered:2026-11-06T12:00:30Z", events[0].ProviderEventID, "the same event read twice is stored once")
}

func TestLettermintMessageEventsAreEmptyBeyondTheProvidersRetention(t *testing.T) {
	t.Parallel()
	client := lettermintAPIClient(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) })

	_, err := client.MessageEvents(context.Background(), "gone")

	assert.ErrorIs(t, err, domain.ErrProviderNotFound)
}

func TestLettermintClientReportsRateLimitAndFailuresWithoutTheToken(t *testing.T) {
	t.Parallel()
	limited := lettermintAPIClient(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTooManyRequests) })
	_, err := limited.MessageEvents(context.Background(), "m")
	assert.ErrorIs(t, err, domain.ErrProviderRateLimited)

	failing := lettermintAPIClient(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadGateway) })
	_, err = failing.MessageEvents(context.Background(), "m")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "502")
	assert.NotContains(t, err.Error(), lettermintTestToken)
}

func TestLettermintMessageIDIsEscapedInThePath(t *testing.T) {
	t.Parallel()
	client := lettermintAPIClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/messages/a%2Fb/events", r.URL.EscapedPath())
		_, _ = w.Write([]byte(`{"data":[]}`))
	})

	_, err := client.MessageEvents(context.Background(), "a/b")

	require.NoError(t, err)
}

func TestLettermintWebhookEnabledReadsTheConfiguredWebhook(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		body    string
		enabled bool
		fails   bool
	}{
		"enabled":   {body: `{"id":"hook-1","enabled":true}`, enabled: true},
		"disabled":  {body: `{"id":"hook-1","enabled":false}`},
		"wrapped":   {body: `{"data":{"id":"hook-1","enabled":true}}`, enabled: true},
		"no answer": {body: `{"id":"hook-1"}`, fails: true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			client := lettermintAPIClient(t, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/v1/webhooks/hook-1", r.URL.Path)
				_, _ = w.Write([]byte(test.body))
			})

			enabled, err := client.WebhookEnabled(context.Background())

			if test.fails {
				assert.Error(t, err, "an answer without the field must not read as disabled or enabled")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.enabled, enabled)
		})
	}
}

// lettermintSuppressionServer answers the list and counts the deletions.
func lettermintSuppressionServer(t *testing.T, list string, deleted *[]string) *lettermintClient {
	t.Helper()
	return lettermintAPIClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			*deleted = append(*deleted, strings.TrimPrefix(r.URL.Path, "/v1/suppressions/"))
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "status": "removed"})
			return
		}
		assert.Equal(t, "/v1/suppressions", r.URL.Path)
		assert.Equal(t, "eltern@example.com", r.URL.Query().Get("filter[value]"))
		_, _ = w.Write([]byte(list))
	})
}

func TestLettermintLiftRemovesOnlyTheExactHardBounce(t *testing.T) {
	t.Parallel()
	var deleted []string
	client := lettermintSuppressionServer(t, `{"data":[
		{"id":"s1","type":"email","value":"Eltern@Example.com","reason":"hard_bounce","scope":"team"},
		{"id":"s2","type":"email","value":"eltern@example.com.evil.org","reason":"hard_bounce","scope":"team"},
		{"id":"s3","type":"domain","value":"eltern@example.com","reason":"hard_bounce","scope":"team"},
		{"id":"s4","type":"email","value":"eltern@example.com","reason":"manual","scope":"project"}
	]}`, &deleted)

	require.NoError(t, client.LiftSuppression(context.Background(), "eltern@example.com"))

	assert.Equal(t, []string{"s1"}, deleted, "partial matches and other reasons stay")
}

func TestLettermintLiftRefusesAComplaintAndRemovesNothing(t *testing.T) {
	t.Parallel()
	var deleted []string
	client := lettermintSuppressionServer(t, `{"data":[
		{"id":"s1","type":"email","value":"eltern@example.com","reason":"hard_bounce","scope":"team"},
		{"id":"s2","type":"email","value":"eltern@example.com","reason":"spam_complaint","scope":"route"}
	]}`, &deleted)

	err := client.LiftSuppression(context.Background(), "eltern@example.com")

	assert.ErrorIs(t, err, domain.ErrSuppressionNeedsReview)
	assert.Empty(t, deleted)
}

func TestLettermintLiftWithoutEntryIsDone(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	client := lettermintAPIClient(t, func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{"data":[]}`))
	})

	require.NoError(t, client.LiftSuppression(context.Background(), "eltern@example.com"))
	assert.Equal(t, int32(1), calls.Load())
}

func TestLettermintLiftChecksLaterPagesBeforeDeleting(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"bounce", "complaint", "error", "endless"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			var deleted []string
			reads := 0
			client := lettermintAPIClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodDelete {
					assert.Equal(t, 2, reads, "read every page before deleting")
					deleted = append(deleted, r.URL.Path)
					w.WriteHeader(http.StatusNoContent)
					return
				}
				reads++
				assert.Equal(t, "eltern@example.com", r.URL.Query().Get("filter[value]"))
				if reads == 1 || scenario == "endless" {
					_, _ = w.Write([]byte(`{"data":[{"id":"s1","type":"email","value":"eltern@example.com","reason":"hard_bounce"}],"next_cursor":"c2"}`))
					return
				}
				assert.Equal(t, "c2", r.URL.Query().Get("page[cursor]"))
				switch scenario {
				case "error":
					w.WriteHeader(http.StatusBadGateway)
				case "complaint":
					_, _ = w.Write([]byte(`{"data":[{"id":"s2","type":"email","value":"eltern@example.com","reason":"spam_complaint"}]}`))
				default:
					_, _ = w.Write([]byte(`{"data":[{"id":"s2","type":"email","value":"eltern@example.com","reason":"hard_bounce"}]}`))
				}
			})
			err := client.LiftSuppression(context.Background(), "eltern@example.com")
			if scenario == "bounce" {
				require.NoError(t, err)
				assert.Equal(t, []string{"/v1/suppressions/s1", "/v1/suppressions/s2"}, deleted)
			} else {
				require.Error(t, err)
				if scenario == "complaint" {
					assert.ErrorIs(t, err, domain.ErrSuppressionNeedsReview)
				}
				assert.Empty(t, deleted)
			}
		})
	}
}
