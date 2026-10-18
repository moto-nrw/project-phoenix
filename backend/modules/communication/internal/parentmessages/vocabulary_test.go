package messaging_test

import (
	"context"
	"testing"

	messaging "github.com/moto-nrw/project-phoenix/modules/communication/internal/parentmessages"

	"github.com/stretchr/testify/assert"

	configModels "github.com/moto-nrw/project-phoenix/modules/settings"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
)

// TestParentEventVocabularyMatchesOwner pins the consumer-facing pill
// vocabulary and settings key to the values the owner writes and reads. The
// request domains emit through services/parentmessaging, which may not
// import the People Directory model or the settings registry, so this is the
// one place the two are held together.
func TestParentEventVocabularyMatchesOwner(t *testing.T) {
	t.Parallel()

	assert.Equal(t, messaging.ParentMessageEventRequestCreated, parentmessaging.EventRequestCreated)
	assert.Equal(t, messaging.ParentMessageEventRequestStatus, parentmessaging.EventRequestStatus)
	assert.Equal(t, messaging.ParentMessageSenderGuardian, parentmessaging.ActorGuardian)
	assert.Equal(t, messaging.ParentMessageSenderStaff, parentmessaging.ActorStaff)
	assert.Equal(t, messaging.ParentMessageRequestStatusOpen, parentmessaging.RequestStatusOpen)
	assert.Equal(t, messaging.ParentMessageRequestStatusDone, parentmessaging.RequestStatusDone)
	assert.Equal(t, messaging.ParentMessageRequestStatusRejected, parentmessaging.RequestStatusRejected)
	assert.Equal(t, messaging.ParentMessageRequestStatusWithdrawn, parentmessaging.RequestStatusWithdrawn)

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
