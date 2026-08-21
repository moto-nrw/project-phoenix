package iot

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validPushSubscription() *PushSubscription {
	return &PushSubscription{
		AccountID: 42,
		Portal:    PushPortalStaff,
		Endpoint:  "https://fcm.googleapis.com/fcm/send/device",
		P256dh:    "p256dh-key",
		Auth:      "auth-key",
	}
}

func TestPushSubscriptionValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*PushSubscription)
		wantErr string
	}{
		{name: "valid staff subscription"},
		{
			name:   "valid parent subscription",
			mutate: func(sub *PushSubscription) { sub.Portal = PushPortalParent },
		},
		{
			name:    "missing account",
			mutate:  func(sub *PushSubscription) { sub.AccountID = 0 },
			wantErr: "account_id is required",
		},
		{
			name:    "invalid portal",
			mutate:  func(sub *PushSubscription) { sub.Portal = "operator" },
			wantErr: "portal must be 'staff' or 'parent'",
		},
		{
			name:    "blank endpoint",
			mutate:  func(sub *PushSubscription) { sub.Endpoint = " " },
			wantErr: "endpoint is required",
		},
		{
			name:    "non-https endpoint",
			mutate:  func(sub *PushSubscription) { sub.Endpoint = "http://fcm.googleapis.com/fcm/send/device" },
			wantErr: "endpoint must be an https URL",
		},
		{
			name:    "untrusted endpoint",
			mutate:  func(sub *PushSubscription) { sub.Endpoint = "https://push.example.org/device" },
			wantErr: "endpoint host is not a trusted push service",
		},
		{
			name:    "private endpoint",
			mutate:  func(sub *PushSubscription) { sub.Endpoint = "https://127.0.0.1/device" },
			wantErr: "endpoint host is not a trusted push service",
		},
		{
			name:    "trusted host with user information",
			mutate:  func(sub *PushSubscription) { sub.Endpoint = "https://user@fcm.googleapis.com/device" },
			wantErr: "endpoint must not contain user information or a fragment",
		},
		{
			name:    "trusted host with non-standard port",
			mutate:  func(sub *PushSubscription) { sub.Endpoint = "https://fcm.googleapis.com:8443/device" },
			wantErr: "endpoint must use the default https port",
		},
		{
			name:    "missing p256dh key",
			mutate:  func(sub *PushSubscription) { sub.P256dh = " " },
			wantErr: "subscription keys are required",
		},
		{
			name:    "missing auth key",
			mutate:  func(sub *PushSubscription) { sub.Auth = " " },
			wantErr: "subscription keys are required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := validPushSubscription()
			if tt.mutate != nil {
				tt.mutate(sub)
			}

			err := sub.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			assert.EqualError(t, err, tt.wantErr)
		})
	}
}

func TestValidatePushEndpointTrustedServices(t *testing.T) {
	t.Parallel()

	for _, endpoint := range []string{
		"https://fcm.googleapis.com/fcm/send/device",
		"https://updates.push.services.mozilla.com/wpush/v2/device",
		"https://web.push.apple.com/device",
		"https://wns2-sg2p.notify.windows.com/w/?token=device",
	} {
		t.Run(endpoint, func(t *testing.T) {
			require.NoError(t, ValidatePushEndpoint(endpoint))
		})
	}
}
