package domain

import "testing"

func TestGuardianWelcomeOrder(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		state      PrecedingMailState
		found      bool
		send, drop bool
	}{
		{name: "sent preceding mail releases the welcome", state: PrecedingMailSent, found: true, send: true},
		{name: "queued preceding mail holds the welcome back", state: PrecedingMailQueued, found: true},
		{name: "cancelled preceding mail drops the welcome", state: PrecedingMailCancelled, found: true, drop: true},
		{name: "undeliverable preceding mail drops the welcome", state: PrecedingMailUndeliverable, found: true, drop: true},
		{name: "missing preceding mail drops the welcome", state: PrecedingMailSent, found: false, drop: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			send, drop := GuardianWelcomeOrder(tc.state, tc.found)
			if send != tc.send || drop != tc.drop {
				t.Fatalf("GuardianWelcomeOrder(%v, %v) = (%v, %v), want (%v, %v)", tc.state, tc.found, send, drop, tc.send, tc.drop)
			}
		})
	}
}
