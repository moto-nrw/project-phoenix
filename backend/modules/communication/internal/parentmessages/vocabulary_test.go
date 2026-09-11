package messaging_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	configModels "github.com/moto-nrw/project-phoenix/models/config"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
)

// TestParentEventVocabularyMatchesOwner pins the consumer-facing pill
// vocabulary and settings key to the values the owner writes and reads. The
// request domains emit through services/parentmessaging, which may not
// import the People Directory model or the settings registry, so this is the
// one place the two are held together.
func TestParentEventVocabularyMatchesOwner(t *testing.T) {
	t.Parallel()

	assert.Equal(t, usersModels.ParentMessageEventRequestCreated, parentmessaging.EventRequestCreated)
	assert.Equal(t, usersModels.ParentMessageEventRequestStatus, parentmessaging.EventRequestStatus)
	assert.Equal(t, usersModels.ParentMessageSenderGuardian, parentmessaging.ActorGuardian)
	assert.Equal(t, usersModels.ParentMessageSenderStaff, parentmessaging.ActorStaff)
	assert.Equal(t, usersModels.ParentMessageRequestStatusOpen, parentmessaging.RequestStatusOpen)
	assert.Equal(t, usersModels.ParentMessageRequestStatusDone, parentmessaging.RequestStatusDone)
	assert.Equal(t, usersModels.ParentMessageRequestStatusRejected, parentmessaging.RequestStatusRejected)
	assert.Equal(t, usersModels.ParentMessageRequestStatusWithdrawn, parentmessaging.RequestStatusWithdrawn)

	// The gate resolves the registry's key: a renamed setting must fail here,
	// not silently fail open in production.
	resolver := keyRecorder{}
	parentmessaging.MessagingEnabled(t.Context(), &resolver, nil)
	assert.Equal(t, configModels.KeyParentNotesEnabled, resolver.key)
}

type keyRecorder struct{ key string }

func (r *keyRecorder) ResolveBool(_ context.Context, key string) (bool, error) {
	r.key = key
	return true, nil
}
