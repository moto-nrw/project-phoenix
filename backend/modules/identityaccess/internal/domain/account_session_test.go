package domain

import (
	"testing"
	"time"
)

func TestCapPortalScopes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want []string
	}{
		{PortalScopeTenant, []string{PortalScopeTenant, PortalScopeOrg}},
		{PortalScopeOrg, []string{PortalScopeTenant, PortalScopeOrg}},
		{PortalScopeParent, []string{PortalScopeParent}},
		{PortalScopeSchool, []string{PortalScopeSchool}},
		{PortalScopeUnknown, []string{PortalScopeUnknown}},
		{"", []string{PortalScopeUnknown}},
	}
	for _, tt := range tests {
		got := CapPortalScopes(tt.in)
		if len(got) != len(tt.want) {
			t.Fatalf("CapPortalScopes(%q) = %v, want %v", tt.in, got, tt.want)
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Fatalf("CapPortalScopes(%q) = %v, want %v", tt.in, got, tt.want)
			}
		}
	}
}

func TestAccountSessionValidate(t *testing.T) {
	t.Parallel()
	now := time.Now()
	replacement := "successor"

	tests := []struct {
		name    string
		session AccountSession
		want    string
		scope   string
	}{
		{"defaults an empty scope to unknown", AccountSession{AccountID: 1, Token: "handle"}, "", PortalScopeUnknown},
		{"keeps a known scope", AccountSession{AccountID: 1, Token: "handle", PortalScope: PortalScopeSchool}, "", PortalScopeSchool},
		{"requires an account", AccountSession{Token: "handle"}, "account ID is required", ""},
		{"requires a handle", AccountSession{AccountID: 1}, "token value is required", ""},
		{"rejects an unknown scope", AccountSession{AccountID: 1, Token: "handle", PortalScope: "kiosk"}, "invalid portal scope", ""},
		{"rejects a half hand-off", AccountSession{AccountID: 1, Token: "handle", RotatedAt: &now}, "rotation handoff must include both timestamp and replacement token", ""},
		{"rejects a proof without a hand-off", AccountSession{AccountID: 1, Token: "handle", RecoveryProofHash: []byte{1}}, "recovery proof hash requires a rotation handoff", ""},
		{"accepts a complete hand-off", AccountSession{AccountID: 1, Token: "handle", RotatedAt: &now, ReplacementToken: &replacement, RecoveryProofHash: []byte{1}}, "", PortalScopeUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			session := tt.session
			err := session.Validate()
			if tt.want == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				if session.PortalScope != tt.scope {
					t.Fatalf("PortalScope = %q, want %q", session.PortalScope, tt.scope)
				}
				return
			}
			if err == nil || err.Error() != tt.want {
				t.Fatalf("Validate() = %v, want %q", err, tt.want)
			}
		})
	}
}
