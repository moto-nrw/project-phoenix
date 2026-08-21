package suggestions

import (
	"context"
	"testing"

	modelSuggestions "github.com/moto-nrw/project-phoenix/models/suggestions"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotificationContext_NilUsesBackground(t *testing.T) {
	t.Parallel()

	var nilCtx context.Context

	ctx := notificationContext(nilCtx)

	require.NotNil(t, ctx)
	assert.NoError(t, ctx.Err())
}

func TestNotificationContext_StripsCancellation(t *testing.T) {
	t.Parallel()

	parent, cancel := context.WithCancel(context.Background())
	cancel()

	ctx := notificationContext(parent)

	require.NotNil(t, ctx)
	assert.NoError(t, ctx.Err())
}

func TestTruncateRunes_ReturnsEmptyWhenLimitNonPositive(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "", truncateRunes("hello", 0))
}

func TestTruncateRunes_ReturnsOriginalWhenWithinLimit(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "hello", truncateRunes("hello", 5))
}

func TestNotifyNewPost_NoDispatcherDoesNothing(t *testing.T) {
	t.Parallel()

	svc := &suggestionsService{ServiceConfig: ServiceConfig{FrontendURL: "https://frontend.test"}, notifyEmails: []string{"ops@example.com"}}

	assert.NotPanics(t, func() {
		svc.notifyNewPost(&modelSuggestions.Post{
			Title:       "Test Post",
			Description: "Test Description",
			AuthorName:  "Alice",
		})
	})
}

func TestNotifyNewPost_NoRecipientsDoesNothing(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	svc := &suggestionsService{ServiceConfig: ServiceConfig{FrontendURL: "https://frontend.test"}, notifyEmails: []string{"ops@example.com"}}

	assert.NotPanics(t, func() {
		svc.notifyNewComment(
			&modelSuggestions.Post{Title: "Test Post"},
			&modelSuggestions.Comment{Content: "Test comment", AuthorName: "Alice"},
		)
	})
}

func TestNotifyNewComment_NoRecipientsDoesNothing(t *testing.T) {
	t.Parallel()

	svc := &suggestionsService{}

	assert.NotPanics(t, func() {
		svc.notifyNewComment(
			&modelSuggestions.Post{Title: "Test Post"},
			&modelSuggestions.Comment{Content: "Test comment", AuthorName: "Alice"},
		)
	})
}

// The actor mappers are the whole seam between the two boards: every read, write
// and read-marker in this service picks its namespace from them, so a wrong
// default would mix a school's suggestions with the parents' feedback.
func TestActorMappers_DefaultToStaffBoard(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	assert.Equal(t, modelSuggestions.ReaderTypeUser, readerTypeFor(ctx))
	assert.Equal(t, modelSuggestions.PostAuthorStaff, postAuthorTypeFor(ctx))
	assert.Equal(t, modelSuggestions.AuthorTypeUser, commentAuthorTypeFor(ctx))
}

func TestActorMappers_ParentContextUsesParentBoard(t *testing.T) {
	t.Parallel()

	ctx := tenant.WithActor(context.Background(), tenant.ActorParent)

	assert.Equal(t, modelSuggestions.ReaderTypeParent, readerTypeFor(ctx))
	assert.Equal(t, modelSuggestions.PostAuthorParent, postAuthorTypeFor(ctx))
	assert.Equal(t, modelSuggestions.AuthorTypeParent, commentAuthorTypeFor(ctx))
}

// The product team reads both boards in one inbox, so the notification subject
// has to say which one an entry came from.
func TestNewPostSubjectPrefix(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "Neues Eltern-Feedback",
		newPostSubjectPrefix(&modelSuggestions.Post{AuthorType: modelSuggestions.PostAuthorParent}))
	assert.Equal(t, "Neuer Vorschlag",
		newPostSubjectPrefix(&modelSuggestions.Post{AuthorType: modelSuggestions.PostAuthorStaff}))
	assert.Equal(t, "Neuer Vorschlag", newPostSubjectPrefix(&modelSuggestions.Post{}))
}
