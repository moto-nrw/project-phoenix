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
		Endpoint:  "https://push.example.org/device",
		P256dh:    "p256dh-key",
		Auth:      "auth-key",
	}
}

func TestPushSubscriptionValidate(t *testing.T) {
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
			mutate:  func(sub *PushSubscription) { sub.Endpoint = "http://push.example.org/device" },
			wantErr: "endpoint must be an https URL",
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
