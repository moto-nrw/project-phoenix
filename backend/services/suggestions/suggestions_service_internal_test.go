package suggestions

import (
	"context"
	"testing"

	modelSuggestions "github.com/moto-nrw/project-phoenix/models/suggestions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotificationContext_NilUsesBackground(t *testing.T) {
	ctx := notificationContext(nil)

	require.NotNil(t, ctx)
	assert.NoError(t, ctx.Err())
}

func TestNotificationContext_StripsCancellation(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	cancel()

	ctx := notificationContext(parent)

	require.NotNil(t, ctx)
	assert.NoError(t, ctx.Err())
}

func TestTruncateRunes_ReturnsEmptyWhenLimitNonPositive(t *testing.T) {
	assert.Equal(t, "", truncateRunes("hello", 0))
}

func TestTruncateRunes_ReturnsOriginalWhenWithinLimit(t *testing.T) {
	assert.Equal(t, "hello", truncateRunes("hello", 5))
}

func TestNotifyNewPost_NoDispatcherDoesNothing(t *testing.T) {
	svc := &suggestionsService{
		notifyEmails: []string{"ops@example.com"},
		frontendURL:  "https://frontend.test",
	}

	assert.NotPanics(t, func() {
		svc.notifyNewPost(&modelSuggestions.Post{
			Title:       "Test Post",
			Description: "Test Description",
			AuthorName:  "Alice",
		})
	})
}

func TestNotifyNewPost_NoRecipientsDoesNothing(t *testing.T) {
	svc := &suggestionsService{}

	assert.NotPanics(t, func() {
		svc.notifyNewPost(&modelSuggestions.Post{
			Title:       "Test Post",
			Description: "Test Description",
			AuthorName:  "Alice",
		})
	})
}

func TestNotifyNewComment_NoDispatcherDoesNothing(t *testing.T) {
	svc := &suggestionsService{
		notifyEmails: []string{"ops@example.com"},
		frontendURL:  "https://frontend.test",
	}

	assert.NotPanics(t, func() {
		svc.notifyNewComment(
			&modelSuggestions.Post{Title: "Test Post"},
			&modelSuggestions.Comment{Content: "Test comment", AuthorName: "Alice"},
		)
	})
}

func TestNotifyNewComment_NoRecipientsDoesNothing(t *testing.T) {
	svc := &suggestionsService{}

	assert.NotPanics(t, func() {
		svc.notifyNewComment(
			&modelSuggestions.Post{Title: "Test Post"},
			&modelSuggestions.Comment{Content: "Test comment", AuthorName: "Alice"},
		)
	})
}
