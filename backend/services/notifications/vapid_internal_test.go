package notifications

import (
	"testing"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func generatedVAPIDConfig(t *testing.T) VAPIDConfig {
	t.Helper()
	privateKey, publicKey, err := webpush.GenerateVAPIDKeys()
	require.NoError(t, err)
	return VAPIDConfig{
		PublicKey:  publicKey,
		PrivateKey: privateKey,
		Subscriber: "mailto:ops@example.org",
	}
}

func TestVAPIDConfigValidate(t *testing.T) {
	t.Run("allows a completely empty disabled configuration", func(t *testing.T) {
		config := VAPIDConfig{}
		require.NoError(t, config.Validate())
		assert.False(t, config.Configured())
	})

	t.Run("accepts a valid key pair and contact URI", func(t *testing.T) {
		config := generatedVAPIDConfig(t)
		require.NoError(t, config.Validate())
		assert.True(t, config.Configured())
		assert.Equal(t, "ops@example.org", config.webPushSubscriber())

		config.Subscriber = "https://example.org/web-push-contact"
		require.NoError(t, config.Validate())
		assert.Equal(t, config.Subscriber, config.webPushSubscriber())
	})

	t.Run("rejects partial configuration", func(t *testing.T) {
		config := VAPIDConfig{PublicKey: "configured"}
		err := config.Validate()
		require.Error(t, err)
		assert.ErrorContains(t, err, "VAPID_PRIVATE_KEY")
		assert.ErrorContains(t, err, "VAPID_SUBSCRIBER")
	})

	t.Run("rejects malformed keys", func(t *testing.T) {
		config := generatedVAPIDConfig(t)
		config.PrivateKey = "not-base64!"
		err := config.Validate()
		require.Error(t, err)
		assert.ErrorContains(t, err, "VAPID_PRIVATE_KEY")
	})

	t.Run("rejects a mismatched key pair", func(t *testing.T) {
		config := generatedVAPIDConfig(t)
		other := generatedVAPIDConfig(t)
		config.PublicKey = other.PublicKey
		err := config.Validate()
		require.Error(t, err)
		assert.ErrorContains(t, err, "do not form a key pair")
	})

	t.Run("rejects an invalid subscriber URI", func(t *testing.T) {
		config := generatedVAPIDConfig(t)
		config.Subscriber = "http://example.org/contact"
		err := config.Validate()
		require.Error(t, err)
		assert.ErrorContains(t, err, "must use mailto or https")
	})
}
